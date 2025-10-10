package repository

import (
	"context"

	"go_bot/internal/telegram/models"
)

// UserRepository 用户数据访问接口
type UserRepository interface {
	// CreateOrUpdate 创建或更新用户
	CreateOrUpdate(ctx context.Context, user *models.User) error

	// GetByTelegramID 根据 Telegram ID 获取用户
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)

	// UpdateLastActive 更新用户最后活跃时间
	UpdateLastActive(ctx context.Context, telegramID int64) error

	// GrantAdmin 授予管理员权限
	GrantAdmin(ctx context.Context, telegramID int64, grantedBy int64) error

	// RevokeAdmin 撤销管理员权限
	RevokeAdmin(ctx context.Context, telegramID int64) error

	// ListAdmins 列出所有管理员
	ListAdmins(ctx context.Context) ([]*models.User, error)

	// GetUserInfo 获取用户完整信息
	GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// GroupRepository 群组数据访问接口
type GroupRepository interface {
	// CreateOrUpdate 创建或更新群组
	CreateOrUpdate(ctx context.Context, group *models.Group) error

	// GetByTelegramID 根据 Telegram ID 获取群组
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.Group, error)

	// MarkBotLeft 标记 Bot 离开群组
	MarkBotLeft(ctx context.Context, telegramID int64) error

	// ListActiveGroups 列出所有活跃群组
	ListActiveGroups(ctx context.Context) ([]*models.Group, error)

	// UpdateSettings 更新群组配置
	UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error

	// UpdateStats 更新群组统计信息
	UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}
