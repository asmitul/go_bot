package service

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// callbackService 回调服务实现
type callbackService struct {
	callbackRepo repository.CallbackRepository
}

// NewCallbackService 创建回调服务实例
func NewCallbackService(callbackRepo repository.CallbackRepository) CallbackService {
	return &callbackService{
		callbackRepo: callbackRepo,
	}
}

// LogCallback 记录回调日志
func (s *callbackService) LogCallback(ctx context.Context, callbackLog *models.CallbackLog) error {
	// 验证必填字段
	if callbackLog.CallbackQueryID == "" {
		return fmt.Errorf("callback_query_id is required")
	}

	// 设置默认值
	if callbackLog.CreatedAt.IsZero() {
		callbackLog.CreatedAt = time.Now().UTC()
	}

	// 保存到数据库
	if err := s.callbackRepo.Create(ctx, callbackLog); err != nil {
		logger.L().Errorf("Failed to log callback: query_id=%s, user_id=%d, action=%s, error=%v",
			callbackLog.CallbackQueryID, callbackLog.UserID, callbackLog.Action, err)
		return fmt.Errorf("记录回调日志失败")
	}

	logger.L().Infof("Callback logged: query_id=%s, user_id=%d, action=%s, answered=%v",
		callbackLog.CallbackQueryID, callbackLog.UserID, callbackLog.Action, callbackLog.Answered)
	return nil
}

// ParseAndHandle 解析并处理回调数据
func (s *callbackService) ParseAndHandle(ctx context.Context, data string) (*models.CallbackData, error) {
	// 解析 callback_data
	callbackData, err := models.ParseCallbackData(data)
	if err != nil {
		logger.L().Warnf("Failed to parse callback data: data=%s, error=%v", data, err)
		return nil, fmt.Errorf("回调数据格式错误")
	}

	logger.L().Debugf("Callback data parsed: action=%s, params=%v", callbackData.Action, callbackData.Params)
	return callbackData, nil
}

// GetUserCallbackHistory 获取用户回调历史
func (s *callbackService) GetUserCallbackHistory(ctx context.Context, userID int64, limit int) ([]*models.CallbackLog, error) {
	if userID == 0 {
		return nil, fmt.Errorf("无效的用户 ID")
	}

	logs, err := s.callbackRepo.GetUserCallbacks(ctx, userID, limit)
	if err != nil {
		logger.L().Errorf("Failed to get user callback history: user_id=%d, error=%v", userID, err)
		return nil, fmt.Errorf("获取回调历史失败")
	}

	logger.L().Debugf("Retrieved user callback history: user_id=%d, count=%d", userID, len(logs))
	return logs, nil
}

// GetCallbacksByAction 根据操作类型查询回调日志
func (s *callbackService) GetCallbacksByAction(ctx context.Context, action string, limit int) ([]*models.CallbackLog, error) {
	if action == "" {
		return nil, fmt.Errorf("操作类型不能为空")
	}

	logs, err := s.callbackRepo.GetByAction(ctx, action, limit)
	if err != nil {
		logger.L().Errorf("Failed to get callbacks by action: action=%s, error=%v", action, err)
		return nil, fmt.Errorf("查询回调日志失败")
	}

	logger.L().Debugf("Retrieved callbacks by action: action=%s, count=%d", action, len(logs))
	return logs, nil
}

// GetErrorCallbacks 获取处理失败的回调日志
func (s *callbackService) GetErrorCallbacks(ctx context.Context, limit int) ([]*models.CallbackLog, error) {
	logs, err := s.callbackRepo.GetErrorCallbacks(ctx, limit)
	if err != nil {
		logger.L().Errorf("Failed to get error callbacks: error=%v", err)
		return nil, fmt.Errorf("获取错误回调日志失败")
	}

	logger.L().Debugf("Retrieved error callbacks: count=%d", len(logs))
	return logs, nil
}

// ValidateCallbackAction 验证回调操作是否有效
func (s *callbackService) ValidateCallbackAction(action string) bool {
	// 定义有效的操作类型列表
	validActions := []string{
		models.CallbackActionAdminPage,
		models.CallbackActionConfirmDelete,
		models.CallbackActionGroupSettings,
		models.CallbackActionWelcomeToggle,
		models.CallbackActionApproveJoin,
		models.CallbackActionRejectJoin,
		models.CallbackActionPagination,
	}

	for _, validAction := range validActions {
		if action == validAction {
			return true
		}
	}

	logger.L().Warnf("Invalid callback action: %s", action)
	return false
}
