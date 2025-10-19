package repository

import (
	"context"
	"fmt"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type forwardRecordRepository struct {
	collection *mongo.Collection
}

// NewForwardRecordRepository 创建转发记录仓储实例
func NewForwardRecordRepository(db *mongo.Database) ForwardRecordRepository {
	return &forwardRecordRepository{
		collection: db.Collection("forward_records"),
	}
}

// CreateRecord 创建转发记录
func (r *forwardRecordRepository) CreateRecord(ctx context.Context, record *models.ForwardRecord) error {
	_, err := r.collection.InsertOne(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to create forward record: %w", err)
	}
	return nil
}

// BulkCreateRecords 批量创建转发记录（性能优化）
func (r *forwardRecordRepository) BulkCreateRecords(ctx context.Context, records []*models.ForwardRecord) error {
	if len(records) == 0 {
		return nil
	}

	// 转换为 []interface{}
	docs := make([]interface{}, len(records))
	for i, record := range records {
		docs[i] = record
	}

	_, err := r.collection.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("failed to bulk create forward records: %w", err)
	}
	return nil
}

// GetSuccessRecordsByTaskID 根据任务ID查询所有成功的转发记录
func (r *forwardRecordRepository) GetSuccessRecordsByTaskID(ctx context.Context, taskID string) ([]*models.ForwardRecord, error) {
	filter := bson.M{
		"task_id": taskID,
		"status":  models.ForwardStatusSuccess,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query forward records: %w", err)
	}
	defer cursor.Close(ctx)

	var records []*models.ForwardRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode forward records: %w", err)
	}

	return records, nil
}

// DeleteRecordsByTaskID 删除转发记录（撤回后清理）
func (r *forwardRecordRepository) DeleteRecordsByTaskID(ctx context.Context, taskID string) error {
	filter := bson.M{"task_id": taskID}

	_, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete forward records: %w", err)
	}
	return nil
}

// EnsureIndexes 确保索引存在
func (r *forwardRecordRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// task_id 索引（用于查询某任务的所有记录）
		{
			Keys: bson.D{{Key: "task_id", Value: 1}},
		},
		// TTL 索引（48小时自动删除）
		{
			Keys:    bson.D{{Key: "created_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(48 * 3600),
		},
		// 复合唯一索引（防止重复转发）
		{
			Keys: bson.D{
				{Key: "task_id", Value: 1},
				{Key: "target_group_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for forward_records: %w", err)
	}

	return nil
}
