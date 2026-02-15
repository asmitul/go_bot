package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoWithdrawQuoteRepository 提款行情快照数据访问层（MongoDB 实现）
type MongoWithdrawQuoteRepository struct {
	collection *mongo.Collection
}

// NewMongoWithdrawQuoteRepository 创建提款行情快照 Repository
func NewMongoWithdrawQuoteRepository(db *mongo.Database) WithdrawQuoteRepository {
	return &MongoWithdrawQuoteRepository{
		collection: db.Collection("withdraw_quote_records"),
	}
}

// Upsert 保存或更新一条快照记录
func (r *MongoWithdrawQuoteRepository) Upsert(ctx context.Context, record *models.WithdrawQuoteRecord) error {
	if record == nil {
		return fmt.Errorf("record is nil")
	}
	if record.MerchantID == 0 {
		return fmt.Errorf("merchant id is required")
	}

	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now

	record.WithdrawNo = strings.TrimSpace(record.WithdrawNo)
	record.OrderNo = strings.TrimSpace(record.OrderNo)

	filter := bson.M{"merchant_id": record.MerchantID}
	switch {
	case record.WithdrawNo != "":
		filter["withdraw_no"] = record.WithdrawNo
	case record.OrderNo != "":
		filter["order_no"] = record.OrderNo
	default:
		_, err := r.collection.InsertOne(ctx, record)
		if err != nil {
			return fmt.Errorf("failed to insert withdraw quote record: %w", err)
		}
		return nil
	}

	setFields := bson.M{
		"merchant_id": record.MerchantID,
		"chat_id":     record.ChatID,
		"user_id":     record.UserID,
		"amount":      record.Amount,
		"rate":        record.Rate,
		"usdt_amount": record.USDTAmount,
		"updated_at":  record.UpdatedAt,
	}

	unsetFields := bson.M{}
	if record.WithdrawNo != "" {
		setFields["withdraw_no"] = record.WithdrawNo
	} else {
		unsetFields["withdraw_no"] = ""
	}
	if record.OrderNo != "" {
		setFields["order_no"] = record.OrderNo
	} else {
		unsetFields["order_no"] = ""
	}

	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			"created_at": record.CreatedAt,
		},
	}
	if len(unsetFields) > 0 {
		update["$unset"] = unsetFields
	}

	opts := options.Update().SetUpsert(true)
	if _, err := r.collection.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("failed to upsert withdraw quote record: %w", err)
	}

	return nil
}

// ListByMerchantAndDateRange 按商户与时间范围查询快照记录
func (r *MongoWithdrawQuoteRepository) ListByMerchantAndDateRange(ctx context.Context, merchantID int64, startTime, endTime time.Time) ([]*models.WithdrawQuoteRecord, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	filter := bson.M{
		"merchant_id": merchantID,
		"created_at": bson.M{
			"$gte": startTime,
			"$lt":  endTime,
		},
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query withdraw quote records: %w", err)
	}
	defer cursor.Close(ctx)

	var records []*models.WithdrawQuoteRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode withdraw quote records: %w", err)
	}
	return records, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoWithdrawQuoteRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "merchant_id", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "merchant_id", Value: 1},
				{Key: "withdraw_no", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{
					"withdraw_no": bson.M{
						"$exists": true,
						"$gt":     "",
					},
				}),
		},
		{
			Keys: bson.D{
				{Key: "merchant_id", Value: 1},
				{Key: "order_no", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{
					"order_no": bson.M{
						"$exists": true,
						"$gt":     "",
					},
				}),
		},
	}

	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("failed to create withdraw quote indexes: %w", err)
	}
	return nil
}
