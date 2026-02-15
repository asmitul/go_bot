package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

const (
	orderCascadeCallbackPrefix = "order_cascade:"

	orderCascadeLookupTimeout = 8 * time.Second
	orderCascadeSendTimeout   = 5 * time.Second
	orderCascadeStateTTL      = 2 * time.Hour
	orderCascadeGroupTimeout  = 3 * time.Second
)

const (
	orderCascadeActionCompleted = "done"
	orderCascadeActionUnpaid    = "unpaid"
	orderCascadeActionMismatch  = "mismatch"
	orderCascadeActionManual    = "manual"
)

var orderCascadeActions = map[string]struct {
	label       string
	description string
}{
	orderCascadeActionCompleted: {
		label:       "✅ 已补单",
		description: "上游已处理此订单，请尽快在商户端复核到账状态。",
	},
	orderCascadeActionUnpaid: {
		label:       "❌ 未付款",
		description: "上游反馈收款未成功，可提醒下游重新提交或检查支付凭证。",
	},
	orderCascadeActionMismatch: {
		label:       "📷 单图不符",
		description: "上游截图与实际订单不一致，请再次确认凭证。",
	},
	orderCascadeActionManual: {
		label:       "🛠 人工处理",
		description: "需要人工介入，请保持沟通并关注后续处理结果。",
	},
}

type orderCascadeState struct {
	Token              string
	MerchantChatID     int64
	MerchantMessageID  int
	MerchantReplyOn    bool
	UpstreamChatID     int64
	UpstreamMessageID  int
	OrderNo            string
	HasMedia           bool
	MerchantOrderFull  string
	InterfaceID        string
	InterfaceName      string
	ChannelName        string
	ChannelCode        string
	SourceGroupTitle   string
	UpstreamGroupTitle string
	BaseMessageText    string
	CreatedAt          time.Time
	ExpiresAt          time.Time
}

type orderCascadeMessagePayload struct {
	MerchantOrderNoFull string
	OrderNo             string
	StatusText          string
}

func (b *Bot) startOrderCascadeWorkflow(group *models.Group, msg *botModels.Message, orderNos []string) {
	if b.paymentService == nil || b.groupService == nil || group == nil || msg == nil {
		return
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 || len(orderNos) == 0 || msg.Chat.ID == 0 {
		return
	}

	detectionTime := time.Now()
	processedOrders := make(map[string]struct{})

	for _, orderNo := range orderNos {
		trimmed := strings.TrimSpace(orderNo)
		if trimmed == "" {
			continue
		}

		orderUpper := strings.ToUpper(trimmed)
		if _, exists := processedOrders[orderUpper]; exists {
			continue
		}

		binding := b.lookupOrderChannelBinding(merchantID, trimmed)
		if binding == nil {
			continue
		}

		interfaceID := strings.TrimSpace(binding.PZID)
		if interfaceID == "" {
			logger.L().Warnf("Order cascade missing interface id: merchant_id=%d order_no=%s", merchantID, orderUpper)
			continue
		}

		upstreamGroup := b.findUpstreamGroupByInterfaceID(interfaceID)
		if upstreamGroup == nil {
			logger.L().Infof("Order cascade skipped, upstream group not found: interface_id=%s order_no=%s", interfaceID, orderUpper)
			continue
		}
		if upstreamGroup.TelegramID == msg.Chat.ID {
			continue
		}
		if upstreamGroup.BotStatus != models.BotStatusActive {
			logger.L().Warnf("Order cascade skipped, upstream bot inactive: group_id=%d order_no=%s", upstreamGroup.TelegramID, orderUpper)
			continue
		}
		if !upstreamGroup.Settings.CascadeForwardEnabled {
			logger.L().Infof("Order cascade skipped, upstream disabled forwarding: group_id=%d order_no=%s", upstreamGroup.TelegramID, orderUpper)
			continue
		}

		interfaceName, _ := resolveCascadeInterfaceDescriptor(upstreamGroup.Settings.InterfaceBindings, interfaceID, binding.PZName)
		statusText := strings.TrimSpace(binding.StatusText)
		if statusText == "" {
			statusText = strings.TrimSpace(binding.Status)
		}

		orderFull := resolveCascadeMerchantOrderNo(binding, orderUpper)
		if orderFull == "" {
			orderFull = orderUpper
		}

		payload := orderCascadeMessagePayload{
			MerchantOrderNoFull: orderFull,
			OrderNo:             orderUpper,
			StatusText:          statusText,
		}

		token := generateOrderCascadeToken()
		markup := buildOrderCascadeKeyboard(token)

		caption := buildOrderCascadeMessage(payload)

		stateHasMedia := false
		var sent *botModels.Message
		var err error
		sendCtx, cancel := context.WithTimeout(context.Background(), orderCascadeSendTimeout)
		switch {
		case len(msg.Photo) > 0:
			sent, err = b.bot.SendPhoto(sendCtx, &bot.SendPhotoParams{
				ChatID:      upstreamGroup.TelegramID,
				Photo:       &botModels.InputFileString{Data: msg.Photo[len(msg.Photo)-1].FileID},
				Caption:     caption,
				ParseMode:   botModels.ParseModeHTML,
				ReplyMarkup: markup,
			})
			stateHasMedia = true
		case msg.Video != nil:
			sent, err = b.bot.SendVideo(sendCtx, &bot.SendVideoParams{
				ChatID:      upstreamGroup.TelegramID,
				Video:       &botModels.InputFileString{Data: msg.Video.FileID},
				Caption:     caption,
				ParseMode:   botModels.ParseModeHTML,
				ReplyMarkup: markup,
			})
			stateHasMedia = true
		default:
			sent, err = b.sendMessageWithMarkupAndMessage(sendCtx, upstreamGroup.TelegramID, caption, markup)
		}
		cancel()
		if err != nil || sent == nil {
			logger.L().Errorf("Failed to send order cascade message: upstream_chat=%d order_no=%s err=%v",
				upstreamGroup.TelegramID, orderUpper, err)
			continue
		}

		state := &orderCascadeState{
			Token:              token,
			MerchantChatID:     msg.Chat.ID,
			MerchantMessageID:  msg.ID,
			MerchantReplyOn:    models.IsCascadeReplyEnabled(group.Settings),
			UpstreamChatID:     upstreamGroup.TelegramID,
			UpstreamMessageID:  sent.ID,
			OrderNo:            orderUpper,
			MerchantOrderFull:  orderFull,
			InterfaceID:        interfaceID,
			InterfaceName:      interfaceName,
			ChannelName:        binding.ChannelName,
			ChannelCode:        binding.ChannelCode,
			SourceGroupTitle:   group.Title,
			UpstreamGroupTitle: upstreamGroup.Title,
			BaseMessageText:    caption,
			HasMedia:           stateHasMedia,
			CreatedAt:          detectionTime,
			ExpiresAt:          detectionTime.Add(orderCascadeStateTTL),
		}

		b.saveOrderCascadeState(state)
		logger.L().Infof("Order cascade forwarded: merchant_chat=%d upstream_chat=%d order_no=%s interface_id=%s",
			msg.Chat.ID, upstreamGroup.TelegramID, orderUpper, interfaceID)
		processedOrders[orderUpper] = struct{}{}
		fullUpper := strings.ToUpper(orderFull)
		if fullUpper != orderUpper {
			processedOrders[fullUpper] = struct{}{}
		}
	}
}

func (b *Bot) lookupOrderChannelBinding(merchantID int64, orderNo string) *paymentservice.OrderChannelBinding {
	ctx, cancel := context.WithTimeout(context.Background(), orderCascadeLookupTimeout)
	defer cancel()

	binding, err := b.paymentService.FindOrderChannelBinding(ctx, merchantID, orderNo, paymentservice.OrderNumberTypeAuto)
	if err != nil {
		logger.L().Warnf("Order cascade lookup failed: merchant_id=%d order_no=%s err=%v", merchantID, orderNo, err)
		return nil
	}
	return binding
}

func (b *Bot) findUpstreamGroupByInterfaceID(interfaceID string) *models.Group {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	group, err := b.groupService.FindGroupByInterfaceID(ctx, interfaceID)
	if err != nil {
		logger.L().Warnf("Failed to resolve upstream group: interface_id=%s err=%v", interfaceID, err)
		return nil
	}
	return group
}

func buildOrderCascadeMessage(payload orderCascadeMessagePayload) string {
	builder := &strings.Builder{}
	builder.WriteString("📦 <b>订单联动提醒</b>\n")
	orderNo := strings.TrimSpace(payload.MerchantOrderNoFull)
	if orderNo == "" {
		orderNo = strings.TrimSpace(payload.OrderNo)
	}
	if orderNo != "" {
		builder.WriteString(fmt.Sprintf("订单号：<code>%s</code>\n", html.EscapeString(orderNo)))
	}
	if payload.StatusText != "" {
		builder.WriteString(fmt.Sprintf("订单状态：%s\n", html.EscapeString(payload.StatusText)))
	}
	builder.WriteString("🤖 Bot 自动转单")
	return builder.String()
}

func buildOrderCascadeKeyboard(token string) *botModels.InlineKeyboardMarkup {
	prefix := func(action string) string {
		return orderCascadeCallbackPrefix + action + ":" + token
	}
	return &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{
				{Text: orderCascadeActionLabel(orderCascadeActionCompleted), CallbackData: prefix(orderCascadeActionCompleted)},
				{Text: orderCascadeActionLabel(orderCascadeActionUnpaid), CallbackData: prefix(orderCascadeActionUnpaid)},
			},
			{
				{Text: orderCascadeActionLabel(orderCascadeActionMismatch), CallbackData: prefix(orderCascadeActionMismatch)},
				{Text: orderCascadeActionLabel(orderCascadeActionManual), CallbackData: prefix(orderCascadeActionManual)},
			},
		},
	}
}

func resolveCascadeInterfaceDescriptor(bindings []models.InterfaceBinding, interfaceID, fallbackName string) (name string, rate string) {
	for _, binding := range bindings {
		if strings.EqualFold(binding.ID, interfaceID) {
			resolved := strings.TrimSpace(binding.Name)
			if resolved == "" && strings.TrimSpace(fallbackName) != "" {
				resolved = fallbackName
			}
			if resolved == "" {
				resolved = fmt.Sprintf("接口 %s", interfaceID)
			}
			return resolved, strings.TrimSpace(binding.Rate)
		}
	}

	cleanName := strings.TrimSpace(fallbackName)
	if cleanName == "" {
		cleanName = fmt.Sprintf("接口 %s", interfaceID)
	}
	return cleanName, ""
}

func resolveCascadeMerchantOrderNo(binding *paymentservice.OrderChannelBinding, fallback string) string {
	candidates := []string{}
	if binding != nil {
		candidates = append(candidates,
			strings.TrimSpace(binding.MerchantOrderNoFull),
			strings.TrimSpace(binding.MerchantOrderNo),
		)
	}
	candidates = append(candidates, strings.TrimSpace(fallback))

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		return candidate
	}

	return strings.TrimSpace(fallback)
}

func generateOrderCascadeToken() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func (b *Bot) saveOrderCascadeState(state *orderCascadeState) {
	if state == nil || state.Token == "" {
		return
	}

	b.orderCascadeMu.Lock()
	defer b.orderCascadeMu.Unlock()

	if b.orderCascadeStates == nil {
		b.orderCascadeStates = make(map[string]*orderCascadeState)
	}

	now := time.Now()
	for token, existing := range b.orderCascadeStates {
		if existing == nil || now.After(existing.ExpiresAt) {
			delete(b.orderCascadeStates, token)
		}
	}

	b.orderCascadeStates[state.Token] = state
}

func (b *Bot) getOrderCascadeState(token string) (*orderCascadeState, bool) {
	if strings.TrimSpace(token) == "" {
		return nil, false
	}

	b.orderCascadeMu.RLock()
	state, ok := b.orderCascadeStates[token]
	b.orderCascadeMu.RUnlock()
	if !ok || state == nil {
		return nil, false
	}

	if time.Now().After(state.ExpiresAt) {
		b.orderCascadeMu.Lock()
		delete(b.orderCascadeStates, token)
		b.orderCascadeMu.Unlock()
		return nil, false
	}

	return state, true
}

func (b *Bot) findOrderCascadeStateByUpstreamMessage(upstreamChatID int64, upstreamMessageID int) (*orderCascadeState, bool) {
	if upstreamChatID == 0 || upstreamMessageID == 0 {
		return nil, false
	}

	now := time.Now()

	b.orderCascadeMu.Lock()
	defer b.orderCascadeMu.Unlock()

	var matched *orderCascadeState
	for token, state := range b.orderCascadeStates {
		if state == nil || now.After(state.ExpiresAt) {
			delete(b.orderCascadeStates, token)
			continue
		}

		if state.UpstreamChatID == upstreamChatID && state.UpstreamMessageID == upstreamMessageID {
			matched = state
		}
	}

	return matched, matched != nil
}

func isOrderCascadeRelayContent(msg *botModels.Message) bool {
	if msg == nil {
		return false
	}

	return msg.Text != "" || len(msg.Photo) > 0 || msg.Video != nil
}

func (b *Bot) tryRelayOrderCascadeReply(ctx context.Context, msg *botModels.Message) bool {
	if msg == nil || msg.From == nil || msg.From.IsBot || msg.ReplyToMessage == nil {
		return false
	}

	if !isOrderCascadeRelayContent(msg) {
		return false
	}

	state, ok := b.findOrderCascadeStateByUpstreamMessage(msg.Chat.ID, msg.ReplyToMessage.ID)
	if !ok || state == nil || state.MerchantChatID == 0 {
		return false
	}

	merchantReplyOn := b.resolveCascadeMerchantReplyMode(state)
	if !merchantReplyOn && strings.TrimSpace(msg.Text) != "" {
		directText := buildOrderCascadeDirectTextReplyMessage(state, msg.Text)
		if _, err := b.sendMessageWithMarkupAndMessage(ctx, state.MerchantChatID, directText, nil); err != nil {
			logger.L().Errorf("Failed to relay upstream text reply to merchant directly: upstream_chat=%d upstream_message=%d merchant_chat=%d err=%v",
				msg.Chat.ID, msg.ID, state.MerchantChatID, err)
			return false
		}
		logger.L().Infof("Cascade text reply relayed directly: upstream_chat=%d upstream_message=%d merchant_chat=%d order_no=%s",
			msg.Chat.ID, msg.ID, state.MerchantChatID, state.OrderNo)
		return true
	}

	if !merchantReplyOn {
		compactText := buildOrderCascadeCompactResultMessage(state, describeOrderCascadeReplyResult(msg))
		if compactText != "" {
			if _, err := b.sendMessageWithMarkupAndMessage(ctx, state.MerchantChatID, compactText, nil); err != nil {
				logger.L().Errorf("Failed to send cascade relay summary to merchant: upstream_chat=%d upstream_message=%d merchant_chat=%d err=%v",
					msg.Chat.ID, msg.ID, state.MerchantChatID, err)
				return false
			}
		}
	}

	params := &bot.CopyMessageParams{
		ChatID:     state.MerchantChatID,
		FromChatID: msg.Chat.ID,
		MessageID:  msg.ID,
	}

	if merchantReplyOn && state.MerchantMessageID > 0 {
		params.ReplyParameters = &botModels.ReplyParameters{
			MessageID:                state.MerchantMessageID,
			AllowSendingWithoutReply: true,
		}
	}

	sendCtx, cancel := context.WithTimeout(ctx, orderCascadeSendTimeout)
	defer cancel()

	if _, err := b.bot.CopyMessage(sendCtx, params); err != nil {
		logger.L().Errorf("Failed to relay upstream reply to merchant: upstream_chat=%d upstream_message=%d merchant_chat=%d merchant_reply_to=%d err=%v",
			msg.Chat.ID, msg.ID, state.MerchantChatID, state.MerchantMessageID, err)
		return false
	}

	logger.L().Infof("Cascade reply relayed: upstream_chat=%d upstream_message=%d merchant_chat=%d merchant_reply_to=%d order_no=%s",
		msg.Chat.ID, msg.ID, state.MerchantChatID, state.MerchantMessageID, state.OrderNo)
	return true
}

func (b *Bot) resolveCascadeMerchantReplyMode(state *orderCascadeState) bool {
	if state == nil {
		return true
	}

	fallback := state.MerchantReplyOn
	if b.groupService == nil || state.MerchantChatID == 0 {
		return fallback
	}

	lookupCtx, cancel := context.WithTimeout(context.Background(), orderCascadeGroupTimeout)
	defer cancel()

	group, err := b.groupService.GetGroupInfo(lookupCtx, state.MerchantChatID)
	if err != nil || group == nil {
		logger.L().Warnf("Order cascade resolve reply mode failed: merchant_chat=%d err=%v", state.MerchantChatID, err)
		return fallback
	}

	return models.IsCascadeReplyEnabled(group.Settings)
}

func buildOrderCascadeFeedbackMessage(state *orderCascadeState, action string, _ *botModels.User, _ time.Time) string {
	if state == nil {
		return ""
	}

	actionLabel := orderCascadeActionLabel(action)
	if state.MerchantReplyOn {
		return actionLabel
	}

	return buildOrderCascadeCompactResultMessage(state, actionLabel)
}

func buildOrderCascadeRelayContextMessage(state *orderCascadeState, actor *botModels.User, timestamp time.Time) string {
	if state == nil {
		return ""
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	orderNo := strings.TrimSpace(state.MerchantOrderFull)
	if orderNo == "" {
		orderNo = strings.TrimSpace(state.OrderNo)
	}

	interfaceName := strings.TrimSpace(state.InterfaceName)
	if interfaceName == "" && strings.TrimSpace(state.InterfaceID) != "" {
		interfaceName = fmt.Sprintf("接口 %s", strings.TrimSpace(state.InterfaceID))
	}

	builder := &strings.Builder{}
	builder.WriteString("📨 <b>上游回复</b>\n")
	if orderNo != "" {
		builder.WriteString(fmt.Sprintf("订单号：<code>%s</code>\n", html.EscapeString(orderNo)))
	}
	if interfaceName != "" {
		builder.WriteString(fmt.Sprintf("接口：%s\n", html.EscapeString(interfaceName)))
	}
	builder.WriteString(fmt.Sprintf("反馈人：%s\n", formatCascadeActor(actor)))
	builder.WriteString(fmt.Sprintf("时间：%s", timestamp.Format("2006-01-02 15:04:05")))
	return strings.TrimRight(builder.String(), "\n")
}

func buildOrderCascadeDirectTextReplyMessage(state *orderCascadeState, text string) string {
	content := strings.TrimSpace(text)
	if content == "" {
		content = "上游回复"
	}
	return buildOrderCascadeCompactResultMessage(state, html.EscapeString(content))
}

func buildOrderCascadeCompactResultMessage(state *orderCascadeState, result string) string {
	trimmedResult := strings.TrimSpace(result)
	if trimmedResult == "" {
		return ""
	}

	builder := &strings.Builder{}
	if orderNo := resolveOrderCascadeDisplayOrderNo(state); orderNo != "" {
		builder.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", html.EscapeString(orderNo)))
	}
	builder.WriteString(fmt.Sprintf("结果：%s", trimmedResult))
	return strings.TrimRight(builder.String(), "\n")
}

func resolveOrderCascadeDisplayOrderNo(state *orderCascadeState) string {
	if state == nil {
		return ""
	}

	orderNo := strings.TrimSpace(state.MerchantOrderFull)
	if orderNo == "" {
		orderNo = strings.TrimSpace(state.OrderNo)
	}
	return orderNo
}

func describeOrderCascadeReplyResult(msg *botModels.Message) string {
	if msg == nil {
		return "上游回复"
	}

	switch {
	case len(msg.Photo) > 0:
		return "上游回复图片"
	case msg.Video != nil:
		return "上游回复视频"
	default:
		return "上游回复"
	}
}

func orderCascadeActionLabel(action string) string {
	if info, ok := orderCascadeActions[action]; ok {
		return info.label
	}
	return html.EscapeString(action)
}

func formatCascadeActor(user *botModels.User) string {
	if user == nil {
		return "未知成员"
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		return fmt.Sprintf("@%s", html.EscapeString(username))
	}
	name := strings.TrimSpace(strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName))
	if name != "" {
		return html.EscapeString(name)
	}
	return fmt.Sprintf("#%d", user.ID)
}

func (b *Bot) editCascadeMessage(ctx context.Context, state *orderCascadeState, originalMsg *botModels.Message, action string, actor *botModels.User, timestamp time.Time) {
	if state == nil || state.UpstreamChatID == 0 || state.UpstreamMessageID == 0 {
		return
	}

	actionLabel := orderCascadeActionLabel(action)
	actorText := formatCascadeActor(actor)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	builder := &strings.Builder{}
	builder.WriteString(state.BaseMessageText)
	builder.WriteString("\n\n<b>最新反馈</b>\n")
	builder.WriteString(fmt.Sprintf("%s · %s · %s", actionLabel, actorText, timestamp.Format("2006-01-02 15:04:05")))

	markup := buildOrderCascadeKeyboard(state.Token)

	useCaption := state.HasMedia
	if originalMsg != nil {
		if len(originalMsg.Photo) > 0 || originalMsg.Video != nil {
			useCaption = true
		}
	}

	if useCaption {
		_, err := b.bot.EditMessageCaption(ctx, &bot.EditMessageCaptionParams{
			ChatID:      state.UpstreamChatID,
			MessageID:   state.UpstreamMessageID,
			Caption:     builder.String(),
			ParseMode:   botModels.ParseModeHTML,
			ReplyMarkup: markup,
		})
		if err != nil {
			logger.L().Errorf("Failed to edit cascade caption: chat_id=%d message_id=%d err=%v",
				state.UpstreamChatID, state.UpstreamMessageID, err)
		}
		return
	}

	b.editMessage(ctx, state.UpstreamChatID, state.UpstreamMessageID, builder.String(), markup)
}
