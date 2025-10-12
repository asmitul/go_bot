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

// MongoMessageRepository 消息数据访问层（MongoDB 实现）
type MongoMessageRepository struct {
	collection *mongo.Collection
}

// NewMongoMessageRepository 创建消息 Repository
func NewMongoMessageRepository(db *mongo.Database) MessageRepository {
	return &MongoMessageRepository{
		collection: db.Collection("messages"),
	}
}

// CreateMessage 创建消息记录
func (r *MongoMessageRepository) CreateMessage(ctx context.Context, message *models.Message) error {
	now := time.Now()
	message.CreatedAt = now
	message.UpdatedAt = now

	// 使用 Upsert 模式，避免重复插入
	filter := bson.M{
		"telegram_message_id": message.TelegramMessageID,
		"chat_id":             message.ChatID,
	}

	setFields := bson.M{
		"user_id":                  message.UserID,
		"message_type":             message.MessageType,
		"text":                     message.Text,
		"caption":                  message.Caption,
		"media_file_id":            message.MediaFileID,
		"media_file_size":          message.MediaFileSize,
		"media_mime_type":          message.MediaMimeType,
		"media_thumbnail_id":       message.MediaThumbnailID,
		"reply_to_message_id":      message.ReplyToMessageID,
		"forward_from_chat_id":     message.ForwardFromChatID,
		"forward_from_message_id":  message.ForwardFromMessageID,
		"is_edited":                message.IsEdited,
		"edited_at":                message.EditedAt,
		"sent_at":                  message.SentAt,
		"updated_at":               message.UpdatedAt,
	}

	setOnInsert := bson.M{
		"created_at": message.CreatedAt,
	}

	update := bson.M{
		"$set":         setFields,
		"$setOnInsert": setOnInsert,
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	return nil
}

// GetByTelegramID 根据 Telegram 消息 ID 和聊天 ID 获取消息
func (r *MongoMessageRepository) GetByTelegramID(ctx context.Context, telegramMessageID, chatID int64) (*models.Message, error) {
	filter := bson.M{
		"telegram_message_id": telegramMessageID,
		"chat_id":             chatID,
	}

	var message models.Message
	err := r.collection.FindOne(ctx, filter).Decode(&message)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("message not found: message_id=%d, chat_id=%d", telegramMessageID, chatID)
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

// UpdateMessageEdit 更新消息编辑信息
func (r *MongoMessageRepository) UpdateMessageEdit(ctx context.Context, telegramMessageID, chatID int64, newText string, editedAt time.Time) error {
	filter := bson.M{
		"telegram_message_id": telegramMessageID,
		"chat_id":             chatID,
	}

	update := bson.M{
		"$set": bson.M{
			"text":       newText,
			"is_edited":  true,
			"edited_at":  editedAt,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update message edit: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("message not found: message_id=%d, chat_id=%d", telegramMessageID, chatID)
	}

	return nil
}

// ListMessagesByChat 列出聊天消息历史（分页）
func (r *MongoMessageRepository) ListMessagesByChat(ctx context.Context, chatID int64, limit, offset int64) ([]*models.Message, error) {
	filter := bson.M{"chat_id": chatID}

	// 按发送时间倒序排列
	opts := options.Find().
		SetSort(bson.D{{Key: "sent_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(offset)

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []*models.Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}

	return messages, nil
}

// CountMessagesByType 按类型统计消息数量
func (r *MongoMessageRepository) CountMessagesByType(ctx context.Context, chatID int64) (map[string]int64, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{"chat_id": chatID},
		},
		{
			"$group": bson.M{
				"_id":   "$message_type",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages by type: %w", err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]int64)
	for cursor.Next(ctx) {
		var doc struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode count result: %w", err)
		}
		result[doc.ID] = doc.Count
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return result, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoMessageRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "telegram_message_id", Value: 1},
				{Key: "chat_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "sent_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "sent_at", Value: -1},
			},
		},
		{
			Keys: bson.D{{Key: "message_type", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
