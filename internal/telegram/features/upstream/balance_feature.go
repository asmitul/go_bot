package upstream

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

var (
	adjustCommandPattern       = regexp.MustCompile(`^([+-])\s*([0-9]+(?:\.[0-9]+)?)(?:\s+(.*))?$`)
	setMinBalanceCommandPrefix = "/set_min_balance"
	setAlertLimitPrefix        = "/set_balance_alert_limit"
)

// BalanceFeature 处理上游余额相关命令
type BalanceFeature struct {
	balanceService service.UpstreamBalanceService
	userService    service.UserService
	groupService   service.GroupService
	nowFunc        func() time.Time
}

// NewBalanceFeature 创建余额功能
func NewBalanceFeature(balanceSvc service.UpstreamBalanceService, userSvc service.UserService, groupSvc service.GroupService) *BalanceFeature {
	return &BalanceFeature{
		balanceService: balanceSvc,
		userService:    userSvc,
		groupService:   groupSvc,
		nowFunc: func() time.Time {
			return time.Now().In(upstreamChinaLocation)
		},
	}
}

// Name 功能名称
func (f *BalanceFeature) Name() string {
	return "upstream_balance"
}

// AllowedGroupTiers 仅上游群可用
func (f *BalanceFeature) AllowedGroupTiers() []models.GroupTier {
	return []models.GroupTier{models.GroupTierUpstream}
}

// Enabled 需要绑定上游接口
func (f *BalanceFeature) Enabled(ctx context.Context, group *models.Group) bool {
	return len(group.Settings.InterfaceBindings) > 0
}

// Match 匹配余额相关指令
func (f *BalanceFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg == nil || msg.Text == "" {
		return false
	}
	text := strings.TrimSpace(msg.Text)
	switch {
	case strings.HasPrefix(text, "/余额"):
		return true
	case strings.HasPrefix(text, setMinBalanceCommandPrefix):
		return true
	case strings.HasPrefix(text, setAlertLimitPrefix):
		return true
	case text == "/日结":
		return true
	default:
		return adjustCommandPattern.MatchString(text)
	}
}

// Process 处理命令
func (f *BalanceFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	if msg.From == nil {
		return nil, false, nil
	}

	isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Failed to check admin permission: user_id=%d err=%v", msg.From.ID, err)
		return respond("❌ 权限检查失败"), true, nil
	}
	if !isAdmin {
		return respond("❌ 仅管理员可以操作余额"), true, nil
	}

	text := strings.TrimSpace(msg.Text)
	switch {
	case strings.HasPrefix(text, "/余额"):
		resp, handlerErr := f.handleQueryBalance(ctx, msg)
		return respond(resp), true, handlerErr
	case strings.HasPrefix(text, setMinBalanceCommandPrefix):
		resp, handlerErr := f.handleSetMinBalance(ctx, msg, text)
		return respond(resp), true, handlerErr
	case strings.HasPrefix(text, setAlertLimitPrefix):
		resp, handlerErr := f.handleSetAlertLimit(ctx, msg, text)
		return respond(resp), true, handlerErr
	case text == "/日结":
		resp, handlerErr := f.handleSettlement(ctx, msg)
		return respond(resp), true, handlerErr
	default:
		if adjustCommandPattern.MatchString(text) {
			resp, handlerErr := f.handleAdjust(ctx, msg, text)
			return respond(resp), true, handlerErr
		}
	}

	return nil, false, nil
}

// Priority 在接口绑定之后
func (f *BalanceFeature) Priority() int {
	return 17
}

func (f *BalanceFeature) handleQueryBalance(ctx context.Context, msg *botModels.Message) (string, error) {
	result, err := f.balanceService.Get(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Query balance failed: chat_id=%d err=%v", msg.Chat.ID, err)
		return "❌ 查询余额失败", nil
	}

	status := "✅ 余额正常"
	if result.Balance < result.MinBalance {
		status = "⚠️ 余额低于阈值"
	}

	return fmt.Sprintf("%s\n当前余额：%s CNY\n最低余额：%s CNY\n告警频率：每小时 %d 次",
		status,
		formatAmount(result.Balance),
		formatAmount(result.MinBalance),
		result.AlertLimitPerHour,
	), nil
}

func (f *BalanceFeature) handleSetMinBalance(ctx context.Context, msg *botModels.Message, text string) (string, error) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "❌ 用法：/set_min_balance 金额", nil
	}

	threshold, err := parseAmount(fields[1])
	if err != nil {
		return fmt.Sprintf("❌ 最低余额格式错误：%v", err), nil
	}

	result, err := f.balanceService.SetMinBalance(ctx, msg.Chat.ID, threshold, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Set min balance failed: chat_id=%d err=%v", msg.Chat.ID, err)
		return "❌ 设置失败", nil
	}

	return fmt.Sprintf("✅ 最低余额已更新为 %s CNY\n当前余额：%s CNY", formatAmount(result.MinBalance), formatAmount(result.Balance)), nil
}

func (f *BalanceFeature) handleSetAlertLimit(ctx context.Context, msg *botModels.Message, text string) (string, error) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "❌ 用法：/set_balance_alert_limit 每小时次数", nil
	}

	limitVal := strings.TrimSpace(fields[1])
	limit, err := strconv.Atoi(limitVal)
	if err != nil || limit <= 0 {
		return "❌ 请输入大于 0 的整数频率", nil
	}

	result, err := f.balanceService.SetAlertLimit(ctx, msg.Chat.ID, limit, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Set alert limit failed: chat_id=%d err=%v", msg.Chat.ID, err)
		return "❌ 设置失败", nil
	}

	return fmt.Sprintf("✅ 告警频率已更新为 每小时 %d 次\n当前余额：%s CNY", result.AlertLimitPerHour, formatAmount(result.Balance)), nil
}

func (f *BalanceFeature) handleSettlement(ctx context.Context, msg *botModels.Message) (string, error) {
	now := f.currentTime()
	target := previousBillingDate(now, upstreamChinaLocation)
	operationID := fmt.Sprintf("settle:%s", target.Format("2006-01-02"))

	result, err := f.balanceService.SettleDaily(ctx, msg.Chat.ID, target, msg.From.ID, operationID)
	if err != nil {
		logger.L().Errorf("Manual settlement failed: chat_id=%d err=%v", msg.Chat.ID, err)
		return fmt.Sprintf("❌ 日结失败：%v", err), nil
	}

	return result.Report, nil
}

func (f *BalanceFeature) handleAdjust(ctx context.Context, msg *botModels.Message, text string) (string, error) {
	matches := adjustCommandPattern.FindStringSubmatch(text)
	if len(matches) < 3 {
		return "❌ 调整格式错误", nil
	}

	sign := matches[1]
	rawAmount := matches[2]
	remark := strings.TrimSpace(matches[3])

	amount, err := parseAmount(rawAmount)
	if err != nil {
		return fmt.Sprintf("❌ 金额格式错误：%v", err), nil
	}
	if amount <= 0 {
		return "❌ 金额必须大于 0", nil
	}

	delta := amount
	action := "加款"
	if sign == "-" {
		delta = -delta
		action = "扣款"
	}

	result, below, err := f.balanceService.Adjust(ctx, msg.Chat.ID, delta, msg.From.ID, remark, "")
	if err != nil {
		logger.L().Errorf("Adjust balance failed: chat_id=%d err=%v", msg.Chat.ID, err)
		return "❌ 调整失败", nil
	}

	status := "✅ 已" + action
	if below {
		status = "⚠️ 已" + action + "（余额低于阈值）"
	}

	return fmt.Sprintf("%s：%s CNY\n当前余额：%s CNY\n最低余额：%s CNY",
		status,
		formatAmount(amount),
		formatAmount(result.Balance),
		formatAmount(result.MinBalance),
	), nil
}

func (f *BalanceFeature) currentTime() time.Time {
	if f.nowFunc != nil {
		return f.nowFunc()
	}
	return time.Now()
}

func parseAmount(raw string) (float64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("金额为空")
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, fmt.Errorf("金额格式错误: %w", err)
	}
	return value, nil
}

func formatAmount(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

func previousBillingDate(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return midnight.AddDate(0, 0, -1)
}
