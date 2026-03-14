package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/payment/sifang"

	botModels "github.com/go-telegram/bot/models"
)

type privateLookupPaymentServiceStub struct {
	detail *paymentservice.OrderDetail
	err    error

	callCount      int
	lastMerchantID int64
	lastOrderNo    string
	lastNumberType paymentservice.OrderNumberType
}

func (s *privateLookupPaymentServiceStub) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*paymentservice.SummaryByPZID, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) CreateOrder(ctx context.Context, merchantID int64, req paymentservice.CreateOrderRequest) (*paymentservice.CreateOrderResult, error) {
	return nil, nil
}

func (s *privateLookupPaymentServiceStub) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	s.callCount++
	s.lastMerchantID = merchantID
	s.lastOrderNo = orderNo
	s.lastNumberType = numberType

	if s.err != nil {
		return nil, s.err
	}
	return s.detail, nil
}

func (s *privateLookupPaymentServiceStub) FindOrderChannelBinding(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderChannelBinding, error) {
	return nil, nil
}

func TestHandleTextMessage_PrivateAdminOrderLookupSuccess(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	paymentSvc := &privateLookupPaymentServiceStub{
		detail: &paymentservice.OrderDetail{
			Order: &paymentservice.Order{
				MerchantOrderNo: "ABCD123456",
				PlatformOrderNo: "PF-1001",
				Status:          "paid",
				StatusText:      "支付成功",
				Amount:          "100.00",
				RealAmount:      "88.50",
			},
			Extended: &paymentservice.OrderExtended{
				OrderID:    "OID-1001",
				ChannelFee: "7.50",
			},
			NotifyLogs: []*paymentservice.NotifyLog{
				{
					Status:      "success",
					StatusText:  "通知成功",
					URL:         "https://callback.example.com",
					AttemptedAt: "2025-11-16 10:23:43",
					Response:    "ok",
				},
			},
		},
	}

	b := &Bot{
		bot:            apiBot,
		userService:    &accountingHandlerStubUserService{isAdmin: true},
		paymentService: paymentSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   1001,
			Text: "2024164ABCD123456",
			Chat: botModels.Chat{
				ID:   60001,
				Type: "private",
			},
			From: &botModels.User{ID: 9001},
		},
	}

	b.handleTextMessage(context.Background(), apiBot, update)

	if paymentSvc.callCount != 1 {
		t.Fatalf("expected GetOrderDetail to be called once, got %d", paymentSvc.callCount)
	}
	if paymentSvc.lastMerchantID != 2024164 {
		t.Fatalf("unexpected merchant id: got %d want %d", paymentSvc.lastMerchantID, 2024164)
	}
	if paymentSvc.lastOrderNo != "ABCD123456" {
		t.Fatalf("unexpected order no: got %q want %q", paymentSvc.lastOrderNo, "ABCD123456")
	}
	if paymentSvc.lastNumberType != paymentservice.OrderNumberTypeMerchant {
		t.Fatalf("unexpected number type: got %q want %q", paymentSvc.lastNumberType, paymentservice.OrderNumberTypeMerchant)
	}

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called")
	}
}

func TestHandleTextMessage_PrivateOrderLookupRejectsNonAdmin(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	paymentSvc := &privateLookupPaymentServiceStub{}
	b := &Bot{
		bot:            apiBot,
		userService:    &accountingHandlerStubUserService{isAdmin: false},
		paymentService: paymentSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   1002,
			Text: "2024164ABCD123456",
			Chat: botModels.Chat{
				ID:   60002,
				Type: "private",
			},
			From: &botModels.User{ID: 9002},
		},
	}

	b.handleTextMessage(context.Background(), apiBot, update)

	if paymentSvc.callCount != 0 {
		t.Fatalf("expected GetOrderDetail not to be called for non-admin")
	}

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called for non-admin rejection")
	}
}

func TestHandleTextMessage_PrivateOrderLookupInvalidFormat(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	paymentSvc := &privateLookupPaymentServiceStub{}
	b := &Bot{
		bot:            apiBot,
		userService:    &accountingHandlerStubUserService{isAdmin: true},
		paymentService: paymentSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   1003,
			Text: "abc123",
			Chat: botModels.Chat{
				ID:   60003,
				Type: "private",
			},
			From: &botModels.User{ID: 9003},
		},
	}

	b.handleTextMessage(context.Background(), apiBot, update)

	if paymentSvc.callCount != 0 {
		t.Fatalf("expected GetOrderDetail not to be called for invalid format")
	}

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called for format error")
	}
}

func TestHandleTextMessage_PrivateOrderLookupNotFound(t *testing.T) {
	apiBot, spy := newTelegramTestBot(t)

	paymentSvc := &privateLookupPaymentServiceStub{
		err: &sifang.APIError{Code: 404, Message: "not found"},
	}
	b := &Bot{
		bot:            apiBot,
		userService:    &accountingHandlerStubUserService{isAdmin: true},
		paymentService: paymentSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   1004,
			Text: "2024164ABCD123456",
			Chat: botModels.Chat{
				ID:   60004,
				Type: "private",
			},
			From: &botModels.User{ID: 9004},
		},
	}

	b.handleTextMessage(context.Background(), apiBot, update)

	if paymentSvc.callCount != 1 {
		t.Fatalf("expected GetOrderDetail to be called once, got %d", paymentSvc.callCount)
	}

	if spy.count("sendMessage") == 0 {
		t.Fatalf("expected sendMessage to be called for order not found")
	}
}

func TestParsePrivateLookupOrderNo(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := parsePrivateLookupOrderNo("2024164ABCD123456")
		if err != nil {
			t.Fatalf("expected valid order number, got err=%v", err)
		}
		if got != "2024164ABCD123456" {
			t.Fatalf("unexpected parsed value: %q", got)
		}
	})

	t.Run("invalid prefix", func(t *testing.T) {
		if _, err := parsePrivateLookupOrderNo("ABC4164ABCD123456"); err == nil {
			t.Fatalf("expected error for non-digit merchant prefix")
		}
	})

	t.Run("invalid length", func(t *testing.T) {
		if _, err := parsePrivateLookupOrderNo("2024164A"); err == nil {
			t.Fatalf("expected error for short order number")
		}
	})
}

func TestBuildPrivateLookupMessageChunksIncludesRequiredSections(t *testing.T) {
	detail := &paymentservice.OrderDetail{
		Order: &paymentservice.Order{
			MerchantOrderNo: "ABCD123456",
			StatusText:      "支付成功",
		},
		Extended: &paymentservice.OrderExtended{
			OrderID: "OID-1",
		},
		NotifyLogs: []*paymentservice.NotifyLog{
			{
				StatusText: "通知成功",
			},
		},
	}

	chunks := buildPrivateLookupMessageChunks("2024164ABCD123456", 2024164, "ABCD123456", detail)
	if len(chunks) == 0 {
		t.Fatalf("expected non-empty chunks")
	}

	all := strings.Join(chunks, "\n")
	if !strings.Contains(all, "<b>订单基础信息</b>") {
		t.Fatalf("expected order section, got %q", all)
	}
	if !strings.Contains(all, "<b>扩展信息</b>") {
		t.Fatalf("expected extended section, got %q", all)
	}
	if !strings.Contains(all, "<b>回调/通知日志</b>") {
		t.Fatalf("expected notify log section, got %q", all)
	}
}
