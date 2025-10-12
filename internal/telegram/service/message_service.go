package service

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// MessageServiceImpl 消息服务实现
type MessageServiceImpl struct {
	messageRepo repository.MessageRepository
	groupRepo   repository.GroupRepository
}

// NewMessageService 创建消息服务
func NewMessageService(messageRepo repository.MessageRepository, groupRepo repository.GroupRepository) MessageService {
	return &MessageServiceImpl{
		messageRepo: messageRepo,
		groupRepo:   groupRepo,
	}
}

// HandleTextMessage 处理文本消息
func (s *MessageServiceImpl) HandleTextMessage(ctx context.Context, msg *TextMessageInfo) error {
	message := &models.Message{
		TelegramMessageID: msg.TelegramMessageID,
		ChatID:            msg.ChatID,
		UserID:            msg.UserID,
		MessageType:       models.MessageTypeText,
		Text:              msg.Text,
		ReplyToMessageID:  msg.ReplyToMessageID,
		SentAt:            msg.SentAt,
	}

	if err := s.messageRepo.CreateMessage(ctx, message); err != nil {
		logger.L().Errorf("Failed to create text message: chat_id=%d, message_id=%d, error=%v",
			msg.ChatID, msg.TelegramMessageID, err)
		return fmt.Errorf("failed to record text message: %w", err)
	}

	// 更新群组统计信息
	s.updateGroupStats(ctx, msg.ChatID, msg.SentAt)

	logger.L().Infof("Text message recorded: chat_id=%d, message_id=%d, user_id=%d",
		msg.ChatID, msg.TelegramMessageID, msg.UserID)
	return nil
}

// HandleMediaMessage 处理媒体消息
func (s *MessageServiceImpl) HandleMediaMessage(ctx context.Context, msg *MediaMessageInfo) error {
	message := &models.Message{
		TelegramMessageID: msg.TelegramMessageID,
		ChatID:            msg.ChatID,
		UserID:            msg.UserID,
		MessageType:       msg.MessageType,
		Caption:           msg.Caption,
		MediaFileID:       msg.MediaFileID,
		MediaFileSize:     msg.MediaFileSize,
		MediaMimeType:     msg.MediaMimeType,
		SentAt:            msg.SentAt,
	}

	if err := s.messageRepo.CreateMessage(ctx, message); err != nil {
		logger.L().Errorf("Failed to create media message: chat_id=%d, message_id=%d, type=%s, error=%v",
			msg.ChatID, msg.TelegramMessageID, msg.MessageType, err)
		return fmt.Errorf("failed to record media message: %w", err)
	}

	// 更新群组统计信息
	s.updateGroupStats(ctx, msg.ChatID, msg.SentAt)

	logger.L().Infof("Media message recorded: chat_id=%d, message_id=%d, type=%s, user_id=%d",
		msg.ChatID, msg.TelegramMessageID, msg.MessageType, msg.UserID)
	return nil
}

// HandleEditedMessage 处理消息编辑
func (s *MessageServiceImpl) HandleEditedMessage(ctx context.Context, telegramMessageID, chatID int64, newText string, editedAt time.Time) error {
	if err := s.messageRepo.UpdateMessageEdit(ctx, telegramMessageID, chatID, newText, editedAt); err != nil {
		logger.L().Errorf("Failed to update edited message: chat_id=%d, message_id=%d, error=%v",
			chatID, telegramMessageID, err)
		return fmt.Errorf("failed to record message edit: %w", err)
	}

	logger.L().Infof("Message edit recorded: chat_id=%d, message_id=%d", chatID, telegramMessageID)
	return nil
}

// RecordChannelPost 记录频道消息
func (s *MessageServiceImpl) RecordChannelPost(ctx context.Context, msg *ChannelPostInfo) error {
	message := &models.Message{
		TelegramMessageID: msg.TelegramMessageID,
		ChatID:            msg.ChatID,
		UserID:            0, // 频道消息没有 user_id
		MessageType:       msg.MessageType,
		Text:              msg.Text,
		MediaFileID:       msg.MediaFileID,
		SentAt:            msg.SentAt,
	}

	if err := s.messageRepo.CreateMessage(ctx, message); err != nil {
		logger.L().Errorf("Failed to create channel post: chat_id=%d, message_id=%d, error=%v",
			msg.ChatID, msg.TelegramMessageID, err)
		return fmt.Errorf("failed to record channel post: %w", err)
	}

	logger.L().Infof("Channel post recorded: chat_id=%d, message_id=%d, type=%s",
		msg.ChatID, msg.TelegramMessageID, msg.MessageType)
	return nil
}

// GetChatMessageHistory 获取聊天消息历史
func (s *MessageServiceImpl) GetChatMessageHistory(ctx context.Context, chatID int64, limit int) ([]*models.Message, error) {
	messages, err := s.messageRepo.ListMessagesByChat(ctx, chatID, int64(limit), 0)
	if err != nil {
		logger.L().Errorf("Failed to get chat message history: chat_id=%d, error=%v", chatID, err)
		return nil, fmt.Errorf("failed to get message history: %w", err)
	}

	return messages, nil
}

// updateGroupStats 更新群组统计信息（内部辅助方法）
func (s *MessageServiceImpl) updateGroupStats(ctx context.Context, chatID int64, messageTime time.Time) {
	// 获取当前群组信息
	group, err := s.groupRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		logger.L().Warnf("Failed to get group for stats update: chat_id=%d, error=%v", chatID, err)
		return
	}

	// 更新统计信息
	stats := group.Stats
	stats.TotalMessages++
	stats.LastMessageAt = messageTime

	if err := s.groupRepo.UpdateStats(ctx, chatID, stats); err != nil {
		logger.L().Warnf("Failed to update group stats: chat_id=%d, error=%v", chatID, err)
		// 不返回错误，仅记录日志
	}
}
