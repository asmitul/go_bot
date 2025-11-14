package upstream

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
)

func TestSummaryFeature_ProcessWithData(t *testing.T) {
	stub := &stubPaymentService{
		summaryByPZID: &paymentservice.SummaryByPZID{
			PZName: "支付宝代收",
			Items: []*paymentservice.SummaryByPZIDItem{
				{
					Date:           "2024-10-26 00:00:00",
					OrderCount:     "5",
					GrossAmount:    "1000.00",
					MerchantIncome: "950.00",
					AgentIncome:    "50.00",
				},
			},
		},
	}

	feature := NewSummaryFeature(stub)
	feature.nowFunc = func() time.Time {
		return time.Date(2024, 10, 26, 12, 0, 0, 0, upstreamChinaLocation)
	}

	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "支付宝渠道", ID: "1024", Rate: "7%"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "上游账单2024-10-26",
		Chat: botModels.Chat{ID: 1001, Type: "supergroup"},
		From: &botModels.User{ID: 42},
	}

	resp, handled, err := feature.Process(context.Background(), msg, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response, got handled=%v resp=%v", handled, resp)
	}

	if !strings.Contains(resp.Text, "📈 上游账单 - 2024-10-26") {
		t.Fatalf("unexpected response text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "渠道名称：支付宝代收") {
		t.Fatalf("expected channel name, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "接口：支付宝渠道 / <code>1024</code>（费率：7%）") {
		t.Fatalf("expected interface descriptor, got %s", resp.Text)
	}
	if stub.lastPZID != "1024" {
		t.Fatalf("expected pzid 1024, got %s", stub.lastPZID)
	}
	if stub.lastStart.Format("2006-01-02 15:04:05") != "2024-10-26 00:00:00" {
		t.Fatalf("unexpected start: %s", stub.lastStart)
	}
	if stub.lastEnd.Format("2006-01-02 15:04:05") != "2024-10-26 23:59:59" {
		t.Fatalf("unexpected end: %s", stub.lastEnd)
	}
}

func TestSummaryFeature_MultipleInterfacesRequireSelection(t *testing.T) {
	stub := &stubPaymentService{}
	feature := NewSummaryFeature(stub)
	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "渠道A", ID: "1001"},
				{Name: "渠道B", ID: "2002"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "上游账单",
		Chat: botModels.Chat{ID: 1002, Type: "group"},
		From: &botModels.User{ID: 1},
	}

	resp, handled, err := feature.Process(context.Background(), msg, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response")
	}
	if !strings.Contains(resp.Text, "可选接口") {
		t.Fatalf("expected prompt, got %s", resp.Text)
	}
}

func TestSummaryFeature_InterfaceSelectionNoData(t *testing.T) {
	stub := &stubPaymentService{
		summaryByPZID: &paymentservice.SummaryByPZID{
			Items: []*paymentservice.SummaryByPZIDItem{},
		},
	}
	feature := NewSummaryFeature(stub)
	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "渠道A", ID: "1001"},
				{Name: "渠道B", ID: "2002"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "上游账单 2002",
		Chat: botModels.Chat{ID: 1003, Type: "supergroup"},
		From: &botModels.User{ID: 99},
	}

	resp, handled, err := feature.Process(context.Background(), msg, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response")
	}
	if !strings.Contains(resp.Text, "暂无上游账单数据") {
		t.Fatalf("expected empty data message, got %s", resp.Text)
	}
	if stub.lastPZID != "2002" {
		t.Fatalf("expected pzid 2002, got %s", stub.lastPZID)
	}
}

func TestSummaryFeature_InvalidInterfaceReturnsError(t *testing.T) {
	stub := &stubPaymentService{}
	feature := NewSummaryFeature(stub)
	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "渠道A", ID: "1001"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "上游账单 9999 2024-10-26",
		Chat: botModels.Chat{ID: 1004, Type: "supergroup"},
		From: &botModels.User{ID: 2},
	}

	resp, handled, err := feature.Process(context.Background(), msg, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response")
	}
	if !strings.Contains(resp.Text, "未绑定接口 ID") {
		t.Fatalf("expected interface error, got %s", resp.Text)
	}
}

type stubPaymentService struct {
	summaryByPZID *paymentservice.SummaryByPZID
	err           error
	lastPZID      string
	lastStart     time.Time
	lastEnd       time.Time
}

func (s *stubPaymentService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*paymentservice.Balance, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*paymentservice.SummaryByDay, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*paymentservice.SummaryByDayChannel, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*paymentservice.SummaryByPZID, error) {
	s.lastPZID = pzid
	s.lastStart = start
	s.lastEnd = end
	return s.summaryByPZID, s.err
}

func (s *stubPaymentService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*paymentservice.ChannelStatus, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*paymentservice.WithdrawList, error) {
	panic("not implemented")
}

func (s *stubPaymentService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts paymentservice.SendMoneyOptions) (*paymentservice.SendMoneyResult, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	panic("not implemented")
}
