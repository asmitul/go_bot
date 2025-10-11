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

// MessageService 消息业务逻辑接口
type MessageService interface {
	// RecordMessage 记录消息
	RecordMessage(ctx context.Context, message *models.Message) error

	// RecordEdit 记录消息编辑
	RecordEdit(ctx context.Context, message *models.Message) error

	// HandleMediaMessage 处理媒体消息
	HandleMediaMessage(ctx context.Context, message *models.Message) error

	// GetChatHistory 获取聊天历史
	GetChatHistory(ctx context.Context, chatID int64, limit int) ([]*models.Message, error)

	// GetUserMessages 获取用户消息历史
	GetUserMessages(ctx context.Context, userID int64, limit int) ([]*models.Message, error)

	// GetMessage 获取单条消息
	GetMessage(ctx context.Context, chatID, messageID int64) (*models.Message, error)

	// CountChatMessages 统计聊天消息数量
	CountChatMessages(ctx context.Context, chatID int64) (int64, error)
}

// CallbackService 回调业务逻辑接口
type CallbackService interface {
	// LogCallback 记录回调日志
	LogCallback(ctx context.Context, callbackLog *models.CallbackLog) error

	// ParseAndHandle 解析并处理回调数据
	ParseAndHandle(ctx context.Context, data string) (*models.CallbackData, error)

	// GetUserCallbackHistory 获取用户回调历史
	GetUserCallbackHistory(ctx context.Context, userID int64, limit int) ([]*models.CallbackLog, error)

	// GetCallbacksByAction 根据操作类型查询回调日志
	GetCallbacksByAction(ctx context.Context, action string, limit int) ([]*models.CallbackLog, error)

	// GetErrorCallbacks 获取处理失败的回调日志
	GetErrorCallbacks(ctx context.Context, limit int) ([]*models.CallbackLog, error)

	// ValidateCallbackAction 验证回调操作是否有效
	ValidateCallbackAction(action string) bool
}

// MemberService 成员业务逻辑接口
type MemberService interface {
	// HandleMemberChange 处理成员状态变化
	HandleMemberChange(ctx context.Context, event *models.ChatMemberEvent) error

	// SendWelcomeMessage 检查是否发送欢迎消息（返回：是否发送、消息内容、错误）
	SendWelcomeMessage(ctx context.Context, chatID, userID int64) (bool, string, error)

	// HandleJoinRequest 处理入群申请
	HandleJoinRequest(ctx context.Context, request *models.JoinRequest) error

	// ApproveJoinRequest 批准入群请求
	ApproveJoinRequest(ctx context.Context, chatID, userID, reviewerID int64, reviewerUsername string) error

	// RejectJoinRequest 拒绝入群请求
	RejectJoinRequest(ctx context.Context, chatID, userID, reviewerID int64, reviewerUsername, reason string) error

	// GetPendingJoinRequests 获取待审批的入群请求列表
	GetPendingJoinRequests(ctx context.Context, chatID int64) ([]*models.JoinRequest, error)

	// GetChatMemberHistory 获取群组成员历史
	GetChatMemberHistory(ctx context.Context, chatID int64, limit int) ([]*models.ChatMemberEvent, error)

	// UpdateWelcomeSettings 更新欢迎消息设置
	UpdateWelcomeSettings(ctx context.Context, chatID int64, enabled bool, text string) error
}

// InlineService 内联查询业务逻辑接口
type InlineService interface {
	// HandleInlineQuery 处理内联查询
	HandleInlineQuery(ctx context.Context, query *models.InlineQueryLog) error

	// HandleChosenResult 处理内联结果选择
	HandleChosenResult(ctx context.Context, result *models.ChosenInlineResultLog) error

	// GetUserQueryHistory 获取用户内联查询历史
	GetUserQueryHistory(ctx context.Context, userID int64, limit int) ([]*models.InlineQueryLog, error)

	// GetPopularQueries 获取热门查询
	GetPopularQueries(ctx context.Context, limit int) ([]string, error)

	// ValidateQuery 验证查询内容是否合法
	ValidateQuery(query string) bool
}

// PollService 投票业务逻辑接口
type PollService interface {
	// HandlePollCreation 处理投票创建
	HandlePollCreation(ctx context.Context, poll *models.PollRecord) error

	// HandlePollUpdate 处理投票更新
	HandlePollUpdate(ctx context.Context, poll *models.PollRecord) error

	// HandlePollAnswer 处理用户投票
	HandlePollAnswer(ctx context.Context, answer *models.PollAnswer) error

	// GetPollResults 获取投票结果
	GetPollResults(ctx context.Context, pollID string) (*models.PollRecord, []*models.PollAnswer, error)

	// GetUserPolls 获取用户创建的投票
	GetUserPolls(ctx context.Context, userID int64, limit int) ([]*models.PollRecord, error)

	// ValidatePoll 验证投票参数是否合法
	ValidatePoll(poll *models.PollRecord) error
}

// ReactionService 反应业务逻辑接口
type ReactionService interface {
	// HandleReaction 处理消息反应
	HandleReaction(ctx context.Context, reaction *models.MessageReactionRecord) error

	// HandleReactionCount 处理反应统计更新
	HandleReactionCount(ctx context.Context, count *models.MessageReactionCountRecord) error

	// GetMessageReactions 获取消息的所有反应
	GetMessageReactions(ctx context.Context, chatID, messageID int64) ([]*models.MessageReactionRecord, error)

	// GetReactionStatistics 获取消息反应统计
	GetReactionStatistics(ctx context.Context, chatID, messageID int64) (*models.MessageReactionCountRecord, error)

	// GetTopReactedMessages 获取反应最多的消息
	GetTopReactedMessages(ctx context.Context, chatID int64, limit int) ([]*models.MessageReactionCountRecord, error)

	// ValidateReaction 验证反应是否合法
	ValidateReaction(reaction *models.MessageReactionRecord) error
}
