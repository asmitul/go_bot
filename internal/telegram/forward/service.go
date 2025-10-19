package forward

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
	"go_bot/internal/telegram/service"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// Service 转发服务实现
type Service struct {
	channelID         int64
	groupService      service.GroupService
	userService       service.UserService
	forwardRecordRepo repository.ForwardRecordRepository
}

// NewService 创建转发服务实例
func NewService(
	channelID int64,
	groupService service.GroupService,
	userService service.UserService,
	forwardRecordRepo repository.ForwardRecordRepository,
) *Service {
	return &Service{
		channelID:         channelID,
		groupService:      groupService,
		userService:       userService,
		forwardRecordRepo: forwardRecordRepo,
	}
}

// HandleChannelMessage 处理频道消息并启动转发任务
func (s *Service) HandleChannelMessage(ctx context.Context, botInterface interface{}, updateInterface interface{}) error {
	// 类型断言
	botInstance, ok := botInterface.(*bot.Bot)
	if !ok {
		return fmt.Errorf("invalid bot instance type")
	}

	update, ok := updateInterface.(*botModels.Update)
	if !ok {
		return fmt.Errorf("invalid update type")
	}

	if update.ChannelPost == nil {
		return nil
	}

	// 检查是否来自配置的频道
	if update.ChannelPost.Chat.ID != s.channelID {
		logger.L().Debugf("Channel message from %d, expected %d, skipping", update.ChannelPost.Chat.ID, s.channelID)
		return nil
	}

	// 生成任务 ID
	taskID := uuid.New().String()

	// 查询所有符合条件的群组
	groups, err := s.groupService.ListActiveGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list active groups: %w", err)
	}

	// 过滤启用转发且已绑定商户号的群组
	var targetGroups []*models.Group
	for _, group := range groups {
		if group.Settings.ForwardEnabled && group.Settings.MerchantID != "" {
			targetGroups = append(targetGroups, group)
		}
	}

	if len(targetGroups) == 0 {
		logger.L().Info("No target groups with forward enabled, skipping forward")
		return nil
	}

	logger.L().Infof("Starting forward task: task_id=%s, channel_message_id=%d, target_groups=%d",
		taskID, update.ChannelPost.ID, len(targetGroups))

	// 异步执行转发任务
	go s.forwardTask(context.Background(), botInstance, update.ChannelPost, targetGroups, taskID)

	return nil
}

// forwardTask 异步转发任务
func (s *Service) forwardTask(ctx context.Context, botInstance *bot.Bot, message *botModels.Message, groups []*models.Group, taskID string) {
	startTime := time.Now()
	limiter := NewRateLimiter(30) // 30条/秒
	defer limiter.Close()

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failedCount := 0
	records := make([]*models.ForwardRecord, 0, len(groups))

	// 并发转发到所有群组
	for _, group := range groups {
		wg.Add(1)
		go func(g *models.Group) {
			defer wg.Done()

			forwardedMsgID, err := s.forwardToGroup(ctx, botInstance, message, g.TelegramID, limiter)

			mu.Lock()
			defer mu.Unlock()

			status := models.ForwardStatusFailed
			if err == nil {
				successCount++
				status = models.ForwardStatusSuccess
				logger.L().Debugf("Forwarded to group %d: message_id=%d", g.TelegramID, forwardedMsgID)
			} else {
				failedCount++
				logger.L().Errorf("Failed to forward to group %d: %v", g.TelegramID, err)
			}

			records = append(records, &models.ForwardRecord{
				TaskID:             taskID,
				ChannelMessageID:   int64(message.ID),
				TargetGroupID:      g.TelegramID,
				ForwardedMessageID: forwardedMsgID,
				Status:             status,
				CreatedAt:          time.Now(),
			})
		}(group)
	}

	// 等待所有转发完成
	wg.Wait()

	// 批量插入记录
	if len(records) > 0 {
		if err := s.forwardRecordRepo.BulkCreateRecords(ctx, records); err != nil {
			logger.L().Errorf("Failed to save forward records: %v", err)
		}
	}

	duration := time.Since(startTime)
	logger.L().Infof("Forward task completed: task_id=%s, success=%d, failed=%d, duration=%v",
		taskID, successCount, failedCount, duration)

	// 发送报告给管理员
	s.sendReportToAdmins(ctx, botInstance, taskID, successCount, failedCount, duration)
}

// forwardToGroup 转发到单个群组（带重试）
func (s *Service) forwardToGroup(ctx context.Context, botInstance *bot.Bot, message *botModels.Message, groupID int64, limiter *RateLimiter) (int64, error) {
	for i := 0; i < 3; i++ {
		// 等待速率限制
		if err := limiter.Wait(ctx); err != nil {
			return 0, fmt.Errorf("rate limiter wait error: %w", err)
		}

		// 尝试转发消息
		msg, err := botInstance.ForwardMessage(ctx, &bot.ForwardMessageParams{
			ChatID:     groupID,
			FromChatID: message.Chat.ID,
			MessageID:  message.ID,
		})

		if err == nil {
			return int64(msg.ID), nil
		}

		// 如果不是最后一次重试，等待2秒后重试
		if i < 2 {
			logger.L().Warnf("Forward attempt %d failed for group %d: %v, retrying in 2s", i+1, groupID, err)
			time.Sleep(2 * time.Second)
		}
	}

	return 0, fmt.Errorf("failed after 3 retries")
}

// RecallForwardedMessages 撤回转发消息
func (s *Service) RecallForwardedMessages(ctx context.Context, botInterface interface{}, taskID string, requesterID int64) (int, int, error) {
	// 验证权限
	isAdmin, err := s.userService.CheckAdminPermission(ctx, requesterID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to check permission: %w", err)
	}
	if !isAdmin {
		return 0, 0, fmt.Errorf("permission denied: only admins can recall messages")
	}

	// 类型断言
	botInstance, ok := botInterface.(*bot.Bot)
	if !ok {
		return 0, 0, fmt.Errorf("invalid bot instance type")
	}

	// 查询转发记录
	records, err := s.forwardRecordRepo.GetSuccessRecordsByTaskID(ctx, taskID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get forward records: %w", err)
	}

	if len(records) == 0 {
		return 0, 0, fmt.Errorf("no records found for task %s", taskID)
	}

	logger.L().Infof("Starting recall: task_id=%s, total_records=%d", taskID, len(records))

	// 批量删除消息
	limiter := NewRateLimiter(30)
	defer limiter.Close()

	successCount := 0
	failedCount := 0

	for _, record := range records {
		if err := limiter.Wait(ctx); err != nil {
			logger.L().Errorf("Rate limiter wait error during recall: %v", err)
			break
		}

		_, err := botInstance.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    record.TargetGroupID,
			MessageID: int(record.ForwardedMessageID),
		})

		if err == nil {
			successCount++
		} else {
			failedCount++
			logger.L().Warnf("Failed to delete message: group=%d, msg_id=%d, err=%v",
				record.TargetGroupID, record.ForwardedMessageID, err)
		}
	}

	// 删除记录
	if err := s.forwardRecordRepo.DeleteRecordsByTaskID(ctx, taskID); err != nil {
		logger.L().Errorf("Failed to delete forward records: %v", err)
	}

	logger.L().Infof("Recall completed: task_id=%s, success=%d, failed=%d", taskID, successCount, failedCount)
	return successCount, failedCount, nil
}

// sendReportToAdmins 发送报告给所有管理员
func (s *Service) sendReportToAdmins(ctx context.Context, botInstance *bot.Bot, taskID string, successCount, failedCount int, duration time.Duration) {
	// 查询所有管理员
	admins, err := s.userService.ListAllAdmins(ctx)
	if err != nil {
		logger.L().Errorf("Failed to list admins: %v", err)
		return
	}

	if len(admins) == 0 {
		logger.L().Warn("No admins found, skipping report")
		return
	}

	// 构造报告消息
	reportText := fmt.Sprintf(
		"📊 频道消息转发完成\n\n"+
			"✅ 成功: %d 个群组\n"+
			"❌ 失败: %d 个群组\n"+
			"⏱️ 耗时: %.2f 秒",
		successCount, failedCount, duration.Seconds(),
	)

	// 添加撤回按钮
	keyboard := &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{{Text: "🗑️ 撤回所有消息", CallbackData: fmt.Sprintf("recall:%s", taskID)}},
		},
	}

	// 发送给所有管理员
	for _, admin := range admins {
		_, err := botInstance.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      admin.TelegramID,
			Text:        reportText,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			logger.L().Errorf("Failed to send report to admin %d: %v", admin.TelegramID, err)
		} else {
			logger.L().Infof("Sent forward report to admin %d", admin.TelegramID)
		}
	}
}
