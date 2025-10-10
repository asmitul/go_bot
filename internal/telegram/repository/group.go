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

// GroupRepository 群组数据访问层
type GroupRepository struct {
	collection *mongo.Collection
}

// NewGroupRepository 创建群组 Repository
func NewGroupRepository(db *mongo.Database) *GroupRepository {
	return &GroupRepository{
		collection: db.Collection("groups"),
	}
}

// CreateOrUpdate 创建或更新群组
func (r *GroupRepository) CreateOrUpdate(ctx context.Context, group *models.Group) error {
	now := time.Now()
	group.UpdatedAt = now

	filter := bson.M{"telegram_id": group.TelegramID}

	setFields := bson.M{
		"type":         group.Type,
		"title":        group.Title,
		"username":     group.Username,
		"description":  group.Description,
		"member_count": group.MemberCount,
		"bot_status":   group.BotStatus,
		"updated_at":   group.UpdatedAt,
	}

	// 如果指定了 BotJoinedAt，则更新
	if !group.BotJoinedAt.IsZero() {
		setFields["bot_joined_at"] = group.BotJoinedAt
	}

	// 如果指定了 BotLeftAt，则更新
	if group.BotLeftAt != nil {
		setFields["bot_left_at"] = group.BotLeftAt
	}

	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			"bot_joined_at": now,
			"created_at":    now,
			"settings": models.GroupSettings{
				WelcomeEnabled: false,
				WelcomeText:    "",
				AntiSpam:       false,
				Language:       "en",
			},
			"stats": models.GroupStats{
				TotalMessages: 0,
				LastMessageAt: now,
			},
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to create or update group: %w", err)
	}

	return nil
}

// GetByTelegramID 根据 Telegram ID 获取群组
func (r *GroupRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.Group, error) {
	var group models.Group
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&group)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("group not found: %d", telegramID)
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}
	return &group, nil
}

// MarkBotLeft 标记 Bot 离开群组
func (r *GroupRepository) MarkBotLeft(ctx context.Context, telegramID int64) error {
	now := time.Now()
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"bot_status": models.BotStatusLeft,
			"bot_left_at": now,
			"updated_at": now,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to mark bot left: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}
	return nil
}

// ListActiveGroups 列出所有活跃群组
func (r *GroupRepository) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
	filter := bson.M{"bot_status": models.BotStatusActive}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list active groups: %w", err)
	}
	defer cursor.Close(ctx)

	var groups []*models.Group
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, fmt.Errorf("failed to decode groups: %w", err)
	}

	return groups, nil
}

// UpdateSettings 更新群组配置
func (r *GroupRepository) UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"settings":   settings,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}
	return nil
}

// UpdateStats 更新群组统计信息
func (r *GroupRepository) UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"stats":      stats,
			"updated_at": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update stats: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}
	return nil
}

// EnsureIndexes 确保索引存在
func (r *GroupRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "telegram_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "bot_status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "type", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
