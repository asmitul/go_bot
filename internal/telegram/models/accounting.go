package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 货币类型常量
const (
	CurrencyUSD = "USD" // USDT
	CurrencyCNY = "CNY" // 人民币
)

// AccountingRecord 收支记账记录
type AccountingRecord struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	ChatID       int64              `bson:"chat_id"`       // 群组 Chat ID
	UserID       int64              `bson:"user_id"`       // 操作用户 ID
	Amount       float64            `bson:"amount"`        // 金额（正数为收入，负数为支出）
	Currency     string             `bson:"currency"`      // 货币类型：USD/CNY
	OriginalExpr string             `bson:"original_expr"` // 原始表达式（如 "100*7.2"）
	RecordedAt   time.Time          `bson:"recorded_at"`   // 记录时间（容器时区：Asia/Shanghai）
	CreatedAt    time.Time          `bson:"created_at"`    // 数据库创建时间
}

// IsIncome 是否为收入记录
func (r *AccountingRecord) IsIncome() bool {
	return r.Amount > 0
}

// IsExpense 是否为支出记录
func (r *AccountingRecord) IsExpense() bool {
	return r.Amount < 0
}
