package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 角色常量
const (
	RoleOwner = "owner" // 最高权限，由 BOT_OWNER_IDS 配置
	RoleAdmin = "admin" // 管理员权限
	RoleUser  = "user"  // 普通用户
)

// User 用户模型
type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	TelegramID   int64              `bson:"telegram_id"`             // Telegram 用户 ID（唯一）
	Username     string             `bson:"username,omitempty"`      // @username
	FirstName    string             `bson:"first_name"`              // 名字
	LastName     string             `bson:"last_name,omitempty"`     // 姓氏
	LanguageCode string             `bson:"language_code,omitempty"` // 语言代码
	IsPremium    bool               `bson:"is_premium"`              // 是否 Telegram Premium 用户
	Role         string             `bson:"role"`                    // 角色：owner/admin/user
	Permissions  []string           `bson:"permissions,omitempty"`   // 自定义权限列表（预留扩展）
	GrantedBy    int64              `bson:"granted_by,omitempty"`    // 权限授予者的 TelegramID
	GrantedAt    *time.Time         `bson:"granted_at,omitempty"`    // 权限授予时间
	CreatedAt    time.Time          `bson:"created_at"`              // 创建时间
	UpdatedAt    time.Time          `bson:"updated_at"`              // 更新时间
	LastActiveAt time.Time          `bson:"last_active_at"`          // 最后活跃时间
}

// IsOwner 是否为 Owner
func (u *User) IsOwner() bool {
	return u.Role == RoleOwner
}

// IsAdmin 是否为管理员（包括 Owner）
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleOwner
}

// CanManageUsers 是否可以管理用户
func (u *User) CanManageUsers() bool {
	return u.IsAdmin()
}

// CanManageGroups 是否可以管理群组
func (u *User) CanManageGroups() bool {
	return u.IsAdmin()
}
