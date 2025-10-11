package service

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// memberService 成员服务实现
type memberService struct {
	memberRepo repository.MemberRepository
	groupRepo  repository.GroupRepository
}

// NewMemberService 创建成员服务实例
func NewMemberService(memberRepo repository.MemberRepository, groupRepo repository.GroupRepository) MemberService {
	return &memberService{
		memberRepo: memberRepo,
		groupRepo:  groupRepo,
	}
}

// HandleMemberChange 处理成员状态变化
func (s *memberService) HandleMemberChange(ctx context.Context, event *models.ChatMemberEvent) error {
	// 记录事件
	if err := s.memberRepo.RecordEvent(ctx, event); err != nil {
		logger.L().Errorf("Failed to record member event: chat_id=%d, user_id=%d, error=%v",
			event.ChatID, event.UserID, err)
		return fmt.Errorf("记录成员事件失败")
	}

	logger.L().Infof("Member event recorded: chat_id=%d, user_id=%d, event_type=%s, status=%s->%s",
		event.ChatID, event.UserID, event.EventType, event.OldStatus, event.NewStatus)

	return nil
}

// SendWelcomeMessage 发送欢迎消息（由 handler 调用）
func (s *memberService) SendWelcomeMessage(ctx context.Context, chatID, userID int64) (bool, string, error) {
	// 检查群组设置
	group, err := s.groupRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		logger.L().Warnf("Failed to get group settings for welcome: chat_id=%d, error=%v", chatID, err)
		return false, "", nil // 不发送欢迎消息，但不报错
	}

	// 检查是否启用欢迎消息
	if !group.Settings.WelcomeEnabled {
		logger.L().Debugf("Welcome message disabled for chat: chat_id=%d", chatID)
		return false, "", nil
	}

	// 获取欢迎文本
	welcomeText := group.Settings.WelcomeText
	if welcomeText == "" {
		welcomeText = "👋 欢迎加入群组！" // 默认欢迎消息
	}

	logger.L().Infof("Sending welcome message: chat_id=%d, user_id=%d", chatID, userID)
	return true, welcomeText, nil
}

// HandleJoinRequest 处理入群申请
func (s *memberService) HandleJoinRequest(ctx context.Context, request *models.JoinRequest) error {
	// 创建入群请求记录
	if err := s.memberRepo.CreateJoinRequest(ctx, request); err != nil {
		logger.L().Errorf("Failed to create join request: chat_id=%d, user_id=%d, error=%v",
			request.ChatID, request.UserID, err)
		return fmt.Errorf("记录入群请求失败")
	}

	logger.L().Infof("Join request created: chat_id=%d, user_id=%d, username=%s",
		request.ChatID, request.UserID, request.Username)

	return nil
}

// ApproveJoinRequest 批准入群请求
func (s *memberService) ApproveJoinRequest(ctx context.Context, chatID, userID, reviewerID int64, reviewerUsername string) error {
	// 获取请求
	request, err := s.memberRepo.GetJoinRequestByUser(ctx, chatID, userID)
	if err != nil {
		logger.L().Errorf("Failed to get join request: chat_id=%d, user_id=%d, error=%v",
			chatID, userID, err)
		return fmt.Errorf("入群请求不存在")
	}

	// 检查状态
	if !request.IsPending() {
		return fmt.Errorf("该请求已处理（状态：%s）", request.Status)
	}

	// 更新状态
	if err := s.memberRepo.UpdateJoinRequestStatus(ctx, chatID, userID, models.JoinRequestStatusApproved, ""); err != nil {
		logger.L().Errorf("Failed to approve join request: chat_id=%d, user_id=%d, error=%v",
			chatID, userID, err)
		return fmt.Errorf("批准请求失败")
	}

	logger.L().Infof("Join request approved: chat_id=%d, user_id=%d, reviewer_id=%d",
		chatID, userID, reviewerID)

	return nil
}

// RejectJoinRequest 拒绝入群请求
func (s *memberService) RejectJoinRequest(ctx context.Context, chatID, userID, reviewerID int64, reviewerUsername, reason string) error {
	// 获取请求
	request, err := s.memberRepo.GetJoinRequestByUser(ctx, chatID, userID)
	if err != nil {
		logger.L().Errorf("Failed to get join request: chat_id=%d, user_id=%d, error=%v",
			chatID, userID, err)
		return fmt.Errorf("入群请求不存在")
	}

	// 检查状态
	if !request.IsPending() {
		return fmt.Errorf("该请求已处理（状态：%s）", request.Status)
	}

	// 更新状态
	if err := s.memberRepo.UpdateJoinRequestStatus(ctx, chatID, userID, models.JoinRequestStatusRejected, reason); err != nil {
		logger.L().Errorf("Failed to reject join request: chat_id=%d, user_id=%d, error=%v",
			chatID, userID, err)
		return fmt.Errorf("拒绝请求失败")
	}

	logger.L().Infof("Join request rejected: chat_id=%d, user_id=%d, reviewer_id=%d, reason=%s",
		chatID, userID, reviewerID, reason)

	return nil
}

// GetPendingJoinRequests 获取待审批的入群请求列表
func (s *memberService) GetPendingJoinRequests(ctx context.Context, chatID int64) ([]*models.JoinRequest, error) {
	requests, err := s.memberRepo.GetPendingRequests(ctx, chatID)
	if err != nil {
		logger.L().Errorf("Failed to get pending requests: chat_id=%d, error=%v", chatID, err)
		return nil, fmt.Errorf("获取入群请求失败")
	}

	logger.L().Debugf("Retrieved pending requests: chat_id=%d, count=%d", chatID, len(requests))
	return requests, nil
}

// GetChatMemberHistory 获取群组成员历史
func (s *memberService) GetChatMemberHistory(ctx context.Context, chatID int64, limit int) ([]*models.ChatMemberEvent, error) {
	events, err := s.memberRepo.GetChatEvents(ctx, chatID, limit)
	if err != nil {
		logger.L().Errorf("Failed to get chat member history: chat_id=%d, error=%v", chatID, err)
		return nil, fmt.Errorf("获取成员历史失败")
	}

	logger.L().Debugf("Retrieved chat member history: chat_id=%d, count=%d", chatID, len(events))
	return events, nil
}

// UpdateWelcomeSettings 更新欢迎消息设置
func (s *memberService) UpdateWelcomeSettings(ctx context.Context, chatID int64, enabled bool, text string) error {
	// 获取群组
	group, err := s.groupRepo.GetByTelegramID(ctx, chatID)
	if err != nil {
		logger.L().Errorf("Failed to get group for welcome settings: chat_id=%d, error=%v", chatID, err)
		return fmt.Errorf("群组不存在")
	}

	// 更新设置
	group.Settings.WelcomeEnabled = enabled
	group.Settings.WelcomeText = text
	group.UpdatedAt = time.Now()

	if err := s.groupRepo.UpdateSettings(ctx, chatID, group.Settings); err != nil {
		logger.L().Errorf("Failed to update welcome settings: chat_id=%d, error=%v", chatID, err)
		return fmt.Errorf("更新欢迎设置失败")
	}

	logger.L().Infof("Welcome settings updated: chat_id=%d, enabled=%v, text_length=%d",
		chatID, enabled, len(text))

	return nil
}
