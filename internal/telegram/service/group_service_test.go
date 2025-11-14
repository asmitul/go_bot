package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

type stubGroupRepository struct {
	storedGroup     *models.Group
	allGroups       []*models.Group
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

func (s *stubGroupRepository) ListAllGroups(ctx context.Context) ([]*models.Group, error) {
	if s.allGroups != nil {
		return s.allGroups, nil
	}
	if s.storedGroup == nil {
		return nil, nil
	}
	return []*models.Group{s.storedGroup}, nil
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

func TestValidateGroupsHealthy(t *testing.T) {
	now := time.Now()
	repo := &stubGroupRepository{
		allGroups: []*models.Group{
			{
				TelegramID:  100,
				Title:       "Healthy",
				Tier:        models.GroupTierBasic,
				BotStatus:   models.BotStatusActive,
				MemberCount: 10,
				BotJoinedAt: now,
				CreatedAt:   now,
				UpdatedAt:   now,
				Stats: models.GroupStats{
					TotalMessages: 5,
					LastMessageAt: now,
				},
				Settings: models.GroupSettings{
					CalculatorEnabled:       true,
					CryptoEnabled:           true,
					CryptoFloatRate:         0.12,
					ForwardEnabled:          true,
					SifangEnabled:           true,
					SifangAutoLookupEnabled: true,
				},
			},
		},
	}

	service := NewGroupService(repo)
	result, err := service.ValidateGroups(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.TotalGroups != 1 {
		t.Fatalf("expected 1 group, got %d", result.TotalGroups)
	}

	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

func TestValidateGroupsDetectsProblems(t *testing.T) {
	repo := &stubGroupRepository{
		allGroups: []*models.Group{
			{
				TelegramID: 200,
				Tier:       "",
				BotStatus:  "mystery",
				Settings: models.GroupSettings{
					MerchantID:              123,
					InterfaceIDs:            []string{"upstream-1"},
					SifangEnabled:           false,
					SifangAutoLookupEnabled: true,
				},
				Stats: models.GroupStats{},
			},
		},
	}

	service := NewGroupService(repo)
	result, err := service.ValidateGroups(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(result.Issues))
	}

	problems := result.Issues[0].Problems
	mustContainProblem(t, problems, "配置冲突")
	mustContainProblem(t, problems, "四方自动查单")
	mustContainProblem(t, problems, "缺少 bot_joined_at")
}

func mustContainProblem(t *testing.T, problems []string, keyword string) {
	t.Helper()
	for _, problem := range problems {
		if strings.Contains(problem, keyword) {
			return
		}
	}
	t.Fatalf("expected problem containing %q, got %v", keyword, problems)
}

var _ repository.GroupRepository = (*stubGroupRepository)(nil)
