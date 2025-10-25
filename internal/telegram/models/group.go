package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Bot 状态常量
const (
	BotStatusActive = "active" // Bot 在群组中活跃
	BotStatusKicked = "kicked" // Bot 被踢出群组
	BotStatusLeft   = "left"   // Bot 主动离开群组
)

// Group 群组模型
type Group struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	TelegramID  int64              `bson:"telegram_id"`           // Telegram Chat ID（唯一）
	Type        string             `bson:"type"`                  // 类型：group/supergroup/channel
	Title       string             `bson:"title"`                 // 群组名称
	Username    string             `bson:"username,omitempty"`    // 公开群组的 @username
	Description string             `bson:"description,omitempty"` // 群组描述
	MemberCount int                `bson:"member_count"`          // 成员数量（定期更新）

	// Bot 状态
	BotStatus   string     `bson:"bot_status"`            // Bot 状态：active/kicked/left
	BotJoinedAt time.Time  `bson:"bot_joined_at"`         // Bot 加入时间
	BotLeftAt   *time.Time `bson:"bot_left_at,omitempty"` // Bot 离开时间

	// 群组配置
	Settings GroupSettings `bson:"settings"` // 群组功能配置

	// 统计信息
	Stats GroupStats `bson:"stats"` // 群组统计数据

	CreatedAt time.Time `bson:"created_at"` // 创建时间
	UpdatedAt time.Time `bson:"updated_at"` // 更新时间
}

// GroupSettings 群组配置
type GroupSettings struct {
	CalculatorEnabled bool    `bson:"calculator_enabled"` // 是否启用计算器功能
	CryptoEnabled     bool    `bson:"crypto_enabled"`     // 是否启用加密货币价格查询功能
	CryptoFloatRate   float64 `bson:"crypto_float_rate"`  // 加密货币价格浮动费率（默认 0.12）
	ForwardEnabled    bool    `bson:"forward_enabled"`    // 是否接收频道转发消息
	AccountingEnabled bool    `bson:"accounting_enabled"` // 是否启用收支记账功能
	MerchantID        int32   `bson:"merchant_id"`        // 商户号（数字类型，0 表示未绑定）
	SifangEnabled     bool    `bson:"sifang_enabled"`     // 是否启用四方支付功能
}

// GroupStats 群组统计信息
type GroupStats struct {
	TotalMessages int64     `bson:"total_messages"`  // 总消息数
	LastMessageAt time.Time `bson:"last_message_at"` // 最后一条消息时间
}

// IsActive Bot 是否在群组中活跃
func (g *Group) IsActive() bool {
	return g.BotStatus == BotStatusActive
}
