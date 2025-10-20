package repository

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoAccountingRepository 收支记账数据访问层（MongoDB 实现）
type MongoAccountingRepository struct {
	collection *mongo.Collection
}

// NewMongoAccountingRepository 创建记账 Repository
func NewMongoAccountingRepository(db *mongo.Database) AccountingRepository {
	return &MongoAccountingRepository{
		collection: db.Collection("accounting_records"),
	}
}

// CreateRecord 创建记账记录
func (r *MongoAccountingRepository) CreateRecord(ctx context.Context, record *models.AccountingRecord) error {
	now := time.Now()
	record.CreatedAt = now

	// 如果没有设置记录时间，使用当前时间
	if record.RecordedAt.IsZero() {
		record.RecordedAt = now
	}

	_, err := r.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to create accounting record: %w", err)
	}

	return nil
}

// GetRecordsByDateRange 按日期范围查询记录
func (r *MongoAccountingRepository) GetRecordsByDateRange(ctx context.Context, chatID int64, startTime, endTime time.Time, currency string) ([]*models.AccountingRecord, error) {
	filter := bson.M{
		"chat_id": chatID,
		"recorded_at": bson.M{
			"$gte": startTime,
			"$lt":  endTime,
		},
	}

	// 如果指定了货币类型，添加过滤条件
	if currency != "" {
		filter["currency"] = currency
	}

	// 按时间升序排序
	opts := options.Find().SetSort(bson.D{{Key: "recorded_at", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounting records: %w", err)
	}
	defer cursor.Close(ctx)

	var records []*models.AccountingRecord
	if err = cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode accounting records: %w", err)
	}

	return records, nil
}

// GetRecentRecords 获取最近N天的记录（用于删除界面）
func (r *MongoAccountingRepository) GetRecentRecords(ctx context.Context, chatID int64, days int) ([]*models.AccountingRecord, error) {
	startTime := time.Now().AddDate(0, 0, -days)

	filter := bson.M{
		"chat_id": chatID,
		"recorded_at": bson.M{
			"$gte": startTime,
		},
	}

	// 按时间降序排序（最新的在前）
	opts := options.Find().SetSort(bson.D{{Key: "recorded_at", Value: -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent accounting records: %w", err)
	}
	defer cursor.Close(ctx)

	var records []*models.AccountingRecord
	if err = cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode accounting records: %w", err)
	}

	return records, nil
}

// DeleteRecord 删除单条记录
func (r *MongoAccountingRepository) DeleteRecord(ctx context.Context, recordID string) error {
	objID, err := primitive.ObjectIDFromHex(recordID)
	if err != nil {
		return fmt.Errorf("invalid record ID: %w", err)
	}

	filter := bson.M{"_id": objID}
	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete accounting record: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("record not found")
	}

	return nil
}

// DeleteAllByChatID 清空群组所有记录
func (r *MongoAccountingRepository) DeleteAllByChatID(ctx context.Context, chatID int64) (int64, error) {
	filter := bson.M{"chat_id": chatID}
	result, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete all accounting records: %w", err)
	}

	return result.DeletedCount, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoAccountingRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// 复合索引：chat_id + recorded_at + currency（支持按群组、时间、货币查询）
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "recorded_at", Value: -1},
				{Key: "currency", Value: 1},
			},
		},
		// 单字段索引：chat_id（支持按群组查询所有记录）
		{
			Keys: bson.D{{Key: "chat_id", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create accounting indexes: %w", err)
	}

	return nil
}
