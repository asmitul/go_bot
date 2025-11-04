package telegram

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

type stubGroupService struct {
	group *models.Group
	err   error
}

func (s *stubGroupService) CreateOrUpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}

func (s *stubGroupService) GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error) {
	return s.group, s.err
}

func (s *stubGroupService) GetOrCreateGroup(ctx context.Context, chatInfo *service.TelegramChatInfo) (*models.Group, error) {
	return s.group, s.err
}

func (s *stubGroupService) MarkBotLeft(ctx context.Context, telegramID int64) error { return nil }
func (s *stubGroupService) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	return nil, nil
}
func (s *stubGroupService) UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	return nil
}
func (s *stubGroupService) LeaveGroup(ctx context.Context, telegramID int64) error { return nil }
func (s *stubGroupService) HandleBotAddedToGroup(ctx context.Context, group *models.Group) error {
	return nil
}
func (s *stubGroupService) HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error {
	return nil
}

type countingPaymentService struct {
	calls int32
}

func (c *countingPaymentService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	return nil, nil
}
func (c *countingPaymentService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	return nil, nil
}
func (c *countingPaymentService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	return nil, nil
}
func (c *countingPaymentService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	return nil, nil
}
func (c *countingPaymentService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	return nil, nil
}
func (c *countingPaymentService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	return nil, nil
}
func (c *countingPaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	atomic.AddInt32(&c.calls, 1)
	return nil, context.Canceled
}

func TestTryTriggerSifangAutoLookupDisabled(t *testing.T) {
	paymentStub := &countingPaymentService{}
	b := &Bot{
		groupService:   &stubGroupService{group: &models.Group{Settings: models.GroupSettings{SifangEnabled: false}}},
		paymentService: paymentStub,
	}

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: 1, Type: "group"},
		Text: "ORDER123",
	}

	b.tryTriggerSifangAutoLookup(context.Background(), msg)

	if atomic.LoadInt32(&paymentStub.calls) != 0 {
		t.Fatalf("expected GetOrderDetail not to be called when disabled")
	}
}

func TestTryTriggerSifangAutoLookupNoOrders(t *testing.T) {
	paymentStub := &countingPaymentService{}
	b := &Bot{
		groupService:   &stubGroupService{group: &models.Group{Settings: models.GroupSettings{SifangEnabled: true, SifangAutoLookupEnabled: true, MerchantID: 100}}},
		paymentService: paymentStub,
	}

	msg := &botModels.Message{
		Chat: botModels.Chat{ID: 1, Type: "supergroup"},
		Text: "纯文本没有订单",
	}

	b.tryTriggerSifangAutoLookup(context.Background(), msg)

	if atomic.LoadInt32(&paymentStub.calls) != 0 {
		t.Fatalf("expected GetOrderDetail not to be called when no orders found")
	}
}
