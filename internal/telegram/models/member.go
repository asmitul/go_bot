package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 成员事件类型常量
const (
	MemberEventJoined     = "joined"     // 成员加入
	MemberEventLeft       = "left"       // 成员离开
	MemberEventPromoted   = "promoted"   // 成员被提升（成为管理员）
	MemberEventDemoted    = "demoted"    // 成员被降级
	MemberEventRestricted = "restricted" // 成员被限制
	MemberEventBanned     = "banned"     // 成员被封禁
)

// 成员状态常量
const (
	MemberStatusMember      = "member"       // 普通成员
	MemberStatusAdmin       = "administrator" // 管理员
	MemberStatusCreator     = "creator"      // 创建者
	MemberStatusLeft        = "left"         // 已离开
	MemberStatusKicked      = "kicked"       // 被踢出
	MemberStatusRestricted  = "restricted"   // 受限制
)

// 入群请求状态常量
const (
	JoinRequestStatusPending  = "pending"  // 待审批
	JoinRequestStatusApproved = "approved" // 已批准
	JoinRequestStatusRejected = "rejected" // 已拒绝
)

// ChatMemberEvent 成员事件记录
type ChatMemberEvent struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	ChatID      int64              `bson:"chat_id"`                  // 群组 ID
	ChatTitle   string             `bson:"chat_title,omitempty"`     // 群组名称
	UserID      int64              `bson:"user_id"`                  // 成员 Telegram ID
	Username    string             `bson:"username,omitempty"`       // 成员用户名
	FirstName   string             `bson:"first_name"`               // 成员名字
	LastName    string             `bson:"last_name,omitempty"`      // 成员姓氏
	EventType   string             `bson:"event_type"`               // 事件类型（joined/left/promoted等）
	OldStatus   string             `bson:"old_status"`               // 旧状态
	NewStatus   string             `bson:"new_status"`               // 新状态
	ChangedBy   int64              `bson:"changed_by,omitempty"`     // 操作者 ID（如果是被管理员操作）
	ChangedByUsername string        `bson:"changed_by_username,omitempty"` // 操作者用户名
	CreatedAt   time.Time          `bson:"created_at"`               // 事件时间
}

// JoinRequest 入群请求
type JoinRequest struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	ChatID         int64              `bson:"chat_id"`                   // 群组 ID
	ChatTitle      string             `bson:"chat_title,omitempty"`      // 群组名称
	UserID         int64              `bson:"user_id"`                   // 申请者 Telegram ID
	Username       string             `bson:"username,omitempty"`        // 申请者用户名
	FirstName      string             `bson:"first_name"`                // 申请者名字
	LastName       string             `bson:"last_name,omitempty"`       // 申请者姓氏
	Bio            string             `bson:"bio,omitempty"`             // 申请者简介
	Status         string             `bson:"status"`                    // 状态（pending/approved/rejected）
	ReviewedBy     int64              `bson:"reviewed_by,omitempty"`     // 审批者 ID
	ReviewedByUsername string         `bson:"reviewed_by_username,omitempty"` // 审批者用户名
	ReviewedAt     *time.Time         `bson:"reviewed_at,omitempty"`     // 审批时间
	ReviewNote     string             `bson:"review_note,omitempty"`     // 审批备注
	InviteLink     string             `bson:"invite_link,omitempty"`     // 邀请链接
	CreatedAt      time.Time          `bson:"created_at"`                // 申请时间
	UpdatedAt      time.Time          `bson:"updated_at"`                // 更新时间
}

// IsPending 是否待审批
func (j *JoinRequest) IsPending() bool {
	return j.Status == JoinRequestStatusPending
}

// IsApproved 是否已批准
func (j *JoinRequest) IsApproved() bool {
	return j.Status == JoinRequestStatusApproved
}

// IsRejected 是否已拒绝
func (j *JoinRequest) IsRejected() bool {
	return j.Status == JoinRequestStatusRejected
}

// IsNewMember 是否为新成员加入事件
func (e *ChatMemberEvent) IsNewMember() bool {
	return e.EventType == MemberEventJoined
}

// IsMemberLeft 是否为成员离开事件
func (e *ChatMemberEvent) IsMemberLeft() bool {
	return e.EventType == MemberEventLeft
}

// IsPromoted 是否为成员提升事件
func (e *ChatMemberEvent) IsPromoted() bool {
	return e.EventType == MemberEventPromoted
}
