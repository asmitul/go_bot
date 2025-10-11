package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// InlineQueryLog 内联查询日志
type InlineQueryLog struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	QueryID      string             `bson:"query_id"`                // Telegram Query ID（唯一）
	UserID       int64              `bson:"user_id"`                 // 查询用户 ID
	Username     string             `bson:"username,omitempty"`      // 查询用户名
	Query        string             `bson:"query"`                   // 查询内容
	Offset       string             `bson:"offset,omitempty"`        // 分页偏移
	ChatType     string             `bson:"chat_type,omitempty"`     // 聊天类型（sender/private/group等）
	Location     *Location          `bson:"location,omitempty"`      // 用户位置（如果有）
	ResultCount  int                `bson:"result_count"`            // 返回结果数量
	CreatedAt    time.Time          `bson:"created_at"`              // 查询时间
}

// ChosenInlineResultLog 内联结果选择日志
type ChosenInlineResultLog struct {
	ID              primitive.ObjectID `bson:"_id,omitempty"`
	ResultID        string             `bson:"result_id"`                   // 选择的结果 ID
	UserID          int64              `bson:"user_id"`                     // 用户 ID
	Username        string             `bson:"username,omitempty"`          // 用户名
	Query           string             `bson:"query"`                       // 原始查询
	InlineMessageID string             `bson:"inline_message_id,omitempty"` // 内联消息 ID
	CreatedAt       time.Time          `bson:"created_at"`                  // 选择时间
}

// Location 位置信息
type Location struct {
	Latitude  float64 `bson:"latitude"`
	Longitude float64 `bson:"longitude"`
}
