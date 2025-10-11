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

const messagesCollection = "messages"

// MongoMessageRepository MongoDB 消息仓库实现
type MongoMessageRepository struct {
	db *mongo.Database
}

// NewMongoMessageRepository 创建消息仓库实例
func NewMongoMessageRepository(db *mongo.Database) MessageRepository {
	return &MongoMessageRepository{db: db}
}

// Create 创建消息记录
func (r *MongoMessageRepository) Create(ctx context.Context, message *models.Message) error {
	if message.TelegramID == 0 || message.ChatID == 0 {
		return fmt.Errorf("telegram_id and chat_id are required")
	}

	// 设置时间戳
	now := time.Now()
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}
	message.UpdatedAt = now

	collection := r.db.Collection(messagesCollection)
	_, err := collection.InsertOne(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

// GetByTelegramID 根据 Telegram Message ID 和 Chat ID 查询消息
func (r *MongoMessageRepository) GetByTelegramID(ctx context.Context, chatID, messageID int64) (*models.Message, error) {
	collection := r.db.Collection(messagesCollection)

	filter := bson.M{
		"chat_id":      chatID,
		"telegram_id": messageID,
	}

	var message models.Message
	err := collection.FindOne(ctx, filter).Decode(&message)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("message not found")
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

// RecordEdit 记录消息编辑
func (r *MongoMessageRepository) RecordEdit(ctx context.Context, message *models.Message) error {
	if message.TelegramID == 0 || message.ChatID == 0 {
		return fmt.Errorf("telegram_id and chat_id are required")
	}

	collection := r.db.Collection(messagesCollection)

	filter := bson.M{
		"chat_id":      message.ChatID,
		"telegram_id": message.TelegramID,
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"text":        message.Text,
			"caption":     message.Caption,
			"is_edited":   true,
			"edited_at":   now,
			"updated_at":  now,
		},
	}

	opts := options.Update().SetUpsert(true)
	result, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to record edit: %w", err)
	}

	// 如果是 upsert 插入的新文档，设置创建时间
	if result.UpsertedCount > 0 {
		updateCreatedAt := bson.M{
			"$setOnInsert": bson.M{
				"created_at": now,
			},
		}
		_, _ = collection.UpdateOne(ctx, filter, updateCreatedAt)
	}

	return nil
}

// GetChatMessages 获取聊天的消息列表
func (r *MongoMessageRepository) GetChatMessages(ctx context.Context, chatID int64, limit int) ([]*models.Message, error) {
	if limit <= 0 {
		limit = 100 // 默认限制
	}

	collection := r.db.Collection(messagesCollection)

	filter := bson.M{"chat_id": chatID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}). // 按时间倒序
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []*models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// GetUserMessages 获取用户发送的消息列表
func (r *MongoMessageRepository) GetUserMessages(ctx context.Context, userID int64, limit int) ([]*models.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	collection := r.db.Collection(messagesCollection)

	filter := bson.M{"user_id": userID}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get user messages: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []*models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// CountChatMessages 统计聊天的消息数量
func (r *MongoMessageRepository) CountChatMessages(ctx context.Context, chatID int64) (int64, error) {
	collection := r.db.Collection(messagesCollection)

	filter := bson.M{"chat_id": chatID}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count messages: %w", err)
	}

	return count, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoMessageRepository) EnsureIndexes(ctx context.Context) error {
	collection := r.db.Collection(messagesCollection)

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "telegram_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "chat_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "message_type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "is_edited", Value: 1}},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for messages: %w", err)
	}

	return nil
}
