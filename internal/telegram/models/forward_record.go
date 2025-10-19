package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ForwardRecord 转发记录（用于撤回功能）
type ForwardRecord struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"`
	TaskID             string             `bson:"task_id"`              // 任务ID (UUID)
	ChannelMessageID   int64              `bson:"channel_message_id"`   // 源频道消息ID
	TargetGroupID      int64              `bson:"target_group_id"`      // 目标群组ID
	ForwardedMessageID int64              `bson:"forwarded_message_id"` // 转发后的消息ID（用于撤回）
	Status             string             `bson:"status"`               // success/failed
	CreatedAt          time.Time          `bson:"created_at"`           // 创建时间（TTL索引）
}

const (
	ForwardStatusSuccess = "success"
	ForwardStatusFailed  = "failed"
)
