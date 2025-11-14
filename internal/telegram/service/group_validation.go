package service

import (
	"context"
	"fmt"
	"slices"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
)

// GroupValidationResult 保存群组校验结果
type GroupValidationResult struct {
	TotalGroups int                    // 数据库中的群组总数
	Issues      []GroupValidationIssue // 需要关注的群组
}

// GroupValidationIssue 描述单个群组存在的问题
type GroupValidationIssue struct {
	GroupID    int64
	Title      string
	StoredTier models.GroupTier
	BotStatus  string
	Problems   []string
}

// ValidateGroups 校验所有群组数据并返回发现的问题
func (s *GroupServiceImpl) ValidateGroups(ctx context.Context) (*GroupValidationResult, error) {
	groups, err := s.groupRepo.ListAllGroups(ctx)
	if err != nil {
		logger.L().Errorf("Failed to list groups for validation: %v", err)
		return nil, fmt.Errorf("获取群组列表失败")
	}

	result := &GroupValidationResult{
		TotalGroups: len(groups),
	}

	for _, group := range groups {
		if group == nil {
			continue
		}
		problems := collectGroupValidationProblems(group)
		if len(problems) == 0 {
			continue
		}

		title := group.Title
		if title == "" {
			title = "(未命名群组)"
		}

		result.Issues = append(result.Issues, GroupValidationIssue{
			GroupID:    group.TelegramID,
			Title:      title,
			StoredTier: group.Tier,
			BotStatus:  group.BotStatus,
			Problems:   problems,
		})
	}

	slices.SortFunc(result.Issues, func(a, b GroupValidationIssue) int {
		switch {
		case a.GroupID < b.GroupID:
			return -1
		case a.GroupID > b.GroupID:
			return 1
		default:
			return 0
		}
	})

	logger.L().Infof("Group validation finished: total=%d issues=%d", result.TotalGroups, len(result.Issues))
	return result, nil
}

func collectGroupValidationProblems(group *models.Group) []string {
	problems := make([]string, 0, 4)

	expectedTier, err := models.DetermineGroupTier(group.Settings)
	if err != nil {
		problems = append(problems, fmt.Sprintf("群组配置冲突: %v", err))
	} else {
		normalizedTier := models.NormalizeGroupTier(group.Tier)
		if group.Tier == "" {
			problems = append(problems, fmt.Sprintf("缺少 tier 字段，应写入：%s", expectedTier))
		} else if normalizedTier != expectedTier {
			problems = append(problems, fmt.Sprintf("tier=%s，但根据配置应为 %s", group.Tier, expectedTier))
		}
	}

	if group.Settings.SifangAutoLookupEnabled && !group.Settings.SifangEnabled {
		problems = append(problems, "已开启「🔍 四方自动查单」，但「🏦 四方支付查询」处于关闭状态")
	}

	switch group.BotStatus {
	case models.BotStatusActive, models.BotStatusKicked, models.BotStatusLeft:
	default:
		problems = append(problems, fmt.Sprintf("未知 bot_status：%s", group.BotStatus))
	}

	if group.BotJoinedAt.IsZero() {
		problems = append(problems, "缺少 bot_joined_at")
	}
	if group.CreatedAt.IsZero() {
		problems = append(problems, "缺少 created_at")
	}
	if group.UpdatedAt.IsZero() {
		problems = append(problems, "缺少 updated_at")
	}
	if group.Stats.LastMessageAt.IsZero() {
		problems = append(problems, "缺少 stats.last_message_at")
	}
	if group.MemberCount < 0 {
		problems = append(problems, "member_count 小于 0")
	}
	if group.Stats.TotalMessages < 0 {
		problems = append(problems, "stats.total_messages 小于 0")
	}

	return problems
}
