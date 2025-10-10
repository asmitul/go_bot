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

// UserRepository 用户数据访问层
type UserRepository struct {
	collection *mongo.Collection
}

// NewUserRepository 创建用户 Repository
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{
		collection: db.Collection("users"),
	}
}

// CreateOrUpdate 创建或更新用户
func (r *UserRepository) CreateOrUpdate(ctx context.Context, user *models.User) error {
	now := time.Now()
	user.UpdatedAt = now

	filter := bson.M{"telegram_id": user.TelegramID}

	setFields := bson.M{
		"username":       user.Username,
		"first_name":     user.FirstName,
		"last_name":      user.LastName,
		"language_code":  user.LanguageCode,
		"is_premium":     user.IsPremium,
		"updated_at":     user.UpdatedAt,
		"last_active_at": user.LastActiveAt,
	}

	// 如果用户指定了角色（如初始化 owner），则更新角色
	if user.Role != "" {
		setFields["role"] = user.Role
	}

	// 如果有授予信息，则更新
	if user.GrantedBy != 0 {
		setFields["granted_by"] = user.GrantedBy
		setFields["granted_at"] = user.GrantedAt
	}

	update := bson.M{
		"$set": setFields,
		"$setOnInsert": bson.M{
			"role":       models.RoleUser, // 默认角色为普通用户
			"created_at": now,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to create or update user: %w", err)
	}

	return nil
}

// GetByTelegramID 根据 Telegram ID 获取用户
func (r *UserRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	var user models.User
	err := r.collection.FindOne(ctx, bson.M{"telegram_id": telegramID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("user not found: %d", telegramID)
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// UpdateLastActive 更新用户最后活跃时间
func (r *UserRepository) UpdateLastActive(ctx context.Context, telegramID int64) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"last_active_at": time.Now(),
		},
	}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update last active: %w", err)
	}
	return nil
}

// GrantAdmin 授予管理员权限
func (r *UserRepository) GrantAdmin(ctx context.Context, telegramID int64, grantedBy int64) error {
	now := time.Now()
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"role":       models.RoleAdmin,
			"granted_by": grantedBy,
			"granted_at": now,
			"updated_at": now,
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to grant admin: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found: %d", telegramID)
	}
	return nil
}

// RevokeAdmin 撤销管理员权限
func (r *UserRepository) RevokeAdmin(ctx context.Context, telegramID int64) error {
	filter := bson.M{"telegram_id": telegramID}
	update := bson.M{
		"$set": bson.M{
			"role":       models.RoleUser,
			"updated_at": time.Now(),
		},
		"$unset": bson.M{
			"granted_by": "",
			"granted_at": "",
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to revoke admin: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("user not found: %d", telegramID)
	}
	return nil
}

// ListAdmins 列出所有管理员
func (r *UserRepository) ListAdmins(ctx context.Context) ([]*models.User, error) {
	filter := bson.M{
		"role": bson.M{
			"$in": []string{models.RoleOwner, models.RoleAdmin},
		},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list admins: %w", err)
	}
	defer cursor.Close(ctx)

	var admins []*models.User
	if err := cursor.All(ctx, &admins); err != nil {
		return nil, fmt.Errorf("failed to decode admins: %w", err)
	}

	return admins, nil
}

// GetUserInfo 获取用户完整信息（同 GetByTelegramID，用于语义区分）
func (r *UserRepository) GetUserInfo(ctx context.Context, telegramID int64) (*models.User, error) {
	return r.GetByTelegramID(ctx, telegramID)
}

// EnsureIndexes 确保索引存在
func (r *UserRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "telegram_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "role", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "last_active_at", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}
