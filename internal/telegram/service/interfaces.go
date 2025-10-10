package service

import (
	"context"

	"go_bot/internal/telegram/models"
)

// UserService 用户业务逻辑接口
type UserService interface {
	// RegisterOrUpdateUser 注册或更新用户
	RegisterOrUpdateUser(ctx context.Context, info *TelegramUserInfo) error

	// GrantAdminPermission 授予管理员权限（包含业务验证）
	GrantAdminPermission(ctx context.Context, targetID, grantedBy int64) error

	// RevokeAdminPermission 撤销管理员权限（包含业务验证）
	RevokeAdminPermission(ctx context.Context, targetID, revokedBy int64) error

	// GetUserInfo 获取用户信息
	GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error)

	// ListAllAdmins 列出所有管理员
	ListAllAdmins(ctx context.Context) ([]*models.User, error)

	// CheckOwnerPermission 检查是否为 Owner
	CheckOwnerPermission(ctx context.Context, telegramID int64) (bool, error)

	// CheckAdminPermission 检查是否为 Admin+
	CheckAdminPermission(ctx context.Context, telegramID int64) (bool, error)

	// UpdateUserActivity 更新用户活跃时间
	UpdateUserActivity(ctx context.Context, telegramID int64) error
}

// GroupService 群组业务逻辑接口
type GroupService interface {
	// CreateOrUpdateGroup 创建或更新群组
	CreateOrUpdateGroup(ctx context.Context, group *models.Group) error

	// GetGroupInfo 获取群组信息
	GetGroupInfo(ctx context.Context, telegramID int64) (*models.Group, error)

	// MarkBotLeft 标记 Bot 离开群组
	MarkBotLeft(ctx context.Context, telegramID int64) error

	// ListActiveGroups 列出所有活跃群组
	ListActiveGroups(ctx context.Context) ([]*models.Group, error)
}

// TelegramUserInfo Telegram 用户信息 DTO
type TelegramUserInfo struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LastName     string
	LanguageCode string
	IsPremium    bool
}
