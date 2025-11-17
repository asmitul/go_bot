package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go_bot/internal/telegram/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	if group.Tier != "" {
		setFields["tier"] = group.Tier
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
			"tier":          models.GroupTierBasic,
			"settings": models.GroupSettings{
				CalculatorEnabled:        true,  // 新群组默认启用计算器功能
				CryptoEnabled:            true,  // 新群组默认启用加密货币功能
				CryptoFloatRate:          0.12,  // 新群组默认浮动费率 0.12
				ForwardEnabled:           true,  // 新群组默认接收频道转发消息
				AccountingEnabled:        false, // 新群组默认关闭收支记账功能
				InterfaceBindings:        nil,   // 初始不绑定接口
				SifangEnabled:            true,  // 新群组默认启用四方支付功能
				SifangAutoLookupEnabled:  true,  // 新群组默认启用四方自动查单
				CascadeForwardEnabled:    true,  // 新群组默认启用订单联动
				CascadeForwardConfigured: true,
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

// FindByInterfaceID 根据接口 ID 查找绑定的群组
func (r *MongoGroupRepository) FindByInterfaceID(ctx context.Context, interfaceID string) (*models.Group, error) {
	cleanID := strings.TrimSpace(interfaceID)
	if cleanID == "" {
		return nil, fmt.Errorf("interface id is required")
	}

	filter := bson.M{
		"settings.interface_bindings": bson.M{
			"$elemMatch": bson.M{
				"id": primitive.Regex{
					Pattern: fmt.Sprintf("^%s$", regexp.QuoteMeta(cleanID)),
					Options: "i",
				},
			},
		},
	}

	var group models.Group
	err := r.collection.FindOne(ctx, filter).Decode(&group)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find group by interface id: %w", err)
	}
	return &group, nil
}

// UpdateBotStatus 更新 Bot 在群组中的状态
func (r *MongoGroupRepository) UpdateBotStatus(ctx context.Context, telegramID int64, status string) error {
	now := time.Now()
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"bot_status":  status,
			"bot_left_at": now,
			"updated_at":  now,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update bot status: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}
	return nil
}

// DeleteGroup 删除群组（Bot 离开时）
func (r *MongoGroupRepository) DeleteGroup(ctx context.Context, telegramID int64) error {
	filter := bson.M{"telegram_id": telegramID}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("group not found: %d", telegramID)
	}

	return nil
}

// ListAllGroups 列出所有群组
func (r *MongoGroupRepository) ListAllGroups(ctx context.Context) ([]*models.Group, error) {
	cursor, err := r.collection.Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	defer cursor.Close(ctx)

	var groups []*models.Group
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, fmt.Errorf("failed to decode groups: %w", err)
	}
	return groups, nil
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
func (r *MongoGroupRepository) UpdateSettings(ctx context.Context, telegramID int64, settings models.GroupSettings, tier models.GroupTier) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"settings":   settings,
			"tier":       tier,
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
func (r *MongoGroupRepository) UpdateStats(ctx context.Context, telegramID int64, stats models.GroupStats) error {
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

// EnsureIndexes 确保索引存在（ttlSeconds 参数保留用于接口一致性，Group 不需要 TTL）
func (r *MongoGroupRepository) EnsureIndexes(ctx context.Context, ttlSeconds int32) error {
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
		{
			Keys: bson.D{{Key: "settings.interface_bindings.id", Value: 1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
