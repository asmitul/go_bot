package telegram

import (
	"context"
	"fmt"
	"html"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

const (
	merchantRateMonitorWorkerLimit = 8
	merchantRateMessageLimit       = 3900
	merchantRateSendTimeout        = 10 * time.Second
)

type merchantRateChange struct {
	ChannelCode string
	ChannelName string
	OldRate     string
	NewRate     string
}

type merchantRateGroupService interface {
	ListActiveGroups(ctx context.Context) ([]*models.Group, error)
}

type merchantRatePaymentService interface {
	GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error)
}

type merchantRateMonitor struct {
	bot            *Bot
	groupService   merchantRateGroupService
	paymentService merchantRatePaymentService
	snapshotRepo   repository.MerchantRateSnapshotRepository
	interval       time.Duration
	now            func() time.Time
	send           func(context.Context, int64, string) error

	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

func newMerchantRateMonitor(
	bot *Bot,
	groupSvc merchantRateGroupService,
	paymentSvc merchantRatePaymentService,
	snapshotRepo repository.MerchantRateSnapshotRepository,
	interval time.Duration,
) *merchantRateMonitor {
	return &merchantRateMonitor{
		bot:            bot,
		groupService:   groupSvc,
		paymentService: paymentSvc,
		snapshotRepo:   snapshotRepo,
		interval:       interval,
		now: func() time.Time {
			return time.Now().In(mustLoadChinaLocation())
		},
	}
}

func (m *merchantRateMonitor) start() {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.done = make(chan struct{})
	go m.run(ctx)
	logger.L().Infof("Merchant rate monitor started: interval=%s", m.monitorInterval())
}

func (m *merchantRateMonitor) stop() {
	if m == nil {
		return
	}

	m.mu.Lock()
	if m.cancel == nil {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	done := m.done
	m.mu.Unlock()

	cancel()
	<-done

	m.mu.Lock()
	m.cancel = nil
	m.done = nil
	m.mu.Unlock()
	logger.L().Info("Merchant rate monitor stopped")
}

func (m *merchantRateMonitor) run(ctx context.Context) {
	defer close(m.done)

	for {
		m.scan(ctx)
		if ctx.Err() != nil {
			return
		}

		timer := time.NewTimer(m.monitorInterval())
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *merchantRateMonitor) monitorInterval() time.Duration {
	if m.interval <= 0 {
		return time.Minute
	}
	return m.interval
}

func (m *merchantRateMonitor) scan(ctx context.Context) {
	if m == nil || ctx.Err() != nil || m.groupService == nil || m.paymentService == nil || m.snapshotRepo == nil {
		return
	}

	groups, err := m.groupService.ListActiveGroups(ctx)
	if err != nil {
		logger.L().Warnf("Merchant rate monitor failed to list groups: %v", err)
		return
	}

	groupsByMerchant := make(map[int64][]*models.Group)
	for _, group := range groups {
		if !isEligibleMerchantGroup(group) {
			continue
		}
		merchantID := int64(group.Settings.MerchantID)
		groupsByMerchant[merchantID] = append(groupsByMerchant[merchantID], group)
	}
	if len(groupsByMerchant) == 0 {
		return
	}

	groupRunner, scanCtx := errgroup.WithContext(ctx)
	groupRunner.SetLimit(merchantRateMonitorWorkerLimit)
	for merchantID, merchantGroups := range groupsByMerchant {
		merchantID := merchantID
		merchantGroups := merchantGroups
		groupRunner.Go(func() error {
			statuses, fetchErr := m.paymentService.GetChannelStatus(scanCtx, merchantID)
			if fetchErr != nil {
				logger.L().Warnf("Merchant rate query failed: merchant_id=%d err=%v", merchantID, fetchErr)
				return nil
			}
			if len(statuses) == 0 {
				logger.L().Warnf("Merchant rate query returned no channels: merchant_id=%d", merchantID)
				return nil
			}

			rates := buildMerchantRateSnapshot(statuses)
			checkedAt := m.now()
			for _, group := range merchantGroups {
				if scanCtx.Err() != nil {
					return scanCtx.Err()
				}
				m.processGroup(scanCtx, group, merchantID, rates, checkedAt)
			}
			return nil
		})
	}

	if waitErr := groupRunner.Wait(); waitErr != nil && ctx.Err() == nil {
		logger.L().Warnf("Merchant rate monitor scan interrupted: %v", waitErr)
	}
}

func (m *merchantRateMonitor) processGroup(
	ctx context.Context,
	group *models.Group,
	merchantID int64,
	rates []models.MerchantChannelRate,
	checkedAt time.Time,
) {
	if group == nil {
		return
	}

	snapshot, err := m.snapshotRepo.GetByChatID(ctx, group.TelegramID)
	if err != nil {
		logger.L().Warnf("Merchant rate snapshot load failed: chat_id=%d merchant_id=%d err=%v",
			group.TelegramID, merchantID, err)
		return
	}

	next := &models.MerchantRateSnapshot{
		ChatID:        group.TelegramID,
		MerchantID:    merchantID,
		Rates:         rates,
		LastCheckedAt: checkedAt,
	}
	if snapshot == nil || snapshot.MerchantID != merchantID {
		if err := m.snapshotRepo.Upsert(ctx, next); err != nil {
			logger.L().Warnf("Merchant rate baseline save failed: chat_id=%d merchant_id=%d err=%v",
				group.TelegramID, merchantID, err)
		}
		return
	}

	changes := compareMerchantRates(snapshot.Rates, rates)
	if len(changes) == 0 {
		if err := m.snapshotRepo.Upsert(ctx, next); err != nil {
			logger.L().Warnf("Merchant rate snapshot refresh failed: chat_id=%d merchant_id=%d err=%v",
				group.TelegramID, merchantID, err)
		}
		return
	}

	messages := buildMerchantRateChangeMessages(changes)
	for _, message := range messages {
		sendCtx, cancel := context.WithTimeout(ctx, merchantRateSendTimeout)
		err := m.sendMessage(sendCtx, group.TelegramID, message)
		cancel()
		if err != nil {
			logger.L().Warnf("Merchant rate notification failed: chat_id=%d merchant_id=%d err=%v",
				group.TelegramID, merchantID, err)
			return
		}
	}

	if err := m.snapshotRepo.Upsert(ctx, next); err != nil {
		logger.L().Warnf("Merchant rate snapshot advance failed: chat_id=%d merchant_id=%d err=%v",
			group.TelegramID, merchantID, err)
		return
	}
	logger.L().Infof("Merchant rate change notified: chat_id=%d merchant_id=%d changes=%d",
		group.TelegramID, merchantID, len(changes))
}

func (m *merchantRateMonitor) sendMessage(ctx context.Context, chatID int64, message string) error {
	if m.send != nil {
		return m.send(ctx, chatID, message)
	}
	if m.bot == nil {
		return fmt.Errorf("telegram bot is unavailable")
	}
	_, err := m.bot.sendMessageWithMarkupAndMessage(ctx, chatID, message, nil)
	return err
}

func buildMerchantRateSnapshot(statuses []*paymentservice.ChannelStatus) []models.MerchantChannelRate {
	rates := make([]models.MerchantChannelRate, 0, len(statuses))
	for _, status := range statuses {
		if status == nil {
			continue
		}
		code := strings.TrimSpace(status.ChannelCode)
		if code == "" || strings.HasSuffix(strings.ToLower(code), "test") {
			continue
		}
		rates = append(rates, models.MerchantChannelRate{
			ChannelCode: code,
			ChannelName: strings.TrimSpace(status.ChannelName),
			Rate:        strings.TrimSpace(status.Rate),
		})
	}
	return models.NormalizeMerchantChannelRates(rates)
}

func compareMerchantRates(
	previous []models.MerchantChannelRate,
	current []models.MerchantChannelRate,
) []merchantRateChange {
	previousByCode := make(map[string]models.MerchantChannelRate, len(previous))
	for _, item := range models.NormalizeMerchantChannelRates(previous) {
		previousByCode[strings.ToLower(item.ChannelCode)] = item
	}

	changes := make([]merchantRateChange, 0)
	for _, item := range models.NormalizeMerchantChannelRates(current) {
		old, exists := previousByCode[strings.ToLower(item.ChannelCode)]
		if !exists {
			continue
		}
		oldCanonical, oldDisplay := normalizeMerchantRate(old.Rate)
		newCanonical, newDisplay := normalizeMerchantRate(item.Rate)
		if oldCanonical == newCanonical {
			continue
		}

		name := item.ChannelName
		if name == "" {
			name = old.ChannelName
		}
		changes = append(changes, merchantRateChange{
			ChannelCode: item.ChannelCode,
			ChannelName: name,
			OldRate:     oldDisplay,
			NewRate:     newDisplay,
		})
	}

	slices.SortFunc(changes, func(a, b merchantRateChange) int {
		return strings.Compare(strings.ToLower(a.ChannelCode), strings.ToLower(b.ChannelCode))
	})
	return changes
}

func normalizeMerchantRate(raw string) (canonical string, display string) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.ReplaceAll(trimmed, "％", "%")
	trimmed = strings.ReplaceAll(trimmed, ",", "")
	if trimmed == "" {
		return "text:-", "-"
	}

	hasPercent := strings.HasSuffix(trimmed, "%")
	numeric := strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
	rate := new(big.Rat)
	if _, ok := rate.SetString(numeric); ok {
		if !hasPercent && rate.Cmp(big.NewRat(1, 1)) <= 0 {
			rate.Mul(rate, big.NewRat(100, 1))
		}
		return "number:" + rate.RatString(), formatRatePercentRat(rate)
	}

	normalized := strings.Join(strings.Fields(trimmed), " ")
	return "text:" + strings.ToLower(normalized), normalized
}

func formatRatePercentRat(rate *big.Rat) string {
	value := strings.TrimRight(strings.TrimRight(rate.FloatString(18), "0"), ".")
	if value == "" || value == "-0" {
		value = "0"
	}
	return value + "%"
}

func buildMerchantRateChangeMessages(changes []merchantRateChange) []string {
	if len(changes) == 0 {
		return nil
	}

	const header = "⚠️<b>商户费率变更</b>\n"

	messages := make([]string, 0, 1)
	var builder strings.Builder
	builder.WriteString(header)
	for _, change := range changes {
		code := html.EscapeString(truncateMerchantRateField(change.ChannelCode, 64))
		name := html.EscapeString(truncateMerchantRateField(change.ChannelName, 80))
		oldRate := html.EscapeString(truncateMerchantRateField(change.OldRate, 40))
		newRate := html.EscapeString(truncateMerchantRateField(change.NewRate, 40))

		label := fmt.Sprintf("<code>%s</code>", code)
		if name != "" {
			label += " " + name
		}
		line := fmt.Sprintf("%s ： <b>%s</b> → <b>%s</b>\n", label, oldRate, newRate)
		if builder.Len()+len(line) > merchantRateMessageLimit && builder.Len() > len(header) {
			messages = append(messages, strings.TrimRight(builder.String(), "\n"))
			builder.Reset()
			builder.WriteString(header)
		}
		builder.WriteString(line)
	}
	if builder.Len() > len(header) {
		messages = append(messages, strings.TrimRight(builder.String(), "\n"))
	}
	return messages
}

func truncateMerchantRateField(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit-1]) + "…"
}
