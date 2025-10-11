package service

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// messageService 消息服务实现
type messageService struct {
	messageRepo repository.MessageRepository
}

// NewMessageService 创建消息服务实例
func NewMessageService(messageRepo repository.MessageRepository) MessageService {
	return &messageService{
		messageRepo: messageRepo,
	}
}

// RecordMessage 记录消息
func (s *messageService) RecordMessage(ctx context.Context, message *models.Message) error {
	if err := s.messageRepo.Create(ctx, message); err != nil {
		logger.L().Errorf("Failed to record message: chat_id=%d, message_id=%d, error=%v",
			message.ChatID, message.TelegramID, err)
		return fmt.Errorf("记录消息失败")
	}

	logger.L().Infof("Message recorded: chat_id=%d, message_id=%d, type=%s, user_id=%d",
		message.ChatID, message.TelegramID, message.MessageType, message.UserID)
	return nil
}

// RecordEdit 记录消息编辑
func (s *messageService) RecordEdit(ctx context.Context, message *models.Message) error {
	if err := s.messageRepo.RecordEdit(ctx, message); err != nil {
		logger.L().Errorf("Failed to record message edit: chat_id=%d, message_id=%d, error=%v",
			message.ChatID, message.TelegramID, err)
		return fmt.Errorf("记录消息编辑失败")
	}

	logger.L().Infof("Message edit recorded: chat_id=%d, message_id=%d, user_id=%d",
		message.ChatID, message.TelegramID, message.UserID)
	return nil
}

// HandleMediaMessage 处理媒体消息
func (s *messageService) HandleMediaMessage(ctx context.Context, message *models.Message) error {
	// 验证是否为媒体消息
	if !message.IsMedia() {
		return fmt.Errorf("不是媒体消息")
	}

	// 记录消息
	if err := s.RecordMessage(ctx, message); err != nil {
		return err
	}

	logger.L().Infof("Media message handled: type=%s, file_id=%s, size=%d bytes",
		message.MessageType, message.FileID, message.FileSize)
	return nil
}

// GetChatHistory 获取聊天历史
func (s *messageService) GetChatHistory(ctx context.Context, chatID int64, limit int) ([]*models.Message, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("无效的聊天 ID")
	}

	messages, err := s.messageRepo.GetChatMessages(ctx, chatID, limit)
	if err != nil {
		logger.L().Errorf("Failed to get chat history: chat_id=%d, error=%v", chatID, err)
		return nil, fmt.Errorf("获取聊天历史失败")
	}

	logger.L().Debugf("Retrieved chat history: chat_id=%d, count=%d", chatID, len(messages))
	return messages, nil
}

// GetUserMessages 获取用户消息历史
func (s *messageService) GetUserMessages(ctx context.Context, userID int64, limit int) ([]*models.Message, error) {
	if userID == 0 {
		return nil, fmt.Errorf("无效的用户 ID")
	}

	messages, err := s.messageRepo.GetUserMessages(ctx, userID, limit)
	if err != nil {
		logger.L().Errorf("Failed to get user messages: user_id=%d, error=%v", userID, err)
		return nil, fmt.Errorf("获取用户消息历史失败")
	}

	logger.L().Debugf("Retrieved user messages: user_id=%d, count=%d", userID, len(messages))
	return messages, nil
}

// GetMessage 获取单条消息
func (s *messageService) GetMessage(ctx context.Context, chatID, messageID int64) (*models.Message, error) {
	if chatID == 0 || messageID == 0 {
		return nil, fmt.Errorf("无效的消息标识")
	}

	message, err := s.messageRepo.GetByTelegramID(ctx, chatID, messageID)
	if err != nil {
		logger.L().Debugf("Message not found: chat_id=%d, message_id=%d", chatID, messageID)
		return nil, fmt.Errorf("消息不存在")
	}

	return message, nil
}

// CountChatMessages 统计聊天消息数量
func (s *messageService) CountChatMessages(ctx context.Context, chatID int64) (int64, error) {
	if chatID == 0 {
		return 0, fmt.Errorf("无效的聊天 ID")
	}

	count, err := s.messageRepo.CountChatMessages(ctx, chatID)
	if err != nil {
		logger.L().Errorf("Failed to count messages: chat_id=%d, error=%v", chatID, err)
		return 0, fmt.Errorf("统计消息数量失败")
	}

	return count, nil
}
