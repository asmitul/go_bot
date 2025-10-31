package sifang

import (
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"

	botModels "github.com/go-telegram/bot/models"
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
	expected := "📑 通道账单 - 2025-10-31\n跑量：0\n成交：0\n笔数：0"
	if got != expected {
		t.Fatalf("unexpected channel message for no items:\n%s", got)
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
	expected := "💸 提款明细 - 2025-10-31\n\n#1 提现单号:W2025 订单号:O1\n金额：100.00 手续费：1.00 渠道：ALIPAY\n状态：paid 创建：2025-10-31 10:00:00 支付：2025-10-31 11:00:00"
	if got != expected {
		t.Fatalf("unexpected withdraw message:\n%s", got)
	}

	gotEmpty := formatWithdrawListMessage("2025-10-31", &paymentservice.WithdrawList{})
	if gotEmpty != "💸 提款明细 - 2025-10-31\n暂无提款记录" {
		t.Fatalf("unexpected empty withdraw message:\n%s", gotEmpty)
	}
}
