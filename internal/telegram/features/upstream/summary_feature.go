package upstream

import (
	"context"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	sifangfeature "go_bot/internal/telegram/features/sifang"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
	telegramservice "go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

const defaultSupplierName = "上游供应商"

const (
	summaryCommand       = "ye"
	legacySummaryCommand = "上游账单"
)

var upstreamChinaLocation = loadChinaLocation()

func loadChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

// SummaryFeature 处理上游账单查询
type SummaryFeature struct {
	paymentService paymentservice.Service
	balanceService telegramservice.UpstreamBalanceService
	nowFunc        func() time.Time
}

// NewSummaryFeature 创建上游账单功能
func NewSummaryFeature(paymentSvc paymentservice.Service, balanceSvc ...telegramservice.UpstreamBalanceService) *SummaryFeature {
	var upstreamBalanceSvc telegramservice.UpstreamBalanceService
	if len(balanceSvc) > 0 {
		upstreamBalanceSvc = balanceSvc[0]
	}
	return &SummaryFeature{
		paymentService: paymentSvc,
		balanceService: upstreamBalanceSvc,
		nowFunc: func() time.Time {
			return time.Now().In(upstreamChinaLocation)
		},
	}
}

// Name 功能名称
func (f *SummaryFeature) Name() string {
	return "upstream_summary"
}

// AllowedGroupTiers 限定仅上游群可用
func (f *SummaryFeature) AllowedGroupTiers() []models.GroupTier {
	return []models.GroupTier{
		models.GroupTierUpstream,
	}
}

// Enabled 启用条件：已绑定至少一个接口 ID
func (f *SummaryFeature) Enabled(ctx context.Context, group *models.Group) bool {
	return len(group.Settings.InterfaceBindings) > 0
}

// Match 匹配「ye」指令
func (f *SummaryFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg == nil || msg.Text == "" {
		return false
	}
	if msg.Chat.Type != "" && msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}
	text := strings.TrimSpace(msg.Text)
	_, ok := summaryCommandPayload(text)
	return ok
}

// Process 处理指令
func (f *SummaryFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	bindings := group.Settings.InterfaceBindings
	if len(bindings) == 0 {
		return respond(fmt.Sprintf("ℹ️ 当前群未绑定任何接口 ID，请先使用「%s」完成绑定", bindCommandGuide)), true, nil
	}

	text := strings.TrimSpace(msg.Text)
	selectedBinding, dateSuffix, err := f.resolveTarget(bindings, text)
	if err != nil {
		return respond(fmt.Sprintf("❌ %v", err)), true, nil
	}

	now := f.currentTime()
	targetDate, err := sifangfeature.ParseSummaryDate(dateSuffix, now, summaryCommand)
	if err != nil {
		return respond(fmt.Sprintf("❌ %v", err)), true, nil
	}

	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)

	targetBindings := f.buildTargetBindings(bindings, selectedBinding)
	rows := make([]upstreamSummaryRow, 0, len(targetBindings))
	for _, binding := range targetBindings {
		row, err := f.queryUpstreamSummary(ctx, msg, binding, start, end, targetDate)
		if err != nil {
			return respond(fmt.Sprintf("❌ 查询上游账单失败：%v", err)), true, nil
		}
		rows = append(rows, row)
	}

	prepaid, err := f.queryPrepaidBalance(ctx, msg.Chat.ID)
	if err != nil {
		return respond(fmt.Sprintf("❌ 查询上游账单失败：%v", err)), true, nil
	}
	yesterdayBalance, err := f.queryYesterdayBalance(ctx, msg.Chat.ID, targetDate)
	if err != nil {
		return respond(fmt.Sprintf("❌ 查询上游账单失败：%v", err)), true, nil
	}

	return respond(formatSupplierBill(defaultSupplierName, start, end, rows, prepaid, yesterdayBalance)), true, nil
}

// Priority 在接口管理之后执行
func (f *SummaryFeature) Priority() int {
	return 18
}

func (f *SummaryFeature) currentTime() time.Time {
	if f.nowFunc != nil {
		return f.nowFunc()
	}
	return time.Now().In(upstreamChinaLocation)
}

func (f *SummaryFeature) buildTargetBindings(bindings []models.InterfaceBinding, selected *models.InterfaceBinding) []models.InterfaceBinding {
	if selected != nil {
		return []models.InterfaceBinding{*selected}
	}
	if len(bindings) == 1 {
		return []models.InterfaceBinding{bindings[0]}
	}
	return bindings
}

func (f *SummaryFeature) resolveTarget(bindings []models.InterfaceBinding, text string) (selectedBinding *models.InterfaceBinding, dateSuffix string, err error) {
	payload, ok := summaryCommandPayload(text)
	if !ok {
		return nil, "", fmt.Errorf("命令格式错误，请使用「%s」", summaryCommand)
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, "", nil
	}

	fields := strings.Fields(payload)
	if len(fields) == 0 {
		return nil, "", nil
	}

	first := fields[0]
	match := matchInterfaceBinding(bindings, first)
	if match != nil {
		selectedBinding = match
		dateSuffix = strings.TrimSpace(payload[len(first):])
		return
	}

	if len(fields) > 1 {
		return nil, "", fmt.Errorf("未绑定接口 ID: %s", html.EscapeString(first))
	}

	return nil, payload, nil
}

func summaryCommandPayload(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	if lower == summaryCommand {
		return "", true
	}
	if strings.HasPrefix(lower, summaryCommand) && len(trimmed) > len(summaryCommand) {
		next := trimmed[len(summaryCommand)]
		if next == ' ' || next == '\t' || (next >= '0' && next <= '9') {
			return trimmed[len(summaryCommand):], true
		}
	}
	if strings.HasPrefix(trimmed, legacySummaryCommand) {
		return strings.TrimPrefix(trimmed, legacySummaryCommand), true
	}
	return "", false
}

func matchInterfaceBinding(bindings []models.InterfaceBinding, candidate string) *models.InterfaceBinding {
	target := strings.ToLower(strings.TrimSpace(candidate))
	if target == "" {
		return nil
	}
	for idx := range bindings {
		if strings.ToLower(bindings[idx].ID) == target {
			return &bindings[idx]
		}
	}
	nameCandidate := strings.TrimSpace(candidate)
	for idx := range bindings {
		if strings.EqualFold(strings.TrimSpace(bindings[idx].Name), nameCandidate) {
			return &bindings[idx]
		}
	}
	return nil
}

func (f *SummaryFeature) queryUpstreamSummary(
	ctx context.Context,
	msg *botModels.Message,
	binding models.InterfaceBinding,
	start, end, targetDate time.Time,
) (upstreamSummaryRow, error) {
	logger.L().Infof("Requesting upstream summary: chat_id=%d pzid=%s start=%s end=%s user=%d",
		msg.Chat.ID, binding.ID,
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"),
		msg.From.ID)

	summary, err := f.paymentService.GetSummaryByDayByPZID(ctx, binding.ID, start, end)
	if err != nil {
		logger.L().Errorf("Upstream summary query failed: chat_id=%d pzid=%s start=%s err=%v",
			msg.Chat.ID, binding.ID, start.Format("2006-01-02"), err)
		return upstreamSummaryRow{}, err
	}

	item := pickSummaryItem(summary, targetDate)
	row, err := buildUpstreamSummaryRow(binding, item)
	if err != nil {
		return upstreamSummaryRow{}, err
	}

	logger.L().Infof("Upstream summary queried: chat_id=%d pzid=%s date=%s user=%d",
		msg.Chat.ID, binding.ID, targetDate.Format("2006-01-02"), msg.From.ID)

	return row, nil
}

func (f *SummaryFeature) queryPrepaidBalance(ctx context.Context, chatID int64) (float64, error) {
	if f.balanceService == nil {
		return 0, nil
	}
	balance, err := f.balanceService.Get(ctx, chatID)
	if err != nil {
		return 0, fmt.Errorf("获取供应商预付失败：%w", err)
	}
	if balance == nil {
		return 0, nil
	}
	return balance.Balance, nil
}

func (f *SummaryFeature) queryYesterdayBalance(ctx context.Context, chatID int64, targetDate time.Time) (*float64, error) {
	if f.balanceService == nil {
		return nil, nil
	}
	snapshot, err := f.balanceService.GetSettlementSnapshot(ctx, chatID, targetDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, fmt.Errorf("获取昨日结余失败：%w", err)
	}
	if snapshot == nil {
		return nil, nil
	}
	return &snapshot.ClosingPrepaid, nil
}

func pickSummaryItem(summary *paymentservice.SummaryByPZID, targetDate time.Time) *paymentservice.SummaryByPZIDItem {
	if summary == nil || len(summary.Items) == 0 {
		return nil
	}
	dateStr := targetDate.Format("2006-01-02")
	for _, item := range summary.Items {
		if item == nil {
			continue
		}
		itemDate := normalizeSummaryDate(item.Date)
		if itemDate == "" {
			continue
		}
		if itemDate == dateStr {
			return item
		}
	}
	return nil
}

type upstreamSummaryRow struct {
	Binding    models.InterfaceBinding
	Gross      float64
	Settlement float64
}

func buildUpstreamSummaryRow(binding models.InterfaceBinding, item *paymentservice.SummaryByPZIDItem) (upstreamSummaryRow, error) {
	row := upstreamSummaryRow{Binding: binding}
	if item == nil {
		return row, nil
	}

	gross, err := parseSummaryAmount(item.GrossAmount)
	if err != nil {
		return upstreamSummaryRow{}, fmt.Errorf("接口 %s 跑量金额格式错误: %w", binding.ID, err)
	}

	settlementRaw := strings.TrimSpace(item.NetAfterUpstream)
	if settlementRaw == "" {
		settlementRaw = item.MerchantIncome
	}
	settlement, err := parseSummaryAmount(settlementRaw)
	if err != nil {
		return upstreamSummaryRow{}, fmt.Errorf("接口 %s 应结算金额格式错误: %w", binding.ID, err)
	}

	row.Gross = gross
	row.Settlement = settlement
	return row, nil
}

func formatSupplierBill(supplierName string, start, end time.Time, rows []upstreamSummaryRow, prepaid float64, yesterdayBalance *float64) string {
	totalGross := 0.0
	totalSettlement := 0.0
	for _, row := range rows {
		totalGross += row.Gross
		totalSettlement += row.Settlement
	}
	settlementDiff := totalSettlement - prepaid

	builder := &strings.Builder{}
	builder.WriteString("<b>📄 供应商账单</b>\n")
	builder.WriteString(fmt.Sprintf("%s | %s\n", html.EscapeString(supplierName), start.Format("2006/01/02")))
	builder.WriteString(fmt.Sprintf("%s - %s\n\n", start.Format("15:04:05"), end.Format("15:04:05")))

	builder.WriteString("<b>📊 通道明细</b>\n")
	for idx, row := range rows {
		builder.WriteString(fmt.Sprintf("<b>%d. %s</b>\n", idx+1, html.EscapeString(bindingDisplayName(row.Binding.Name))))
		builder.WriteString(fmt.Sprintf("费率：<code>%s</code>\n", html.EscapeString(displayRate(row.Binding.Rate))))
		builder.WriteString(fmt.Sprintf("跑量：<code>%s</code>\n", formatCopyAmount(row.Gross)))
		builder.WriteString(fmt.Sprintf("应结算：<code>%s</code>\n", formatCopyAmount(row.Settlement)))
		if idx < len(rows)-1 {
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")
	builder.WriteString("<b>📈 汇总</b>\n")
	if yesterdayBalance != nil {
		builder.WriteString(fmt.Sprintf("昨日结余 <code>%s</code>\n", formatCopyAmount(*yesterdayBalance)))
	} else {
		builder.WriteString("昨日结余 <code>暂无</code>\n")
	}
	builder.WriteString(fmt.Sprintf("跑量 <code>%s</code> | 应结算 <code>%s</code>\n",
		formatCopyAmount(totalGross),
		formatCopyAmount(totalSettlement)))
	builder.WriteString(fmt.Sprintf("预付 <code>%s</code> | 结算差额 <code>%s</code>\n", formatCopyAmount(prepaid), formatCopyAmount(settlementDiff)))

	builder.WriteString(fmt.Sprintf("公式：<code>%s</code> - %s = <code>%s</code>",
		formatCopyAmount(totalSettlement),
		formatFormulaSubtrahend(prepaid),
		formatCopyAmount(settlementDiff)))

	return builder.String()
}

func displayRate(rate string) string {
	trimmed := strings.TrimSpace(rate)
	if trimmed == "" {
		return "0%"
	}
	if !strings.HasSuffix(trimmed, "%") {
		return trimmed + "%"
	}
	return trimmed
}

func parseSummaryAmount(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value := strings.ReplaceAll(trimmed, ",", "")
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return amount, nil
}

func formatCopyAmount(value float64) string {
	rounded := math.Round(value*100) / 100
	if math.Abs(rounded) == 0 {
		rounded = 0
	}

	formatted := strconv.FormatFloat(rounded, 'f', 2, 64)
	if strings.HasSuffix(formatted, ".00") {
		return strings.TrimSuffix(formatted, ".00")
	}
	return formatted
}

func formatFormulaSubtrahend(value float64) string {
	formatted := fmt.Sprintf("<code>%s</code>", formatCopyAmount(value))
	if value < 0 {
		return fmt.Sprintf("(%s)", formatted)
	}
	return formatted
}

func normalizeSummaryDate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.Format("2006-01-02")
		}
	}

	if len(trimmed) >= 10 {
		candidate := trimmed[:10]
		if t, err := time.Parse("2006-01-02", candidate); err == nil {
			return t.Format("2006-01-02")
		}
		if t, err := time.Parse("2006/01/02", candidate); err == nil {
			return t.Format("2006-01-02")
		}
	}

	return ""
}
