package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MessageReactionRecord 消息反应记录
type MessageReactionRecord struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	ChatID      int64              `bson:"chat_id"`                 // 聊天 ID
	MessageID   int64              `bson:"message_id"`              // 消息 ID
	UserID      int64              `bson:"user_id"`                 // 用户 ID
	Username    string             `bson:"username,omitempty"`      // 用户名
	Reactions   []Reaction         `bson:"reactions"`               // 反应列表
	CreatedAt   time.Time          `bson:"created_at"`              // 创建时间
	UpdatedAt   time.Time          `bson:"updated_at"`              // 更新时间
}

// MessageReactionCountRecord 消息反应统计记录
type MessageReactionCountRecord struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	ChatID          int64              `bson:"chat_id"`                  // 聊天 ID
	MessageID       int64              `bson:"message_id"`               // 消息 ID
	ReactionCounts  []ReactionCount    `bson:"reaction_counts"`          // 反应统计
	TotalCount      int                `bson:"total_count"`              // 总反应数
	CreatedAt       time.Time          `bson:"created_at"`               // 创建时间
	UpdatedAt       time.Time          `bson:"updated_at"`               // 更新时间
}

// Reaction 反应（表情）
type Reaction struct {
	Type  string `bson:"type"`            // 反应类型（emoji/custom_emoji）
	Emoji string `bson:"emoji,omitempty"` // 表情符号
}

// ReactionCount 反应统计
type ReactionCount struct {
	Reaction Reaction `bson:"reaction"` // 反应
	Count    int      `bson:"count"`    // 数量
}

// HasReaction 是否包含指定反应
func (r *MessageReactionRecord) HasReaction(emoji string) bool {
	for _, reaction := range r.Reactions {
		if reaction.Emoji == emoji {
			return true
		}
	}
	return false
}

// GetTopReaction 获取最热门的反应
func (r *MessageReactionCountRecord) GetTopReaction() *ReactionCount {
	if len(r.ReactionCounts) == 0 {
		return nil
	}

	maxCount := r.ReactionCounts[0].Count
	topIdx := 0

	for i, rc := range r.ReactionCounts {
		if rc.Count > maxCount {
			maxCount = rc.Count
			topIdx = i
		}
	}

	return &r.ReactionCounts[topIdx]
}
