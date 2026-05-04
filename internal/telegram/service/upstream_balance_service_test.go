package service

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
)

func TestUpstreamBalanceServiceSettleDailyUsesReturnedSettlementAmount(t *testing.T) {
	group := &models.Group{
		TelegramID: -1001,
		Title:      "上游测试群",
		Tier:       models.GroupTierUpstream,
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "丰辉8001移动网厅", ID: "8001", Rate: "8.5%"},
			},
		},
	}
	balanceRepo := &stubUpstreamBalanceRepo{
		balance: &models.UpstreamBalance{
			GroupID:           -1001,
			Balance:           3000,
			MinBalance:        0,
			AlertLimitPerHour: 3,
		},
	}
	paymentSvc := &stubSettlementPaymentService{
		summary: &paymentservice.SummaryByPZID{
			PZName: "丰辉",
			Items: []*paymentservice.SummaryByPZIDItem{
				{
					Date:             "2026-05-02",
					GrossAmount:      "2150",
					MerchantIncome:   "1999.50",
					NetAfterUpstream: "1999.50",
				},
			},
		},
	}
	svc := NewUpstreamBalanceService(balanceRepo, &stubGroupRepository{storedGroup: group}, paymentSvc)

	result, err := svc.SettleDaily(context.Background(), -1001, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), 0, "auto-settle:-1001:2026-05-02")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if balanceRepo.lastDelta != -1999.50 {
		t.Fatalf("expected settlement delta -1999.50, got %.2f", balanceRepo.lastDelta)
	}
	if result.Balance != 1000.50 {
		t.Fatalf("expected balance 1000.50, got %.2f", result.Balance)
	}
	if !strings.Contains(result.Report, "📊 自动日结 - <code>2026-05-02</code>") {
		t.Fatalf("expected copyable settlement date in report, got %s", result.Report)
	}
	if !strings.Contains(result.Report, "丰辉8001移动网厅 (<code>8001</code>)") {
		t.Fatalf("expected copyable interface ID in report, got %s", result.Report)
	}
	if !strings.Contains(result.Report, "跑量：<code>2150.00</code>，应结算：<code>1999.50</code>") {
		t.Fatalf("expected copyable item amounts in report, got %s", result.Report)
	}
	if !strings.Contains(result.Report, "应结算合计：<code>1999.50</code> CNY") {
		t.Fatalf("expected settlement total in report, got %s", result.Report)
	}
	if !strings.Contains(result.Report, "日结前预付：<code>3000.00</code> CNY") ||
		!strings.Contains(result.Report, "日结后预付：<code>1000.50</code> CNY") {
		t.Fatalf("expected before/after prepaid in report, got %s", result.Report)
	}
}

type stubUpstreamBalanceRepo struct {
	balance      *models.UpstreamBalance
	lastDelta    float64
	lastSnapshot *models.UpstreamSettlementSnapshot
}

func (s *stubUpstreamBalanceRepo) Get(ctx context.Context, groupID int64) (*models.UpstreamBalance, error) {
	if s.balance == nil {
		s.balance = &models.UpstreamBalance{GroupID: groupID}
	}
	return s.balance, nil
}

func (s *stubUpstreamBalanceRepo) Adjust(ctx context.Context, groupID int64, delta float64, operatorID int64, remark string, opType models.BalanceOperationType, operationID string, metadata map[string]string) (*models.UpstreamBalance, error) {
	current, _ := s.Get(ctx, groupID)
	s.lastDelta = delta
	current.Balance += delta
	return current, nil
}

func (s *stubUpstreamBalanceRepo) SetMinBalance(ctx context.Context, groupID int64, threshold float64, operatorID int64) (*models.UpstreamBalance, error) {
	current, _ := s.Get(ctx, groupID)
	current.MinBalance = threshold
	return current, nil
}

func (s *stubUpstreamBalanceRepo) SetAlertLimit(ctx context.Context, groupID int64, limit int, operatorID int64) (*models.UpstreamBalance, error) {
	current, _ := s.Get(ctx, groupID)
	current.AlertLimitPerHour = limit
	return current, nil
}

func (s *stubUpstreamBalanceRepo) ListAll(ctx context.Context) ([]*models.UpstreamBalance, error) {
	if s.balance == nil {
		return nil, nil
	}
	return []*models.UpstreamBalance{s.balance}, nil
}

func (s *stubUpstreamBalanceRepo) ListAdjustmentLogsByDateRange(ctx context.Context, groupID int64, startTime, endTime time.Time) ([]*models.UpstreamBalanceLog, error) {
	return nil, nil
}

func (s *stubUpstreamBalanceRepo) CreateSettlementSnapshot(ctx context.Context, snapshot *models.UpstreamSettlementSnapshot) (*models.UpstreamSettlementSnapshot, error) {
	if s.lastSnapshot == nil {
		clone := *snapshot
		s.lastSnapshot = &clone
	}
	return s.lastSnapshot, nil
}

func (s *stubUpstreamBalanceRepo) GetSettlementSnapshot(ctx context.Context, groupID int64, date string) (*models.UpstreamSettlementSnapshot, error) {
	if s.lastSnapshot == nil || s.lastSnapshot.GroupID != groupID || s.lastSnapshot.Date != date {
		return nil, nil
	}
	return s.lastSnapshot, nil
}

func (s *stubUpstreamBalanceRepo) EnsureIndexes(ctx context.Context) error {
	return nil
}

type stubSettlementPaymentService struct {
	summary *paymentservice.SummaryByPZID
}

func (s *stubSettlementPaymentService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*paymentservice.SummaryByPZID, error) {
	return s.summary, nil
}

func (s *stubSettlementPaymentService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) CreateOrder(ctx context.Context, merchantID int64, req paymentservice.CreateOrderRequest) (*paymentservice.CreateOrderResult, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	panic("not implemented")
}

func (s *stubSettlementPaymentService) FindOrderChannelBinding(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderChannelBinding, error) {
	return nil, nil
}
