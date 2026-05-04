package upstream

import (
	"context"
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	telegramservice "go_bot/internal/telegram/service"

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

	balanceStub := &stubBalanceService{
		balance:         4392.05,
		snapshotClosing: 1884,
		adjustmentLogs: []*models.UpstreamBalanceLog{
			{
				GroupID:   1001,
				Delta:     -1000,
				Type:      models.BalanceOpDebit,
				Remark:    "扣款",
				CreatedAt: time.Date(2024, 10, 26, 10, 32, 1, 0, upstreamChinaLocation),
			},
			{
				GroupID:   1001,
				Delta:     1000,
				Type:      models.BalanceOpCredit,
				CreatedAt: time.Date(2024, 10, 26, 14, 5, 22, 0, upstreamChinaLocation),
			},
		},
	}
	feature := NewSummaryFeature(stub, balanceStub)
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
		Text: "ye2024-10-26",
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

	if !strings.Contains(resp.Text, "<b>📄 供应商账单</b>") {
		t.Fatalf("unexpected response text: %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "上游供应商 | 2024/10/26") {
		t.Fatalf("expected supplier name, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "<b>1. 支付宝渠道</b>") {
		t.Fatalf("expected interface descriptor, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "费率：<code>7%</code>\n跑量：<code>1000</code>\n应结算：<code>950</code>") {
		t.Fatalf("expected rate, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "昨日结余 <code>1884</code>") {
		t.Fatalf("expected yesterday balance, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "预付 <code>4392.05</code> | 结算差额 <code>-3442.05</code>") {
		t.Fatalf("expected prepaid balance, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "公式：<code>950</code> - <code>4392.05</code> = <code>-3442.05</code>") {
		t.Fatalf("expected settlement formula, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "💸 出入账记录（总计 0｜2 笔）\n<blockquote>10:32:01      +1000      扣款\n14:05:22      -1000</blockquote>") {
		t.Fatalf("expected adjustment log section, got %s", resp.Text)
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

func TestSummaryFeature_NegativePrepaidUsesParenthesesInFormula(t *testing.T) {
	stub := &stubPaymentService{
		summaryByPZID: &paymentservice.SummaryByPZID{
			Items: []*paymentservice.SummaryByPZIDItem{
				{
					Date:           "2024-10-26",
					GrossAmount:    "1000",
					MerchantIncome: "950",
				},
			},
		},
	}

	feature := NewSummaryFeature(stub, &stubBalanceService{balance: -3000})
	feature.nowFunc = func() time.Time {
		return time.Date(2024, 10, 26, 12, 0, 0, 0, upstreamChinaLocation)
	}

	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "测试渠道", ID: "1024", Rate: "7%"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "ye",
		Chat: botModels.Chat{ID: 1001, Type: "supergroup"},
		From: &botModels.User{ID: 42},
	}

	resp, handled, err := feature.Process(context.Background(), msg, group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response")
	}
	if !strings.Contains(resp.Text, "预付 <code>-3000</code> | 结算差额 <code>3950</code>") {
		t.Fatalf("expected settlement diff, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "公式：<code>950</code> - (<code>-3000</code>) = <code>3950</code>") {
		t.Fatalf("expected parenthesized negative prepaid formula, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "💸 出入账记录\n暂无出入账记录") {
		t.Fatalf("expected empty adjustment log section, got %s", resp.Text)
	}
}

func TestSummaryFeature_MultipleInterfacesOutputAll(t *testing.T) {
	stub := &stubPaymentService{
		summaryByPZIDByInterface: map[string]*paymentservice.SummaryByPZID{
			"1001": {
				PZName: "渠道A-通道",
				Items: []*paymentservice.SummaryByPZIDItem{
					{Date: "2024-10-25", OrderCount: "2", GrossAmount: "200", MerchantIncome: "190", AgentIncome: "10"},
				},
			},
			"2002": {
				PZName: "渠道B-通道",
				Items: []*paymentservice.SummaryByPZIDItem{
					{Date: "2024-10-25", OrderCount: "3", GrossAmount: "300", MerchantIncome: "285", AgentIncome: "15"},
				},
			},
		},
	}
	balanceStub := &stubBalanceService{balance: 4392.05}
	feature := NewSummaryFeature(stub, balanceStub)
	feature.nowFunc = func() time.Time {
		return time.Date(2024, 10, 25, 10, 0, 0, 0, upstreamChinaLocation)
	}
	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "渠道A", ID: "1001"},
				{Name: "渠道B", ID: "2002"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "ye",
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
	if strings.Count(resp.Text, "<b>📄 供应商账单</b>") != 1 {
		t.Fatalf("expected one supplier bill, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "<b>1. 渠道A</b>") {
		t.Fatalf("expected channel A summary, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "<b>2. 渠道B</b>") {
		t.Fatalf("expected channel B summary, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "跑量 <code>500</code> | 应结算 <code>475</code>") {
		t.Fatalf("expected total gross, got %s", resp.Text)
	}
	if len(stub.calls) != 2 || stub.calls[0] != "1001" || stub.calls[1] != "2002" {
		t.Fatalf("unexpected stub call order: %#v", stub.calls)
	}
}

func TestSummaryFeature_InterfaceSelectionNoData(t *testing.T) {
	stub := &stubPaymentService{
		summaryByPZID: &paymentservice.SummaryByPZID{
			Items: []*paymentservice.SummaryByPZIDItem{},
		},
	}
	feature := NewSummaryFeature(stub, &stubBalanceService{balance: 100})
	group := &models.Group{
		Settings: models.GroupSettings{
			InterfaceBindings: []models.InterfaceBinding{
				{Name: "渠道A", ID: "1001"},
				{Name: "渠道B", ID: "2002"},
			},
		},
	}
	msg := &botModels.Message{
		Text: "ye 2002",
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
	if !strings.Contains(resp.Text, "<b>1. 渠道B</b>") {
		t.Fatalf("expected selected interface row, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "跑量：<code>0</code>") {
		t.Fatalf("expected zero gross, got %s", resp.Text)
	}
	if !strings.Contains(resp.Text, "应结算：<code>0</code>") {
		t.Fatalf("expected zero settlement, got %s", resp.Text)
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
		Text: "ye 9999 2024-10-26",
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

func TestSummaryFeature_MatchCommandBoundary(t *testing.T) {
	feature := NewSummaryFeature(&stubPaymentService{})
	tests := []struct {
		text string
		want bool
	}{
		{text: "ye", want: true},
		{text: "YE 2024-10-26", want: true},
		{text: "ye2024-10-26", want: true},
		{text: "上游账单", want: true},
		{text: "yes", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			msg := &botModels.Message{
				Text: tt.text,
				Chat: botModels.Chat{ID: 1001, Type: "supergroup"},
			}
			if got := feature.Match(context.Background(), msg); got != tt.want {
				t.Fatalf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

type stubPaymentService struct {
	summaryByPZID            *paymentservice.SummaryByPZID
	summaryByPZIDByInterface map[string]*paymentservice.SummaryByPZID
	err                      error
	lastPZID                 string
	lastStart                time.Time
	lastEnd                  time.Time
	calls                    []string
}

type stubBalanceService struct {
	balance         float64
	err             error
	lastDelta       float64
	lastRemark      string
	below           bool
	snapshotClosing float64
	hasSnapshot     bool
	adjustmentLogs  []*models.UpstreamBalanceLog
}

func (s *stubBalanceService) Adjust(ctx context.Context, groupID int64, delta float64, operatorID int64, remark string, operationID string) (*telegramservice.UpstreamBalanceResult, bool, error) {
	if s.err != nil {
		return nil, false, s.err
	}
	s.lastDelta = delta
	s.lastRemark = remark
	s.balance += delta
	return &telegramservice.UpstreamBalanceResult{
		GroupID: groupID,
		Balance: s.balance,
	}, s.below, nil
}

func (s *stubBalanceService) SetMinBalance(ctx context.Context, groupID int64, threshold float64, operatorID int64) (*telegramservice.UpstreamBalanceResult, error) {
	panic("not implemented")
}

func (s *stubBalanceService) SetAlertLimit(ctx context.Context, groupID int64, limit int, operatorID int64) (*telegramservice.UpstreamBalanceResult, error) {
	panic("not implemented")
}

func (s *stubBalanceService) Get(ctx context.Context, groupID int64) (*telegramservice.UpstreamBalanceResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &telegramservice.UpstreamBalanceResult{
		GroupID: groupID,
		Balance: s.balance,
	}, nil
}

func (s *stubBalanceService) GetSettlementSnapshot(ctx context.Context, groupID int64, date time.Time) (*telegramservice.UpstreamSettlementSnapshotResult, error) {
	if !s.hasSnapshot && s.snapshotClosing == 0 {
		return nil, nil
	}
	return &telegramservice.UpstreamSettlementSnapshotResult{
		GroupID:          groupID,
		Date:             date.Format("2006-01-02"),
		ClosingPrepaid:   s.snapshotClosing,
		SettlementAmount: 0,
	}, nil
}

func (s *stubBalanceService) ListAll(ctx context.Context) ([]*telegramservice.UpstreamBalanceResult, error) {
	panic("not implemented")
}

func (s *stubBalanceService) ListAdjustmentLogs(ctx context.Context, groupID int64, startTime, endTime time.Time) ([]*models.UpstreamBalanceLog, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.adjustmentLogs, nil
}

func (s *stubBalanceService) SettleDaily(ctx context.Context, groupID int64, targetDate time.Time, operatorID int64, operationID string) (*telegramservice.SettlementResult, error) {
	panic("not implemented")
}

func (s *stubBalanceService) SubscribeEvents() <-chan *models.UpstreamBalanceEvent {
	panic("not implemented")
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
	s.calls = append(s.calls, pzid)
	if summary, ok := s.summaryByPZIDByInterface[pzid]; ok {
		return summary, s.err
	}
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

func (s *stubPaymentService) CreateOrder(ctx context.Context, merchantID int64, req paymentservice.CreateOrderRequest) (*paymentservice.CreateOrderResult, error) {
	panic("not implemented")
}

func (s *stubPaymentService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderDetail, error) {
	panic("not implemented")
}

func (s *stubPaymentService) FindOrderChannelBinding(ctx context.Context, merchantID int64, orderNo string, numberType paymentservice.OrderNumberType) (*paymentservice.OrderChannelBinding, error) {
	return nil, nil
}
