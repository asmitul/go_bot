package service

import (
	"context"
	"time"

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

	// UpdateGroupSettings 更新群组配置
	UpdateGroupSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error

	// LeaveGroup Bot 离开群组（删除群组记录）
	LeaveGroup(ctx context.Context, telegramID int64) error

	// HandleBotAddedToGroup Bot 被添加到群组
	HandleBotAddedToGroup(ctx context.Context, group *models.Group) error

	// HandleBotRemovedFromGroup Bot 被移出群组
	HandleBotRemovedFromGroup(ctx context.Context, telegramID int64, reason string) error
}

// MessageService 消息业务逻辑接口
type MessageService interface {
	// HandleTextMessage 处理文本消息
	HandleTextMessage(ctx context.Context, msg *TextMessageInfo) error

	// HandleMediaMessage 处理媒体消息
	HandleMediaMessage(ctx context.Context, msg *MediaMessageInfo) error

	// HandleEditedMessage 处理消息编辑
	HandleEditedMessage(ctx context.Context, telegramMessageID, chatID int64, newText string, editedAt time.Time) error

	// RecordChannelPost 记录频道消息
	RecordChannelPost(ctx context.Context, msg *ChannelPostInfo) error

	// GetChatMessageHistory 获取聊天消息历史
	GetChatMessageHistory(ctx context.Context, chatID int64, limit int) ([]*models.Message, error)
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

// TextMessageInfo 文本消息信息 DTO
type TextMessageInfo struct {
	TelegramMessageID int64
	ChatID            int64
	UserID            int64
	Text              string
	ReplyToMessageID  int64
	SentAt            time.Time
}

// MediaMessageInfo 媒体消息信息 DTO
type MediaMessageInfo struct {
	TelegramMessageID int64
	ChatID            int64
	UserID            int64
	MessageType       string
	Caption           string
	MediaFileID       string
	MediaFileSize     int64
	MediaMimeType     string
	SentAt            time.Time
}

// ChannelPostInfo 频道消息信息 DTO
type ChannelPostInfo struct {
	TelegramMessageID int64
	ChatID            int64
	MessageType       string // text/photo/video...
	Text              string
	MediaFileID       string
	SentAt            time.Time
}
