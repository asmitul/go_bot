package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/service"
	sifanglookup "go_bot/internal/telegram/sifang"

	botModels "github.com/go-telegram/bot/models"
)

const (
	maxAutoLookupOrders    = 5
	orderLookupTimeout     = 10 * time.Second
	orderLookupSendTimeout = 5 * time.Second
)

func (b *Bot) tryTriggerSifangAutoLookup(ctx context.Context, msg *botModels.Message, fileNames ...string) {
        if b.paymentService == nil || b.groupService == nil || msg == nil {
                return
        }

        if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
                return
	}

	chatInfo := &service.TelegramChatInfo{
		ChatID:   msg.Chat.ID,
		Type:     string(msg.Chat.Type),
		Title:    msg.Chat.Title,
		Username: msg.Chat.Username,
	}

	group, err := b.groupService.GetOrCreateGroup(ctx, chatInfo)
	if err != nil {
		logger.L().Warnf("Failed to load group for sifang auto lookup: chat_id=%d err=%v", msg.Chat.ID, err)
		return
	}

	if !group.Settings.SifangEnabled || !group.Settings.SifangAutoLookupEnabled {
		return
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 {
		return
	}

	var parts []string
	if msg.Text != "" {
		parts = append(parts, msg.Text)
	}
	if msg.Caption != "" {
		parts = append(parts, msg.Caption)
	}

	for _, name := range fileNames {
		normalized := sifanglookup.NormalizeFileName(name)
		if normalized != "" {
			parts = append(parts, normalized)
		}
	}

	orderNos := sifanglookup.ExtractOrderNumbers(parts...)
	if len(orderNos) == 0 {
		return
	}

	if len(orderNos) > maxAutoLookupOrders {
		orderNos = append([]string{}, orderNos[:maxAutoLookupOrders]...)
	} else {
		orderNos = append([]string{}, orderNos...)
	}

	go b.performSifangOrderLookup(msg.Chat.ID, msg.ID, merchantID, orderNos)
}

func (b *Bot) performSifangOrderLookup(chatID int64, messageID int, merchantID int64, orderNos []string) {
	if b.paymentService == nil {
		return
	}

	var results []string
	for _, orderNo := range orderNos {
		lookupCtx, cancel := context.WithTimeout(context.Background(), orderLookupTimeout)
		detail, err := b.paymentService.GetOrderDetail(lookupCtx, merchantID, orderNo, paymentservice.OrderNumberTypeAuto)
		cancel()

		if err != nil {
			logger.L().Warnf("Sifang auto lookup failed: chat_id=%d merchant_id=%d order_no=%s err=%v", chatID, merchantID, orderNo, err)
			results = append(results, formatLookupFailure(orderNo))
			continue
		}
		if detail == nil || detail.Order == nil {
			logger.L().Warnf("Sifang auto lookup returned empty detail: chat_id=%d merchant_id=%d order_no=%s", chatID, merchantID, orderNo)
			results = append(results, formatLookupFailure(orderNo))
			continue
		}

		results = append(results, formatLookupSuccess(orderNo, detail))
	}

	if len(results) == 0 {
		return
	}

	builder := &strings.Builder{}
	builder.WriteString("🔎 <b>四方订单自动查单</b>\n")
	builder.WriteString(strings.Join(results, "\n\n"))

	sendCtx, cancel := context.WithTimeout(context.Background(), orderLookupSendTimeout)
	defer cancel()

	if _, err := b.sendMessageWithMarkupAndMessage(sendCtx, chatID, builder.String(), nil, messageID); err != nil {
		logger.L().Errorf("Failed to send sifang auto lookup result: chat_id=%d message_id=%d err=%v", chatID, messageID, err)
	}
}

func formatLookupFailure(orderNo string) string {
	return fmt.Sprintf("<b>%s</b>\n未找到订单", html.EscapeString(orderNo))
}

func formatLookupSuccess(orderNo string, detail *paymentservice.OrderDetail) string {
	if detail == nil || detail.Order == nil {
		return formatLookupFailure(orderNo)
	}

	order := detail.Order
	status := strings.TrimSpace(order.StatusText)
	if status == "" {
		status = strings.TrimSpace(order.Status)
	}
	if status == "" {
		status = "-"
	}

	notifyStatus := strings.TrimSpace(order.NotifyStatusText)
	if notifyStatus == "" {
		notifyStatus = strings.TrimSpace(order.NotifyStatus)
	}
	if notifyStatus != "" {
		notifyStatus = fmt.Sprintf("（通知：%s）", html.EscapeString(notifyStatus))
	}

	amount := strings.TrimSpace(order.RealAmount)
	if amount == "" {
		amount = strings.TrimSpace(order.Amount)
	}
	if amount == "" {
		amount = "-"
	}

	updatedAt := extractOrderUpdateTime(detail)

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("<b>%s</b>\n", html.EscapeString(orderNo)))
	builder.WriteString(fmt.Sprintf("状态：%s%s\n", html.EscapeString(status), notifyStatus))
	builder.WriteString(fmt.Sprintf("金额：%s\n", html.EscapeString(amount)))
	builder.WriteString(fmt.Sprintf("更新时间：%s", html.EscapeString(updatedAt)))

	if platformNo := strings.TrimSpace(order.PlatformOrderNo); platformNo != "" && !strings.EqualFold(platformNo, orderNo) {
		builder.WriteString(fmt.Sprintf("\n平台单号：%s", html.EscapeString(platformNo)))
	}

	return builder.String()
}

func extractOrderUpdateTime(detail *paymentservice.OrderDetail) string {
	if detail == nil {
		return "-"
	}

	if detail.Extended != nil && strings.TrimSpace(detail.Extended.UpdatedAt) != "" {
		return strings.TrimSpace(detail.Extended.UpdatedAt)
	}

	order := detail.Order
	if order == nil {
		return "-"
	}

	candidates := []string{order.CompletedAt, order.PaidAt, order.CreatedAt, order.ExpiredAt}
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" {
			return trimmed
		}
	}
	return "-"
}
