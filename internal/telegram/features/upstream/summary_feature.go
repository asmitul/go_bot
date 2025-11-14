package upstream

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	sifangfeature "go_bot/internal/telegram/features/sifang"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
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
	nowFunc        func() time.Time
}

// NewSummaryFeature 创建上游账单功能
func NewSummaryFeature(paymentSvc paymentservice.Service) *SummaryFeature {
	return &SummaryFeature{
		paymentService: paymentSvc,
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
	return len(group.Settings.InterfaceIDs) > 0
}

// Match 匹配「上游账单」指令
func (f *SummaryFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg == nil || msg.Text == "" {
		return false
	}
	if msg.Chat.Type != "" && msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}
	text := strings.TrimSpace(msg.Text)
	return strings.HasPrefix(text, "上游账单")
}

// Process 处理指令
func (f *SummaryFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	interfaceIDs := group.Settings.InterfaceIDs
	if len(interfaceIDs) == 0 {
		return respond("ℹ️ 当前群未绑定任何接口 ID，请先使用「绑定接口 [接口ID]」完成绑定"), true, nil
	}

	text := strings.TrimSpace(msg.Text)
	selected, dateSuffix, err := f.resolveTarget(interfaceIDs, text)
	if err != nil {
		return respond(fmt.Sprintf("❌ %v", err)), true, nil
	}
	if selected == "" {
		if len(interfaceIDs) == 1 {
			selected = interfaceIDs[0]
		} else {
			return respond(buildInterfacePrompt(interfaceIDs)), true, nil
		}
	}

	now := f.currentTime()
	targetDate, err := sifangfeature.ParseSummaryDate(dateSuffix, now, "上游账单")
	if err != nil {
		return respond(fmt.Sprintf("❌ %v", err)), true, nil
	}

	start := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	end := start.Add(24*time.Hour - time.Second)
	logger.L().Infof("Requesting upstream summary: chat_id=%d pzid=%s start=%s end=%s user=%d",
		msg.Chat.ID, selected,
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"),
		msg.From.ID)

	summary, err := f.paymentService.GetSummaryByDayByPZID(ctx, selected, start, end)
	if err != nil {
		logger.L().Errorf("Upstream summary query failed: chat_id=%d pzid=%s start=%s err=%v",
			msg.Chat.ID, selected, start.Format("2006-01-02"), err)
		return respond(fmt.Sprintf("❌ 查询上游账单失败：%v", err)), true, nil
	}

	item := pickSummaryItem(summary, targetDate)
	message := formatUpstreamSummary(selected, targetDate, item)

	logger.L().Infof("Upstream summary queried: chat_id=%d pzid=%s date=%s user=%d",
		msg.Chat.ID, selected, targetDate.Format("2006-01-02"), msg.From.ID)

	return respond(message), true, nil
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

func (f *SummaryFeature) resolveTarget(interfaceIDs []string, text string) (selectedID string, dateSuffix string, err error) {
	payload := strings.TrimSpace(strings.TrimPrefix(text, "上游账单"))
	if payload == "" {
		return "", "", nil
	}

	fields := strings.Fields(payload)
	if len(fields) == 0 {
		return "", "", nil
	}

	first := fields[0]
	match, ok := matchInterfaceID(interfaceIDs, first)
	if ok {
		selectedID = match
		dateSuffix = strings.TrimSpace(payload[len(first):])
		return
	}

	if len(fields) > 1 {
		return "", "", fmt.Errorf("未绑定接口 ID: %s", html.EscapeString(first))
	}

	return "", payload, nil
}

func buildInterfacePrompt(interfaceIDs []string) string {
	builder := strings.Builder{}
	builder.WriteString("ℹ️ 当前群绑定了多个接口，请使用「上游账单 [接口ID] [可选日期]」指定要查询的接口。\n\n可选接口：\n")
	for _, id := range interfaceIDs {
		builder.WriteString(fmt.Sprintf("• %s\n", html.EscapeString(id)))
	}
	return builder.String()
}

func matchInterfaceID(interfaceIDs []string, candidate string) (string, bool) {
	target := strings.ToLower(strings.TrimSpace(candidate))
	if target == "" {
		return "", false
	}
	for _, id := range interfaceIDs {
		if strings.ToLower(id) == target {
			return id, true
		}
	}
	return "", false
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

func formatUpstreamSummary(interfaceID string, date time.Time, item *paymentservice.SummaryByPZIDItem) string {
	dateStr := date.Format("2006-01-02")
	if item == nil {
		return fmt.Sprintf("ℹ️ %s 暂无上游账单数据（接口 <code>%s</code>）",
			dateStr, html.EscapeString(interfaceID))
	}

	orderCount := safeValue(item.OrderCount, "0")
	grossAmount := safeValue(item.GrossAmount, "0")
	merchantIncome := safeValue(item.MerchantIncome, "0")
	agentIncome := safeValue(item.AgentIncome, "0")

	return fmt.Sprintf("📈 上游账单 - %s\n接口：<code>%s</code>\n跑量：%s\n商户实收：%s\n代理收益：%s\n订单数：%s",
		dateStr,
		html.EscapeString(interfaceID),
		html.EscapeString(grossAmount),
		html.EscapeString(merchantIncome),
		html.EscapeString(agentIncome),
		html.EscapeString(orderCount),
	)
}

func safeValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
