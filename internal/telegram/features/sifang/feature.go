package sifang

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
)

var (
	chinaLocation    = mustLoadChinaLocation()
	dateSuffixRegexp = regexp.MustCompile(`^[0-9\s./\-年月日号]*$`)
)

func mustLoadChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// Feature 四方支付功能
type Feature struct {
	paymentService paymentservice.Service
}

// New 创建四方支付功能实例
func New(paymentSvc paymentservice.Service) *Feature {
	return &Feature{
		paymentService: paymentSvc,
	}
}

// Name 功能名称
func (f *Feature) Name() string {
	return "sifang_payment"
}

// Enabled 仅在群组启用且服务已配置时生效
func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
	return group.Settings.SifangEnabled
}

// Match 支持命令：
//   - 余额
//   - 账单 / 账单10月26（可指定日期）
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return false
	}

	if _, ok := extractDateSuffix(text, "余额"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "账单"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "通道账单"); ok {
		return true
	}

	if _, ok := extractDateSuffix(text, "提款明细"); ok {
		return true
	}

	if text == "费率" {
		return true
	}

	return false
}

// Process 执行四方支付查询
func (f *Feature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (string, bool, error) {
	if f.paymentService == nil {
		return "❌ 未配置四方支付服务，请联系管理员", true, nil
	}

	if msg.From == nil {
		return "", false, nil
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 {
		return "ℹ️ 当前群组未绑定商户号，请先使用「绑定 [商户号]」命令", true, nil
	}

	text := strings.TrimSpace(msg.Text)
	if suffix, ok := extractDateSuffix(text, "余额"); ok {
		return f.handleBalance(ctx, merchantID, suffix)
	}

	if text == "费率" {
		return f.handleChannelRates(ctx, merchantID)
	}

	if _, ok := extractDateSuffix(text, "账单"); ok {
		return f.handleSummary(ctx, merchantID, text)
	}

	if _, ok := extractDateSuffix(text, "通道账单"); ok {
		return f.handleChannelSummary(ctx, merchantID, text)
	}

	if _, ok := extractDateSuffix(text, "提款明细"); ok {
		return f.handleWithdrawList(ctx, merchantID, text)
	}

	return "", false, nil
}

// Priority 设置为 25，介于商户绑定与行情功能之间
func (f *Feature) Priority() int {
	return 25
}

func (f *Feature) handleBalance(ctx context.Context, merchantID int64, rawSuffix string) (string, bool, error) {
	now := time.Now().In(chinaLocation)
	targetDate, err := parseBalanceDate(rawSuffix, now)
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	historyDays := calculateHistoryDays(targetDate, now)
	nowMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if historyDays > 365 {
		historyDays = 365
		targetDate = nowMidnight.AddDate(0, 0, -historyDays)
	}

	balance, err := f.paymentService.GetBalance(ctx, merchantID, historyDays)
	if err != nil {
		logger.L().Errorf("Sifang balance query failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, err)
		return fmt.Sprintf("❌ 查询余额失败：%v", err), true, nil
	}
	if balance == nil {
		logger.L().Warnf("Sifang balance query returned empty result: merchant_id=%d, history_days=%d", merchantID, historyDays)
		return "ℹ️ 暂未取得余额数据，请稍后重试", true, nil
	}

	amount := strings.TrimSpace(balance.Balance)
	if historyDays > 0 {
		amount = strings.TrimSpace(balance.HistoryBalance)
	}
	amount = emptyFallback(amount, "未知")

	merchant := balance.MerchantID
	if merchant == "" {
		merchant = strconv.FormatInt(merchantID, 10)
	}

	logger.L().Infof("Sifang balance queried: merchant_id=%s history_days=%d date=%s", merchant, historyDays, targetDate.Format("2006-01-02"))
	return amount, true, nil
}

func (f *Feature) handleSummary(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "账单"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "账单")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	summary, err := f.paymentService.GetSummaryByDay(ctx, merchantID, targetDate)
	if err != nil {
		logger.L().Errorf("Sifang summary query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询账单失败：%v", err), true, nil
	}

	if summary == nil {
		return fmt.Sprintf("ℹ️ %s 暂无账单数据", targetDate.Format("2006-01-02")), true, nil
	}

	if strings.TrimSpace(summary.Date) == "" {
		summary.Date = targetDate.Format("2006-01-02")
	}

	historyDays := calculateHistoryDays(targetDate, now)
	balanceAmount, balanceErr := f.queryBalanceAmount(ctx, merchantID, historyDays)
	withdrawMessage, withdrawErr := f.queryWithdrawMessage(ctx, merchantID, targetDate)

	logger.L().Infof("Sifang summary queried: merchant_id=%d, date=%s", merchantID, summary.Date)
	message := formatSummaryMessage(summary)

	if withdrawErr != nil {
		logger.L().Errorf("Sifang withdraw list in summary failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), withdrawErr)
	} else if withdrawMessage != "" {
		message = fmt.Sprintf("%s\n\n%s", message, withdrawMessage)
	}

	if balanceErr != nil {
		logger.L().Errorf("Sifang balance in summary failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, balanceErr)
	} else if balanceAmount != "" {
		message = fmt.Sprintf("%s\n\n余额：%s", message, balanceAmount)
	}

	return message, true, nil
}

func (f *Feature) queryBalanceAmount(ctx context.Context, merchantID int64, historyDays int) (string, error) {
	balance, err := f.paymentService.GetBalance(ctx, merchantID, historyDays)
	if err != nil {
		return "", err
	}
	if balance == nil {
		return "", fmt.Errorf("empty balance response")
	}
	amount := strings.TrimSpace(balance.Balance)
	if historyDays > 0 {
		amount = strings.TrimSpace(balance.HistoryBalance)
	}
	return emptyFallback(amount, "未知"), nil
}

func (f *Feature) queryWithdrawMessage(ctx context.Context, merchantID int64, targetDate time.Time) (string, error) {
	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)

	list, err := f.paymentService.GetWithdrawList(ctx, merchantID, start, end, 1, 100)
	if err != nil {
		return "", err
	}

	return formatWithdrawListMessage(targetDate.Format("2006-01-02"), list), nil
}

func parseSummaryDate(raw string, now time.Time, usage string) (time.Time, error) {
	usage = strings.TrimSpace(usage)
	if usage == "" {
		usage = "账单"
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}

	normalized := strings.ToLower(raw)
	normalized = strings.ReplaceAll(normalized, "日", "")
	normalized = strings.ReplaceAll(normalized, "号", "")
	normalized = strings.ReplaceAll(normalized, "年", "-")
	normalized = strings.ReplaceAll(normalized, "月", "-")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	normalized = strings.ReplaceAll(normalized, ".", "-")
	normalized = strings.Trim(normalized, "- ")
	if normalized == "" {
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
	}

	parts := strings.Split(normalized, "-")
	var (
		year  int
		month int
		day   int
		err   error
	)

	switch len(parts) {
	case 3:
		year, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		day, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
	case 2:
		year = now.Year()
		month, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
		day, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
		}
	default:
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「%s」或「%s10月26」", usage, usage)
	}

	candidate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
	if candidate.Month() != time.Month(month) || candidate.Day() != day || candidate.Year() != year {
		return time.Time{}, fmt.Errorf("日期不存在，请检查月份和日期")
	}

	if len(parts) == 2 && candidate.After(now) {
		candidate = candidate.AddDate(-1, 0, 0)
	}

	return candidate, nil
}

func parseBalanceDate(raw string, now time.Time) (time.Time, error) {
	return parseSummaryDate(raw, now, "余额")
}

func calculateHistoryDays(target, now time.Time) int {
	targetMidnight := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	nowMidnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if targetMidnight.After(nowMidnight) {
		return 0
	}

	days := int(nowMidnight.Sub(targetMidnight).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}

func formatSummaryMessage(summary *paymentservice.SummaryByDay) string {
	var sb strings.Builder

	date := strings.TrimSpace(summary.Date)
	if date == "" {
		date = "-"
	}
	sb.WriteString(fmt.Sprintf("📑 账单 - %s\n", html.EscapeString(date)))

	if value := strings.TrimSpace(summary.TotalAmount); value != "" {
		sb.WriteString(fmt.Sprintf("跑量：%s\n", html.EscapeString(value)))
	}
	if combinedIncome := combineAmounts(summary.MerchantIncome, summary.AgentIncome); combinedIncome != "" {
		sb.WriteString(fmt.Sprintf("成交：%s\n", html.EscapeString(combinedIncome)))
	}
	if value := strings.TrimSpace(summary.OrderCount); value != "" {
		sb.WriteString(fmt.Sprintf("笔数：%s\n", html.EscapeString(value)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (f *Feature) handleChannelSummary(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "通道账单"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "通道账单")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	items, err := f.paymentService.GetSummaryByDayByChannel(ctx, merchantID, targetDate)
	if err != nil {
		logger.L().Errorf("Sifang channel summary query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询通道账单失败：%v", err), true, nil
	}

	if len(items) == 0 {
		return fmt.Sprintf("ℹ️ %s 暂无通道账单数据", targetDate.Format("2006-01-02")), true, nil
	}

	logger.L().Infof("Sifang channel summary queried: merchant_id=%d, date=%s, channels=%d", merchantID, targetDate.Format("2006-01-02"), len(items))

	message := formatChannelSummaryMessage(targetDate.Format("2006-01-02"), items)

	historyDays := calculateHistoryDays(targetDate, now)
	balanceAmount, balanceErr := f.queryBalanceAmount(ctx, merchantID, historyDays)
	withdrawMessage, withdrawErr := f.queryWithdrawMessage(ctx, merchantID, targetDate)

	if withdrawErr != nil {
		logger.L().Errorf("Sifang withdraw list in channel summary failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), withdrawErr)
	} else if withdrawMessage != "" {
		message = fmt.Sprintf("%s\n\n%s", message, withdrawMessage)
	}

	if balanceErr != nil {
		logger.L().Errorf("Sifang balance in channel summary failed: merchant_id=%d, history_days=%d, err=%v", merchantID, historyDays, balanceErr)
	} else if balanceAmount != "" {
		message = fmt.Sprintf("%s\n\n余额：%s", message, balanceAmount)
	}

	return message, true, nil
}

func formatChannelSummaryMessage(date string, items []*paymentservice.SummaryByDayChannel) string {
	if len(items) == 0 {
		return fmt.Sprintf("ℹ️ %s 暂无通道账单数据", html.EscapeString(date))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📑 通道账单 - %s\n", html.EscapeString(date)))

	for _, item := range items {
		name := strings.TrimSpace(item.ChannelName)
		code := strings.TrimSpace(item.ChannelCode)

		sb.WriteString("\n")
		switch {
		case name != "" && code != "":
			sb.WriteString(fmt.Sprintf("%s：<code>%s</code>\n", html.EscapeString(name), html.EscapeString(code)))
		case name != "":
			sb.WriteString(fmt.Sprintf("%s\n", html.EscapeString(name)))
		case code != "":
			sb.WriteString(fmt.Sprintf("<code>%s</code>\n", html.EscapeString(code)))
		default:
			sb.WriteString("-\n")
		}

		volume := strings.TrimSpace(item.TotalAmount)
		if volume == "" {
			volume = "0"
		}
		sb.WriteString(fmt.Sprintf("跑量：%s\n", html.EscapeString(volume)))

		combined := combineAmounts(item.MerchantIncome, item.AgentIncome)
		if combined == "" {
			combined = "0"
		}
		sb.WriteString(fmt.Sprintf("成交：%s\n", html.EscapeString(combined)))

		count := strings.TrimSpace(item.OrderCount)
		if count == "" {
			count = "0"
		}
		sb.WriteString(fmt.Sprintf("笔数：%s\n", html.EscapeString(count)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (f *Feature) handleWithdrawList(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "提款明细"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now, "提款明细")
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)

	list, err := f.paymentService.GetWithdrawList(ctx, merchantID, start, end, 1, 10)
	if err != nil {
		logger.L().Errorf("Sifang withdraw list query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询提款明细失败：%v", err), true, nil
	}

	message := formatWithdrawListMessage(targetDate.Format("2006-01-02"), list)
	logger.L().Infof("Sifang withdraw list queried: merchant_id=%d, date=%s, count=%d", merchantID, targetDate.Format("2006-01-02"), len(list.Items))
	return message, true, nil
}

func formatWithdrawListMessage(date string, list *paymentservice.WithdrawList) string {
	var sb strings.Builder

	totalAmount := 0.0
	itemCount := 0
	for _, item := range list.Items {
		if amount, ok := parseAmountToFloat(item.Amount); ok {
			totalAmount += amount
		}
		itemCount++
	}

	title := "💸 提款明细"

	if itemCount == 0 {
		return fmt.Sprintf("%s\n暂无提款记录", title)
	}

	sb.WriteString(fmt.Sprintf("%s（总计 %s｜%d 笔）\n", title, html.EscapeString(formatFloat(totalAmount)), itemCount))

	for _, item := range list.Items {
		created := strings.TrimSpace(item.CreatedAt)
		timePart := extractTime(created)
		if timePart == "" {
			timePart = "--:--:--"
		}

		amount := strings.TrimSpace(item.Amount)
		if amount == "" {
			amount = "0"
		}

		sb.WriteString(fmt.Sprintf("%s      %s\n", html.EscapeString(timePart), html.EscapeString(amount)))
	}

	return strings.TrimRight(sb.String(), "\n")
}

func combineAmounts(merchant, agent string) string {
	merchant = strings.TrimSpace(merchant)
	agent = strings.TrimSpace(agent)

	if merchant == "" && agent == "" {
		return ""
	}

	merchantVal, ok1 := parseAmountToFloat(merchant)
	agentVal, ok2 := parseAmountToFloat(agent)

	if ok1 || ok2 {
		total := 0.0
		if ok1 {
			total += merchantVal
		}
		if ok2 {
			total += agentVal
		}
		return formatFloat(total)
	}

	if agent == "" {
		return merchant
	}
	if merchant == "" {
		return agent
	}
	return merchant + agent
}

func parseAmountToFloat(input string) (float64, bool) {
	if input == "" {
		return 0, false
	}
	cleaned := strings.ReplaceAll(input, ",", "")
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

func formatFloat(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.2f", value)
}

func extractTime(datetime string) string {
	datetime = strings.TrimSpace(datetime)
	if datetime == "" {
		return ""
	}

	if len(datetime) >= 8 {
		idx := strings.LastIndex(datetime, " ")
		if idx >= 0 && idx+1 < len(datetime) {
			timePart := datetime[idx+1:]
			if len(timePart) == 8 {
				return timePart
			}
		}

		if len(datetime) >= 8 {
			candidate := datetime[len(datetime)-8:]
			if strings.Count(candidate, ":") == 2 {
				return candidate
			}
		}
	}

	return ""
}

func extractDateSuffix(text, prefix string) (string, bool) {
	if !strings.HasPrefix(text, prefix) {
		return "", false
	}

	suffix := text[len(prefix):]
	if !isValidDateSuffix(suffix) {
		return "", false
	}
	return suffix, true
}

func isValidDateSuffix(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	return dateSuffixRegexp.MatchString(trimmed)
}

func (f *Feature) handleChannelRates(ctx context.Context, merchantID int64) (string, bool, error) {
	statuses, err := f.paymentService.GetChannelStatus(ctx, merchantID)
	if err != nil {
		logger.L().Errorf("Sifang channel status query failed: merchant_id=%d, err=%v", merchantID, err)
		return fmt.Sprintf("❌ 查询费率失败：%v", err), true, nil
	}

	if len(statuses) == 0 {
		return "ℹ️ 暂无通道状态数据", true, nil
	}

	message := formatChannelRatesMessage(statuses)
	logger.L().Infof("Sifang channel status queried: merchant_id=%d, channels=%d", merchantID, len(statuses))
	return message, true, nil
}

func formatChannelRatesMessage(items []*paymentservice.ChannelStatus) string {
	if len(items) == 0 {
		return "ℹ️ 暂无通道状态数据"
	}

	var sb strings.Builder
	sb.WriteString("📡 通道费率\n")
	sb.WriteString("<pre>")
	sb.WriteString("状态  通道代码    费率   通道名称\n")
	sb.WriteString("———————————————————————————————\n")

	for _, item := range items {
		if item == nil {
			continue
		}

		originalCode := strings.TrimSpace(item.ChannelCode)
		if strings.HasSuffix(strings.ToLower(originalCode), "test") {
			continue
		}

		status := "❌"
		if item.SystemEnabled && item.MerchantEnabled {
			status = "✅"
		}

		code := originalCode
		if code == "" {
			code = "-"
		}
		name := strings.TrimSpace(item.ChannelName)
		if name == "" {
			name = "-"
		}

		rate := formatChannelRate(item.Rate)

		line := fmt.Sprintf("%s %-8s %-6s %s\n",
			status,
			html.EscapeString(code),
			html.EscapeString(rate),
			html.EscapeString(name),
		)
		sb.WriteString(line)
	}

	output := strings.TrimRight(sb.String(), "\n")
	return output + "\n</pre>"
}

func formatChannelRate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "-" {
		return "-"
	}

	hasPercent := strings.ContainsAny(raw, "%％")
	normalized := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(raw, "%"), "％"))
	normalized = strings.ReplaceAll(normalized, ",", "")
	if normalized == "" {
		return "-"
	}

	if value, err := strconv.ParseFloat(normalized, 64); err == nil {
		if hasPercent || value > 1 {
			return strconv.FormatFloat(value, 'f', -1, 64) + "%"
		}
		return strconv.FormatFloat(value*100, 'f', -1, 64) + "%"
	}

	return raw
}
