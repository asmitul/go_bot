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

// MessageRepository 消息数据访问接口
type MessageRepository interface {
	// Create 创建消息记录
	Create(ctx context.Context, message *models.Message) error

	// GetByTelegramID 根据 Telegram Message ID 和 Chat ID 查询消息
	GetByTelegramID(ctx context.Context, chatID, messageID int64) (*models.Message, error)

	// RecordEdit 记录消息编辑
	RecordEdit(ctx context.Context, message *models.Message) error

	// GetChatMessages 获取聊天的消息列表
	GetChatMessages(ctx context.Context, chatID int64, limit int) ([]*models.Message, error)

	// GetUserMessages 获取用户发送的消息列表
	GetUserMessages(ctx context.Context, userID int64, limit int) ([]*models.Message, error)

	// CountChatMessages 统计聊天的消息数量
	CountChatMessages(ctx context.Context, chatID int64) (int64, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// CallbackRepository 回调数据访问接口
type CallbackRepository interface {
	// Create 创建回调日志记录
	Create(ctx context.Context, callbackLog *models.CallbackLog) error

	// GetByQueryID 根据 Callback Query ID 查询回调日志
	GetByQueryID(ctx context.Context, queryID string) (*models.CallbackLog, error)

	// GetUserCallbacks 获取用户的回调历史
	GetUserCallbacks(ctx context.Context, userID int64, limit int) ([]*models.CallbackLog, error)

	// GetByAction 根据操作类型查询回调日志
	GetByAction(ctx context.Context, action string, limit int) ([]*models.CallbackLog, error)

	// CountUserCallbacks 统计用户的回调次数
	CountUserCallbacks(ctx context.Context, userID int64) (int64, error)

	// GetErrorCallbacks 获取处理失败的回调日志
	GetErrorCallbacks(ctx context.Context, limit int) ([]*models.CallbackLog, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// MemberRepository 成员数据访问接口
type MemberRepository interface {
	// RecordEvent 记录成员事件
	RecordEvent(ctx context.Context, event *models.ChatMemberEvent) error

	// CreateJoinRequest 创建入群请求
	CreateJoinRequest(ctx context.Context, request *models.JoinRequest) error

	// UpdateJoinRequestStatus 更新入群请求状态
	UpdateJoinRequestStatus(ctx context.Context, requestID, reviewerID int64, status, note string) error

	// GetPendingRequests 获取待审批的入群请求
	GetPendingRequests(ctx context.Context, chatID int64) ([]*models.JoinRequest, error)

	// GetJoinRequestByUser 根据用户和群组获取入群请求
	GetJoinRequestByUser(ctx context.Context, chatID, userID int64) (*models.JoinRequest, error)

	// GetChatEvents 获取群组的成员事件历史
	GetChatEvents(ctx context.Context, chatID int64, limit int) ([]*models.ChatMemberEvent, error)

	// GetUserEvents 获取用户的成员事件历史
	GetUserEvents(ctx context.Context, userID int64, limit int) ([]*models.ChatMemberEvent, error)

	// CountChatMembers 统计群组当前成员数
	CountChatMembers(ctx context.Context, chatID int64) (int64, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// InlineRepository 内联查询数据访问接口
type InlineRepository interface {
	// LogQuery 记录内联查询
	LogQuery(ctx context.Context, query *models.InlineQueryLog) error

	// LogChosenResult 记录内联结果选择
	LogChosenResult(ctx context.Context, result *models.ChosenInlineResultLog) error

	// GetUserQueries 获取用户的内联查询历史
	GetUserQueries(ctx context.Context, userID int64, limit int) ([]*models.InlineQueryLog, error)

	// GetPopularQueries 获取热门查询（按频率）
	GetPopularQueries(ctx context.Context, limit int) ([]string, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// PollRepository 投票数据访问接口
type PollRepository interface {
	// CreatePoll 创建投票记录
	CreatePoll(ctx context.Context, poll *models.PollRecord) error

	// UpdatePoll 更新投票状态
	UpdatePoll(ctx context.Context, poll *models.PollRecord) error

	// GetPollByID 根据 Poll ID 获取投票
	GetPollByID(ctx context.Context, pollID string) (*models.PollRecord, error)

	// RecordAnswer 记录投票回答
	RecordAnswer(ctx context.Context, answer *models.PollAnswer) error

	// GetPollAnswers 获取投票的所有回答
	GetPollAnswers(ctx context.Context, pollID string) ([]*models.PollAnswer, error)

	// GetUserPolls 获取用户创建的投票列表
	GetUserPolls(ctx context.Context, userID int64, limit int) ([]*models.PollRecord, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}

// ReactionRepository 反应数据访问接口
type ReactionRepository interface {
	// RecordReaction 记录消息反应
	RecordReaction(ctx context.Context, reaction *models.MessageReactionRecord) error

	// UpdateReactionCount 更新消息反应统计
	UpdateReactionCount(ctx context.Context, count *models.MessageReactionCountRecord) error

	// GetMessageReactions 获取消息的所有反应
	GetMessageReactions(ctx context.Context, chatID, messageID int64) ([]*models.MessageReactionRecord, error)

	// GetReactionCount 获取消息反应统计
	GetReactionCount(ctx context.Context, chatID, messageID int64) (*models.MessageReactionCountRecord, error)

	// GetTopReactedMessages 获取反应最多的消息
	GetTopReactedMessages(ctx context.Context, chatID int64, limit int) ([]*models.MessageReactionCountRecord, error)

	// EnsureIndexes 确保索引存在
	EnsureIndexes(ctx context.Context) error
}
