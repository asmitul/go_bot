package sifang

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"

	botModels "github.com/go-telegram/bot/models"
)

func TestParseSummaryDate_DefaultsToToday(t *testing.T) {
	loc := mustLoadChinaLocation()
	now := time.Date(2024, 10, 27, 15, 30, 0, 0, loc)
	got, err := parseSummaryDate("", now, "账单")
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
	got, err := parseSummaryDate("10月26", now, "账单")
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
	got, err := parseSummaryDate("12月31", now, "账单")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2023, 12, 31, 0, 0, 0, 0, loc)
	if !got.Equal(expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestParseSummaryDate_InvalidFormat(t *testing.T) {
	if _, err := parseSummaryDate("abc", time.Now(), "账单"); err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestParseSummaryDate_InvalidDate(t *testing.T) {
	if _, err := parseSummaryDate("2023-02-29", time.Now(), "账单"); err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestParseBalanceDate_RewritesErrorMessage(t *testing.T) {
	_, err := parseBalanceDate("not-a-date", time.Now())
	if err == nil {
		t.Fatalf("expected error for invalid balance date")
	}
	if !strings.Contains(err.Error(), "余额") {
		t.Fatalf("expected error message to mention 余额, got %v", err)
	}
}

func TestCalculateHistoryDays(t *testing.T) {
	loc := mustLoadChinaLocation()
	now := time.Date(2024, 11, 5, 12, 0, 0, 0, loc)

	tests := []struct {
		name     string
		target   time.Time
		expected int
	}{
		{
			name:     "Today",
			target:   time.Date(2024, 11, 5, 0, 0, 0, 0, loc),
			expected: 0,
		},
		{
			name:     "Yesterday",
			target:   time.Date(2024, 11, 4, 0, 0, 0, 0, loc),
			expected: 1,
		},
		{
			name:     "ThreeDaysAgo",
			target:   time.Date(2024, 11, 2, 23, 0, 0, 0, loc),
			expected: 3,
		},
		{
			name:     "FutureClamped",
			target:   time.Date(2024, 11, 6, 0, 0, 0, 0, loc),
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := calculateHistoryDays(tc.target, now); got != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, got)
			}
		})
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

func TestFormatChannelSummaryMessage(t *testing.T) {
	items := []*paymentservice.SummaryByDayChannel{
		{
			ChannelCode:    "USDT",
			ChannelName:    "USDT通道",
			TotalAmount:    "5000.00",
			MerchantIncome: "4800.00",
			AgentIncome:    "100.00",
			OrderCount:     "20",
		},
		{
			ChannelCode:    "ALIPAY",
			ChannelName:    "支付宝",
			TotalAmount:    "2000",
			MerchantIncome: "1800",
			AgentIncome:    "",
			OrderCount:     "5",
		},
	}

	got := formatChannelSummaryMessage("2025-10-31", items)
	expected := "📑 通道账单 - 2025-10-31\n\nUSDT通道：<code>USDT</code>\n跑量：5000.00\n成交：4900\n笔数：20\n\n支付宝：<code>ALIPAY</code>\n跑量：2000\n成交：1800\n笔数：5"
	if got != expected {
		t.Fatalf("unexpected channel message:\n%s", got)
	}
}

func TestFormatChannelSummaryMessage_NoItems(t *testing.T) {
	got := formatChannelSummaryMessage("2025-10-31", nil)
	expected := "ℹ️ 2025-10-31 暂无通道账单数据"
	if got != expected {
		t.Fatalf("unexpected channel message for no items:\n%s", got)
	}
}

func TestFormatChannelRate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"0.1", "10%"},
		{"0.065", "6.5%"},
		{"6.5%", "6.5%"},
		{"10", "10%"},
		{"", "-"},
		{"-", "-"},
	}

	for _, tc := range tests {
		if got := formatChannelRate(tc.input); got != tc.expected {
			t.Fatalf("formatChannelRate(%q) expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}

func TestFormatChannelRatesMessage(t *testing.T) {
	items := []*paymentservice.ChannelStatus{
		{
			ChannelCode:     "cjwxhf",
			ChannelName:     "微信话费慢充",
			SystemEnabled:   true,
			MerchantEnabled: true,
			Rate:            "0.10",
		},
		{
			ChannelCode:     "tbsqhf",
			ChannelName:     "淘宝授权话费",
			SystemEnabled:   true,
			MerchantEnabled: false,
			Rate:            "",
		},
		{
			ChannelCode:     "wxhftest",
			ChannelName:     "微信测试",
			SystemEnabled:   true,
			MerchantEnabled: true,
			Rate:            "0.08",
		},
	}

	message := formatChannelRatesMessage(items)
	if !strings.Contains(message, "📡 通道费率") {
		t.Fatalf("expected header, got %s", message)
	}
	if !strings.Contains(message, "✅") || !strings.Contains(message, "❌") {
		t.Fatalf("expected status icons, got %s", message)
	}
	if !strings.Contains(message, "cjwxhf") || !strings.Contains(message, "tbsqhf") {
		t.Fatalf("expected channel codes, got %s", message)
	}
	if !strings.Contains(message, "10%") {
		t.Fatalf("expected formatted rate, got %s", message)
	}
	if !strings.Contains(message, "</pre>") {
		t.Fatalf("expected preformatted block, got %s", message)
	}
	if strings.Contains(message, "wxhftest") {
		t.Fatalf("expected test channel to be skipped, got %s", message)
	}
}

func TestMatchIgnoresNonCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "账单不对呀",
	}
	if f.Match(nil, msg) {
		t.Fatalf("expected non-command to be ignored")
	}
}

func TestMatchAcceptsChannelCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "通道账单10月26",
	}
	if !f.Match(nil, msg) {
		t.Fatalf("expected command to match")
	}
}

func TestMatchAcceptsBalanceWithDate(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "余额10月30",
	}
	if !f.Match(nil, msg) {
		t.Fatalf("expected balance command with date to match")
	}
}

func TestMatchAcceptsRateCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "费率",
	}
	if !f.Match(nil, msg) {
		t.Fatalf("expected rate command to match")
	}
}

func TestMatchAcceptsWithdrawCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "提款明细",
	}
	if !f.Match(nil, msg) {
		t.Fatalf("expected withdraw command to match")
	}
}

func TestFormatWithdrawListMessage(t *testing.T) {
	list := &paymentservice.WithdrawList{
		Items: []*paymentservice.Withdraw{
			{
				WithdrawNo: "W2025",
				OrderNo:    "O1",
				Amount:     "100.00",
				Fee:        "1.00",
				Status:     "paid",
				CreatedAt:  "2025-10-31 10:00:00",
				PaidAt:     "2025-10-31 11:00:00",
				Channel:    "ALIPAY",
			},
		},
	}

	got := formatWithdrawListMessage("2025-10-31", list)
	expected := "💸 提款明细（总计 100｜1 笔）\n10:00:00      100.00"
	if got != expected {
		t.Fatalf("unexpected withdraw message:\n%s", got)
	}

	gotEmpty := formatWithdrawListMessage("2025-10-31", &paymentservice.WithdrawList{})
	if gotEmpty != "💸 提款明细\n暂无提款记录" {
		t.Fatalf("unexpected empty withdraw message:\n%s", gotEmpty)
	}
}

func TestHandleChannelRates(t *testing.T) {
	fake := &fakePaymentService{
		channelStatusResp: []*paymentservice.ChannelStatus{
			{
				ChannelCode:     "zft",
				ChannelName:     "直付通",
				SystemEnabled:   true,
				MerchantEnabled: true,
				Rate:            "0.09",
			},
		},
	}
	feature := &Feature{paymentService: fake}

	message, handled, err := feature.handleChannelRates(context.Background(), 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled to be true")
	}
	if !strings.Contains(message, "直付通") || !strings.Contains(message, "9%") {
		t.Fatalf("unexpected message: %s", message)
	}
}

func TestHandleBalanceReturnsCurrentAmount(t *testing.T) {
	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance:        "123.45",
			HistoryBalance: "67.89",
			MerchantID:     "1001",
		},
	}
	feature := &Feature{paymentService: fake}

	amount, _, err := feature.handleBalance(context.Background(), 1001, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amount != "123.45" {
		t.Fatalf("expected current balance, got %s", amount)
	}
	if fake.lastHistoryDays != 0 {
		t.Fatalf("expected history_days 0, got %d", fake.lastHistoryDays)
	}
}

func TestHandleBalanceReturnsHistoryAmount(t *testing.T) {
	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance:        "123.45",
			HistoryBalance: "67.89",
			MerchantID:     "1001",
		},
	}
	feature := &Feature{paymentService: fake}

	amount, _, err := feature.handleBalance(context.Background(), 1001, "2000-01-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amount != "67.89" {
		t.Fatalf("expected history balance, got %s", amount)
	}
	if fake.lastHistoryDays <= 0 {
		t.Fatalf("expected history_days > 0, got %d", fake.lastHistoryDays)
	}
}

func TestHandleSummaryIncludesWithdrawAndBalance(t *testing.T) {
	now := time.Now().In(chinaLocation)
	today := now.Format("2006-01-02")
	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance:        "5000",
			HistoryBalance: "4000",
		},
		withdrawResp: &paymentservice.WithdrawList{
			Items: []*paymentservice.Withdraw{
				{Amount: "100", CreatedAt: today + " 10:00:00"},
			},
		},
	}
	feature := &Feature{paymentService: fake}

	message, _, err := feature.handleSummary(context.Background(), 1001, "账单")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(message, "📑 账单 - ") {
		t.Fatalf("expected summary header, got %s", message)
	}
	if !strings.Contains(message, "💸 提款明细（总计 ") {
		t.Fatalf("expected withdraw section, got %s", message)
	}
	if !strings.Contains(message, "余额：5000") {
		t.Fatalf("expected balance amount, got %s", message)
	}
}

func TestHandleSummaryUsesHistoryBalanceForPastDate(t *testing.T) {
	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance:        "5000",
			HistoryBalance: "4000",
		},
		withdrawResp: &paymentservice.WithdrawList{},
	}
	feature := &Feature{paymentService: fake}

	message, _, err := feature.handleSummary(context.Background(), 1001, "账单01-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(message, "余额：4000") {
		t.Fatalf("expected history balance in message, got %s", message)
	}
	if fake.lastHistoryDays <= 0 {
		t.Fatalf("expected history_days > 0, got %d", fake.lastHistoryDays)
	}
}

func TestHandleChannelSummaryIncludesWithdrawAndBalance(t *testing.T) {
	now := time.Now().In(chinaLocation)
	today := now.Format("2006-01-02")
	fake := &fakePaymentService{
		channelSummaryResp: []*paymentservice.SummaryByDayChannel{
			{
				ChannelCode:    "USDT",
				ChannelName:    "USDT通道",
				TotalAmount:    "5000",
				MerchantIncome: "4800",
				AgentIncome:    "100",
				OrderCount:     "20",
			},
		},
		balanceResp: &paymentservice.Balance{
			Balance:        "5000",
			HistoryBalance: "4000",
		},
		withdrawResp: &paymentservice.WithdrawList{
			Items: []*paymentservice.Withdraw{
				{Amount: "100", CreatedAt: today + " 08:00:00"},
			},
		},
	}
	feature := &Feature{paymentService: fake}

	message, _, err := feature.handleChannelSummary(context.Background(), 1001, "通道账单")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(message, "📑 通道账单 - ") {
		t.Fatalf("expected channel summary header, got %s", message)
	}
	if !strings.Contains(message, "💸 提款明细（总计 ") {
		t.Fatalf("expected withdraw section, got %s", message)
	}
	if !strings.Contains(message, "余额：5000") {
		t.Fatalf("expected balance amount, got %s", message)
	}
	if fake.lastHistoryDays != 0 {
		t.Fatalf("expected history_days 0, got %d", fake.lastHistoryDays)
	}
}

func TestHandleChannelSummaryUsesHistoryBalanceForPastDate(t *testing.T) {
	fake := &fakePaymentService{
		channelSummaryResp: []*paymentservice.SummaryByDayChannel{
			{
				ChannelCode:    "USDT",
				ChannelName:    "USDT通道",
				TotalAmount:    "5000",
				MerchantIncome: "4800",
				AgentIncome:    "100",
				OrderCount:     "20",
			},
		},
		balanceResp: &paymentservice.Balance{
			Balance:        "5000",
			HistoryBalance: "4000",
		},
		withdrawResp: &paymentservice.WithdrawList{},
	}
	feature := &Feature{paymentService: fake}

	message, _, err := feature.handleChannelSummary(context.Background(), 1001, "通道账单01-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(message, "余额：4000") {
		t.Fatalf("expected history balance in channel summary, got %s", message)
	}
	if fake.lastHistoryDays <= 0 {
		t.Fatalf("expected history_days > 0, got %d", fake.lastHistoryDays)
	}
}

type fakePaymentService struct {
	balanceResp        *paymentservice.Balance
	balanceErr         error
	summaryResp        *paymentservice.SummaryByDay
	summaryErr         error
	channelSummaryResp []*paymentservice.SummaryByDayChannel
	channelSummaryErr  error
	withdrawResp       *paymentservice.WithdrawList
	withdrawErr        error
	channelStatusResp  []*paymentservice.ChannelStatus
	channelStatusErr   error
	lastHistoryDays    int
}

func (f *fakePaymentService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	f.lastHistoryDays = historyDays
	if f.balanceErr != nil {
		return nil, f.balanceErr
	}
	return f.balanceResp, nil
}

func (f *fakePaymentService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	if f.summaryErr != nil {
		return nil, f.summaryErr
	}
	if f.summaryResp != nil {
		return f.summaryResp, nil
	}
	return &paymentservice.SummaryByDay{
		Date:           date.Format("2006-01-02"),
		OrderCount:     "10",
		SuccessCount:   "9",
		TotalAmount:    "1000",
		MerchantIncome: "900",
		AgentIncome:    "90",
	}, nil
}

func (f *fakePaymentService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	if f.channelSummaryErr != nil {
		return nil, f.channelSummaryErr
	}
	if f.channelSummaryResp != nil {
		return f.channelSummaryResp, nil
	}
	return []*paymentservice.SummaryByDayChannel{}, nil
}

func (f *fakePaymentService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	if f.withdrawErr != nil {
		return nil, f.withdrawErr
	}
	if f.withdrawResp != nil {
		return f.withdrawResp, nil
	}
	return &paymentservice.WithdrawList{}, nil
}

func (f *fakePaymentService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	if f.channelStatusErr != nil {
		return nil, f.channelStatusErr
	}
	return f.channelStatusResp, nil
}
