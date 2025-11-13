package service

import (
	"context"
	"errors"
	"testing"

	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

type stubGroupRepository struct {
	storedGroup     *models.Group
	lastUpdatedTier models.GroupTier
}

func (s *stubGroupRepository) CreateOrUpdate(ctx context.Context, group *models.Group) error {
	clone := *group
	clone.Settings = group.Settings
	clone.Stats = group.Stats
	s.storedGroup = &clone
	return nil
}

func (s *stubGroupRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.Group, error) {
	if s.storedGroup == nil {
		return nil, errors.New("not found")
	}
	return s.storedGroup, nil
}

func (s *stubGroupRepository) UpdateBotStatus(ctx context.Context, telegramID int64, status string) error {
	return nil
}

func (s *stubGroupRepository) DeleteGroup(ctx context.Context, telegramID int64) error {
	return nil
}

func (s *stubGroupRepository) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	return nil, nil
}

func (s *stubGroupRepository) UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings, tier models.GroupTier) error {
	s.lastUpdatedTier = tier
	if s.storedGroup != nil {
		s.storedGroup.Settings = settings
		s.storedGroup.Tier = tier
	}
	return nil
}

func (s *stubGroupRepository) UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error {
	return nil
}

func (s *stubGroupRepository) EnsureIndexes(ctx context.Context, ttlSeconds int32) error {
	return nil
}

func TestGroupServiceGetOrCreateGroupSetsDefaultAutoLookup(t *testing.T) {
	repo := &stubGroupRepository{}
	service := NewGroupService(repo)

	chatInfo := &TelegramChatInfo{
		ChatID: 123,
		Type:   "supergroup",
		Title:  "Test Group",
	}

	group, err := service.GetOrCreateGroup(context.Background(), chatInfo)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if group.Settings.SifangAutoLookupEnabled != true {
		t.Fatalf("expected default auto lookup to be true, got %v", group.Settings.SifangAutoLookupEnabled)
	}

	if repo.storedGroup == nil {
		t.Fatalf("expected repository to store group")
	}

	if repo.storedGroup.Settings.SifangAutoLookupEnabled != true {
		t.Fatalf("expected stored group to have auto lookup enabled by default")
	}

	if repo.storedGroup.Tier != models.GroupTierBasic {
		t.Fatalf("expected stored group tier to default to basic, got %s", repo.storedGroup.Tier)
	}
}

func TestUpdateGroupSettingsSetsMerchantTier(t *testing.T) {
	repo := &stubGroupRepository{
		storedGroup: &models.Group{TelegramID: 1},
	}
	service := NewGroupService(repo)

	settings := models.GroupSettings{
		MerchantID: 123,
	}

	if err := service.UpdateGroupSettings(context.Background(), 1, settings); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastUpdatedTier != models.GroupTierMerchant {
		t.Fatalf("expected tier to be merchant, got %s", repo.lastUpdatedTier)
	}
}

func TestUpdateGroupSettingsRejectsConflictingBindings(t *testing.T) {
	repo := &stubGroupRepository{
		storedGroup: &models.Group{TelegramID: 1},
	}
	service := NewGroupService(repo)

	settings := models.GroupSettings{
		MerchantID:   123,
		InterfaceIDs: []string{"abc"},
	}

	if err := service.UpdateGroupSettings(context.Background(), 1, settings); err == nil {
		t.Fatalf("expected error when both merchant and interface are set")
	}
}

func TestHandleBotRemovedFromGroupResetsBindings(t *testing.T) {
	repo := &stubGroupRepository{
		storedGroup: &models.Group{
			TelegramID: 1,
			Settings: models.GroupSettings{
				MerchantID:   456,
				InterfaceIDs: []string{"iface-1", "iface-2"},
			},
		},
	}
	service := NewGroupService(repo)

	if err := service.HandleBotRemovedFromGroup(context.Background(), 1, "left"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.storedGroup.Settings.MerchantID != 0 {
		t.Fatalf("expected merchant ID to be cleared, got %d", repo.storedGroup.Settings.MerchantID)
	}

	if len(repo.storedGroup.Settings.InterfaceIDs) != 0 {
		t.Fatalf("expected interface IDs to be cleared, got %v", repo.storedGroup.Settings.InterfaceIDs)
	}

	if repo.storedGroup.Tier != models.GroupTierBasic {
		t.Fatalf("expected tier to downgrade to basic, got %s", repo.storedGroup.Tier)
	}
}

var _ repository.GroupRepository = (*stubGroupRepository)(nil)
