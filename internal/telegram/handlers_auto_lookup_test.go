package telegram

import (
	"context"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/payment/sifang"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

func TestHandleTextMessage_BotMessageTriggersSifangAutoLookup(t *testing.T) {
	paymentSvc := &autoLookupTestPaymentService{
		orderDetailCalled: make(chan string, 1),
	}
	groupSvc := &autoLookupTestGroupService{
		group: &models.Group{
			TelegramID: -1001,
			Type:       "group",
			Title:      "test-group",
			BotStatus:  models.BotStatusActive,
			Settings: models.GroupSettings{
				MerchantID:              123456,
				SifangEnabled:           true,
				SifangAutoLookupEnabled: true,
				CascadeForwardEnabled:   true,
			},
		},
	}

	b := &Bot{
		groupService:   groupSvc,
		paymentService: paymentSvc,
	}

	update := &botModels.Update{
		Message: &botModels.Message{
			ID:   9001,
			Text: "A123456789",
			Chat: botModels.Chat{
				ID:    -1001,
				Type:  "group",
				Title: "test-group",
			},
			From: &botModels.User{ID: 777, IsBot: true},
		},
	}

	b.handleTextMessage(context.Background(), nil, update)

	select {
	case got := <-paymentSvc.orderDetailCalled:
		if got != "A123456789" {
			t.Fatalf("unexpected order number: got %q want %q", got, "A123456789")
		}
	case <-time.After(time.Second):
		t.Fatal("expected auto lookup for bot message, but GetOrderDetail was not called")
	}
}

type autoLookupTestGroupService struct {
	group *models.Group
}

func (s *autoLookupTestGroupService) CreateOrUpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}

func (s *autoLookupTestGroupService) GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error) {
	return s.group, nil
}

func (s *autoLookupTestGroupService) GetOrCreateGroup(ctx context.Context, chatInfo *service.TelegramChatInfo) (*models.Group, error) {
	return s.group, nil
}

func (s *autoLookupTestGroupService) FindGroupByInterfaceID(ctx context.Context, interfaceID string) (*models.Group, error) {
	return nil, nil
}

func (s *autoLookupTestGroupService) MarkBotLeft(ctx context.Context, telegramID int64) error {
	return nil
}

func (s *autoLookupTestGroupService) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	return nil, nil
}

func (s *autoLookupTestGroupService) UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	return nil
}

func (s *autoLookupTestGroupService) LeaveGroup(ctx context.Context, telegramID int64) error {
	return nil
}

func (s *autoLookupTestGroupService) HandleBotAddedToGroup(ctx context.Context, group *models.Group) error {
	return nil
}

func (s *autoLookupTestGroupService) HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error {
	return nil
}

func (s *autoLookupTestGroupService) ValidateGroups(ctx context.Context) (*service.GroupValidationResult, error) {
	return nil, nil
}

func (s *autoLookupTestGroupService) RepairGroups(ctx context.Context) (*service.GroupRepairResult, error) {
	return nil, nil
}

type autoLookupTestPaymentService struct {
	orderDetailCalled chan string
}

func (s *autoLookupTestPaymentService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*paymentservice.SummaryByPZID, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) CreateOrder(ctx context.Context, merchantID int64, req paymentservice.CreateOrderRequest) (*paymentservice.CreateOrderResult, error) {
	return nil, nil
}

func (s *autoLookupTestPaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	s.orderDetailCalled <- orderNo
	return nil, &sifang.APIError{Code: 404, Message: "not found"}
}

func (s *autoLookupTestPaymentService) FindOrderChannelBinding(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderChannelBinding, error) {
	return nil, nil
}
