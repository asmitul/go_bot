package telegram

import (
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
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    123,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 2,
			BotStatus:  models.BotStatusLeft,
			Settings: models.GroupSettings{
				MerchantID:    456,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 3,
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    0,
				SifangEnabled: true,
			},
		},
		{
			TelegramID: 4,
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:    789,
				SifangEnabled: false,
			},
		},
	}

	eligible := filterEligibleMerchantGroups(groups)
	if len(eligible) != 1 {
		t.Fatalf("expected 1 eligible group, got %d", len(eligible))
	}

	if eligible[0].TelegramID != 1 {
		t.Fatalf("expected group 1 to be eligible, got %d", eligible[0].TelegramID)
	}
}
