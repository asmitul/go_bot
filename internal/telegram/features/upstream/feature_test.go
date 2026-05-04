package upstream

import (
	"context"
	"strings"
	"testing"

	"go_bot/internal/telegram/models"
	telegramservice "go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

func TestFeature_UnbindSpecificInterface(t *testing.T) {
	groupSvc := &stubGroupService{
		group: &models.Group{
			TelegramID: -1001,
			Tier:       models.GroupTierUpstream,
			Settings: models.GroupSettings{
				InterfaceBindings: []models.InterfaceBinding{
					{Name: "接口A", ID: "1001", Rate: "7%"},
					{Name: "接口B", ID: "2002", Rate: "8%"},
				},
			},
		},
	}
	feature := New(groupSvc, stubAdminUserService{})
	msg := &botModels.Message{
		Text: "解绑接口 1001",
		Chat: botModels.Chat{ID: -1001, Type: "supergroup"},
		From: &botModels.User{ID: 9001},
	}

	resp, handled, err := feature.Process(context.Background(), msg, groupSvc.group)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled || resp == nil {
		t.Fatalf("expected handled response")
	}
	if !strings.Contains(resp.Text, "✅ 已解绑接口") || !strings.Contains(resp.Text, "1001") {
		t.Fatalf("unexpected response: %s", resp.Text)
	}

	got := groupSvc.updatedSettings.InterfaceBindings
	if len(got) != 1 {
		t.Fatalf("expected one remaining binding, got %#v", got)
	}
	if got[0].ID != "2002" {
		t.Fatalf("expected remaining interface 2002, got %#v", got)
	}
}

type stubGroupService struct {
	group           *models.Group
	updatedSettings models.GroupSettings
}

func (s *stubGroupService) CreateOrUpdateGroup(ctx context.Context, group *models.Group) error {
	panic("not implemented")
}

func (s *stubGroupService) GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error) {
	return s.group, nil
}

func (s *stubGroupService) GetOrCreateGroup(ctx context.Context, chatInfo *telegramservice.TelegramChatInfo) (*models.Group, error) {
	panic("not implemented")
}

func (s *stubGroupService) FindGroupByInterfaceID(ctx context.Context, interfaceID string) (*models.Group, error) {
	panic("not implemented")
}

func (s *stubGroupService) MarkBotLeft(ctx context.Context, telegramID int64) error {
	panic("not implemented")
}

func (s *stubGroupService) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	panic("not implemented")
}

func (s *stubGroupService) UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	s.updatedSettings = settings
	s.group.Settings = settings
	return nil
}

func (s *stubGroupService) LeaveGroup(ctx context.Context, telegramID int64) error {
	panic("not implemented")
}

func (s *stubGroupService) HandleBotAddedToGroup(ctx context.Context, group *models.Group) error {
	panic("not implemented")
}

func (s *stubGroupService) HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error {
	panic("not implemented")
}

func (s *stubGroupService) ValidateGroups(ctx context.Context) (*telegramservice.GroupValidationResult, error) {
	panic("not implemented")
}

func (s *stubGroupService) RepairGroups(ctx context.Context) (*telegramservice.GroupRepairResult, error) {
	panic("not implemented")
}
