package repository

import (
	"context"
	"time"

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

	// DeleteGroup 删除群组（Bot 离开时）
	DeleteGroup(ctx context.Context, telegramID int64) error

	// ListActiveGroups 列出所有活跃群组
	ListActiveGroups(ctx context.Context) ([]*models.Group, error)

	// UpdateSettings 更新群组配置
	UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error

	// UpdateStats 更新群组统计信息
	UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// MessageRepository 消息数据访问接口
type MessageRepository interface {
	// CreateMessage 创建消息记录
	CreateMessage(ctx context.Context, message *models.Message) error

	// GetByTelegramID 根据 Telegram 消息 ID 和聊天 ID 获取消息
	GetByTelegramID(ctx context.Context, telegramMessageID, chatID int64) (*models.Message, error)

	// UpdateMessageEdit 更新消息编辑信息
	UpdateMessageEdit(ctx context.Context, telegramMessageID, chatID int64, newText string, editedAt time.Time) error

	// ListMessagesByChat 列出聊天消息历史（分页）
	ListMessagesByChat(ctx context.Context, chatID int64, limit, offset int64) ([]*models.Message, error)

	// CountMessagesByType 按类型统计消息数量
	CountMessagesByType(ctx context.Context, chatID int64) (map[string]int64, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}
