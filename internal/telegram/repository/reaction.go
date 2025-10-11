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

const (
	messageReactionsCollection     = "message_reactions"
	messageReactionCountsCollection = "message_reaction_counts"
)

// MongoReactionRepository MongoDB 反应仓库实现
type MongoReactionRepository struct {
	db *mongo.Database
}

// NewMongoReactionRepository 创建反应仓库实例
func NewMongoReactionRepository(db *mongo.Database) ReactionRepository {
	return &MongoReactionRepository{db: db}
}

// RecordReaction 记录消息反应
func (r *MongoReactionRepository) RecordReaction(ctx context.Context, reaction *models.MessageReactionRecord) error {
	if reaction.ChatID == 0 || reaction.MessageID == 0 || reaction.UserID == 0 {
		return fmt.Errorf("chat_id, message_id and user_id are required")
	}

	now := time.Now().UTC()
	if reaction.CreatedAt.IsZero() {
		reaction.CreatedAt = now
	}
	reaction.UpdatedAt = now

	collection := r.db.Collection(messageReactionsCollection)

	// 使用 upsert 更新用户对消息的反应
	filter := bson.M{
		"chat_id":    reaction.ChatID,
		"message_id": reaction.MessageID,
		"user_id":    reaction.UserID,
	}

	update := bson.M{
		"$set": reaction,
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to record reaction: %w", err)
	}

	return nil
}

// UpdateReactionCount 更新消息反应统计
func (r *MongoReactionRepository) UpdateReactionCount(ctx context.Context, count *models.MessageReactionCountRecord) error {
	if count.ChatID == 0 || count.MessageID == 0 {
		return fmt.Errorf("chat_id and message_id are required")
	}

	now := time.Now().UTC()
	if count.CreatedAt.IsZero() {
		count.CreatedAt = now
	}
	count.UpdatedAt = now

	collection := r.db.Collection(messageReactionCountsCollection)

	// 使用 upsert 更新统计
	filter := bson.M{
		"chat_id":    count.ChatID,
		"message_id": count.MessageID,
	}

	update := bson.M{
		"$set": count,
	}

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update reaction count: %w", err)
	}

	return nil
}

// GetMessageReactions 获取消息的所有反应
func (r *MongoReactionRepository) GetMessageReactions(ctx context.Context, chatID, messageID int64) ([]*models.MessageReactionRecord, error) {
	collection := r.db.Collection(messageReactionsCollection)

	filter := bson.M{
		"chat_id":    chatID,
		"message_id": messageID,
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get message reactions: %w", err)
	}
	defer cursor.Close(ctx)

	var reactions []*models.MessageReactionRecord
	if err := cursor.All(ctx, &reactions); err != nil {
		return nil, fmt.Errorf("failed to decode reactions: %w", err)
	}

	return reactions, nil
}

// GetReactionCount 获取消息反应统计
func (r *MongoReactionRepository) GetReactionCount(ctx context.Context, chatID, messageID int64) (*models.MessageReactionCountRecord, error) {
	collection := r.db.Collection(messageReactionCountsCollection)

	filter := bson.M{
		"chat_id":    chatID,
		"message_id": messageID,
	}

	var count models.MessageReactionCountRecord
	err := collection.FindOne(ctx, filter).Decode(&count)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("reaction count not found")
		}
		return nil, fmt.Errorf("failed to get reaction count: %w", err)
	}

	return &count, nil
}

// GetTopReactedMessages 获取反应最多的消息
func (r *MongoReactionRepository) GetTopReactedMessages(ctx context.Context, chatID int64, limit int) ([]*models.MessageReactionCountRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	collection := r.db.Collection(messageReactionCountsCollection)

	filter := bson.M{"chat_id": chatID}
	opts := options.Find().
		SetSort(bson.D{{Key: "total_count", Value: -1}}).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get top reacted messages: %w", err)
	}
	defer cursor.Close(ctx)

	var counts []*models.MessageReactionCountRecord
	if err := cursor.All(ctx, &counts); err != nil {
		return nil, fmt.Errorf("failed to decode counts: %w", err)
	}

	return counts, nil
}

// EnsureIndexes 确保索引存在
func (r *MongoReactionRepository) EnsureIndexes(ctx context.Context) error {
	// message_reactions 索引
	reactionsCollection := r.db.Collection(messageReactionsCollection)
	reactionsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "message_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "message_id", Value: 1},
			},
		},
		{
			Keys: bson.D{{Key: "user_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
	}

	_, err := reactionsCollection.Indexes().CreateMany(ctx, reactionsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for message_reactions: %w", err)
	}

	// message_reaction_counts 索引
	countsCollection := r.db.Collection(messageReactionCountsCollection)
	countsIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "chat_id", Value: 1},
				{Key: "message_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "chat_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "total_count", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "updated_at", Value: -1}},
		},
	}

	_, err = countsCollection.Indexes().CreateMany(ctx, countsIndexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes for message_reaction_counts: %w", err)
	}

	return nil
}
