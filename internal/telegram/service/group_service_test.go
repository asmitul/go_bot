package service

import (
	"context"
	"errors"
	"testing"

	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

type stubGroupRepository struct {
	storedGroup *models.Group
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

func (s *stubGroupRepository) UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	if s.storedGroup != nil {
		s.storedGroup.Settings = settings
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
}

var _ repository.GroupRepository = (*stubGroupRepository)(nil)
