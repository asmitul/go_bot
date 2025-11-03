package telegram

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

var (
	orderNumberRegexp      = regexp.MustCompile(`(?i)[a-z0-9]{6,}`)
	maxAutoOrderPerMessage = 3
)

type orderLookupRequest struct {
	original string
	composed string
}

func (b *Bot) maybeHandleAutoOrderLookup(ctx context.Context, msg *botModels.Message, parts ...string) {
	if msg == nil || msg.Chat.ID == 0 {
		return
	}

	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return
	}

	if b.paymentService == nil {
		return
	}

	combined := combineOrderText(parts...)
	if combined == "" {
		return
	}

	numbers := b.extractOrderNumbers(ctx, combined)
	if len(numbers) == 0 {
		return
	}

	chatInfo := &service.TelegramChatInfo{
		ChatID:   msg.Chat.ID,
		Type:     string(msg.Chat.Type),
		Title:    msg.Chat.Title,
		Username: msg.Chat.Username,
	}

	group, err := b.getOrCreateGroupCached(ctx, chatInfo)
	if err != nil {
		logger.L().Warnf("auto order lookup: get group failed chat=%d err=%v", msg.Chat.ID, err)
		return
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 || !group.Settings.SifangEnabled {
		return
	}

	merchantPrefix := fmt.Sprintf("%d", merchantID)

	processed := make(map[string]struct{})
	requests := make([]orderLookupRequest, 0, len(numbers))
	for _, num := range numbers {
		if _, exists := processed[num]; exists {
			continue
		}
		processed[num] = struct{}{}

		composed := num
		if !strings.HasPrefix(composed, merchantPrefix) {
			composed = merchantPrefix + composed
		}
		requests = append(requests, orderLookupRequest{original: num, composed: composed})
	}

	if len(requests) == 0 {
		return
	}

	maxSuccess := int32(maxAutoOrderPerMessage)
	if maxSuccess <= 0 {
		maxSuccess = 1
	}

	concurrency := b.autoOrderLookupConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	if concurrency == 1 || len(requests) == 1 {
		var successCount int32
		for _, req := range requests {
			if successCount >= maxSuccess {
				break
			}
			if b.lookupAndSendOrder(ctx, msg, merchantID, req.original, req.composed) {
				successCount++
				if successCount >= maxSuccess {
					break
				}
			}
		}
		return
	}

	var wg sync.WaitGroup
	limiter := make(chan struct{}, concurrency)
	var successCount atomic.Int32
	for _, req := range requests {
		if successCount.Load() >= maxSuccess {
			break
		}
		req := req
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case limiter <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-limiter }()

			if successCount.Load() >= maxSuccess {
				return
			}
			if b.lookupAndSendOrder(ctx, msg, merchantID, req.original, req.composed) {
				if successCount.Add(1) >= maxSuccess {
					return
				}
			}
		}()
	}

	wg.Wait()
}

func extractMediaOrderParts(msg *botModels.Message) []string {
	if msg == nil {
		return nil
	}

	var parts []string
	if caption := strings.TrimSpace(msg.Caption); caption != "" {
		parts = append(parts, caption)
	}

	if msg.Document != nil {
		if name := strings.TrimSpace(msg.Document.FileName); name != "" {
			parts = append(parts, name)
		}
	}

	if msg.Video != nil {
		if name := strings.TrimSpace(msg.Video.FileName); name != "" {
			parts = append(parts, name)
		}
	}

	if msg.Audio != nil {
		if title := strings.TrimSpace(msg.Audio.Title); title != "" {
			parts = append(parts, title)
		}
		if performer := strings.TrimSpace(msg.Audio.Performer); performer != "" {
			parts = append(parts, performer)
		}
		if name := strings.TrimSpace(msg.Audio.FileName); name != "" {
			parts = append(parts, name)
		}
	}

	if msg.Animation != nil {
		if name := strings.TrimSpace(msg.Animation.FileName); name != "" {
			parts = append(parts, name)
		}
	}

	if msg.Sticker != nil {
		if emoji := strings.TrimSpace(msg.Sticker.Emoji); emoji != "" {
			parts = append(parts, emoji)
		}
	}

	return parts
}

func combineOrderText(parts ...string) string {
	builder := strings.Builder{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(part)
	}
	return builder.String()
}

func (b *Bot) extractOrderNumbers(ctx context.Context, content string) []string {
	candidates := make(map[string]struct{})

	if matches := orderNumberRegexp.FindAllString(content, -1); len(matches) > 0 {
		for _, match := range matches {
			cleaned := sanitizeOrderNumber(match)
			if isValidOrderCandidate(cleaned) {
				candidates[cleaned] = struct{}{}
			}
		}
	}

	if b.orderNumberExtractor != nil {
		extractCtx, cancel := context.WithTimeout(ctx, b.orderNumberExtractorTimeout)
		defer cancel()

		extracted, err := b.orderNumberExtractor(extractCtx, content)
		if err != nil {
			logger.L().Warnf("auto order lookup: xai extraction failed err=%v", err)
		} else {
			for _, item := range extracted {
				cleaned := sanitizeOrderNumber(item)
				if isValidOrderCandidate(cleaned) {
					candidates[cleaned] = struct{}{}
				}
			}
		}
	}

	results := make([]string, 0, len(candidates))
	for candidate := range candidates {
		results = append(results, candidate)
	}
	sort.Strings(results)
	return results
}

func sanitizeOrderNumber(raw string) string {
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	for _, r := range raw {
		switch {
		case unicode.IsDigit(r):
			builder.WriteRune(r)
		case unicode.IsLetter(r):
			builder.WriteRune(unicode.ToUpper(r))
		}
	}
	return builder.String()
}

func isValidOrderCandidate(value string) bool {
	if len(value) < 8 || len(value) > 64 {
		return false
	}

	digitCount := 0
	for _, r := range value {
		if unicode.IsDigit(r) {
			digitCount++
		}
	}
	return digitCount >= 4
}

func (b *Bot) lookupAndSendOrder(ctx context.Context, msg *botModels.Message, merchantID int64, original, composed string) bool {
	lookupCtx, cancel := context.WithTimeout(ctx, b.autoOrderLookupTimeout)
	defer cancel()

	merchantPrefix := strconv.FormatInt(merchantID, 10)
	queryOrder := original
	if strings.HasPrefix(queryOrder, merchantPrefix) {
		trimmed := strings.TrimPrefix(queryOrder, merchantPrefix)
		if trimmed != "" {
			queryOrder = trimmed
		}
	}

	if order, found, ok := b.orderLookupCache.Get(merchantID, queryOrder); ok {
		if !found {
			b.sendAutoOrderMessage(ctx, msg, merchantID, original, queryOrder, composed, nil)
			return true
		}

		b.sendAutoOrderMessage(ctx, msg, merchantID, original, queryOrder, composed, order)
		return true
	}

	filter := paymentservice.OrderFilter{
		MerchantOrderNo: queryOrder,
		Page:            1,
		PageSize:        1,
	}

	orders, err := b.paymentService.GetOrders(lookupCtx, merchantID, filter)
	if err != nil {
		logger.L().Warnf("auto order lookup failed: merchant=%d order=%s err=%v", merchantID, composed, err)
		return false
	}

	if orders == nil || len(orders.Items) == 0 {
		logger.L().Infof("auto order lookup empty result: merchant=%d order=%s query=%s", merchantID, composed, queryOrder)
		b.orderLookupCache.Set(merchantID, queryOrder, nil, false)
		b.sendAutoOrderMessage(ctx, msg, merchantID, original, queryOrder, composed, nil)
		return true
	}

	order := orders.Items[0]
	b.orderLookupCache.Set(merchantID, queryOrder, order, true)
	b.sendAutoOrderMessage(ctx, msg, merchantID, original, queryOrder, composed, order)
	return true
}

func (b *Bot) getOrCreateGroupCached(ctx context.Context, chatInfo *service.TelegramChatInfo) (*models.Group, error) {
	if chatInfo == nil {
		return nil, fmt.Errorf("chat info is required")
	}

	if group, ok := b.groupCache.Get(chatInfo.ChatID); ok {
		return group, nil
	}

	group, err := b.groupService.GetOrCreateGroup(ctx, chatInfo)
	if err != nil {
		return nil, err
	}

	b.groupCache.Set(chatInfo.ChatID, group)
	return group, nil
}

func (b *Bot) sendAutoOrderMessage(ctx context.Context, msg *botModels.Message, merchantID int64, original, queryOrder, composed string, order *paymentservice.Order) {
	var lines []string
	lines = append(lines, "📦 <b>订单自动查询</b>")
	lines = append(lines, fmt.Sprintf("商户号：<code>%d</code>", merchantID))
	lines = append(lines, fmt.Sprintf("检测到订单号：<code>%s</code>", html.EscapeString(original)))
	lines = append(lines, fmt.Sprintf("查询订单号：<code>%s</code>", html.EscapeString(composed)))

	if order == nil {
		lines = append(lines, "<b>未找到相关订单信息。</b>")
	} else {
		if order.MerchantOrderNo != "" && order.MerchantOrderNo != queryOrder {
			lines = append(lines, fmt.Sprintf("返回商户订单号：<code>%s</code>", html.EscapeString(order.MerchantOrderNo)))
		}
		if order.PlatformOrderNo != "" {
			lines = append(lines, fmt.Sprintf("平台订单号：<code>%s</code>", html.EscapeString(order.PlatformOrderNo)))
		}
		if order.Amount != "" {
			lines = append(lines, fmt.Sprintf("金额：<code>%s</code>", html.EscapeString(order.Amount)))
		}
		if order.RealAmount != "" {
			lines = append(lines, fmt.Sprintf("实收金额：<code>%s</code>", html.EscapeString(order.RealAmount)))
		}
		if order.StatusText != "" {
			lines = append(lines, fmt.Sprintf("状态：<b>%s</b>", html.EscapeString(order.StatusText)))
		} else if order.Status != "" {
			lines = append(lines, fmt.Sprintf("状态：<b>%s</b>", html.EscapeString(order.Status)))
		}
		if order.PayStatus != "" {
			lines = append(lines, fmt.Sprintf("支付状态：<code>%s</code>", html.EscapeString(order.PayStatus)))
		}
		if order.NotifyStatus != "" {
			lines = append(lines, fmt.Sprintf("通知状态：<code>%s</code>", html.EscapeString(order.NotifyStatus)))
		}
		if order.Channel != "" {
			lines = append(lines, fmt.Sprintf("通道：<code>%s</code>", html.EscapeString(order.Channel)))
		}
		if order.CreatedAt != "" {
			lines = append(lines, fmt.Sprintf("创建时间：<code>%s</code>", html.EscapeString(order.CreatedAt)))
		}
		if order.PaidAt != "" {
			lines = append(lines, fmt.Sprintf("支付时间：<code>%s</code>", html.EscapeString(order.PaidAt)))
		}
	}

	b.sendMessage(ctx, msg.Chat.ID, strings.Join(lines, "\n"), msg.ID)
}
