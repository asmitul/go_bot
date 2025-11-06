package telegram

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

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

	startTime := time.Now()
	now := time.Now().In(s.location)
	targetDate := previousBillingDate(now, s.location)

	runCtx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	groups, err := s.bot.groupService.ListActiveGroups(runCtx)
	if err != nil {
		logger.L().Errorf("Daily bill push failed to list groups: %v", err)
		duration := time.Since(startTime)
		note := fmt.Sprintf("获取群组失败: %v", err)
		s.notifyOwners(parent, targetDate, 0, 0, 0, duration, note, nil)
		return
	}

	eligible := filterEligibleMerchantGroups(groups)
	if len(eligible) == 0 {
		logger.L().Infof("Daily bill push skipped: no eligible groups for %s", targetDate.Format("2006-01-02"))
		duration := time.Since(startTime)
		note := "无符合条件的群组，已跳过推送。"
		s.notifyOwners(parent, targetDate, 0, 0, 0, duration, note, nil)
		return
	}

	logger.L().Infof("Daily bill push started for %d groups, target_date=%s", len(eligible), targetDate.Format("2006-01-02"))

	const workerLimit = 8

	successCount := 0
	failureDetails := make([]string, 0)
	aborted := false
	var mu sync.Mutex

	groupRunner, groupCtx := errgroup.WithContext(runCtx)
	groupRunner.SetLimit(workerLimit)

	for _, group := range eligible {
		group := group
		merchantID := int64(group.Settings.MerchantID)

		groupRunner.Go(func() error {
			if groupCtx.Err() != nil {
				return groupCtx.Err()
			}

			ctxWithTimeout, cancelGroup := context.WithTimeout(groupCtx, 15*time.Second)
			defer cancelGroup()

			message, err := s.bot.sifangFeature.BuildSummaryMessage(ctxWithTimeout, merchantID, targetDate)
			if err != nil {
				logger.L().Errorf("Daily bill push failed: chat_id=%d, merchant_id=%d, err=%v", group.TelegramID, merchantID, err)
				mu.Lock()
				failureDetails = append(failureDetails, fmt.Sprintf("chat_id=%d, merchant_id=%d: %v", group.TelegramID, merchantID, err))
				mu.Unlock()
				return nil
			}

			if message == "" {
				logger.L().Warnf("Daily bill push produced empty message: chat_id=%d", group.TelegramID)
				mu.Lock()
				failureDetails = append(failureDetails, fmt.Sprintf("chat_id=%d: 生成的消息为空", group.TelegramID))
				mu.Unlock()
				return nil
			}

			if _, sendErr := s.bot.sendMessageWithMarkupAndMessage(ctxWithTimeout, group.TelegramID, message, nil); sendErr != nil {
				logger.L().Errorf("Daily bill push failed to send: chat_id=%d, merchant_id=%d, err=%v", group.TelegramID, merchantID, sendErr)
				mu.Lock()
				failureDetails = append(failureDetails, fmt.Sprintf("chat_id=%d, merchant_id=%d: 发送失败 (%v)", group.TelegramID, merchantID, sendErr))
				mu.Unlock()
				return nil
			}

			logger.L().Infof("Daily bill push sent: chat_id=%d, merchant_id=%d, target_date=%s", group.TelegramID, merchantID, targetDate.Format("2006-01-02"))
			mu.Lock()
			successCount++
			mu.Unlock()

			return nil
		})
	}

	if err := groupRunner.Wait(); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			logger.L().Warn("Daily bill push aborted: context canceled")
			aborted = true
		} else {
			logger.L().Warnf("Daily bill push encountered unexpected error: %v", err)
		}
	}

	duration := time.Since(startTime)
	failureCount := len(failureDetails)
	note := ""
	if aborted {
		note = "任务在完成前被取消。"
	}

	logger.L().Infof("Daily bill push completed for %d groups (success=%d, failure=%d), target_date=%s", len(eligible), successCount, failureCount, targetDate.Format("2006-01-02"))

	s.notifyOwners(parent, targetDate, len(eligible), successCount, failureCount, duration, note, failureDetails)
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

func (s *dailySummaryScheduler) notifyOwners(parent context.Context, targetDate time.Time, total, success, failure int, duration time.Duration, note string, failureDetails []string) {
	if s == nil {
		return
	}
	if len(s.bot.ownerIDs) == 0 {
		return
	}
	if parent != nil && parent.Err() != nil {
		return
	}

	baseCtx := parent
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	notifyCtx, cancel := context.WithTimeout(baseCtx, 30*time.Second)
	defer cancel()

	report := buildDailySummaryReport(targetDate, total, success, failure, duration, note, failureDetails)

	for _, ownerID := range s.bot.ownerIDs {
		if _, err := s.bot.sendMessageWithMarkupAndMessage(notifyCtx, ownerID, report, nil); err != nil {
			logger.L().Errorf("Daily bill push failed to notify owner %d: %v", ownerID, err)
		}
	}
}

func buildDailySummaryReport(targetDate time.Time, total, success, failure int, duration time.Duration, note string, failureDetails []string) string {
	builder := &strings.Builder{}
	builder.WriteString("📊 每日账单推送报告\n")
	builder.WriteString(fmt.Sprintf("日期：%s\n", targetDate.Format("2006-01-02")))
	builder.WriteString(fmt.Sprintf("目标群组：%d\n", total))
	builder.WriteString(fmt.Sprintf("成功：%d\n", success))
	builder.WriteString(fmt.Sprintf("失败：%d\n", failure))
	builder.WriteString(fmt.Sprintf("耗时：%s\n", duration.Round(time.Millisecond)))

	if note != "" {
		builder.WriteString(note)
		builder.WriteString("\n")
	}

	if failure > 0 && len(failureDetails) > 0 {
		builder.WriteString("失败详情：\n")
		for _, detail := range failureDetails {
			builder.WriteString("• ")
			builder.WriteString(detail)
			builder.WriteString("\n")
		}
	}

	return strings.TrimRight(builder.String(), "\n")
}
