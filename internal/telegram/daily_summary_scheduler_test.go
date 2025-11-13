package telegram

import (
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"
)

func TestNextDailyRun(t *testing.T) {
	loc := mustLoadChinaLocation()

	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "BeforeSchedule",
			now:      time.Date(2024, 10, 1, 0, 0, 2, 0, loc),
			expected: time.Date(2024, 10, 1, 0, 0, 5, 0, loc),
		},
		{
			name:     "AfterSchedule",
			now:      time.Date(2024, 10, 1, 0, 1, 0, 0, loc),
			expected: time.Date(2024, 10, 2, 0, 0, 5, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nextDailyRun(tc.now, loc)
			if !got.Equal(tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestPreviousBillingDate(t *testing.T) {
	loc := mustLoadChinaLocation()

	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "SameDay",
			now:      time.Date(2024, 10, 2, 0, 0, 10, 0, loc),
			expected: time.Date(2024, 10, 1, 0, 0, 0, 0, loc),
		},
		{
			name:     "CrossMonth",
			now:      time.Date(2024, 3, 1, 8, 0, 0, 0, loc),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := previousBillingDate(tc.now, loc)
			if !got.Equal(tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestFilterEligibleMerchantGroups(t *testing.T) {
	loc := mustLoadChinaLocation()
	_ = loc // ensure location is initialized for consistency with other helpers

	groups := []*models.Group{
		nil,
		{
			TelegramID: 1,
			Tier:       models.GroupTierMerchant,
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    123,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 2,
			Tier:       models.GroupTierMerchant,
			BotStatus:  models.BotStatusLeft,
			Settings: models.GroupSettings{
				MerchantID:    456,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 3,
			Tier:       models.GroupTierMerchant,
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    0,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 4,
			Tier:       models.GroupTierUpstream,
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    789,
				SifangEnabled: false,
			},
		},
		{
			TelegramID: 5,
			Tier:       "",
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    321,
				SifangEnabled: true,
			},
		},
	}

	eligible := filterEligibleMerchantGroups(groups)
	if len(eligible) != 2 {
		t.Fatalf("expected 2 eligible groups, got %d", len(eligible))
	}

	expectedIDs := map[int64]struct{}{1: {}, 5: {}}
	for _, g := range eligible {
		if _, ok := expectedIDs[g.TelegramID]; !ok {
			t.Fatalf("unexpected eligible group: %d", g.TelegramID)
		}
		delete(expectedIDs, g.TelegramID)
	}
}

func TestBuildDailySummaryReport(t *testing.T) {
	targetDate := time.Date(2024, 10, 2, 0, 0, 0, 0, time.UTC)
	duration := 90*time.Second + 125*time.Millisecond
	note := "任务在完成前被取消。"
	failures := []string{
		"chat_id=1, merchant_id=2: 超时",
		"chat_id=3: 生成的消息为空",
	}

	report := buildDailySummaryReport(targetDate, 5, 3, len(failures), duration, note, failures)

	expectedLines := []string{
		"📊 每日账单推送报告",
		"日期：2024-10-02",
		"目标群组：5",
		"成功：3",
		"失败：2",
		"耗时：1m30.125s",
		note,
		"失败详情：",
		"• chat_id=1, merchant_id=2: 超时",
		"• chat_id=3: 生成的消息为空",
	}

	for _, line := range expectedLines {
		if !strings.Contains(report, line) {
			t.Fatalf("expected report to contain %q, got %q", line, report)
		}
	}
}

func TestBuildDailySummaryReportWithoutFailures(t *testing.T) {
	targetDate := time.Date(2024, 10, 3, 0, 0, 0, 0, time.UTC)
	duration := 5*time.Second + 500*time.Millisecond

	report := buildDailySummaryReport(targetDate, 2, 2, 0, duration, "", nil)

	unexpected := []string{"失败详情", "•"}
	for _, item := range unexpected {
		if strings.Contains(report, item) {
			t.Fatalf("did not expect report to contain %q, got %q", item, report)
		}
	}

	if !strings.Contains(report, "耗时：5.5s") {
		t.Fatalf("expected report duration to be rounded to milliseconds, got %q", report)
	}
}
