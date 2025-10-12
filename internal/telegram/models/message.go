package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 消息类型常量
const (
	MessageTypeText      = "text"
	MessageTypePhoto     = "photo"
	MessageTypeVideo     = "video"
	MessageTypeDocument  = "document"
	MessageTypeVoice     = "voice"
	MessageTypeAudio     = "audio"
	MessageTypeSticker   = "sticker"
	MessageTypeAnimation = "animation"
	MessageTypeChannelPost = "channel_post"
)

// Message 消息模型
type Message struct {
	ID                primitive.ObjectID `bson:"_id,omitempty"`
	TelegramMessageID int64              `bson:"telegram_message_id"` // Telegram 消息 ID
	ChatID            int64              `bson:"chat_id"`             // 所属聊天 ID
	UserID            int64              `bson:"user_id"`             // 发送者 ID（频道消息可能为 0）

	// 消息内容
	MessageType string `bson:"message_type"`         // 消息类型
	Text        string `bson:"text,omitempty"`       // 文本内容
	Caption     string `bson:"caption,omitempty"`    // 媒体说明文字

	// 媒体信息
	MediaFileID      string `bson:"media_file_id,omitempty"`      // 文件 ID
	MediaFileSize    int64  `bson:"media_file_size,omitempty"`    // 文件大小
	MediaMimeType    string `bson:"media_mime_type,omitempty"`    // MIME 类型
	MediaThumbnailID string `bson:"media_thumbnail_id,omitempty"` // 缩略图 ID

	// 关联信息
	ReplyToMessageID     int64 `bson:"reply_to_message_id,omitempty"`      // 回复的消息 ID
	ForwardFromChatID    int64 `bson:"forward_from_chat_id,omitempty"`     // 转发来源聊天 ID
	ForwardFromMessageID int64 `bson:"forward_from_message_id,omitempty"`  // 转发来源消息 ID

	// 编辑信息
	IsEdited bool       `bson:"is_edited"`           // 是否被编辑过
	EditedAt *time.Time `bson:"edited_at,omitempty"` // 编辑时间

	// 时间信息
	SentAt    time.Time `bson:"sent_at"`    // 发送时间
	CreatedAt time.Time `bson:"created_at"` // 记录创建时间
	UpdatedAt time.Time `bson:"updated_at"` // 记录更新时间
}

// IsMediaMessage 是否为媒体消息
func (m *Message) IsMediaMessage() bool {
	switch m.MessageType {
	case MessageTypePhoto, MessageTypeVideo, MessageTypeDocument,
		MessageTypeVoice, MessageTypeAudio, MessageTypeSticker, MessageTypeAnimation:
		return true
	default:
		return false
	}
}

// IsChannelPost 是否为频道消息
func (m *Message) IsChannelPost() bool {
	return m.MessageType == MessageTypeChannelPost
}
