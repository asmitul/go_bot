package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"
)

type balanceAlertState struct {
	low          bool
	windowStart  time.Time
	sentInWindow int
	lastScan     time.Time
}

const monitorDefaultAlertLimit = 3

type upstreamBalanceMonitor struct {
	bot            *Bot
	balanceService service.UpstreamBalanceService
	groupService   service.GroupService
	alertSender    func(ctx context.Context, group *models.Group, balance, minBalance float64) error
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	statesMu       sync.Mutex
	states         map[int64]*balanceAlertState
	interval       time.Duration
}

func newUpstreamBalanceMonitor(bot *Bot, balanceSvc service.UpstreamBalanceService, groupSvc service.GroupService) *upstreamBalanceMonitor {
	return &upstreamBalanceMonitor{
		bot:            bot,
		balanceService: balanceSvc,
		groupService:   groupSvc,
		states:         make(map[int64]*balanceAlertState),
		interval:       10 * time.Minute, // base ticker; per-group间隔在评估时控制
	}
}

func (m *upstreamBalanceMonitor) start() {
	if m == nil || m.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	m.wg.Add(2)
	go func() {
		defer m.wg.Done()
		m.consumeEvents(ctx)
	}()

	go func() {
		defer m.wg.Done()
		m.runPeriodic(ctx)
	}()

	logger.L().Info("Upstream balance monitor started")
}

func (m *upstreamBalanceMonitor) stop() {
	if m == nil || m.cancel == nil {
		return
	}
	m.cancel()
	m.wg.Wait()
	m.cancel = nil
	logger.L().Info("Upstream balance monitor stopped")
}

func (m *upstreamBalanceMonitor) consumeEvents(ctx context.Context) {
	events := m.balanceService.SubscribeEvents()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			if ev == nil {
				continue
			}
			group, err := m.groupService.GetGroupInfo(ctx, ev.GroupID)
			if err != nil {
				logger.L().Warnf("Balance monitor failed to load group %d: %v", ev.GroupID, err)
				continue
			}
			m.evaluateAndAlert(ctx, group, ev.Balance, ev.MinBalance, ev.AlertLimitPerHour, false)
		}
	}
}

func (m *upstreamBalanceMonitor) runPeriodic(ctx context.Context) {
	interval := m.interval
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scanBalances(ctx)
		}
	}
}

func (m *upstreamBalanceMonitor) scanBalances(ctx context.Context) {
	groups, err := m.groupService.ListActiveGroups(ctx)
	if err != nil {
		logger.L().Warnf("Balance monitor list groups failed: %v", err)
		return
	}

	eligible := make(map[int64]*models.Group, len(groups))
	for _, g := range groups {
		if models.NormalizeGroupTier(g.Tier) != models.GroupTierUpstream {
			continue
		}
		if len(g.Settings.InterfaceBindings) == 0 {
			continue
		}
		if !models.IsBalanceMonitorEnabled(g.Settings) {
			continue
		}
		eligible[g.TelegramID] = g
	}

	if len(eligible) == 0 {
		return
	}

	results, err := m.balanceService.ListAll(ctx)
	if err != nil {
		logger.L().Warnf("Balance monitor list balances failed: %v", err)
		return
	}

	for _, res := range results {
		group := eligible[res.GroupID]
		if group == nil {
			continue
		}
		m.evaluateAndAlert(ctx, group, res.Balance, res.MinBalance, res.AlertLimitPerHour, true)
	}
}

func (m *upstreamBalanceMonitor) evaluateAndAlert(ctx context.Context, group *models.Group, balance, minBalance float64, limit int, enforceInterval bool) {
	if group == nil {
		return
	}
	if group.Settings.BalanceMonitorConfigured && !group.Settings.BalanceMonitorEnabled {
		return
	}
	m.statesMu.Lock()
	state, ok := m.states[group.TelegramID]
	if !ok {
		state = &balanceAlertState{}
		m.states[group.TelegramID] = state
	}
	now := time.Now()

	if state.windowStart.IsZero() || now.Sub(state.windowStart) >= time.Hour {
		state.windowStart = now
		state.sentInWindow = 0
	}

	if enforceInterval {
		interval := models.BalanceMonitorIntervalMinutes(group.Settings)
		if interval <= 0 {
			interval = 5 * time.Minute
		}
		if !state.lastScan.IsZero() && now.Sub(state.lastScan) < interval {
			m.statesMu.Unlock()
			return
		}
		state.lastScan = now
	}

	isLow := balance < minBalance
	if !isLow {
		state.low = false
		m.statesMu.Unlock()
		return
	}

	if limit <= 0 {
		limit = monitorDefaultAlertLimit
	}

	if state.sentInWindow >= limit {
		m.statesMu.Unlock()
		return
	}

	state.low = true
	state.sentInWindow++
	m.statesMu.Unlock()

	sendAlert := m.sendAlert
	if m.alertSender != nil {
		sendAlert = m.alertSender
	}

	if err := sendAlert(ctx, group, balance, minBalance); err != nil {
		logger.L().Warnf("Balance alert failed: chat_id=%d err=%v", group.TelegramID, err)
		m.statesMu.Lock()
		state.sentInWindow--
		m.statesMu.Unlock()
		return
	}
}

func (m *upstreamBalanceMonitor) sendAlert(ctx context.Context, group *models.Group, balance, minBalance float64) error {
	alertCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	text := fmt.Sprintf(
		"⚠️ 上游余额不足\n当前余额：%s CNY\n最低余额：%s CNY\n建议立即加款，例如发送「+1000」或调整阈值：/set_min_balance 金额",
		formatAmount(balance),
		formatAmount(minBalance),
	)

	_, err := m.bot.sendMessageWithMarkupAndMessage(alertCtx, group.TelegramID, text, nil)
	return err
}

func formatAmount(value float64) string {
	return fmt.Sprintf("%.2f", value)
}
