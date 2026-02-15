package sifang

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	cryptofeature "go_bot/internal/telegram/features/crypto"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

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

func TestExpirePending(t *testing.T) {
	feature := New(nil, nil)

	pending, err := feature.createPendingSend(100, 200, 300, 123.45, "")
	if err != nil {
		t.Fatalf("unexpected error creating pending send: %v", err)
	}

	feature.mu.Lock()
	feature.pending[pending.token].createdAt = time.Now().Add(-SendMoneyConfirmTTL - time.Second)
	feature.mu.Unlock()

	if !feature.ExpirePending(pending.token) {
		t.Fatalf("expected pending send to expire")
	}

	// 再次调用应返回 false，因为已删除
	if feature.ExpirePending(pending.token) {
		t.Fatalf("expected no pending record after expiration")
	}

	// 新的 pending 仍在有效期内，不应过期
	active, err := feature.createPendingSend(100, 200, 300, 50, "")
	if err != nil {
		t.Fatalf("unexpected error creating active pending: %v", err)
	}

	if feature.ExpirePending(active.token) {
		t.Fatalf("expected active pending not to expire yet")
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
	if f.Match(context.Background(), msg) {
		t.Fatalf("expected non-command to be ignored")
	}
}

func TestMatchAcceptsChannelCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "通道账单10月26",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected command to match")
	}
}

func TestMatchAcceptsBalanceWithDate(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "余额10月30",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected balance command with date to match")
	}
}

func TestMatchAcceptsRateCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "费率",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected rate command to match")
	}
}

func TestMatchAcceptsWithdrawCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "提款明细",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected withdraw command to match")
	}
}

func TestMatchAcceptsSendMoneyCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "下发 100",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected send money command to match")
	}
}

func TestMatchAcceptsCreateOrderCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "模拟下单 50",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected create order command to match")
	}
}

func TestMatchAcceptsCreateOrderAliasCommand(t *testing.T) {
	f := &Feature{}
	msg := &botModels.Message{
		Chat: botModels.Chat{Type: "group"},
		Text: "模拟创建订单 50",
	}
	if !f.Match(context.Background(), msg) {
		t.Fatalf("expected create order alias command to match")
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
	expected := "💸 提款明细（总计 100｜1 笔）\n<blockquote>10:00:00      100.00</blockquote>"
	if got != expected {
		t.Fatalf("unexpected withdraw message:\n%s", got)
	}

	gotEmpty := formatWithdrawListMessage("2025-10-31", &paymentservice.WithdrawList{})
	if gotEmpty != "💸 提款明细\n暂无提款记录" {
		t.Fatalf("unexpected empty withdraw message:\n%s", gotEmpty)
	}
}

func TestFormatWithdrawListMessageWithQuotes(t *testing.T) {
	list := &paymentservice.WithdrawList{
		Items: []*paymentservice.Withdraw{
			{
				WithdrawNo: "W2026",
				Amount:     "694.00",
				CreatedAt:  "2026-02-15 16:21:29",
			},
			{
				WithdrawNo: "W-OLD",
				Amount:     "300.00",
				CreatedAt:  "2026-02-15 16:20:49",
			},
		},
	}

	lookup := buildWithdrawQuoteLookup([]*models.WithdrawQuoteRecord{
		{
			WithdrawNo: "W2026",
			Rate:       6.94,
			USDTAmount: 100,
		},
	})

	got := formatWithdrawListMessageWithQuotes("2026-02-15", list, lookup)
	expected := "💸 提款明细（总计 994｜2 笔）\n<blockquote>16:21:29      694.00      6.94 ✖️ 100 U\n16:20:49      300.00</blockquote>"
	if got != expected {
		t.Fatalf("unexpected withdraw message with quote:\n%s", got)
	}
}

func TestFormatWithdrawListMessageWithIncompleteQuoteFallback(t *testing.T) {
	list := &paymentservice.WithdrawList{
		Items: []*paymentservice.Withdraw{
			{
				WithdrawNo: "W2027",
				Amount:     "500.00",
				CreatedAt:  "2026-02-15 12:00:00",
			},
		},
	}

	lookup := buildWithdrawQuoteLookup([]*models.WithdrawQuoteRecord{
		{
			WithdrawNo: "W2027",
			Rate:       6.94,
			USDTAmount: 0,
		},
	})

	got := formatWithdrawListMessageWithQuotes("2026-02-15", list, lookup)
	expected := "💸 提款明细（总计 500｜1 笔）\n<blockquote>12:00:00      500.00</blockquote>"
	if got != expected {
		t.Fatalf("unexpected fallback message:\n%s", got)
	}
}

func TestParseSendMoneyPayload_Number(t *testing.T) {
	amount, code, err := parseSendMoneyPayload(" 1,234.5678 ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amount != 1234.57 {
		t.Fatalf("expected rounded amount 1234.57, got %.2f", amount)
	}
	if code != "" {
		t.Fatalf("expected empty google code, got %s", code)
	}
}

func TestParseSendMoneyPayload_ExpressionWithGoogleCode(t *testing.T) {
	amount, code, err := parseSendMoneyPayload("(1+2)*3  123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if amount != 9 {
		t.Fatalf("expected amount 9, got %.2f", amount)
	}
	if code != "123456" {
		t.Fatalf("expected google code 123456, got %s", code)
	}
}

func TestParseSendMoneyPayload_Invalid(t *testing.T) {
	if _, _, err := parseSendMoneyPayload(""); err == nil {
		t.Fatalf("expected error for empty payload")
	}
	if _, _, err := parseSendMoneyPayload("abc"); err == nil {
		t.Fatalf("expected error for invalid payload")
	}
	if _, _, err := parseSendMoneyPayload("-100"); err == nil {
		t.Fatalf("expected error for negative amount")
	}
}

func TestFormatSendMoneyMessage(t *testing.T) {
	result := &paymentservice.SendMoneyResult{
		MerchantID: "2024164",
		Withdraw: &paymentservice.Withdraw{
			Amount: "0.00",
		},
	}

	message := formatSendMoneyMessage(2024164, 21750, result)
	expected := "已成功下发 <code>21750</code> 元给商户 <code>2024164</code>"
	if message != expected {
		t.Fatalf("unexpected send money message: %s", message)
	}
}

func TestHandleSendMoneyCreatesPending(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "下发 12",
	}

	resp, handled, err := feature.handleSendMoney(ctx, msg, 2023100, cryptofeature.DefaultFloatRate, msg.Text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled true")
	}
	if resp == nil || resp.ReplyMarkup == nil {
		t.Fatalf("expected inline keyboard response")
	}

	markup, ok := resp.ReplyMarkup.(*botModels.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected InlineKeyboardMarkup")
	}
	if len(markup.InlineKeyboard) != 1 || len(markup.InlineKeyboard[0]) != 2 {
		t.Fatalf("unexpected keyboard layout: %+v", markup.InlineKeyboard)
	}

	token := ""
	for data := range feature.pending {
		token = data
		break
	}
	if token == "" {
		t.Fatalf("expected pending token stored")
	}

	if fakeSvc.lastSendAmount != 0 {
		t.Fatalf("expected no send to occur before confirmation")
	}
}

func TestHandleSendMoneyCreatesPendingFromQuoteCommand(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	originalFetch := fetchC2COrders
	fetchC2COrders = func(ctx context.Context, paymentMethod string) ([]cryptofeature.C2COrder, error) {
		if paymentMethod != "aliPay" {
			t.Fatalf("unexpected payment method: %s", paymentMethod)
		}
		return []cryptofeature.C2COrder{
			{Price: "7.10", NickName: "M1"},
			{Price: "7.20", NickName: "M2"},
			{Price: "7.30", NickName: "M3"},
		}, nil
	}
	t.Cleanup(func() {
		fetchC2COrders = originalFetch
	})

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "下发 z3 100 123456",
	}

	resp, handled, err := feature.handleSendMoney(ctx, msg, 2023100, 0.12, msg.Text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled true")
	}
	if resp == nil || resp.ReplyMarkup == nil {
		t.Fatalf("expected inline keyboard response")
	}
	if !strings.Contains(resp.Text, "OTC商家实时价格") {
		t.Fatalf("expected otc header in confirmation text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "信息来源: 欧易 <b>支付宝</b>") {
		t.Fatalf("expected source section in confirmation text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "✅<b>7.30        M3</b>___➕<b>0.12</b>🟰<code>7.42</code>⬅️") {
		t.Fatalf("expected selected row in confirmation text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "<code>7.42</code> ✖️ <code>100</code> <b>U</b> 🟰 <code>742.00</code> <b>¥</b>") {
		t.Fatalf("expected total row in confirmation text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "是否确认下发 742 元 | 2023100") {
		t.Fatalf("unexpected confirmation text: %s", resp.Text)
	}

	token := ""
	for data := range feature.pending {
		token = data
		break
	}
	if token == "" {
		t.Fatalf("expected pending token stored")
	}

	pending := feature.pending[token]
	if pending == nil {
		t.Fatalf("expected pending data")
	}
	if pending.amount != 742 {
		t.Fatalf("expected pending amount 742, got %.2f", pending.amount)
	}
	if pending.googleCode != "123456" {
		t.Fatalf("expected google code 123456, got %s", pending.googleCode)
	}
}

func TestHandleSendMoneyQuoteCommandRequiresUSDTAmount(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "下发 z3",
	}

	resp, handled, err := feature.handleSendMoney(ctx, msg, 2023100, 0.12, msg.Text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled true")
	}
	if resp == nil {
		t.Fatalf("expected response")
	}
	if !strings.Contains(resp.Text, "缺少U金额") {
		t.Fatalf("unexpected response text: %s", resp.Text)
	}
}

func TestHandleSendMoneyCallbackConfirm(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{
		sendMoneyResult: &paymentservice.SendMoneyResult{
			MerchantID: "2023100",
			Withdraw:   &paymentservice.Withdraw{Amount: "12.00", WithdrawNo: "NO1"},
		},
	}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "下发 12",
	}
	resp, handled, err := feature.handleSendMoney(ctx, msg, 2023100, cryptofeature.DefaultFloatRate, msg.Text)
	if err != nil || !handled || resp == nil {
		t.Fatalf("unexpected setup result: resp=%v handled=%v err=%v", resp, handled, err)
	}

	token := ""
	for data := range feature.pending {
		token = data
		break
	}
	if token == "" {
		t.Fatalf("token not stored")
	}

	query := &botModels.CallbackQuery{
		From:    botModels.User{ID: 123},
		Message: botModels.MaybeInaccessibleMessage{Message: &botModels.Message{Chat: botModels.Chat{ID: -1}, ID: 99}},
	}

	result, err := feature.HandleSendMoneyCallback(ctx, query, sendMoneyActionConfirm, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.ShouldEdit {
		t.Fatalf("expected edit result")
	}
	if !strings.Contains(result.Text, "已成功下发") {
		t.Fatalf("unexpected success text: %s", result.Text)
	}
	if fakeSvc.lastSendAmount != 12 {
		t.Fatalf("expected send amount 12, got %.2f", fakeSvc.lastSendAmount)
	}
}

func TestHandleSendMoneyCallbackConfirmPersistsQuoteSnapshot(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{
		sendMoneyResult: &paymentservice.SendMoneyResult{
			MerchantID: "2023100",
			Withdraw: &paymentservice.Withdraw{
				Amount:     "694.00",
				WithdrawNo: "WQ-1",
				OrderNo:    "ORDER-1",
			},
		},
	}
	stubUser := &stubUserService{isAdmin: true}
	quoteRepo := &fakeWithdrawQuoteRepo{}
	feature := New(fakeSvc, stubUser)
	feature.SetWithdrawQuoteRepository(quoteRepo)

	originalFetch := fetchC2COrders
	fetchC2COrders = func(ctx context.Context, paymentMethod string) ([]cryptofeature.C2COrder, error) {
		return []cryptofeature.C2COrder{
			{Price: "6.82", NickName: "M1"},
		}, nil
	}
	t.Cleanup(func() {
		fetchC2COrders = originalFetch
	})

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "下发 z1 100",
	}
	resp, handled, err := feature.handleSendMoney(ctx, msg, 2023100, 0.12, msg.Text)
	if err != nil || !handled || resp == nil {
		t.Fatalf("unexpected setup result: resp=%v handled=%v err=%v", resp, handled, err)
	}

	token := ""
	for data := range feature.pending {
		token = data
		break
	}
	if token == "" {
		t.Fatalf("token not stored")
	}

	query := &botModels.CallbackQuery{
		From:    botModels.User{ID: 123},
		Message: botModels.MaybeInaccessibleMessage{Message: &botModels.Message{Chat: botModels.Chat{ID: -1}, ID: 99}},
	}
	result, err := feature.HandleSendMoneyCallback(ctx, query, sendMoneyActionConfirm, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.ShouldEdit {
		t.Fatalf("expected edit result")
	}

	if len(quoteRepo.records) != 1 {
		t.Fatalf("expected one quote record, got %d", len(quoteRepo.records))
	}
	record := quoteRepo.records[0]
	if record.WithdrawNo != "WQ-1" || record.OrderNo != "ORDER-1" {
		t.Fatalf("unexpected keys: %#v", record)
	}
	if record.Rate != 6.94 || record.USDTAmount != 100 {
		t.Fatalf("unexpected quote snapshot: rate=%.2f usdt=%.2f", record.Rate, record.USDTAmount)
	}
	if record.Amount != 694 {
		t.Fatalf("unexpected amount: %.2f", record.Amount)
	}
}

func TestHandleSendMoneyCallbackCancel(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -5, Type: "group"},
		From: &botModels.User{ID: 555},
		Text: "下发 20",
	}
	resp, handled, err := feature.handleSendMoney(ctx, msg, 2024001, cryptofeature.DefaultFloatRate, msg.Text)
	if err != nil || !handled || resp == nil {
		t.Fatalf("unexpected setup result: resp=%v handled=%v err=%v", resp, handled, err)
	}

	token := ""
	for data := range feature.pending {
		token = data
		break
	}

	query := &botModels.CallbackQuery{
		From:    botModels.User{ID: 555},
		Message: botModels.MaybeInaccessibleMessage{Message: &botModels.Message{Chat: botModels.Chat{ID: -5}, ID: 77}},
	}

	result, err := feature.HandleSendMoneyCallback(ctx, query, sendMoneyActionCancel, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.ShouldEdit {
		t.Fatalf("expected edit result for cancel")
	}
	if !strings.Contains(result.Text, "已取消下发") {
		t.Fatalf("unexpected cancel text: %s", result.Text)
	}
	if _, ok := feature.pending[token]; ok {
		t.Fatalf("expected pending cleared")
	}
}

func TestHandleCreateOrder(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{
		createOrderResp: &paymentservice.CreateOrderResult{
			MerchantID:      "2023100",
			MerchantOrderNo: "M-2026",
			Amount:          "88.80",
			ChannelCode:     "wxhftest",
			PaymentURL:      "https://example.com/pay/ok",
			Status:          "0",
		},
	}
	stubUser := &stubUserService{isAdmin: true}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "模拟下单 88.8 wxhftest M-2026",
	}

	respText, handled, err := feature.handleCreateOrder(ctx, msg, 2023100, msg.Text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if !strings.Contains(respText, "🧪 模拟下单成功") {
		t.Fatalf("unexpected response: %s", respText)
	}
	if !strings.Contains(respText, "M-2026") || !strings.Contains(respText, "https://example.com/pay/ok") {
		t.Fatalf("missing order details in response: %s", respText)
	}
	if fakeSvc.lastCreateOrderMerchantID != 2023100 {
		t.Fatalf("expected merchant id 2023100, got %d", fakeSvc.lastCreateOrderMerchantID)
	}
	if fakeSvc.lastCreateOrderReq.Amount != 88.8 {
		t.Fatalf("expected amount 88.8, got %.2f", fakeSvc.lastCreateOrderReq.Amount)
	}
	if fakeSvc.lastCreateOrderReq.ChannelCode != "wxhftest" || fakeSvc.lastCreateOrderReq.MerchantOrderNo != "M-2026" {
		t.Fatalf("unexpected create order request: %#v", fakeSvc.lastCreateOrderReq)
	}
}

func TestHandleCreateOrder_RequiresAdmin(t *testing.T) {
	ctx := context.Background()
	fakeSvc := &fakePaymentService{}
	stubUser := &stubUserService{isAdmin: false}
	feature := New(fakeSvc, stubUser)

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: -1, Type: "group"},
		From: &botModels.User{ID: 123},
		Text: "模拟下单 50",
	}

	respText, handled, err := feature.handleCreateOrder(ctx, msg, 2023100, msg.Text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled=true")
	}
	if !strings.Contains(respText, "仅管理员可以模拟下单") {
		t.Fatalf("unexpected response: %s", respText)
	}
	if fakeSvc.lastCreateOrderReq.Amount != 0 {
		t.Fatalf("expected no create order call, got %#v", fakeSvc.lastCreateOrderReq)
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
				{Amount: "100", Status: "paid", CreatedAt: today + " 10:00:00"},
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

func TestHandleSummaryWithdrawOnlyKeepsSuccessfulItems(t *testing.T) {
	now := time.Now().In(chinaLocation)
	today := now.Format("2006-01-02")
	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance: "5000",
		},
		withdrawResp: &paymentservice.WithdrawList{
			Items: []*paymentservice.Withdraw{
				{Amount: "100", Status: "paid", CreatedAt: today + " 10:00:00"},
				{Amount: "200", Status: "cancelled", CreatedAt: today + " 11:00:00"},
			},
		},
	}
	feature := &Feature{paymentService: fake}

	message, _, err := feature.handleSummary(context.Background(), 1001, "账单")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(message, "总计 100｜1 笔") {
		t.Fatalf("expected only successful withdraw in summary, got %s", message)
	}
	if strings.Contains(message, "11:00:00      200") {
		t.Fatalf("expected failed withdraw excluded, got %s", message)
	}
}

func TestBuildSummaryMessageMatchesHandleSummary(t *testing.T) {
	now := time.Now().In(chinaLocation)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, chinaLocation)

	fake := &fakePaymentService{
		balanceResp: &paymentservice.Balance{
			Balance:        "5000",
			HistoryBalance: "4000",
		},
		withdrawResp: &paymentservice.WithdrawList{
			Items: []*paymentservice.Withdraw{
				{Amount: "100", Status: "paid", CreatedAt: today.Format("2006-01-02") + " 10:00:00"},
			},
		},
	}

	feature := &Feature{paymentService: fake}

	expected, _, err := feature.handleSummary(context.Background(), 1001, "账单")
	if err != nil {
		t.Fatalf("unexpected error from handleSummary: %v", err)
	}

	actual, err := feature.BuildSummaryMessage(context.Background(), 1001, today)
	if err != nil {
		t.Fatalf("unexpected error from BuildSummaryMessage: %v", err)
	}

	if expected != actual {
		t.Fatalf("expected messages to match\nhandleSummary: %s\nBuildSummaryMessage: %s", expected, actual)
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
				{Amount: "100", Status: "paid", CreatedAt: today + " 08:00:00"},
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

func TestHandleWithdrawListOnlyKeepsSuccessfulItems(t *testing.T) {
	now := time.Now().In(chinaLocation)
	today := now.Format("2006-01-02")
	fake := &fakePaymentService{
		withdrawResp: &paymentservice.WithdrawList{
			Items: []*paymentservice.Withdraw{
				{Amount: "100", Status: "paid", CreatedAt: today + " 10:00:00"},
				{Amount: "200", Status: "cancelled", CreatedAt: today + " 11:00:00"},
			},
		},
	}
	feature := &Feature{paymentService: fake}

	message, handled, err := feature.handleWithdrawList(context.Background(), 1001, "提款明细")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected command to be handled")
	}
	if !strings.Contains(message, "总计 100｜1 笔") {
		t.Fatalf("expected only successful withdraw in details, got %s", message)
	}
	if strings.Contains(message, "11:00:00      200") {
		t.Fatalf("expected failed withdraw excluded, got %s", message)
	}
}

type fakePaymentService struct {
	balanceResp               *paymentservice.Balance
	balanceErr                error
	summaryResp               *paymentservice.SummaryByDay
	summaryErr                error
	channelSummaryResp        []*paymentservice.SummaryByDayChannel
	channelSummaryErr         error
	withdrawResp              *paymentservice.WithdrawList
	withdrawErr               error
	channelStatusResp         []*paymentservice.ChannelStatus
	channelStatusErr          error
	lastHistoryDays           int
	sendMoneyResult           *paymentservice.SendMoneyResult
	sendMoneyErr              error
	lastSendAmount            float64
	createOrderResp           *paymentservice.CreateOrderResult
	createOrderErr            error
	lastCreateOrderReq        paymentservice.CreateOrderRequest
	lastCreateOrderMerchantID int64
	orderDetailResp           *paymentservice.OrderDetail
	orderDetailErr            error
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

func (f *fakePaymentService) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*paymentservice.SummaryByPZID, error) {
	return nil, nil
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

func (f *fakePaymentService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	f.lastSendAmount = amount
	if f.sendMoneyErr != nil {
		return nil, f.sendMoneyErr
	}
	return f.sendMoneyResult, nil
}

func (f *fakePaymentService) CreateOrder(ctx context.Context, merchantID int64, req paymentservice.CreateOrderRequest) (*paymentservice.CreateOrderResult, error) {
	f.lastCreateOrderMerchantID = merchantID
	f.lastCreateOrderReq = req
	if f.createOrderErr != nil {
		return nil, f.createOrderErr
	}
	return f.createOrderResp, nil
}

func (f *fakePaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	if f.orderDetailErr != nil {
		return nil, f.orderDetailErr
	}
	return f.orderDetailResp, nil
}

func (f *fakePaymentService) FindOrderChannelBinding(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderChannelBinding, error) {
	return nil, nil
}

type stubUserService struct {
	isAdmin bool
}

func (s *stubUserService) RegisterOrUpdateUser(ctx context.Context, info *service.TelegramUserInfo) error {
	return nil
}

func (s *stubUserService) GrantAdminPermission(ctx context.Context, targetID, grantedBy int64) error {
	return nil
}

func (s *stubUserService) RevokeAdminPermission(ctx context.Context, targetID, revokedBy int64) error {
	return nil
}

func (s *stubUserService) GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error) {
	return nil, nil
}

func (s *stubUserService) ListAllAdmins(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}

func (s *stubUserService) CheckOwnerPermission(ctx context.Context, telegramID int64) (bool, error) {
	return false, nil
}

func (s *stubUserService) CheckAdminPermission(ctx context.Context, telegramID int64) (bool, error) {
	return s.isAdmin, nil
}

func (s *stubUserService) UpdateUserActivity(ctx context.Context, telegramID int64) error {
	return nil
}

type fakeWithdrawQuoteRepo struct {
	records []*models.WithdrawQuoteRecord
}

func (r *fakeWithdrawQuoteRepo) Upsert(ctx context.Context, record *models.WithdrawQuoteRecord) error {
	if record == nil {
		return nil
	}
	cloned := *record
	r.records = append(r.records, &cloned)
	return nil
}

func (r *fakeWithdrawQuoteRepo) ListByMerchantAndDateRange(ctx context.Context, merchantID int64, startTime, endTime time.Time) ([]*models.WithdrawQuoteRecord, error) {
	return r.records, nil
}

func (r *fakeWithdrawQuoteRepo) EnsureIndexes(ctx context.Context) error {
	return nil
}
