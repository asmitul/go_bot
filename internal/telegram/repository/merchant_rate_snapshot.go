package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoMerchantRateSnapshotRepository 使用 MongoDB 保存商户费率基线。
type MongoMerchantRateSnapshotRepository struct {
	collection *mongo.Collection
}

// NewMongoMerchantRateSnapshotRepository 创建商户费率快照 Repository。
func NewMongoMerchantRateSnapshotRepository(db *mongo.Database) MerchantRateSnapshotRepository {
	return &MongoMerchantRateSnapshotRepository{
		collection: db.Collection("merchant_rate_snapshots"),
	}
}

func (r *MongoMerchantRateSnapshotRepository) GetByChatID(
	ctx context.Context,
	chatID int64,
) (*models.MerchantRateSnapshot, error) {
	if chatID == 0 {
		return nil, fmt.Errorf("chat id is required")
	}

	var snapshot models.MerchantRateSnapshot
	err := r.collection.FindOne(ctx, bson.M{"chat_id": chatID}).Decode(&snapshot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get merchant rate snapshot: %w", err)
	}
	snapshot.Rates = models.NormalizeMerchantChannelRates(snapshot.Rates)
	return &snapshot, nil
}

func (r *MongoMerchantRateSnapshotRepository) Upsert(
	ctx context.Context,
	snapshot *models.MerchantRateSnapshot,
) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is nil")
	}
	if snapshot.ChatID == 0 {
		return fmt.Errorf("chat id is required")
	}
	if snapshot.MerchantID <= 0 {
		return fmt.Errorf("merchant id is required")
	}

	now := time.Now()
	if snapshot.LastCheckedAt.IsZero() {
		snapshot.LastCheckedAt = now
	}
	snapshot.UpdatedAt = now
	snapshot.Rates = models.NormalizeMerchantChannelRates(snapshot.Rates)

	update := bson.M{
		"$set": bson.M{
			"merchant_id":     snapshot.MerchantID,
			"rates":           snapshot.Rates,
			"last_checked_at": snapshot.LastCheckedAt,
			"updated_at":      snapshot.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"chat_id":    snapshot.ChatID,
			"created_at": now,
		},
	}

	if _, err := r.collection.UpdateOne(
		ctx,
		bson.M{"chat_id": snapshot.ChatID},
		update,
		options.Update().SetUpsert(true),
	); err != nil {
		return fmt.Errorf("failed to upsert merchant rate snapshot: %w", err)
	}
	return nil
}

func (r *MongoMerchantRateSnapshotRepository) EnsureIndexes(ctx context.Context) error {
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: "chat_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err := r.collection.Indexes().CreateOne(ctx, index); err != nil {
		return fmt.Errorf("failed to create merchant rate snapshot index: %w", err)
	}
	return nil
}
