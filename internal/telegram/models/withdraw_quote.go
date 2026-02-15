package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WithdrawQuoteRecord 保存下发时的汇率与U数量快照
type WithdrawQuoteRecord struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	MerchantID int64              `bson:"merchant_id"`
	ChatID     int64              `bson:"chat_id,omitempty"`
	UserID     int64              `bson:"user_id,omitempty"`
	WithdrawNo string             `bson:"withdraw_no,omitempty"`
	OrderNo    string             `bson:"order_no,omitempty"`
	Amount     float64            `bson:"amount"`
	Rate       float64            `bson:"rate"`
	USDTAmount float64            `bson:"usdt_amount"`
	CreatedAt  time.Time          `bson:"created_at"`
	UpdatedAt  time.Time          `bson:"updated_at"`
}
