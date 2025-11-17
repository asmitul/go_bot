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
	notifyFailureBodyLimit = 200
	notifyFailureURLLimit  = 120
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
	go b.startOrderCascadeWorkflow(group, msg, orderNos)
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

	if _, err := b.sendMessageWithMarkupAndMessage(sendCtx, chatID, builder.String(), nil); err != nil {
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

	if failureSection, logInfo := buildNotifyFailureSection(detail); failureSection != "" {
		builder.WriteString("\n")
		builder.WriteString(failureSection)

		logger.L().Warnf(
			"Sifang notify failure: order_no=%s notify_status=%s notify_status_text=%s last_error=%s callback_url=%s attempted_at=%s log_status=%s log_status_text=%s response_snippet=%s",
			orderNo,
			logInfo.NotifyStatus,
			logInfo.NotifyStatusText,
			logInfo.LastError,
			logInfo.URL,
			logInfo.AttemptedAt,
			logInfo.LogStatus,
			logInfo.LogStatusText,
			logInfo.ResponseSnippet,
		)
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

type notifyFailureLogInfo struct {
	NotifyStatus     string
	NotifyStatusText string
	LastError        string
	AttemptedAt      string
	URL              string
	LogStatus        string
	LogStatusText    string
	ResponseSnippet  string
}

func buildNotifyFailureSection(detail *paymentservice.OrderDetail) (string, *notifyFailureLogInfo) {
	if detail == nil || detail.Order == nil {
		return "", nil
	}

	order := detail.Order
	lastError := strings.TrimSpace(order.NotifyLastError)
	notifyStatus := strings.TrimSpace(order.NotifyStatus)
	notifyStatusText := strings.TrimSpace(order.NotifyStatusText)

	var logEntry *paymentservice.NotifyLog
	if count := len(detail.NotifyLogs); count > 0 {
		logEntry = detail.NotifyLogs[count-1]
	}

	var (
		logStatus     string
		logStatusText string
		attemptedAt   string
		url           string
		response      string
	)

	if logEntry != nil {
		logStatus = strings.TrimSpace(logEntry.Status)
		logStatusText = strings.TrimSpace(logEntry.StatusText)
		attemptedAt = strings.TrimSpace(logEntry.AttemptedAt)
		url = strings.TrimSpace(logEntry.URL)
		response = strings.TrimSpace(logEntry.Response)
	}

	if !shouldReportNotifyFailure(lastError, notifyStatus, notifyStatusText, logStatus, logStatusText) {
		return "", nil
	}

	var section strings.Builder
	section.WriteString("<b>通知失败详情</b>\n")

	if lastError != "" {
		section.WriteString(fmt.Sprintf("最后错误：%s\n", html.EscapeString(truncateForDisplay(lastError, notifyFailureBodyLimit))))
	}
	if attemptedAt != "" {
		section.WriteString(fmt.Sprintf("最近回调：%s\n", html.EscapeString(attemptedAt)))
	}
	if url != "" {
		section.WriteString(fmt.Sprintf("URL：%s\n", html.EscapeString(truncateForDisplay(url, notifyFailureURLLimit))))
	}
	if statusLine := combineStatus(logStatus, logStatusText); statusLine != "" {
		section.WriteString(fmt.Sprintf("状态：%s\n", html.EscapeString(statusLine)))
	}
	if response != "" {
		section.WriteString(fmt.Sprintf("响应：%s\n", html.EscapeString(truncateForDisplay(response, notifyFailureBodyLimit))))
	}

	rendered := strings.TrimRight(section.String(), "\n")
	info := &notifyFailureLogInfo{
		NotifyStatus:     notifyStatus,
		NotifyStatusText: notifyStatusText,
		LastError:        lastError,
		AttemptedAt:      attemptedAt,
		URL:              url,
		LogStatus:        logStatus,
		LogStatusText:    logStatusText,
		ResponseSnippet:  truncateForDisplay(response, notifyFailureBodyLimit),
	}

	return rendered, info
}

func shouldReportNotifyFailure(lastError, status, statusText, logStatus, logStatusText string) bool {
	if lastError != "" {
		return true
	}

	candidates := []string{status, statusText, logStatus, logStatusText}
	for _, candidate := range candidates {
		if indicatesFailure(candidate) {
			return true
		}
	}

	return false
}

func combineStatus(status, statusText string) string {
	trimmedStatus := strings.TrimSpace(status)
	trimmedText := strings.TrimSpace(statusText)

	if trimmedStatus == "" {
		return trimmedText
	}
	if trimmedText == "" || strings.EqualFold(trimmedStatus, trimmedText) {
		return trimmedStatus
	}
	return fmt.Sprintf("%s（%s）", trimmedStatus, trimmedText)
}

func indicatesFailure(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	failureIndicators := []string{"fail", "error", "timeout", "denied", "reject"}
	for _, indicator := range failureIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	localizedIndicators := []string{"失败", "错误", "超时", "拒绝"}
	for _, indicator := range localizedIndicators {
		if strings.Contains(trimmed, indicator) {
			return true
		}
	}

	return false
}

func truncateForDisplay(value string, limit int) string {
	if limit <= 0 {
		return ""
	}

	trimmed := strings.TrimSpace(value)
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	if limit == 1 {
		return string(runes[0])
	}
	return string(runes[:limit-1]) + "…"
}
