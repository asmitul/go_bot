package telegram

import (
	"context"
	"errors"
	"testing"

	"go_bot/internal/telegram/models"
)

func TestUpstreamBalanceMonitorEvaluateAndAlertLowBalanceNoPanic(t *testing.T) {
	monitor := &upstreamBalanceMonitor{
		states: make(map[int64]*balanceAlertState),
		alertSender: func(ctx context.Context, group *models.Group, balance, minBalance float64) error {
			return nil
		},
	}

	group := &models.Group{TelegramID: 1001}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("evaluateAndAlert should not panic, recovered: %v", r)
		}
	}()

	monitor.evaluateAndAlert(context.Background(), group, 99, 100, 1, false)

	state := monitor.states[group.TelegramID]
	if state == nil {
		t.Fatalf("expected state to be initialized")
	}
	if !state.low {
		t.Fatalf("expected low flag to be true")
	}
	if state.sentInWindow != 1 {
		t.Fatalf("expected sentInWindow=1, got %d", state.sentInWindow)
	}
}

func TestUpstreamBalanceMonitorEvaluateAndAlertRollbackOnSendFailure(t *testing.T) {
	monitor := &upstreamBalanceMonitor{
		states: make(map[int64]*balanceAlertState),
		alertSender: func(ctx context.Context, group *models.Group, balance, minBalance float64) error {
			return errors.New("send failed")
		},
	}

	group := &models.Group{TelegramID: 1002}

	monitor.evaluateAndAlert(context.Background(), group, 10, 100, 1, false)

	state := monitor.states[group.TelegramID]
	if state == nil {
		t.Fatalf("expected state to be initialized")
	}
	if state.sentInWindow != 0 {
		t.Fatalf("expected sentInWindow rollback to 0, got %d", state.sentInWindow)
	}
}

func TestUpstreamBalanceMonitorEvaluateAndAlertEnforceInterval(t *testing.T) {
	alertCount := 0
	monitor := &upstreamBalanceMonitor{
		states: make(map[int64]*balanceAlertState),
		alertSender: func(ctx context.Context, group *models.Group, balance, minBalance float64) error {
			alertCount++
			return nil
		},
	}

	group := &models.Group{
		TelegramID: 1003,
		Settings: models.GroupSettings{
			BalanceMonitorInterval: 10,
		},
	}

	monitor.evaluateAndAlert(context.Background(), group, 10, 100, 5, true)
	monitor.evaluateAndAlert(context.Background(), group, 10, 100, 5, true)

	if alertCount != 1 {
		t.Fatalf("expected 1 alert due to interval gate, got %d", alertCount)
	}
}
