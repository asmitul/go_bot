package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
)

func TestMerchantRateMonitorScanBuildsBaselineAndNotifiesChanges(t *testing.T) {
	groups := &merchantRateGroupStub{groups: []*models.Group{
		merchantRateTestGroup(-1001, 2024164),
		merchantRateTestGroup(-1002, 2024164),
		{
			TelegramID: -1003,
			Tier:       models.GroupTierMerchant,
			BotStatus:  models.BotStatusLeft,
			Settings: models.GroupSettings{
				MerchantID: 2024164, SifangEnabled: true,
			},
		},
	}}
	payment := &merchantRatePaymentStub{statuses: map[int64][]*paymentservice.ChannelStatus{
		2024164: {
			{ChannelCode: "alpha", ChannelName: "Alpha & Pay", Rate: "0.07", MerchantEnabled: false},
			{ChannelCode: "beta", ChannelName: "Beta", Rate: "8"},
			{ChannelCode: "ignoreTest", ChannelName: "Test", Rate: "99"},
		},
	}}
	repo := newMerchantRateSnapshotStub()
	var sentMu sync.Mutex
	sent := make([]string, 0)
	monitor := newMerchantRateMonitor(nil, groups, payment, repo, time.Minute)
	monitor.now = func() time.Time {
		return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	}
	monitor.send = func(_ context.Context, _ int64, message string) error {
		sentMu.Lock()
		defer sentMu.Unlock()
		sent = append(sent, message)
		return nil
	}

	monitor.scan(context.Background())
	if len(sent) != 0 {
		t.Fatalf("first scan should be silent, got %d messages", len(sent))
	}
	if payment.callCount(2024164) != 1 {
		t.Fatalf("same merchant should be queried once, got %d", payment.callCount(2024164))
	}
	for _, chatID := range []int64{-1001, -1002} {
		snapshot := repo.snapshot(chatID)
		if snapshot == nil || len(snapshot.Rates) != 2 {
			t.Fatalf("unexpected baseline for chat %d: %#v", chatID, snapshot)
		}
	}

	payment.setStatuses(2024164, []*paymentservice.ChannelStatus{
		{ChannelCode: "alpha", ChannelName: "Alpha & Pay", Rate: "7%"},
		{ChannelCode: "beta", ChannelName: "Beta", Rate: "8.5"},
		{ChannelCode: "ignoreTest", ChannelName: "Test", Rate: "100"},
	})
	monitor.scan(context.Background())

	if len(sent) != 2 {
		t.Fatalf("expected one message for each matching group, got %d", len(sent))
	}
	for _, message := range sent {
		if !strings.Contains(message, "<code>beta</code>") || !strings.Contains(message, "8%</b> → <b>8.5%") {
			t.Fatalf("unexpected notification: %s", message)
		}
		if strings.Contains(message, "alpha") || strings.Contains(message, "ignoreTest") {
			t.Fatalf("equivalent or test rate should not be reported: %s", message)
		}
	}

	monitor.scan(context.Background())
	if len(sent) != 2 {
		t.Fatalf("unchanged rate should not be notified again, got %d messages", len(sent))
	}
}

func TestMerchantRateMonitorIgnoresAddedRemovedAndSwitchChanges(t *testing.T) {
	groups := &merchantRateGroupStub{groups: []*models.Group{merchantRateTestGroup(-1001, 2024164)}}
	payment := &merchantRatePaymentStub{statuses: map[int64][]*paymentservice.ChannelStatus{
		2024164: {
			{ChannelCode: "alpha", Rate: "7", SystemEnabled: false, MerchantEnabled: false},
			{ChannelCode: "charlie", Rate: "9", SystemEnabled: true, MerchantEnabled: true},
		},
	}}
	repo := newMerchantRateSnapshotStub()
	repo.snapshots[-1001] = &models.MerchantRateSnapshot{
		ChatID: -1001, MerchantID: 2024164,
		Rates: []models.MerchantChannelRate{
			{ChannelCode: "alpha", Rate: "7"},
			{ChannelCode: "beta", Rate: "8"},
		},
	}
	sendCalls := 0
	monitor := newMerchantRateMonitor(nil, groups, payment, repo, time.Minute)
	monitor.send = func(context.Context, int64, string) error {
		sendCalls++
		return nil
	}

	monitor.scan(context.Background())
	if sendCalls != 0 {
		t.Fatalf("channel additions, removals and switches should be silent, got %d sends", sendCalls)
	}
	snapshot := repo.snapshot(-1001)
	if snapshot == nil || len(snapshot.Rates) != 2 || snapshot.Rates[1].ChannelCode != "charlie" {
		t.Fatalf("expected current channels to replace baseline, got %#v", snapshot)
	}
}

func TestMerchantRateMonitorSendFailureDoesNotAdvanceSnapshot(t *testing.T) {
	groups := &merchantRateGroupStub{groups: []*models.Group{merchantRateTestGroup(-1001, 2024164)}}
	payment := &merchantRatePaymentStub{statuses: map[int64][]*paymentservice.ChannelStatus{
		2024164: {{ChannelCode: "alpha", Rate: "8"}},
	}}
	repo := newMerchantRateSnapshotStub()
	repo.snapshots[-1001] = &models.MerchantRateSnapshot{
		ChatID: -1001, MerchantID: 2024164,
		Rates: []models.MerchantChannelRate{{ChannelCode: "alpha", Rate: "7"}},
	}
	monitor := newMerchantRateMonitor(nil, groups, payment, repo, time.Minute)
	monitor.send = func(context.Context, int64, string) error { return errors.New("telegram unavailable") }

	monitor.scan(context.Background())
	snapshot := repo.snapshot(-1001)
	if snapshot == nil || snapshot.Rates[0].Rate != "7" {
		t.Fatalf("failed notification must retain old baseline, got %#v", snapshot)
	}
}

func TestMerchantRateMonitorRebindsSilentlyAndSkipsFetchFailures(t *testing.T) {
	groups := &merchantRateGroupStub{groups: []*models.Group{merchantRateTestGroup(-1001, 2024164)}}
	payment := &merchantRatePaymentStub{
		statuses: map[int64][]*paymentservice.ChannelStatus{
			2024164: {{ChannelCode: "alpha", Rate: "8"}},
		},
		err: errors.New("upstream unavailable"),
	}
	repo := newMerchantRateSnapshotStub()
	repo.snapshots[-1001] = &models.MerchantRateSnapshot{
		ChatID: -1001, MerchantID: 2023001,
		Rates: []models.MerchantChannelRate{{ChannelCode: "alpha", Rate: "7"}},
	}
	sendCalls := 0
	monitor := newMerchantRateMonitor(nil, groups, payment, repo, time.Minute)
	monitor.send = func(context.Context, int64, string) error {
		sendCalls++
		return nil
	}

	monitor.scan(context.Background())
	if snapshot := repo.snapshot(-1001); snapshot.MerchantID != 2023001 {
		t.Fatalf("fetch failure must not alter snapshot, got %#v", snapshot)
	}

	payment.mu.Lock()
	payment.err = nil
	payment.mu.Unlock()
	monitor.scan(context.Background())
	if sendCalls != 0 {
		t.Fatalf("merchant rebind should establish a silent baseline, got %d sends", sendCalls)
	}
	if snapshot := repo.snapshot(-1001); snapshot.MerchantID != 2024164 || snapshot.Rates[0].Rate != "8" {
		t.Fatalf("unexpected rebound baseline: %#v", snapshot)
	}
}

func TestMerchantRateMonitorStartStop(t *testing.T) {
	monitor := newMerchantRateMonitor(
		nil,
		&merchantRateGroupStub{},
		&merchantRatePaymentStub{},
		newMerchantRateSnapshotStub(),
		time.Hour,
	)
	monitor.start()
	monitor.start()
	monitor.stop()
	monitor.stop()

	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	if monitor.cancel != nil || monitor.done != nil {
		t.Fatal("expected monitor lifecycle state to be cleared")
	}
}

func TestNormalizeMerchantRate(t *testing.T) {
	tests := []struct {
		left  string
		right string
		equal bool
		want  string
	}{
		{left: "0.07", right: "7%", equal: true, want: "7%"},
		{left: "7", right: "7.0％", equal: true, want: "7%"},
		{left: "1", right: "100%", equal: true, want: "100%"},
		{left: "-", right: "", equal: true, want: "-"},
		{left: "7", right: "8", equal: false, want: "7%"},
	}

	for _, tc := range tests {
		leftCanonical, leftDisplay := normalizeMerchantRate(tc.left)
		rightCanonical, _ := normalizeMerchantRate(tc.right)
		if (leftCanonical == rightCanonical) != tc.equal {
			t.Fatalf("unexpected comparison for %q and %q", tc.left, tc.right)
		}
		if leftDisplay != tc.want {
			t.Fatalf("expected display %q for %q, got %q", tc.want, tc.left, leftDisplay)
		}
	}
}

func TestBuildMerchantRateChangeMessagesEscapesAndChunks(t *testing.T) {
	changes := make([]merchantRateChange, 0, 80)
	for i := 0; i < 80; i++ {
		changes = append(changes, merchantRateChange{
			ChannelCode: "a<b>",
			ChannelName: strings.Repeat("通道&", 20),
			OldRate:     "7%",
			NewRate:     "8%",
		})
	}
	messages := buildMerchantRateChangeMessages(2024164, changes, time.Unix(0, 0).UTC())
	if len(messages) < 2 {
		t.Fatalf("expected long notification to be chunked, got %d chunks", len(messages))
	}
	for _, message := range messages {
		if len(message) > merchantRateMessageLimit {
			t.Fatalf("message exceeds limit: %d", len(message))
		}
		if strings.Contains(message, "a<b>") || !strings.Contains(message, "a&lt;b&gt;") {
			t.Fatalf("message was not escaped: %s", message)
		}
	}
}

func merchantRateTestGroup(chatID int64, merchantID int32) *models.Group {
	return &models.Group{
		TelegramID: chatID,
		Tier:       models.GroupTierMerchant,
		BotStatus:  models.BotStatusActive,
		Settings: models.GroupSettings{
			MerchantID: merchantID, SifangEnabled: true,
		},
	}
}

type merchantRateGroupStub struct {
	groups []*models.Group
	err    error
}

func (s *merchantRateGroupStub) ListActiveGroups(context.Context) ([]*models.Group, error) {
	return s.groups, s.err
}

type merchantRatePaymentStub struct {
	mu       sync.Mutex
	statuses map[int64][]*paymentservice.ChannelStatus
	calls    map[int64]int
	err      error
}

func (s *merchantRatePaymentStub) GetChannelStatus(
	_ context.Context,
	merchantID int64,
) ([]*paymentservice.ChannelStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calls == nil {
		s.calls = make(map[int64]int)
	}
	s.calls[merchantID]++
	return s.statuses[merchantID], s.err
}

func (s *merchantRatePaymentStub) callCount(merchantID int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[merchantID]
}

func (s *merchantRatePaymentStub) setStatuses(
	merchantID int64,
	statuses []*paymentservice.ChannelStatus,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[merchantID] = statuses
}

type merchantRateSnapshotStub struct {
	mu        sync.Mutex
	snapshots map[int64]*models.MerchantRateSnapshot
	getErr    error
	upsertErr error
}

func newMerchantRateSnapshotStub() *merchantRateSnapshotStub {
	return &merchantRateSnapshotStub{snapshots: make(map[int64]*models.MerchantRateSnapshot)}
}

func (s *merchantRateSnapshotStub) GetByChatID(
	_ context.Context,
	chatID int64,
) (*models.MerchantRateSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	return cloneMerchantRateSnapshot(s.snapshots[chatID]), nil
}

func (s *merchantRateSnapshotStub) Upsert(
	_ context.Context,
	snapshot *models.MerchantRateSnapshot,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.snapshots[snapshot.ChatID] = cloneMerchantRateSnapshot(snapshot)
	return nil
}

func (s *merchantRateSnapshotStub) EnsureIndexes(context.Context) error { return nil }

func (s *merchantRateSnapshotStub) snapshot(chatID int64) *models.MerchantRateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneMerchantRateSnapshot(s.snapshots[chatID])
}

func cloneMerchantRateSnapshot(snapshot *models.MerchantRateSnapshot) *models.MerchantRateSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	clone.Rates = append([]models.MerchantChannelRate(nil), snapshot.Rates...)
	return &clone
}
