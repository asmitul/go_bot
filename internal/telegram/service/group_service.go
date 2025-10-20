package service

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// GroupServiceImpl 群组服务实现
type GroupServiceImpl struct {
	groupRepo repository.GroupRepository
}

// NewGroupService 创建群组服务
func NewGroupService(groupRepo repository.GroupRepository) GroupService {
	return &GroupServiceImpl{
		groupRepo: groupRepo,
	}
}

// CreateOrUpdateGroup 创建或更新群组
func (s *GroupServiceImpl) CreateOrUpdateGroup(ctx context.Context, group *models.Group) error {
	if err := s.groupRepo.CreateOrUpdate(ctx, group); err != nil {
		logger.L().Errorf("Failed to create/update group %d: %v", group.TelegramID, err)
		return fmt.Errorf("failed to create/update group: %w", err)
	}

	logger.L().Infof("Group %d (%s) created/updated", group.TelegramID, group.Title)
	return nil
}

// GetGroupInfo 获取群组信息
func (s *GroupServiceImpl) GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error) {
	group, err := s.groupRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		logger.L().Errorf("Failed to get group info for %d: %v", telegramID, err)
		return nil, fmt.Errorf("获取群组信息失败")
	}
	return group, nil
}

// GetOrCreateGroup 获取或创建群组记录（智能处理，群组不存在时自动创建）
func (s *GroupServiceImpl) GetOrCreateGroup(ctx context.Context, chatInfo *TelegramChatInfo) (*models.Group, error) {
	// 先尝试获取
	group, err := s.groupRepo.GetByTelegramID(ctx, chatInfo.ChatID)
	if err == nil {
		return group, nil
	}

	// 不存在则创建默认群组记录
	logger.L().Infof("Group %d not found, auto-creating...", chatInfo.ChatID)

	newGroup := &models.Group{
		TelegramID: chatInfo.ChatID,
		Type:       chatInfo.Type,
		Title:      chatInfo.Title,
		Username:   chatInfo.Username,
		BotStatus:  models.BotStatusActive,
		Settings:   models.GroupSettings{}, // 默认配置（所有功能关闭）
		Stats:      models.GroupStats{},
		// BotJoinedAt、CreatedAt、UpdatedAt 由 CreateOrUpdate 的 $setOnInsert 自动设置
	}

	if err := s.groupRepo.CreateOrUpdate(ctx, newGroup); err != nil {
		logger.L().Errorf("Failed to auto-create group %d: %v", chatInfo.ChatID, err)
		return nil, fmt.Errorf("自动创建群组失败")
	}

	logger.L().Infof("Auto-created group record: chat_id=%d, title=%s", chatInfo.ChatID, chatInfo.Title)
	return newGroup, nil
}

// MarkBotLeft 标记 Bot 离开群组
func (s *GroupServiceImpl) MarkBotLeft(ctx context.Context, telegramID int64) error {
	if err := s.groupRepo.MarkBotLeft(ctx, telegramID); err != nil {
		logger.L().Errorf("Failed to mark bot left for group %d: %v", telegramID, err)
		return fmt.Errorf("标记失败: %w", err)
	}

	logger.L().Infof("Bot left group %d", telegramID)
	return nil
}

// ListActiveGroups 列出所有活跃群组
func (s *GroupServiceImpl) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	groups, err := s.groupRepo.ListActiveGroups(ctx)
	if err != nil {
		logger.L().Errorf("Failed to list active groups: %v", err)
		return nil, fmt.Errorf("获取活跃群组列表失败")
	}
	return groups, nil
}

// UpdateGroupSettings 更新群组配置
func (s *GroupServiceImpl) UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	if err := s.groupRepo.UpdateSettings(ctx, telegramID, settings); err != nil {
		logger.L().Errorf("Failed to update group settings for %d: %v", telegramID, err)
		return fmt.Errorf("更新群组配置失败: %w", err)
	}

	logger.L().Infof("Group settings updated: group_id=%d", telegramID)
	return nil
}

// LeaveGroup Bot 离开群组（删除群组记录）
func (s *GroupServiceImpl) LeaveGroup(ctx context.Context, telegramID int64) error {
	// 检查群组是否存在
	_, err := s.groupRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		logger.L().Errorf("Group %d not found for leave: %v", telegramID, err)
		return fmt.Errorf("群组不存在")
	}

	// 删除群组记录
	if err := s.groupRepo.DeleteGroup(ctx, telegramID); err != nil {
		logger.L().Errorf("Failed to delete group %d: %v", telegramID, err)
		return fmt.Errorf("离开群组失败: %w", err)
	}

	logger.L().Infof("Bot left and deleted group %d", telegramID)
	return nil
}

// HandleBotAddedToGroup Bot 被添加到群组
func (s *GroupServiceImpl) HandleBotAddedToGroup(ctx context.Context, group *models.Group) error {
	// 设置状态为活跃
	group.BotStatus = models.BotStatusActive

	if err := s.groupRepo.CreateOrUpdate(ctx, group); err != nil {
		logger.L().Errorf("Failed to handle bot added to group %d: %v", group.TelegramID, err)
		return fmt.Errorf("记录 Bot 加入群组失败: %w", err)
	}

	logger.L().Infof("Bot added to group %d (%s)", group.TelegramID, group.Title)
	return nil
}

// HandleBotRemovedFromGroup Bot 被移出群组
func (s *GroupServiceImpl) HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error {
	// 根据原因设置不同的状态
	status := models.BotStatusKicked
	if reason == "left" {
		status = models.BotStatusLeft
	}

	// 获取群组信息以检查商户号绑定
	group, err := s.groupRepo.GetByTelegramID(ctx, telegramID)
	if err == nil && group.Settings.MerchantID != "" {
		// 自动解绑商户号
		oldMerchantID := group.Settings.MerchantID
		settings := group.Settings
		settings.MerchantID = ""

		if err := s.groupRepo.UpdateSettings(ctx, telegramID, settings); err != nil {
			logger.L().Warnf("Failed to auto-unbind merchant ID when bot removed: group_id=%d, merchant_id=%s, err=%v",
				telegramID, oldMerchantID, err)
		} else {
			logger.L().Infof("Auto-unbound merchant ID: group_id=%d, merchant_id=%s", telegramID, oldMerchantID)
		}
	}

	// 标记 Bot 离开
	if err := s.groupRepo.MarkBotLeft(ctx, telegramID); err != nil {
		logger.L().Errorf("Failed to handle bot removed from group %d: %v", telegramID, err)
		return fmt.Errorf("记录 Bot 离开群组失败: %w", err)
	}

	logger.L().Infof("Bot removed from group %d, reason=%s, status=%s", telegramID, reason, status)
	return nil
}
