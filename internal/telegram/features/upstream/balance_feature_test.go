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

func TestBalanceFeature_AdjustCommandSignSemantics(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		wantDelta  float64
		wantStatus string
		wantPrepay string
		wantDiff   string
	}{
		{
			name:       "minus increases prepaid",
			text:       "-1000 充值",
			wantDelta:  1000,
			wantStatus: "✅ 已增加预付：1000.00 CNY",
			wantPrepay: "当前预付：3000.00 CNY",
			wantDiff:   "结算差额：-1000.50 CNY",
		},
		{
			name:       "plus decreases prepaid",
			text:       "+1000 扣减",
			wantDelta:  -1000,
			wantStatus: "✅ 已减少预付：1000.00 CNY",
			wantPrepay: "当前预付：1000.00 CNY",
			wantDiff:   "结算差额：999.50 CNY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balanceSvc := &stubBalanceService{balance: 2000}
			paymentSvc := &stubPaymentService{
				summaryByPZID: &paymentservice.SummaryByPZID{
					Items: []*paymentservice.SummaryByPZIDItem{
						{Date: "2024-10-26", GrossAmount: "2150", MerchantIncome: "1999.50"},
					},
				},
			}
			feature := NewBalanceFeature(balanceSvc, stubAdminUserService{}, nil, paymentSvc)
			feature.nowFunc = func() time.Time {
				return time.Date(2024, 10, 26, 12, 0, 0, 0, upstreamChinaLocation)
			}
			group := &models.Group{
				Tier: models.GroupTierUpstream,
				Settings: models.GroupSettings{
					InterfaceBindings: []models.InterfaceBinding{{Name: "测试接口", ID: "1001"}},
				},
			}
			msg := &botModels.Message{
				Text: tt.text,
				Chat: botModels.Chat{ID: -1001, Type: "supergroup"},
				From: &botModels.User{ID: 9001},
			}

			resp, handled, err := feature.Process(context.Background(), msg, group)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !handled || resp == nil {
				t.Fatalf("expected handled response")
			}
			if balanceSvc.lastDelta != tt.wantDelta {
				t.Fatalf("expected delta %.2f, got %.2f", tt.wantDelta, balanceSvc.lastDelta)
			}
			if !strings.Contains(resp.Text, tt.wantStatus) {
				t.Fatalf("expected status %q, got %s", tt.wantStatus, resp.Text)
			}
			if !strings.Contains(resp.Text, tt.wantPrepay) {
				t.Fatalf("expected current prepaid line %q, got %s", tt.wantPrepay, resp.Text)
			}
			if !strings.Contains(resp.Text, tt.wantDiff) {
				t.Fatalf("expected settlement diff line %q, got %s", tt.wantDiff, resp.Text)
			}
		})
	}
}

type stubAdminUserService struct{}

func (stubAdminUserService) RegisterOrUpdateUser(ctx context.Context, info *telegramservice.TelegramUserInfo) error {
	panic("not implemented")
}

func (stubAdminUserService) GrantAdminPermission(ctx context.Context, targetID, grantedBy int64) error {
	panic("not implemented")
}

func (stubAdminUserService) RevokeAdminPermission(ctx context.Context, targetID, revokedBy int64) error {
	panic("not implemented")
}

func (stubAdminUserService) GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error) {
	panic("not implemented")
}

func (stubAdminUserService) ListAllAdmins(ctx context.Context) ([]*models.User, error) {
	panic("not implemented")
}

func (stubAdminUserService) CheckOwnerPermission(ctx context.Context, telegramID int64) (bool, error) {
	return false, nil
}

func (stubAdminUserService) CheckAdminPermission(ctx context.Context, telegramID int64) (bool, error) {
	return true, nil
}

func (stubAdminUserService) UpdateUserActivity(ctx context.Context, telegramID int64) error {
	return nil
}
