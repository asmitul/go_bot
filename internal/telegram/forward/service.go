package forward

import (
	"context"
	"errors"
	"fmt"
	"sort"
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

const (
	forwardRatePerSecond         = 20
	recallRatePerSecond          = 20
	forwardMaxRetryAttempts      = 5
	defaultForwardRetryDelay     = 2 * time.Second
	maxForwardExponentialBackoff = 10 * time.Second
)

// Service 转发服务实现
type Service struct {
	channelID            int64
	groupService         service.GroupService
	userService          service.UserService
	forwardRecordRepo    repository.ForwardRecordRepository
	mediaGroupCollectors map[string]*MediaGroupCollector // 媒体组收集器（key: mediaGroupID）
	collectorMutex       sync.RWMutex
}

// NewService 创建转发服务实例
func NewService(
	channelID int64,
	groupService service.GroupService,
	userService service.UserService,
	forwardRecordRepo repository.ForwardRecordRepository,
) *Service {
	return &Service{
		channelID:            channelID,
		groupService:         groupService,
		userService:          userService,
		forwardRecordRepo:    forwardRecordRepo,
		mediaGroupCollectors: make(map[string]*MediaGroupCollector),
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

	// 查询所有符合条件的群组
	groups, err := s.groupService.ListActiveGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list active groups: %w", err)
	}

	// 过滤启用转发的目标群组，排除私聊
	var targetGroups []*models.Group
	for _, group := range groups {
		if !group.Settings.ForwardEnabled {
			continue
		}

		if group.Type == "private" {
			logger.L().Debugf("Skipping private chat from forward targets: chat_id=%d", group.TelegramID)
			continue
		}

		targetGroups = append(targetGroups, group)
	}

	if len(targetGroups) == 0 {
		logger.L().Info("No target groups with forward enabled, skipping forward")
		return nil
	}

	// 检查是否为媒体组
	if update.ChannelPost.MediaGroupID != "" {
		// 媒体组消息，使用收集器
		logger.L().Debugf("Media group message detected: media_group_id=%s, message_id=%d",
			update.ChannelPost.MediaGroupID, update.ChannelPost.ID)
		return s.handleMediaGroupMessage(ctx, botInstance, update.ChannelPost, targetGroups)
	}

	// 单条消息，直接转发
	taskID := uuid.New().String()
	logger.L().Infof("Starting forward task: task_id=%s, channel_message_id=%d, target_groups=%d",
		taskID, update.ChannelPost.ID, len(targetGroups))

	// 异步执行转发任务
	go s.forwardTask(context.Background(), botInstance, update.ChannelPost, targetGroups, taskID)

	return nil
}

// forwardTask 异步转发任务
func (s *Service) forwardTask(ctx context.Context, botInstance *bot.Bot, message *botModels.Message, groups []*models.Group, taskID string) {
	startTime := time.Now()
	limiter := NewRateLimiter(forwardRatePerSecond)
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
	var lastErr error
	for attempt := 1; attempt <= forwardMaxRetryAttempts; attempt++ {
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

		lastErr = err

		if !shouldRetryForward(err) {
			return 0, fmt.Errorf("failed to forward to group %d: %w", groupID, err)
		}

		if attempt < forwardMaxRetryAttempts {
			delay := calculateForwardRetryDelay(err, attempt, groupID)
			logger.L().Warnf("Forward attempt %d/%d failed for group %d: %v, retrying in %v",
				attempt, forwardMaxRetryAttempts, groupID, err, delay)
			if err := sleepWithContext(ctx, delay); err != nil {
				return 0, fmt.Errorf("forward retry interrupted for group %d: %w", groupID, err)
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("unknown forward error")
	}
	return 0, fmt.Errorf("failed to forward to group %d after %d attempts: %w",
		groupID, forwardMaxRetryAttempts, lastErr)
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
	limiter := NewRateLimiter(recallRatePerSecond)
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

// handleMediaGroupMessage 处理媒体组消息
func (s *Service) handleMediaGroupMessage(ctx context.Context, botInstance *bot.Bot, message *botModels.Message, groups []*models.Group) error {
	mediaGroupID := message.MediaGroupID

	s.collectorMutex.Lock()
	collector, exists := s.mediaGroupCollectors[mediaGroupID]
	if !exists {
		// 创建新的收集器
		collector = NewMediaGroupCollector(1500*time.Millisecond, func(messages []*botModels.Message) {
			taskID := uuid.New().String()
			logger.L().Infof("Starting media group forward task: task_id=%s, media_group_id=%s, message_count=%d, target_groups=%d",
				taskID, mediaGroupID, len(messages), len(groups))

			// 异步转发媒体组
			go s.forwardMediaGroup(context.Background(), botInstance, messages, groups, taskID)

			// 清理收集器
			s.collectorMutex.Lock()
			delete(s.mediaGroupCollectors, mediaGroupID)
			s.collectorMutex.Unlock()
		})
		s.mediaGroupCollectors[mediaGroupID] = collector
	}
	s.collectorMutex.Unlock()

	// 添加消息到收集器
	collector.Add(message)
	return nil
}

// forwardMediaGroup 批量转发媒体组
func (s *Service) forwardMediaGroup(ctx context.Context, botInstance *bot.Bot, messages []*botModels.Message, groups []*models.Group, taskID string) {
	startTime := time.Now()
	limiter := NewRateLimiter(forwardRatePerSecond)
	defer limiter.Close()

	// 提取消息 ID 列表
	messageIDs := make([]int, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}
	sort.Ints(messageIDs)

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	failedCount := 0
	records := make([]*models.ForwardRecord, 0)

	// 并发转发到所有群组
	for _, group := range groups {
		wg.Add(1)
		go func(g *models.Group) {
			defer wg.Done()

			forwardedMsgIDs, err := s.forwardMediaGroupToGroup(ctx, botInstance, messages[0].Chat.ID, messageIDs, g.TelegramID, limiter)

			mu.Lock()
			defer mu.Unlock()

			if err == nil {
				successCount++
				// 记录每条转发的消息
				for i, fwdID := range forwardedMsgIDs {
					records = append(records, &models.ForwardRecord{
						TaskID:             taskID,
						ChannelMessageID:   int64(messageIDs[i]),
						TargetGroupID:      g.TelegramID,
						ForwardedMessageID: int64(fwdID),
						Status:             models.ForwardStatusSuccess,
						CreatedAt:          time.Now(),
					})
				}
				logger.L().Debugf("Forwarded media group to group %d: %d messages", g.TelegramID, len(forwardedMsgIDs))
			} else {
				failedCount++
				logger.L().Errorf("Failed to forward media group to group %d: %v", g.TelegramID, err)
			}
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
	logger.L().Infof("Media group forward task completed: task_id=%s, media_count=%d, success=%d, failed=%d, duration=%v",
		taskID, len(messages), successCount, failedCount, duration)

	// 发送报告给管理员
	s.sendReportToAdmins(ctx, botInstance, taskID, successCount, failedCount, duration)
}

// forwardMediaGroupToGroup 转发媒体组到单个群组（带重试）
func (s *Service) forwardMediaGroupToGroup(ctx context.Context, botInstance *bot.Bot, fromChatID int64, messageIDs []int, groupID int64, limiter *RateLimiter) ([]int, error) {
	var lastErr error
	for attempt := 1; attempt <= forwardMaxRetryAttempts; attempt++ {
		// 等待速率限制
		if err := limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter wait error: %w", err)
		}

		// 使用 ForwardMessages API 批量转发
		result, err := botInstance.ForwardMessages(ctx, &bot.ForwardMessagesParams{
			ChatID:     groupID,
			FromChatID: fromChatID,
			MessageIDs: messageIDs,
		})

		if err == nil {
			// 提取转发后的消息 ID
			ids := make([]int, len(result))
			for j, msgID := range result {
				ids[j] = msgID.ID
			}
			return ids, nil
		}

		lastErr = err

		if !shouldRetryForward(err) {
			return nil, fmt.Errorf("failed to forward media group to group %d: %w", groupID, err)
		}

		if attempt < forwardMaxRetryAttempts {
			delay := calculateForwardRetryDelay(err, attempt, groupID)
			logger.L().Warnf("Media group forward attempt %d/%d failed for group %d: %v, retrying in %v",
				attempt, forwardMaxRetryAttempts, groupID, err, delay)
			if err := sleepWithContext(ctx, delay); err != nil {
				return nil, fmt.Errorf("media group retry interrupted for group %d: %w", groupID, err)
			}
		}
	}

	if lastErr == nil {
		lastErr = errors.New("unknown media group forward error")
	}
	return nil, fmt.Errorf("failed to forward media group to group %d after %d attempts: %w",
		groupID, forwardMaxRetryAttempts, lastErr)
}

func shouldRetryForward(err error) bool {
	if err == nil {
		return false
	}

	if bot.IsTooManyRequestsError(err) {
		return true
	}

	// 这类错误通常是永久性错误（比如 bot 被踢出群、群不存在、无权限），无需重试。
	if errors.Is(err, bot.ErrorForbidden) ||
		errors.Is(err, bot.ErrorBadRequest) ||
		errors.Is(err, bot.ErrorUnauthorized) ||
		errors.Is(err, bot.ErrorNotFound) {
		return false
	}

	return true
}

func calculateForwardRetryDelay(err error, attempt int, groupID int64) time.Duration {
	var tooManyErr *bot.TooManyRequestsError
	if errors.As(err, &tooManyErr) {
		retryAfter := time.Duration(tooManyErr.RetryAfter) * time.Second
		if retryAfter <= 0 {
			retryAfter = defaultForwardRetryDelay
		}
		return retryAfter + forwardRetryJitter(groupID)
	}

	if attempt < 1 {
		attempt = 1
	}

	delay := time.Second * time.Duration(1<<uint(attempt-1))
	if delay > maxForwardExponentialBackoff {
		return maxForwardExponentialBackoff
	}
	return delay
}

func forwardRetryJitter(groupID int64) time.Duration {
	if groupID < 0 {
		groupID = -groupID
	}
	return time.Duration(groupID%5+1) * 200 * time.Millisecond
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
