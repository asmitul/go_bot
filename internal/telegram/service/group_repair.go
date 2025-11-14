package service

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
)

// GroupRepairResult 描述自动修复结果
type GroupRepairResult struct {
	TotalGroups        int // 扫描的群组数
	UpdatedGroups      int // 实际写入的群组数
	TierFixed          int // 修复 tier 的群组数
	AutoLookupDisabled int // 自动关闭四方查单的群组数
	SkippedGroups      int // 因冲突或更新失败跳过的群组数
}

// RepairGroups 自动修复可矫正的问题，如缺少 tier 或四方开关冲突
func (s *GroupServiceImpl) RepairGroups(ctx context.Context) (*GroupRepairResult, error) {
	groups, err := s.groupRepo.ListAllGroups(ctx)
	if err != nil {
		logger.L().Errorf("Failed to list groups for repair: %v", err)
		return nil, fmt.Errorf("获取群组列表失败")
	}

	result := &GroupRepairResult{
		TotalGroups: len(groups),
	}

	for _, group := range groups {
		if group == nil {
			continue
		}

		group.Settings.InterfaceIDs = models.NormalizeInterfaceIDs(group.Settings.InterfaceIDs)

		expectedTier, tierErr := models.DetermineGroupTier(group.Settings)
		if tierErr != nil {
			// 配置冲突，无法安全修复
			logger.L().Warnf("Skip repairing group %d due to conflicting settings: %v", group.TelegramID, tierErr)
			result.SkippedGroups++
			continue
		}

		needsTierFix := models.NormalizeGroupTier(group.Tier) != expectedTier
		needsAutoLookupFix := group.Settings.SifangAutoLookupEnabled && !group.Settings.SifangEnabled

		if !needsTierFix && !needsAutoLookupFix {
			continue
		}

		settings := group.Settings
		if needsAutoLookupFix {
			settings.SifangAutoLookupEnabled = false
			result.AutoLookupDisabled++
		}

		if err := s.groupRepo.UpdateSettings(ctx, group.TelegramID, settings, expectedTier); err != nil {
			logger.L().Errorf("Failed to repair group %d: %v", group.TelegramID, err)
			result.SkippedGroups++
			continue
		}

		if needsTierFix {
			result.TierFixed++
		}
		result.UpdatedGroups++

		logger.L().Infof("Group repaired: chat_id=%d tier_fixed=%t auto_lookup_disabled=%t",
			group.TelegramID, needsTierFix, needsAutoLookupFix)
	}

	return result, nil
}
