package models

import (
	"errors"
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GroupTier 表示群组分级
type GroupTier string

const (
	GroupTierBasic    GroupTier = "basic"
	GroupTierMerchant GroupTier = "merchant"
	GroupTierUpstream GroupTier = "upstream"
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
	Tier        GroupTier          `bson:"tier"`                  // 群组等级：basic/merchant/upstream

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
	CalculatorEnabled       bool     `bson:"calculator_enabled"`         // 是否启用计算器功能
	CryptoEnabled           bool     `bson:"crypto_enabled"`             // 是否启用加密货币价格查询功能
	CryptoFloatRate         float64  `bson:"crypto_float_rate"`          // 加密货币价格浮动费率（默认 0.12）
	ForwardEnabled          bool     `bson:"forward_enabled"`            // 是否接收频道转发消息
	AccountingEnabled       bool     `bson:"accounting_enabled"`         // 是否启用收支记账功能
	MerchantID              int32    `bson:"merchant_id"`                // 商户号（数字类型，0 表示未绑定）
	InterfaceIDs            []string `bson:"interface_ids,omitempty"`    // 接口 ID 列表
	SifangEnabled           bool     `bson:"sifang_enabled"`             // 是否启用四方支付功能
	SifangAutoLookupEnabled bool     `bson:"sifang_auto_lookup_enabled"` // 是否启用四方支付自动查单
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

// DetermineGroupTier 根据配置推导群组等级
func DetermineGroupTier(settings GroupSettings) (GroupTier, error) {
	hasMerchant := settings.MerchantID > 0
	interfaceIDs := NormalizeInterfaceIDs(settings.InterfaceIDs)
	hasInterface := len(interfaceIDs) > 0

	switch {
	case hasMerchant && hasInterface:
		return GroupTierBasic, errors.New("群组不能同时绑定商户号和接口 ID")
	case hasInterface:
		return GroupTierUpstream, nil
	case hasMerchant:
		return GroupTierMerchant, nil
	default:
		return GroupTierBasic, nil
	}
}

// NormalizeInterfaceIDs 去重、去空格并过滤空值
func NormalizeInterfaceIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ids))
	clean := make([]string, 0, len(ids))
	for _, raw := range ids {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		clean = append(clean, trimmed)
	}

	slices.SortFunc(clean, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})

	if len(clean) == 0 {
		return nil
	}
	return clean
}

// NormalizeGroupTier 确保群等级始终有效
func NormalizeGroupTier(tier GroupTier) GroupTier {
	if tier == "" {
		return GroupTierBasic
	}
	return tier
}

// IsTierAllowed 判断当前群等级是否在允许列表中
func IsTierAllowed(current GroupTier, allowed []GroupTier) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, candidate := range allowed {
		if candidate == current {
			return true
		}
	}
	return false
}

// GroupTierDisplayName 返回群等级的可读名称
func GroupTierDisplayName(tier GroupTier) string {
	switch tier {
	case GroupTierMerchant:
		return "商户群"
	case GroupTierUpstream:
		return "上游群"
	default:
		return "普通群"
	}
}

// FormatAllowedTierList 将允许的群等级格式化为可读描述
func FormatAllowedTierList(allowed []GroupTier) string {
	if len(allowed) == 0 {
		return "所有群类型"
	}

	names := make([]string, 0, len(allowed))
	for _, tier := range allowed {
		names = append(names, GroupTierDisplayName(tier))
	}
	return strings.Join(names, " / ")
}
