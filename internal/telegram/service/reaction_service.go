package service

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// reactionService 反应服务实现
type reactionService struct {
	reactionRepo repository.ReactionRepository
}

// NewReactionService 创建反应服务
func NewReactionService(reactionRepo repository.ReactionRepository) ReactionService {
	return &reactionService{
		reactionRepo: reactionRepo,
	}
}

// HandleReaction 处理消息反应
func (s *reactionService) HandleReaction(ctx context.Context, reaction *models.MessageReactionRecord) error {
	// 验证反应数据
	if err := s.ValidateReaction(reaction); err != nil {
		logger.L().WithError(err).Warn("反应数据验证失败")
		return err
	}

	// 记录反应
	if err := s.reactionRepo.RecordReaction(ctx, reaction); err != nil {
		logger.L().WithError(err).Error("记录消息反应失败")
		return fmt.Errorf("记录反应失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":    reaction.ChatID,
		"message_id": reaction.MessageID,
		"user_id":    reaction.UserID,
		"reactions":  len(reaction.Reactions),
	}).Info("消息反应记录成功")

	return nil
}

// HandleReactionCount 处理反应统计更新
func (s *reactionService) HandleReactionCount(ctx context.Context, count *models.MessageReactionCountRecord) error {
	// 更新反应统计
	if err := s.reactionRepo.UpdateReactionCount(ctx, count); err != nil {
		logger.L().WithError(err).Error("更新反应统计失败")
		return fmt.Errorf("更新统计失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":     count.ChatID,
		"message_id":  count.MessageID,
		"total_count": count.TotalCount,
	}).Info("反应统计更新成功")

	return nil
}

// GetMessageReactions 获取消息的所有反应
func (s *reactionService) GetMessageReactions(ctx context.Context, chatID, messageID int64) ([]*models.MessageReactionRecord, error) {
	reactions, err := s.reactionRepo.GetMessageReactions(ctx, chatID, messageID)
	if err != nil {
		logger.L().WithError(err).WithFields(map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
		}).Error("获取消息反应失败")
		return nil, fmt.Errorf("获取反应失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"count":      len(reactions),
	}).Info("消息反应获取成功")

	return reactions, nil
}

// GetReactionStatistics 获取消息反应统计
func (s *reactionService) GetReactionStatistics(ctx context.Context, chatID, messageID int64) (*models.MessageReactionCountRecord, error) {
	count, err := s.reactionRepo.GetReactionCount(ctx, chatID, messageID)
	if err != nil {
		logger.L().WithError(err).WithFields(map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
		}).Error("获取反应统计失败")
		return nil, fmt.Errorf("获取统计失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":     chatID,
		"message_id":  messageID,
		"total_count": count.TotalCount,
	}).Info("反应统计获取成功")

	return count, nil
}

// GetTopReactedMessages 获取反应最多的消息
func (s *reactionService) GetTopReactedMessages(ctx context.Context, chatID int64, limit int) ([]*models.MessageReactionCountRecord, error) {
	counts, err := s.reactionRepo.GetTopReactedMessages(ctx, chatID, limit)
	if err != nil {
		logger.L().WithError(err).WithField("chat_id", chatID).Error("获取热门消息失败")
		return nil, fmt.Errorf("获取热门消息失败: %w", err)
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id": chatID,
		"count":   len(counts),
	}).Info("热门消息获取成功")

	return counts, nil
}

// ValidateReaction 验证反应是否合法
func (s *reactionService) ValidateReaction(reaction *models.MessageReactionRecord) error {
	// 验证必填字段
	if reaction.ChatID == 0 {
		return fmt.Errorf("chat_id 不能为空")
	}
	if reaction.MessageID == 0 {
		return fmt.Errorf("message_id 不能为空")
	}
	if reaction.UserID == 0 {
		return fmt.Errorf("user_id 不能为空")
	}

	// 验证反应列表
	if len(reaction.Reactions) == 0 {
		// 允许空反应列表（表示移除所有反应）
		return nil
	}

	// 验证每个反应
	for _, r := range reaction.Reactions {
		if r.Type == "" {
			return fmt.Errorf("反应类型不能为空")
		}
		if r.Type != "emoji" && r.Type != "custom_emoji" {
			return fmt.Errorf("反应类型无效: %s", r.Type)
		}
		if r.Type == "emoji" && r.Emoji == "" {
			return fmt.Errorf("emoji 类型的反应必须包含 emoji 字段")
		}
	}

	return nil
}
