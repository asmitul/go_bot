package telegram

import (
	"context"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
)

type dailySummaryScheduler struct {
	bot      *Bot
	cancel   context.CancelFunc
	done     chan struct{}
	location *time.Location
}

func newDailySummaryScheduler(bot *Bot) *dailySummaryScheduler {
	return &dailySummaryScheduler{
		bot:      bot,
		location: mustLoadChinaLocation(),
	}
}

func (s *dailySummaryScheduler) start() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})

	go s.run(ctx)
	logger.L().Info("Daily bill push scheduler started")
}

func (s *dailySummaryScheduler) stop() {
	if s == nil {
		return
	}
	if s.cancel == nil {
		return
	}

	s.cancel()
	<-s.done
	s.cancel = nil
	s.done = nil
	logger.L().Info("Daily bill push scheduler stopped")
}

func (s *dailySummaryScheduler) run(ctx context.Context) {
	defer close(s.done)

	for {
		now := time.Now().In(s.location)
		next := nextDailyRun(now, s.location)
		wait := time.Until(next)
		if wait <= 0 {
			wait = time.Second
		}

		timer := time.NewTimer(wait)
		logger.L().Debugf("Daily bill push waiting %s until %s", wait.String(), next.Format(time.RFC3339))

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.dispatch(ctx)
		}
	}
}

func (s *dailySummaryScheduler) dispatch(parent context.Context) {
	if parent.Err() != nil {
		return
	}

	now := time.Now().In(s.location)
	targetDate := previousBillingDate(now, s.location)

	runCtx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	groups, err := s.bot.groupService.ListActiveGroups(runCtx)
	if err != nil {
		logger.L().Errorf("Daily bill push failed to list groups: %v", err)
		return
	}

	eligible := filterEligibleMerchantGroups(groups)
	if len(eligible) == 0 {
		logger.L().Infof("Daily bill push skipped: no eligible groups for %s", targetDate.Format("2006-01-02"))
		return
	}

	logger.L().Infof("Daily bill push started for %d groups, target_date=%s", len(eligible), targetDate.Format("2006-01-02"))

	for _, group := range eligible {
		if runCtx.Err() != nil {
			logger.L().Warn("Daily bill push aborted: context canceled")
			return
		}

		merchantID := int64(group.Settings.MerchantID)
		groupCtx, cancelGroup := context.WithTimeout(runCtx, 15*time.Second)

		message, err := s.bot.sifangFeature.BuildSummaryMessage(groupCtx, merchantID, targetDate)
		if err != nil {
			cancelGroup()
			logger.L().Errorf("Daily bill push failed: chat_id=%d, merchant_id=%d, err=%v", group.TelegramID, merchantID, err)
			continue
		}

		if message == "" {
			cancelGroup()
			logger.L().Warnf("Daily bill push produced empty message: chat_id=%d", group.TelegramID)
			continue
		}

		s.bot.sendMessage(groupCtx, group.TelegramID, message)
		logger.L().Infof("Daily bill push sent: chat_id=%d, merchant_id=%d, target_date=%s", group.TelegramID, merchantID, targetDate.Format("2006-01-02"))
		cancelGroup()
	}

	logger.L().Infof("Daily bill push completed for %d groups, target_date=%s", len(eligible), targetDate.Format("2006-01-02"))
}

func filterEligibleMerchantGroups(groups []*models.Group) []*models.Group {
	eligible := make([]*models.Group, 0, len(groups))
	for _, group := range groups {
		if isEligibleMerchantGroup(group) {
			eligible = append(eligible, group)
		}
	}
	return eligible
}

func isEligibleMerchantGroup(group *models.Group) bool {
	if group == nil {
		return false
	}
	if !group.IsActive() {
		return false
	}
	if group.Settings.MerchantID <= 0 {
		return false
	}
	if !group.Settings.SifangEnabled {
		return false
	}
	return true
}

func nextDailyRun(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 5, 0, location)
	if !next.After(local) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

func previousBillingDate(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return midnight.AddDate(0, 0, -1)
}

func mustLoadChinaLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return loc
}
