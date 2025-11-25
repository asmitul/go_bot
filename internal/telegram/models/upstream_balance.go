package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BalanceOperationType 标记余额操作类型
type BalanceOperationType string

const (
	BalanceOpCredit        BalanceOperationType = "credit"
	BalanceOpDebit         BalanceOperationType = "debit"
	BalanceOpSettlement    BalanceOperationType = "settlement"
	BalanceOpSetMinBalance BalanceOperationType = "set_min_balance"
	BalanceOpAlertLimit    BalanceOperationType = "set_alert_limit"
)

// UpstreamBalance 表示单个上游群的余额与阈值
type UpstreamBalance struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	GroupID           int64              `bson:"group_id"`                       // Telegram 群组 ID
	Balance           float64            `bson:"balance"`                        // 当前余额（CNY）
	MinBalance        float64            `bson:"min_balance"`                    // 最低余额阈值
	AlertLimitPerHour int                `bson:"alert_limit_per_hour,omitempty"` // 每小时告警次数上限
	CreatedAt         time.Time          `bson:"created_at"`
	UpdatedAt         time.Time          `bson:"updated_at"`
}

// UpstreamBalanceLog 记录每一次调整
type UpstreamBalanceLog struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty"`
	GroupID     int64                `bson:"group_id"`
	OperatorID  int64                `bson:"operator_id"`
	Delta       float64              `bson:"delta"`
	Balance     float64              `bson:"balance"`
	Type        BalanceOperationType `bson:"type"`
	Remark      string               `bson:"remark,omitempty"`
	OperationID string               `bson:"operation_id,omitempty"`
	CreatedAt   time.Time            `bson:"created_at"`
	Metadata    map[string]string    `bson:"metadata,omitempty"`
}

// UpstreamBalanceEvent 用于监控告警
type UpstreamBalanceEvent struct {
	GroupID           int64
	Balance           float64
	MinBalance        float64
	AlertLimitPerHour int
	BelowMin          bool
	OccurredAt        time.Time
	Trigger           string
}
