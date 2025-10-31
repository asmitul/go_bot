package sifang

import (
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
)

func TestParseSummaryDate_DefaultsToToday(t *testing.T) {
	loc := mustLoadChinaLocation()
	now := time.Date(2024, 10, 27, 15, 30, 0, 0, loc)
	got, err := parseSummaryDate("", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2024, 10, 27, 0, 0, 0, 0, loc)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseSummaryDate_MonthDayCurrentYear(t *testing.T) {
	loc := mustLoadChinaLocation()
	now := time.Date(2024, 11, 5, 10, 0, 0, 0, loc)
	got, err := parseSummaryDate("10月26", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2024, 10, 26, 0, 0, 0, 0, loc)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseSummaryDate_MonthDayPreviousYearWhenFuture(t *testing.T) {
	loc := mustLoadChinaLocation()
	now := time.Date(2024, 1, 2, 9, 0, 0, 0, loc)
	got, err := parseSummaryDate("12月31", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2023, 12, 31, 0, 0, 0, 0, loc)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseSummaryDate_InvalidFormat(t *testing.T) {
	if _, err := parseSummaryDate("abc", time.Now()); err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestParseSummaryDate_InvalidDate(t *testing.T) {
	if _, err := parseSummaryDate("2023-02-29", time.Now()); err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestFormatSummaryMessage(t *testing.T) {
	summary := &paymentservice.SummaryByDay{
		Date:           "2025-10-31",
		TotalAmount:    "4650.00",
		MerchantIncome: "4,231.50",
		AgentIncome:    "105.25",
		OrderCount:     "40",
	}

	got := formatSummaryMessage(summary)
	expected := "📑 账单 - 2025-10-31\n跑量：4650.00\n成交：4336.75\n笔数：40"
	if got != expected {
		t.Fatalf("unexpected message:\n%s", got)
	}
}
