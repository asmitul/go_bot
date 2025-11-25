package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

const (
	defaultAlertLimitPerHour = 3
)

// UpstreamBalanceServiceImpl 上游群余额服务
type UpstreamBalanceServiceImpl struct {
	repo           repository.UpstreamBalanceRepository
	groupRepo      repository.GroupRepository
	paymentService paymentservice.Service
	events         chan *models.UpstreamBalanceEvent
	location       *time.Location
}

type settlementItem struct {
	Binding     models.InterfaceBinding
	Volume      float64
	Rate        float64
	PZName      string
	Deduction   float64
	RawAmount   string
	RawRate     string
	Description string
}

// NewUpstreamBalanceService 创建服务实例
func NewUpstreamBalanceService(
	repo repository.UpstreamBalanceRepository,
	groupRepo repository.GroupRepository,
	paymentSvc paymentservice.Service,
) UpstreamBalanceService {
	return &UpstreamBalanceServiceImpl{
		repo:           repo,
		groupRepo:      groupRepo,
		paymentService: paymentSvc,
		events:         make(chan *models.UpstreamBalanceEvent, 128),
		location:       mustLoadChinaLocation(),
	}
}

// Adjust 调整余额
func (s *UpstreamBalanceServiceImpl) Adjust(ctx context.Context, groupID int64, delta float64, operatorID int64, remark string, operationID string) (*UpstreamBalanceResult, bool, error) {
	if delta == 0 {
		return nil, false, fmt.Errorf("调整金额不能为 0")
	}

	if err := s.ensureUpstreamGroup(ctx, groupID); err != nil {
		return nil, false, err
	}

	opType := models.BalanceOpDebit
	if delta > 0 {
		opType = models.BalanceOpCredit
	}

	balance, err := s.repo.Adjust(ctx, groupID, delta, operatorID, remark, opType, operationID, nil)
	if err != nil {
		return nil, false, err
	}

	result := toBalanceResult(balance)
	below := result.Balance < result.MinBalance
	s.publishEvent(&models.UpstreamBalanceEvent{
		GroupID:           groupID,
		Balance:           result.Balance,
		MinBalance:        result.MinBalance,
		AlertLimitPerHour: result.AlertLimitPerHour,
		BelowMin:          below,
		OccurredAt:        time.Now(),
		Trigger:           "adjust",
	})

	return result, below, nil
}

// SetMinBalance 设置最低余额
func (s *UpstreamBalanceServiceImpl) SetMinBalance(ctx context.Context, groupID int64, threshold float64, operatorID int64) (*UpstreamBalanceResult, error) {
	if threshold < 0 {
		return nil, fmt.Errorf("最低余额不能为负数")
	}

	if err := s.ensureUpstreamGroup(ctx, groupID); err != nil {
		return nil, err
	}

	balance, err := s.repo.SetMinBalance(ctx, groupID, threshold, operatorID)
	if err != nil {
		return nil, err
	}

	result := toBalanceResult(balance)
	s.publishEvent(&models.UpstreamBalanceEvent{
		GroupID:           balance.GroupID,
		Balance:           result.Balance,
		MinBalance:        result.MinBalance,
		AlertLimitPerHour: result.AlertLimitPerHour,
		BelowMin:          result.Balance < result.MinBalance,
		OccurredAt:        time.Now(),
		Trigger:           "set_min_balance",
	})
	return result, nil
}

// SetAlertLimit 更新告警频率
func (s *UpstreamBalanceServiceImpl) SetAlertLimit(ctx context.Context, groupID int64, limit int, operatorID int64) (*UpstreamBalanceResult, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("告警频率必须大于 0")
	}

	if err := s.ensureUpstreamGroup(ctx, groupID); err != nil {
		return nil, err
	}

	balance, err := s.repo.SetAlertLimit(ctx, groupID, limit, operatorID)
	if err != nil {
		return nil, err
	}

	result := toBalanceResult(balance)
	s.publishEvent(&models.UpstreamBalanceEvent{
		GroupID:           balance.GroupID,
		Balance:           result.Balance,
		MinBalance:        result.MinBalance,
		AlertLimitPerHour: result.AlertLimitPerHour,
		BelowMin:          result.Balance < result.MinBalance,
		OccurredAt:        time.Now(),
		Trigger:           "set_alert_limit",
	})
	return result, nil
}

// Get 查询余额
func (s *UpstreamBalanceServiceImpl) Get(ctx context.Context, groupID int64) (*UpstreamBalanceResult, error) {
	if err := s.ensureUpstreamGroup(ctx, groupID); err != nil {
		return nil, err
	}

	balance, err := s.repo.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}

	return toBalanceResult(balance), nil
}

// ListAll 列出全部余额
func (s *UpstreamBalanceServiceImpl) ListAll(ctx context.Context) ([]*UpstreamBalanceResult, error) {
	balances, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]*UpstreamBalanceResult, 0, len(balances))
	for _, b := range balances {
		results = append(results, toBalanceResult(b))
	}
	return results, nil
}

// SettleDaily 日结扣费
func (s *UpstreamBalanceServiceImpl) SettleDaily(ctx context.Context, groupID int64, targetDate time.Time, operatorID int64, operationID string) (*SettlementResult, error) {
	if s.paymentService == nil {
		return nil, fmt.Errorf("支付服务未配置，无法日结")
	}

	group, err := s.groupRepo.GetByTelegramID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("获取群组失败: %w", err)
	}
	if err := s.validateUpstreamGroup(group); err != nil {
		return nil, err
	}

	loc := s.location
	if loc == nil {
		loc = time.Local
	}
	target := targetDate.In(loc)
	if target.IsZero() {
		now := time.Now().In(loc)
		target = previousBillingDate(now, loc)
	}

	start := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24*time.Hour - time.Second)

	items := make([]settlementItem, 0, len(group.Settings.InterfaceBindings))
	errors := make([]string, 0)
	totalDeduction := 0.0

	for _, binding := range group.Settings.InterfaceBindings {
		summary, sumErr := s.paymentService.GetSummaryByDayByPZID(ctx, binding.ID, start, end)
		if sumErr != nil {
			logger.L().Errorf("SettleDaily summary failed: chat_id=%d pzid=%s err=%v", groupID, binding.ID, sumErr)
			errors = append(errors, fmt.Sprintf("接口 %s 查询失败: %v", binding.ID, sumErr))
			continue
		}

		itemSummary := pickPZIDItem(summary, target)
		if itemSummary == nil {
			items = append(items, settlementItem{
				Binding:     binding,
				Volume:      0,
				Rate:        0,
				PZName:      trim(summary.PZName),
				Deduction:   0,
				RawAmount:   "",
				RawRate:     binding.Rate,
				Description: "无数据",
			})
			continue
		}

		volume, parseVolumeErr := parseAmount(itemSummary.GrossAmount)
		if parseVolumeErr != nil {
			errors = append(errors, fmt.Sprintf("接口 %s 跑量解析失败: %v", binding.ID, parseVolumeErr))
			continue
		}

		rate, parseRateErr := parseRate(binding.Rate)
		if parseRateErr != nil {
			errors = append(errors, fmt.Sprintf("接口 %s 费率解析失败: %v", binding.ID, parseRateErr))
			continue
		}

		deduction := volume * rate
		totalDeduction += deduction
		items = append(items, settlementItem{
			Binding:   binding,
			Volume:    volume,
			Rate:      rate,
			PZName:    trim(summary.PZName),
			Deduction: deduction,
			RawAmount: itemSummary.GrossAmount,
			RawRate:   binding.Rate,
		})
	}

	var balanceResult *UpstreamBalanceResult
	below := false
	if totalDeduction > 0 {
		remark := fmt.Sprintf("日结 %s", target.Format("2006-01-02"))
		balance, belowMin, adjustErr := s.Adjust(ctx, groupID, -totalDeduction, operatorID, remark, operationID)
		if adjustErr != nil {
			return nil, adjustErr
		}
		below = belowMin
		balanceResult = balance
	} else {
		current, getErr := s.repo.Get(ctx, groupID)
		if getErr != nil {
			return nil, getErr
		}
		balanceResult = toBalanceResult(current)
		below = balanceResult.Balance < balanceResult.MinBalance
	}

	report := s.buildSettlementReport(group, target, items, totalDeduction, balanceResult, errors)

	return &SettlementResult{
		GroupID:        groupID,
		TargetDate:     target,
		TotalDeduction: totalDeduction,
		Balance:        balanceResult.Balance,
		BelowMin:       below,
		Report:         report,
	}, nil
}

// SubscribeEvents 获取调整事件通道
func (s *UpstreamBalanceServiceImpl) SubscribeEvents() <-chan *models.UpstreamBalanceEvent {
	return s.events
}

func (s *UpstreamBalanceServiceImpl) ensureUpstreamGroup(ctx context.Context, groupID int64) error {
	group, err := s.groupRepo.GetByTelegramID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("群组不存在")
	}
	return s.validateUpstreamGroup(group)
}

func (s *UpstreamBalanceServiceImpl) validateUpstreamGroup(group *models.Group) error {
	if group == nil {
		return fmt.Errorf("群组不存在")
	}
	if models.NormalizeGroupTier(group.Tier) != models.GroupTierUpstream {
		return fmt.Errorf("仅上游群可使用余额功能")
	}
	if len(group.Settings.InterfaceBindings) == 0 {
		return fmt.Errorf("未绑定上游接口，无法使用余额功能")
	}
	return nil
}

func (s *UpstreamBalanceServiceImpl) publishEvent(ev *models.UpstreamBalanceEvent) {
	if ev == nil {
		return
	}
	select {
	case s.events <- ev:
	default:
		logger.L().Warn("Upstream balance event channel full, dropping event")
	}
}

func (s *UpstreamBalanceServiceImpl) buildSettlementReport(
	group *models.Group,
	target time.Time,
	items []settlementItem,
	total float64,
	balance *UpstreamBalanceResult,
	errors []string,
) string {
	builder := &strings.Builder{}
	builder.WriteString(fmt.Sprintf("📊 日结 - %s\n", target.Format("2006-01-02")))
	builder.WriteString(fmt.Sprintf("群组：%s\n\n", group.Title))

	if len(items) == 0 {
		builder.WriteString("未获取到任何接口的账单数据。\n")
	} else {
		for _, it := range items {
			desc := it.Description
			if desc == "" {
				desc = fmt.Sprintf("跑量：%s，费率：%s%%", formatMoney(it.Volume), formatRatePercent(it.Rate))
			}
			builder.WriteString(fmt.Sprintf("• %s (%s)\n", bindingDisplayName(it.Binding.Name), it.Binding.ID))
			if it.PZName != "" {
				builder.WriteString(fmt.Sprintf("  渠道：%s\n", it.PZName))
			}
			builder.WriteString(fmt.Sprintf("  %s\n", desc))
			if it.Deduction > 0 {
				builder.WriteString(fmt.Sprintf("  扣减：%s CNY\n", formatMoney(it.Deduction)))
			}
		}
		builder.WriteString("\n")
	}

	builder.WriteString(fmt.Sprintf("总扣减：%s CNY\n", formatMoney(total)))
	builder.WriteString(fmt.Sprintf("当前余额：%s CNY\n", formatMoney(balance.Balance)))
	builder.WriteString(fmt.Sprintf("最低余额：%s CNY\n", formatMoney(balance.MinBalance)))
	if balance.Balance < balance.MinBalance {
		builder.WriteString("⚠️ 余额低于阈值，请尽快加款。\n")
	}

	if len(errors) > 0 {
		builder.WriteString("\n⚠️ 以下接口日结失败：\n")
		for _, msg := range errors {
			builder.WriteString("• ")
			builder.WriteString(msg)
			builder.WriteString("\n")
		}
	}

	return strings.TrimSpace(builder.String())
}

func toBalanceResult(balance *models.UpstreamBalance) *UpstreamBalanceResult {
	if balance == nil {
		return nil
	}
	alertLimit := balance.AlertLimitPerHour
	if alertLimit == 0 {
		alertLimit = defaultAlertLimitPerHour
	}
	return &UpstreamBalanceResult{
		GroupID:           balance.GroupID,
		Balance:           balance.Balance,
		MinBalance:        balance.MinBalance,
		AlertLimitPerHour: alertLimit,
		UpdatedAt:         balance.UpdatedAt,
	}
}

func parseRate(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("费率为空")
	}

	if strings.HasSuffix(trimmed, "%") {
		value := strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
		rate, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("费率格式错误: %w", err)
		}
		return rate / 100, nil
	}

	rate, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("费率格式错误: %w", err)
	}
	if rate > 1 {
		return rate / 100, nil
	}
	return rate, nil
}

func parseAmount(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式错误: %w", err)
	}
	return value, nil
}

func pickPZIDItem(summary *paymentservice.SummaryByPZID, targetDate time.Time) *paymentservice.SummaryByPZIDItem {
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

func trim(s string) string {
	return strings.TrimSpace(s)
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func formatRatePercent(v float64) string {
	return fmt.Sprintf("%.2f", v*100)
}

func mustLoadChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}

func previousBillingDate(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return midnight.AddDate(0, 0, -1)
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

func bindingDisplayName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "(未命名接口)"
	}
	return clean
}
