package service

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/repository"
)

// UserServiceImpl 用户服务实现
type UserServiceImpl struct {
	userRepo repository.UserRepository
}

// NewUserService 创建用户服务
func NewUserService(userRepo repository.UserRepository) UserService {
	return &UserServiceImpl{
		userRepo: userRepo,
	}
}

// RegisterOrUpdateUser 注册或更新用户
func (s *UserServiceImpl) RegisterOrUpdateUser(ctx context.Context, info *TelegramUserInfo) error {
	user := &models.User{
		TelegramID:   info.TelegramID,
		Username:     info.Username,
		FirstName:    info.FirstName,
		LastName:     info.LastName,
		LanguageCode: info.LanguageCode,
		IsPremium:    info.IsPremium,
		UpdatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	if err := s.userRepo.CreateOrUpdate(ctx, user); err != nil {
		logger.L().Errorf("Failed to register/update user %d: %v", info.TelegramID, err)
		return fmt.Errorf("failed to register user: %w", err)
	}

	logger.L().Infof("User %d (%s) registered/updated", info.TelegramID, info.Username)
	return nil
}

// GrantAdminPermission 授予管理员权限（包含业务验证）
func (s *UserServiceImpl) GrantAdminPermission(ctx context.Context, targetID, grantedBy int64) error {
	// 1. 验证授权者权限
	granter, err := s.userRepo.GetByTelegramID(ctx, grantedBy)
	if err != nil {
		logger.L().Errorf("Granter %d not found: %v", grantedBy, err)
		return fmt.Errorf("授权者不存在")
	}

	if !granter.IsOwner() {
		logger.L().Warnf("User %d attempted to grant admin without owner permission", grantedBy)
		return fmt.Errorf("只有 Owner 可以授予管理员权限")
	}

	// 2. 检查目标用户是否存在
	target, err := s.userRepo.GetByTelegramID(ctx, targetID)
	if err != nil {
		logger.L().Errorf("Target user %d not found: %v", targetID, err)
		return fmt.Errorf("目标用户不存在")
	}

	// 3. 检查是否已经是管理员
	if target.IsAdmin() {
		logger.L().Infof("User %d is already an admin", targetID)
		return fmt.Errorf("用户已经是管理员")
	}

	// 4. 执行授权
	if err := s.userRepo.GrantAdmin(ctx, targetID, grantedBy); err != nil {
		logger.L().Errorf("Failed to grant admin to %d: %v", targetID, err)
		return fmt.Errorf("授权失败: %w", err)
	}

	logger.L().Infof("User %d granted admin permission by %d", targetID, grantedBy)
	return nil
}

// RevokeAdminPermission 撤销管理员权限（包含业务验证）
func (s *UserServiceImpl) RevokeAdminPermission(ctx context.Context, targetID, revokedBy int64) error {
	// 1. 验证撤销者权限
	revoker, err := s.userRepo.GetByTelegramID(ctx, revokedBy)
	if err != nil {
		logger.L().Errorf("Revoker %d not found: %v", revokedBy, err)
		return fmt.Errorf("撤销者不存在")
	}

	if !revoker.IsOwner() {
		logger.L().Warnf("User %d attempted to revoke admin without owner permission", revokedBy)
		return fmt.Errorf("只有 Owner 可以撤销管理员权限")
	}

	// 2. 检查目标用户
	target, err := s.userRepo.GetByTelegramID(ctx, targetID)
	if err != nil {
		logger.L().Errorf("Target user %d not found: %v", targetID, err)
		return fmt.Errorf("目标用户不存在")
	}

	// 3. 不能撤销 Owner
	if target.IsOwner() {
		logger.L().Warnf("User %d attempted to revoke owner permission", revokedBy)
		return fmt.Errorf("不能撤销 Owner 权限")
	}

	// 4. 检查是否已经是普通用户
	if target.Role == models.RoleUser {
		logger.L().Infof("User %d is already a regular user", targetID)
		return fmt.Errorf("用户已经是普通用户")
	}

	// 5. 执行撤销
	if err := s.userRepo.RevokeAdmin(ctx, targetID); err != nil {
		logger.L().Errorf("Failed to revoke admin from %d: %v", targetID, err)
		return fmt.Errorf("撤销失败: %w", err)
	}

	logger.L().Infof("User %d admin permission revoked by %d", targetID, revokedBy)
	return nil
}

// GetUserInfo 获取用户信息
func (s *UserServiceImpl) GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error) {
	user, err := s.userRepo.GetUserInfo(ctx, telegramID)
	if err != nil {
		logger.L().Errorf("Failed to get user info for %d: %v", telegramID, err)
		return nil, fmt.Errorf("获取用户信息失败")
	}
	return user, nil
}

// ListAllAdmins 列出所有管理员
func (s *UserServiceImpl) ListAllAdmins(ctx context.Context) ([]*models.User, error) {
	admins, err := s.userRepo.ListAdmins(ctx)
	if err != nil {
		logger.L().Errorf("Failed to list admins: %v", err)
		return nil, fmt.Errorf("获取管理员列表失败")
	}
	return admins, nil
}

// CheckOwnerPermission 检查是否为 Owner
func (s *UserServiceImpl) CheckOwnerPermission(ctx context.Context, telegramID int64) (bool, error) {
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return false, err
	}
	return user.IsOwner(), nil
}

// CheckAdminPermission 检查是否为 Admin+
func (s *UserServiceImpl) CheckAdminPermission(ctx context.Context, telegramID int64) (bool, error) {
	user, err := s.userRepo.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return false, err
	}
	return user.IsAdmin(), nil
}

// UpdateUserActivity 更新用户活跃时间
func (s *UserServiceImpl) UpdateUserActivity(ctx context.Context, telegramID int64) error {
	if err := s.userRepo.UpdateLastActive(ctx, telegramID); err != nil {
		logger.L().Warnf("Failed to update user activity for %d: %v", telegramID, err)
		// 不返回错误，仅记录日志
	}
	return nil
}
