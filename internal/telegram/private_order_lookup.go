package telegram

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"

	botModels "github.com/go-telegram/bot/models"
)

const (
	privateLookupMerchantPrefixLength = 7
	privateLookupOrderMinLength       = 10
	privateLookupOrderMaxLength       = 60
	privateLookupTimeout              = 10 * time.Second
	privateLookupSendTimeout          = 5 * time.Second
	privateLookupChunkLimit           = 3500
	privateLookupFieldLimit           = 300
	privateLookupLogPayloadLimit      = 500
	privateLookupMaxNotifyLogs        = 20
)

var privateLookupOrderRegexp = regexp.MustCompile(`^\d{7}[A-Za-z0-9]{3,53}$`)

func (b *Bot) tryHandlePrivateAdminOrderLookup(ctx context.Context, msg *botModels.Message) bool {
	if msg == nil || msg.From == nil || strings.TrimSpace(msg.Text) == "" {
		return false
	}

	if msg.Chat.Type != "private" {
		return false
	}

	if b.userService == nil {
		logger.L().Error("Private order lookup skipped: user service is nil")
		b.sendErrorMessage(ctx, msg.Chat.ID, "用户服务未初始化，无法查单", msg.ID)
		return true
	}

	isAdmin, err := b.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Private order lookup admin check failed: user_id=%d err=%v", msg.From.ID, err)
		b.sendErrorMessage(ctx, msg.Chat.ID, "权限检查失败，请稍后重试", msg.ID)
		return true
	}
	if !isAdmin {
		logger.L().Warnf("Private order lookup unauthorized: user_id=%d chat_id=%d", msg.From.ID, msg.Chat.ID)
		b.sendErrorMessage(ctx, msg.Chat.ID, "此功能仅限管理员使用", msg.ID)
		return true
	}

	fullOrderNo, parseErr := parsePrivateLookupOrderNo(msg.Text)
	if parseErr != nil {
		b.sendErrorMessage(ctx, msg.Chat.ID, parseErr.Error(), msg.ID)
		return true
	}

	merchantID, merchantOrderNo, splitErr := splitPrivateLookupOrderNo(fullOrderNo)
	if splitErr != nil {
		b.sendErrorMessage(ctx, msg.Chat.ID, splitErr.Error(), msg.ID)
		return true
	}

	if b.paymentService == nil {
		b.sendErrorMessage(ctx, msg.Chat.ID, "四方支付服务未配置，无法查单", msg.ID)
		return true
	}

	lookupCtx, cancel := context.WithTimeout(context.Background(), privateLookupTimeout)
	detail, lookupErr := b.paymentService.GetOrderDetail(
		lookupCtx,
		merchantID,
		merchantOrderNo,
		paymentservice.OrderNumberTypeMerchant,
	)
	cancel()

	if lookupErr != nil {
		switch {
		case paymentservice.IsOrderNotFoundError(lookupErr):
			b.sendErrorMessage(ctx, msg.Chat.ID, "未找到该订单，请确认订单号是否正确", msg.ID)
		case isPrivateLookupMerchantMatchError(lookupErr):
			b.sendErrorMessage(
				ctx,
				msg.Chat.ID,
				fmt.Sprintf("无法匹配商户：订单号前 7 位商户号 %d 未配置", merchantID),
				msg.ID,
			)
		default:
			logger.L().Errorf("Private order lookup failed: merchant_id=%d order_no=%s err=%v", merchantID, merchantOrderNo, lookupErr)
			b.sendErrorMessage(ctx, msg.Chat.ID, "查询失败，请稍后重试", msg.ID)
		}
		return true
	}

	if detail == nil || detail.Order == nil {
		logger.L().Warnf("Private order lookup returned empty detail: merchant_id=%d order_no=%s", merchantID, merchantOrderNo)
		b.sendErrorMessage(ctx, msg.Chat.ID, "查询失败：订单详情为空", msg.ID)
		return true
	}

	chunks := buildPrivateLookupMessageChunks(fullOrderNo, merchantID, merchantOrderNo, detail)
	if len(chunks) == 0 {
		b.sendErrorMessage(ctx, msg.Chat.ID, "查询失败：结果为空", msg.ID)
		return true
	}

	for idx, chunk := range chunks {
		sendCtx, sendCancel := context.WithTimeout(context.Background(), privateLookupSendTimeout)
		replyTo := 0
		if idx == 0 {
			replyTo = msg.ID
		}

		if _, err := b.sendMessageWithMarkupAndMessage(sendCtx, msg.Chat.ID, chunk, nil, replyTo); err != nil {
			sendCancel()
			logger.L().Errorf(
				"Private order lookup send failed: chat_id=%d chunk=%d merchant_id=%d order_no=%s err=%v",
				msg.Chat.ID,
				idx+1,
				merchantID,
				merchantOrderNo,
				err,
			)
			if idx == 0 {
				b.sendErrorMessage(ctx, msg.Chat.ID, "查询失败：结果发送异常", msg.ID)
			}
			return true
		}
		sendCancel()
	}

	logger.L().Infof("Private order lookup success: user_id=%d merchant_id=%d order_no=%s", msg.From.ID, merchantID, merchantOrderNo)
	return true
}

func parsePrivateLookupOrderNo(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", fmt.Errorf("请输入完整订单号")
	}

	if len(trimmed) < privateLookupOrderMinLength || len(trimmed) > privateLookupOrderMaxLength {
		return "", fmt.Errorf("订单号格式错误：长度需在 %d-%d 位", privateLookupOrderMinLength, privateLookupOrderMaxLength)
	}

	if !privateLookupOrderRegexp.MatchString(trimmed) {
		return "", fmt.Errorf("订单号格式错误：需整条消息仅包含完整订单号，且前 7 位必须为商户号")
	}

	return trimmed, nil
}

func splitPrivateLookupOrderNo(fullOrderNo string) (merchantID int64, merchantOrderNo string, err error) {
	trimmed := strings.TrimSpace(fullOrderNo)
	if len(trimmed) <= privateLookupMerchantPrefixLength {
		return 0, "", fmt.Errorf("订单号格式错误：商户号后缺少订单主体")
	}

	prefix := trimmed[:privateLookupMerchantPrefixLength]
	merchantID, err = strconv.ParseInt(prefix, 10, 64)
	if err != nil || merchantID <= 0 {
		return 0, "", fmt.Errorf("订单号格式错误：前 7 位商户号无效")
	}

	merchantOrderNo = strings.TrimSpace(trimmed[privateLookupMerchantPrefixLength:])
	if merchantOrderNo == "" {
		return 0, "", fmt.Errorf("订单号格式错误：商户号后缺少订单主体")
	}

	return merchantID, merchantOrderNo, nil
}

func isPrivateLookupMerchantMatchError(err error) bool {
	if err == nil {
		return false
	}

	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(lower, "merchant key not found") ||
		strings.Contains(lower, "merchant id is required") ||
		strings.Contains(lower, "merchant id is invalid")
}

func buildPrivateLookupMessageChunks(
	fullOrderNo string,
	merchantID int64,
	merchantOrderNo string,
	detail *paymentservice.OrderDetail,
) []string {
	lines := []string{
		"🔎 <b>私聊订单详情</b>",
		fmt.Sprintf("完整订单号：<code>%s</code>", html.EscapeString(strings.TrimSpace(fullOrderNo))),
		fmt.Sprintf("商户号：<code>%d</code>", merchantID),
		fmt.Sprintf("商户订单号：<code>%s</code>", html.EscapeString(strings.TrimSpace(merchantOrderNo))),
		"",
		"<b>订单基础信息</b>",
	}
	lines = append(lines, buildPrivateLookupOrderLines(detail.Order)...)

	lines = append(lines, "", "<b>扩展信息</b>")
	lines = append(lines, buildPrivateLookupExtendedLines(detail.Extended)...)

	lines = append(lines, "", "<b>回调/通知日志</b>")
	lines = append(lines, buildPrivateLookupNotifyLogLines(detail.NotifyLogs)...)

	return splitPrivateLookupLines(lines, privateLookupChunkLimit)
}

func buildPrivateLookupOrderLines(order *paymentservice.Order) []string {
	if order == nil {
		return []string{"暂无订单基础信息"}
	}

	lines := make([]string, 0, 32)

	appendPrivateLookupLine(&lines, "商户订单号", order.MerchantOrderNo, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "平台订单号", order.PlatformOrderNo, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "状态", combineStatus(order.StatusText, order.Status), false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通知状态", combineStatus(order.NotifyStatusText, order.NotifyStatus), false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通知次数", order.NotifyTimes, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通知最后错误", order.NotifyLastError, false, privateLookupLogPayloadLimit)
	appendPrivateLookupLine(&lines, "订单金额", order.Amount, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "商户实收", order.RealAmount, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通道代码", order.ChannelCode, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通道名称", order.ChannelName, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "创建时间", order.CreatedAt, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "支付时间", order.PaidAt, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "完成时间", order.CompletedAt, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "过期时间", order.ExpiredAt, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "通知地址", order.NotifyURL, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "返回地址", order.ReturnURL, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "订单描述", order.Description, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "附加参数", order.Attach, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "客户端 IP", order.ClientIP, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "币种", order.Currency, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "用户 ID", order.UserID, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "支付链接", order.PaymentURL, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "银行代码", order.BankCode, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "银行账号", order.BankAccount, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "银行账户名", order.BankAccountName, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "开户支行", order.BankBranch, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "付款人姓名", order.BuyerName, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "付款人 ID", order.BuyerID, true, privateLookupFieldLimit)

	if len(order.Extra) > 0 {
		keys := make([]string, 0, len(order.Extra))
		for key := range order.Extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			appendPrivateLookupLine(
				&lines,
				"扩展字段 "+key,
				order.Extra[key],
				false,
				privateLookupFieldLimit,
			)
		}
	}

	if len(lines) == 0 {
		return []string{"暂无订单基础信息"}
	}

	return lines
}

func buildPrivateLookupExtendedLines(extended *paymentservice.OrderExtended) []string {
	if extended == nil {
		return []string{"暂无扩展信息"}
	}

	lines := make([]string, 0, 16)
	appendPrivateLookupLine(&lines, "订单 ID", extended.OrderID, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "商户 ID", extended.MerchantID, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "渠道 ID", extended.ChannelID, true, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "渠道手续费", extended.ChannelFee, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "渠道费率", extended.ChannelFeeRate, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "渠道成本", extended.ChannelCost, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "扣量状态", combineStatus(extended.DeductStatusText, extended.DeductStatus), false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "扣量金额", extended.DeductAmount, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "扣量原因", extended.DeductReason, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "备注", extended.Remark, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "创建时间", extended.CreatedAt, false, privateLookupFieldLimit)
	appendPrivateLookupLine(&lines, "更新时间", extended.UpdatedAt, false, privateLookupFieldLimit)

	lines = append(lines, fmt.Sprintf("风控标记：%s", formatPrivateLookupBoolean(extended.RiskFlag)))
	lines = append(lines, fmt.Sprintf("人工处理：%s", formatPrivateLookupBoolean(extended.Manual)))

	if len(lines) == 0 {
		return []string{"暂无扩展信息"}
	}

	return lines
}

func buildPrivateLookupNotifyLogLines(logs []*paymentservice.NotifyLog) []string {
	if len(logs) == 0 {
		return []string{"暂无回调/通知日志"}
	}

	start := 0
	if len(logs) > privateLookupMaxNotifyLogs {
		start = len(logs) - privateLookupMaxNotifyLogs
	}
	displayLogs := logs[start:]

	lines := make([]string, 0, len(displayLogs)*8+2)
	lines = append(lines, fmt.Sprintf("共 %d 条，展示最近 %d 条", len(logs), len(displayLogs)))

	for idx, logEntry := range displayLogs {
		if logEntry == nil {
			continue
		}

		lines = append(lines, fmt.Sprintf("日志 #%d", idx+1))
		appendPrivateLookupLine(&lines, "回调时间", logEntry.AttemptedAt, false, privateLookupFieldLimit)
		appendPrivateLookupLine(&lines, "回调状态", combineStatus(logEntry.StatusText, logEntry.Status), false, privateLookupFieldLimit)
		appendPrivateLookupLine(&lines, "回调地址", logEntry.URL, false, privateLookupFieldLimit)
		appendPrivateLookupLine(&lines, "耗时", logEntry.Duration, false, privateLookupFieldLimit)
		appendPrivateLookupLine(&lines, "重试次数", logEntry.Retry, false, privateLookupFieldLimit)
		appendPrivateLookupLine(&lines, "回调请求", logEntry.Request, false, privateLookupLogPayloadLimit)
		appendPrivateLookupLine(&lines, "回调响应", logEntry.Response, false, privateLookupLogPayloadLimit)

		if idx != len(displayLogs)-1 {
			lines = append(lines, "")
		}
	}

	if len(lines) == 0 {
		return []string{"暂无回调/通知日志"}
	}

	return lines
}

func appendPrivateLookupLine(lines *[]string, label, value string, codeStyle bool, limit int) {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return
	}
	if limit > 0 {
		clean = truncateForDisplay(clean, limit)
	}

	escapedLabel := html.EscapeString(strings.TrimSpace(label))
	escapedValue := html.EscapeString(clean)
	if codeStyle {
		*lines = append(*lines, fmt.Sprintf("%s：<code>%s</code>", escapedLabel, escapedValue))
		return
	}
	*lines = append(*lines, fmt.Sprintf("%s：%s", escapedLabel, escapedValue))
}

func splitPrivateLookupLines(lines []string, limit int) []string {
	if limit <= 0 {
		limit = privateLookupChunkLimit
	}

	chunks := make([]string, 0, 2)
	var current strings.Builder
	currentLen := 0

	flush := func() {
		if currentLen == 0 {
			return
		}
		chunks = append(chunks, strings.TrimRight(current.String(), "\n"))
		current.Reset()
		currentLen = 0
	}

	for _, rawLine := range lines {
		line := rawLine
		if privateLookupRuneLen(line) > limit-1 {
			line = truncateForDisplay(line, limit-1)
		}
		addition := line + "\n"
		addLen := privateLookupRuneLen(addition)

		if currentLen > 0 && currentLen+addLen > limit {
			flush()
		}

		if addLen > limit {
			addition = truncateForDisplay(line, limit-1) + "\n"
			addLen = privateLookupRuneLen(addition)
		}

		current.WriteString(addition)
		currentLen += addLen
	}

	flush()
	return chunks
}

func privateLookupRuneLen(text string) int {
	return len([]rune(text))
}

func formatPrivateLookupBoolean(value bool) string {
	if value {
		return "是"
	}
	return "否"
}
