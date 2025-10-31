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

	if text == "余额" {
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
	if text == "余额" {
		return f.handleBalance(ctx, merchantID)
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

func (f *Feature) handleBalance(ctx context.Context, merchantID int64) (string, bool, error) {
	balance, err := f.paymentService.GetBalance(ctx, merchantID)
	if err != nil {
		logger.L().Errorf("Sifang balance query failed: merchant_id=%d, err=%v", merchantID, err)
		return fmt.Sprintf("❌ 查询余额失败：%v", err), true, nil
	}

	merchant := balance.MerchantID
	if merchant == "" {
		merchant = strconv.FormatInt(merchantID, 10)
	}

	var sb strings.Builder
	// sb.WriteString("🏦 四方支付余额\n")
	// sb.WriteString(fmt.Sprintf("商户号：%s\n", merchant))
	// sb.WriteString(fmt.Sprintf("可用余额：%s\n", emptyFallback(balance.Balance, "未知")))
	// sb.WriteString(fmt.Sprintf("待提现：%s\n", emptyFallback(balance.PendingWithdraw, "0")))
	// if balance.Currency != "" {
	// 	sb.WriteString(fmt.Sprintf("币种：%s\n", balance.Currency))
	// }
	// if balance.UpdatedAt != "" {
	// 	sb.WriteString(fmt.Sprintf("更新时间：%s\n", balance.UpdatedAt))
	// }

	sb.WriteString(fmt.Sprintf("%s", emptyFallback(balance.Balance, "未知")))

	logger.L().Infof("Sifang balance queried: merchant_id=%s", merchant)
	return sb.String(), true, nil
}

func (f *Feature) handleSummary(ctx context.Context, merchantID int64, text string) (string, bool, error) {
	dateText := strings.TrimSpace(strings.TrimPrefix(text, "账单"))
	now := time.Now().In(chinaLocation)
	targetDate, err := parseSummaryDate(dateText, now)
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

	logger.L().Infof("Sifang summary queried: merchant_id=%d, date=%s", merchantID, summary.Date)
	return formatSummaryMessage(summary), true, nil
}

func parseSummaryDate(raw string, now time.Time) (time.Time, error) {
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
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
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
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
		}
		month, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
		}
		day, err = strconv.Atoi(parts[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
		}
	case 2:
		year = now.Year()
		month, err = strconv.Atoi(parts[0])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
		}
		day, err = strconv.Atoi(parts[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
		}
	default:
		return time.Time{}, fmt.Errorf("日期格式错误，请使用「账单」或「账单10月26」")
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
	targetDate, err := parseSummaryDate(dateText, now)
	if err != nil {
		return fmt.Sprintf("❌ %v", err), true, nil
	}

	items, err := f.paymentService.GetSummaryByDayByChannel(ctx, merchantID, targetDate)
	if err != nil {
		logger.L().Errorf("Sifang channel summary query failed: merchant_id=%d, date=%s, err=%v", merchantID, targetDate.Format("2006-01-02"), err)
		return fmt.Sprintf("❌ 查询通道账单失败：%v", err), true, nil
	}

	message := formatChannelSummaryMessage(targetDate.Format("2006-01-02"), items)
	logger.L().Infof("Sifang channel summary queried: merchant_id=%d, date=%s, channels=%d", merchantID, targetDate.Format("2006-01-02"), len(items))
	return message, true, nil
}

func formatChannelSummaryMessage(date string, items []*paymentservice.SummaryByDayChannel) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📑 通道账单 - %s\n", html.EscapeString(date)))

	if len(items) == 0 {
		sb.WriteString("跑量：0\n成交：0\n笔数：0")
		return sb.String()
	}

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
	targetDate, err := parseSummaryDate(dateText, now)
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
	sb.WriteString(fmt.Sprintf("💸 提款明细 - %s\n", html.EscapeString(date)))

	if list == nil || len(list.Items) == 0 {
		sb.WriteString("暂无提款记录")
		return sb.String()
	}

	for i, item := range list.Items {
		sb.WriteString(fmt.Sprintf("\n#%d", i+1))
		if item.WithdrawNo != "" {
			sb.WriteString(fmt.Sprintf(" 提现单号:%s", html.EscapeString(item.WithdrawNo)))
		}
		if item.OrderNo != "" {
			sb.WriteString(fmt.Sprintf(" 订单号:%s", html.EscapeString(item.OrderNo)))
		}
		sb.WriteString("\n")

		amount := strings.TrimSpace(item.Amount)
		if amount == "" {
			amount = "0"
		}
		sb.WriteString(fmt.Sprintf("金额：%s", html.EscapeString(amount)))
		if fee := strings.TrimSpace(item.Fee); fee != "" {
			sb.WriteString(fmt.Sprintf(" 手续费：%s", html.EscapeString(fee)))
		}
		if ch := strings.TrimSpace(item.Channel); ch != "" {
			sb.WriteString(fmt.Sprintf(" 渠道：%s", html.EscapeString(ch)))
		}
		sb.WriteString("\n")

		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "-"
		}
		sb.WriteString(fmt.Sprintf("状态：%s", html.EscapeString(status)))

		if created := strings.TrimSpace(item.CreatedAt); created != "" {
			sb.WriteString(fmt.Sprintf(" 创建：%s", html.EscapeString(created)))
		}
		if paid := strings.TrimSpace(item.PaidAt); paid != "" {
			sb.WriteString(fmt.Sprintf(" 支付：%s", html.EscapeString(paid)))
		}
		sb.WriteString("\n")
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
