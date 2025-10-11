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

// MongoGroupRepository 群组数据访问层（MongoDB 实现）
type MongoGroupRepository struct {
	collection *mongo.Collection
}

// NewMongoGroupRepository 创建群组 Repository
func NewMongoGroupRepository(db *mongo.Database) GroupRepository {
	return &MongoGroupRepository{
		collection: db.Collection("groups"),
	}
}

// CreateOrUpdate 创建或更新群组
func (r *MongoGroupRepository) CreateOrUpdate(ctx context.Context, group *models.Group) error {
	now := time.Now().UTC()
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

	// 如果指定了 Settings（非零值），则更新
	// 注意：Language 是必填字段，所以判断 Language 非空即可
	if group.Settings.Language != "" {
		setFields["settings"] = group.Settings
	}

	// 总是更新 Stats（即使是零值，例如新群组 TotalMessages=0 也需要写入）
	// 仅当 Stats 结构体未被初始化时（LastMessageAt 为零值且 TotalMessages 为 0）才跳过
	if group.Stats.TotalMessages != 0 || !group.Stats.LastMessageAt.IsZero() {
		setFields["stats"] = group.Stats
	}

	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			// bot_joined_at 记录 Bot 首次加入群组的时间
			// 注意：如果 Bot 被踢出后重新加入，此字段不会更新（保留首次加入时间）
			// 如果需要追踪最后一次加入时间，应该使用 $set 而不是 $setOnInsert
			"bot_joined_at": now,
			"created_at":    now,
			"settings": models.GroupSettings{
				WelcomeEnabled: true,
				WelcomeText:    "",
				AntiSpam:       false,
				Language:       "zh",
			},
			// Stats 初始化为零值，LastMessageAt 会在第一条消息时设置
			"stats": models.GroupStats{
				TotalMessages: 0,
				LastMessageAt: time.Time{}, // 零值，表示还没有消息
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
func (r *MongoGroupRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.Group, error) {
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
func (r *MongoGroupRepository) MarkBotLeft(ctx context.Context, telegramID int64) error {
	now := time.Now().UTC()
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
func (r *MongoGroupRepository) ListActiveGroups(ctx context.Context) ([]*models.Group, error) {
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
func (r *MongoGroupRepository) UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"settings":   settings,
			"updated_at": time.Now().UTC(),
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
func (r *MongoGroupRepository) UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"stats":      stats,
			"updated_at": time.Now().UTC(),
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

// UpdateSettingField 更新群组配置的单个字段（字段级更新）
// fieldName: settings 中的字段名（如 "welcome_enabled", "welcome_text", "anti_spam", "language"）
// value: 字段的新值
func (r *MongoGroupRepository) UpdateSettingField(ctx context.Context, telegramID int64, fieldName string, value interface{}) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"settings." + fieldName: value,
			"updated_at":            time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update setting field %s: %w", fieldName, err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}
	return nil
}

// EnsureIndexes 确保索引存在
func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context) error {
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
