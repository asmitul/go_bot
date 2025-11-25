package telegram

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
)

type upstreamSettlementScheduler struct {
	bot      *Bot
	cancel   context.CancelFunc
	done     chan struct{}
	location *time.Location
}

func newUpstreamSettlementScheduler(bot *Bot) *upstreamSettlementScheduler {
	return &upstreamSettlementScheduler{
		bot:      bot,
		location: mustLoadChinaLocation(),
	}
}

func (s *upstreamSettlementScheduler) start() {
	if s == nil || s.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})

	go s.run(ctx)
	logger.L().Info("Upstream settlement scheduler started")
}

func (s *upstreamSettlementScheduler) stop() {
	if s == nil || s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
	s.cancel = nil
	s.done = nil
	logger.L().Info("Upstream settlement scheduler stopped")
}

func (s *upstreamSettlementScheduler) run(ctx context.Context) {
	defer close(s.done)

	for {
		now := time.Now().In(s.location)
		next := nextDailyRun(now, s.location)
		wait := time.Until(next)
		if wait <= 0 {
			wait = time.Second
		}

		timer := time.NewTimer(wait)
		logger.L().Debugf("Upstream settlement waiting %s until %s", wait.String(), next.Format(time.RFC3339))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.dispatch(ctx)
		}
	}
}

func (s *upstreamSettlementScheduler) dispatch(parent context.Context) {
	if parent.Err() != nil {
		return
	}

	startTime := time.Now()
	targetDate := previousBillingDate(time.Now().In(s.location), s.location)
	runCtx, cancel := context.WithTimeout(parent, 3*time.Minute)
	defer cancel()

	groups, err := s.bot.groupService.ListActiveGroups(runCtx)
	if err != nil {
		logger.L().Errorf("Upstream settlement failed to list groups: %v", err)
		return
	}

	eligible := filterEligibleUpstreamGroups(groups)
	if len(eligible) == 0 {
		logger.L().Infof("Upstream settlement skipped: no eligible groups for %s", targetDate.Format("2006-01-02"))
		return
	}

	logger.L().Infof("Upstream settlement started for %d groups, target_date=%s", len(eligible), targetDate.Format("2006-01-02"))

	const workerLimit = 8
	var mu sync.Mutex
	failures := make([]string, 0)

	eg, egCtx := errgroup.WithContext(runCtx)
	eg.SetLimit(workerLimit)

	for _, group := range eligible {
		group := group
		eg.Go(func() error {
			settleCtx, cancelGroup := context.WithTimeout(egCtx, 20*time.Second)
			defer cancelGroup()

			operationID := fmt.Sprintf("auto-settle:%d:%s", group.TelegramID, targetDate.Format("2006-01-02"))
			if err := s.settleWithRetry(settleCtx, group, targetDate, operationID); err != nil {
				mu.Lock()
				failures = append(failures, fmt.Sprintf("%d(%s): %v", group.TelegramID, group.Title, err))
				mu.Unlock()
			}
			return nil
		})
	}

	_ = eg.Wait()

	duration := time.Since(startTime)
	logger.L().Infof("Upstream settlement completed for %d groups (failures=%d) duration=%s", len(eligible), len(failures), duration.Round(time.Millisecond))

	if len(failures) > 0 {
		logger.L().Warnf("Upstream settlement failures: %v", failures)
	}
}

func (s *upstreamSettlementScheduler) settleWithRetry(ctx context.Context, group *models.Group, targetDate time.Time, operationID string) error {
	const maxAttempts = 3

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		result, err := s.bot.balanceService.SettleDaily(ctx, group.TelegramID, targetDate, 0, operationID)
		if err == nil {
			if _, sendErr := s.bot.sendMessageWithMarkupAndMessage(ctx, group.TelegramID, result.Report, nil); sendErr != nil {
				logger.L().Warnf("Upstream settlement send failed: chat_id=%d err=%v", group.TelegramID, sendErr)
			} else {
				logger.L().Infof("Upstream settlement sent: chat_id=%d date=%s", group.TelegramID, targetDate.Format("2006-01-02"))
			}
			return nil
		}

		lastErr = err
		logger.L().Warnf("Upstream settlement attempt %d failed: chat_id=%d err=%v", attempt, group.TelegramID, err)

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}

		if attempt < maxAttempts {
			time.Sleep(time.Duration(attempt) * time.Second * 2)
		}
	}

	return lastErr
}

func filterEligibleUpstreamGroups(groups []*models.Group) []*models.Group {
	result := make([]*models.Group, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		if models.NormalizeGroupTier(g.Tier) != models.GroupTierUpstream {
			continue
		}
		if len(g.Settings.InterfaceBindings) == 0 {
			continue
		}
		if !g.IsActive() {
			continue
		}
		result = append(result, g)
	}
	return result
}
