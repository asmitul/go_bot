package service

import (
	"context"
	"testing"

	"go_bot/internal/telegram/models"
)

type stubGroupService struct {
	updateCalls  int
	lastSettings models.GroupSettings
}

func (s *stubGroupService) CreateOrUpdateGroup(ctx context.Context, group *models.Group) error {
	return nil
}

func (s *stubGroupService) GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error) {
	return nil, nil
}

func (s *stubGroupService) GetOrCreateGroup(ctx context.Context, chatInfo *TelegramChatInfo) (*models.Group, error) {
	return nil, nil
}

func (s *stubGroupService) FindGroupByInterfaceID(ctx context.Context, interfaceID string) (*models.Group, error) {
	return nil, nil
}

func (s *stubGroupService) MarkBotLeft(ctx context.Context, telegramID int64) error {
	return nil
}

func (s *stubGroupService) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	return nil, nil
}

func (s *stubGroupService) UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	s.updateCalls++
	s.lastSettings = settings
	return nil
}

func (s *stubGroupService) LeaveGroup(ctx context.Context, telegramID int64) error {
	return nil
}

func (s *stubGroupService) HandleBotAddedToGroup(ctx context.Context, group *models.Group) error {
	return nil
}

func (s *stubGroupService) HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error {
	return nil
}

func (s *stubGroupService) ValidateGroups(ctx context.Context) (*GroupValidationResult, error) {
	return &GroupValidationResult{}, nil
}

func (s *stubGroupService) RepairGroups(ctx context.Context) (*GroupRepairResult, error) {
	return &GroupRepairResult{}, nil
}

func TestConfigMenuServiceHandleToggle_DisabledWhenSifangOff(t *testing.T) {
	svc := NewConfigMenuService(&stubGroupService{})
	group := &models.Group{Settings: models.GroupSettings{
		SifangEnabled:           false,
		SifangAutoLookupEnabled: true,
	}}

	items := []models.ConfigItem{
		{
			ID:   "sifang_auto_lookup_enabled",
			Type: models.ConfigTypeToggle,
			Name: "四方自动查单",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.SifangAutoLookupEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.SifangAutoLookupEnabled = val
			},
			ToggleDisabled: func(g *models.Group) (bool, string) {
				if !g.Settings.SifangEnabled {
					return true, "需先开启四方支付"
				}
				return false, ""
			},
		},
	}

	msg, shouldUpdate, err := svc.handleToggle(context.Background(), group, "sifang_auto_lookup_enabled", items)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if shouldUpdate {
		t.Fatalf("expected no menu update when toggle disabled")
	}
	expectedMessage := "⚠️ 需先开启四方支付"
	if msg != expectedMessage {
		t.Fatalf("expected message %q, got %q", expectedMessage, msg)
	}
	if !group.Settings.SifangAutoLookupEnabled {
		t.Fatalf("expected auto lookup setting to remain true")
	}
}

func TestConfigMenuServiceHandleToggle_TogglesWhenAvailable(t *testing.T) {
	stubSvc := &stubGroupService{}
	svc := NewConfigMenuService(stubSvc)
	group := &models.Group{Settings: models.GroupSettings{
		SifangEnabled:           true,
		SifangAutoLookupEnabled: true,
	}}

	items := []models.ConfigItem{
		{
			ID:   "sifang_auto_lookup_enabled",
			Type: models.ConfigTypeToggle,
			Name: "四方自动查单",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.SifangAutoLookupEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.SifangAutoLookupEnabled = val
			},
			ToggleDisabled: func(g *models.Group) (bool, string) {
				if !g.Settings.SifangEnabled {
					return true, "需先开启四方支付"
				}
				return false, ""
			},
		},
	}

	msg, shouldUpdate, err := svc.handleToggle(context.Background(), group, "sifang_auto_lookup_enabled", items)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !shouldUpdate {
		t.Fatalf("expected menu update when toggle succeeds")
	}

	expectedMessage := "✅ 四方自动查单 已关闭"
	if msg != expectedMessage {
		t.Fatalf("expected message %q, got %q", expectedMessage, msg)
	}

	if group.Settings.SifangAutoLookupEnabled {
		t.Fatalf("expected auto lookup setting to be toggled off")
	}
	if stubSvc.updateCalls != 1 {
		t.Fatalf("expected updateGroupSettings to be called once, got %d", stubSvc.updateCalls)
	}
	if stubSvc.lastSettings.SifangAutoLookupEnabled {
		t.Fatalf("expected persisted setting to be false")
	}
}
