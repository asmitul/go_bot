package repository

import (
	"context"
	"fmt"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const callbackLogsCollection = "callback_logs"

// MongoCallbackRepository MongoDB 回调仓库实现
type MongoCallbackRepository struct {
	db *mongo.Database
}

// NewMongoCallbackRepository 创建回调仓库实例
func NewMongoCallbackRepository(db *mongo.Database) CallbackRepository {
	return &MongoCallbackRepository{db: db}
}

// Create 创建回调日志记录
func (r *MongoCallbackRepository) Create(ctx context.Context, callbackLog *models.CallbackLog) error {
	if callbackLog.CallbackQueryID == "" {
		return fmt.Errorf("callback_query_id is required")
	}

	// 设置时间戳
	if callbackLog.CreatedAt.IsZero() {
		callbackLog.CreatedAt = time.Now()
	}

	collection := r.db.Collection(callbackLogsCollection)
	_, err := collection.InsertOne(ctx, callbackLog)
	if err != nil {
		return fmt.Errorf("failed to create callback log: %w", err)
	}

	return nil
}

// GetByQueryID 根据 Callback Query ID 查询回调日志
func (r *MongoCallbackRepository) GetByQueryID(ctx context.Context, queryID string) (*models.CallbackLog, error) {
	collection := r.db.Collection(callbackLogsCollection)

	filter := bson.M{"callback_query_id": queryID}

	var log models.CallbackLog
	err := collection.FindOne(ctx, filter).Decode(&log)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("callback log not found")
		}
		return nil, fmt.Errorf("failed to get callback log: %w", err)
	}

	return &log, nil
}

// GetUserCallbacks 获取用户的回调历史
func (r *MongoCallbackRepository) GetUserCallbacks(ctx context.Context, userID int64, limit int) ([]*models.CallbackLog, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(callbackLogsCollection)

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user callbacks: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.CallbackLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode callback logs: %w", err)
	}

	return logs, nil
}

// GetByAction 根据操作类型查询回调日志
func (r *MongoCallbackRepository) GetByAction(ctx context.Context, action string, limit int) ([]*models.CallbackLog, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(callbackLogsCollection)

	filter := bson.M{"action": action}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get callbacks by action: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.CallbackLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode callback logs: %w", err)
	}

	return logs, nil
}

// CountUserCallbacks 统计用户的回调次数
func (r *MongoCallbackRepository) CountUserCallbacks(ctx context.Context, userID int64) (int64, error) {
	collection := r.db.Collection(callbackLogsCollection)

	filter := bson.M{"user_id": userID}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count callbacks: %w", err)
	}

	return count, nil
}

// GetErrorCallbacks 获取处理失败的回调日志
func (r *MongoCallbackRepository) GetErrorCallbacks(ctx context.Context, limit int) ([]*models.CallbackLog, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(callbackLogsCollection)

	filter := bson.M{"error": bson.M{"$ne": ""}}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get error callbacks: %w", err)
	}
	defer cursor.Close(ctx)

	var logs []*models.CallbackLog
	if err := cursor.All(ctx, &logs); err != nil {
		return nil, fmt.Errorf("failed to decode callback logs: %w", err)
	}

	return logs, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoCallbackRepository) EnsureIndexes(ctx context.Context) error {
	collection := r.db.Collection(callbackLogsCollection)

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "callback_query_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "action", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "answered", Value: 1}},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for callback_logs: %w", err)
	}

	return nil
}
