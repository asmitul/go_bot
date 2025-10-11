package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 消息类型常量
const (
	MessageTypeText      = "text"      // 文本消息
	MessageTypePhoto     = "photo"     // 图片
	MessageTypeVideo     = "video"     // 视频
	MessageTypeDocument  = "document"  // 文件
	MessageTypeVoice     = "voice"     // 语音
	MessageTypeAudio     = "audio"     // 音频
	MessageTypeSticker   = "sticker"   // 贴纸
	MessageTypeAnimation = "animation" // 动图（GIF）
	MessageTypeLocation  = "location"  // 位置
	MessageTypeContact   = "contact"   // 联系人
	MessageTypePoll      = "poll"      // 投票
)

// Message 消息记录模型
type Message struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	TelegramID     int64              `bson:"telegram_id"`              // Telegram Message ID
	ChatID         int64              `bson:"chat_id"`                  // 所属聊天 ID
	UserID         int64              `bson:"user_id"`                  // 发送者 Telegram User ID
	Username       string             `bson:"username,omitempty"`       // 发送者用户名
	MessageType    string             `bson:"message_type"`             // 消息类型（text/photo/video等）
	Text           string             `bson:"text,omitempty"`           // 文本内容
	Caption        string             `bson:"caption,omitempty"`        // 媒体说明文字
	FileID         string             `bson:"file_id,omitempty"`        // 文件 ID（用于下载/转发）
	FileUniqueID   string             `bson:"file_unique_id,omitempty"` // 文件唯一 ID
	FileSize       int64              `bson:"file_size,omitempty"`      // 文件大小（字节）
	FileName       string             `bson:"file_name,omitempty"`      // 文件名（Document 类型）
	MimeType       string             `bson:"mime_type,omitempty"`      // MIME 类型
	Duration       int                `bson:"duration,omitempty"`       // 音视频时长（秒）
	Width          int                `bson:"width,omitempty"`          // 图片/视频宽度
	Height         int                `bson:"height,omitempty"`         // 图片/视频高度
	IsEdited       bool               `bson:"is_edited"`                // 是否被编辑过
	EditedAt       *time.Time         `bson:"edited_at,omitempty"`      // 编辑时间
	IsChannelPost  bool               `bson:"is_channel_post"`          // 是否为频道消息
	ForwardFromID  int64              `bson:"forward_from_id,omitempty"` // 转发来源用户 ID
	ForwardFromChatID int64           `bson:"forward_from_chat_id,omitempty"` // 转发来源聊天 ID
	ReplyToMessageID int64            `bson:"reply_to_message_id,omitempty"` // 回复的消息 ID
	CreatedAt      time.Time          `bson:"created_at"`               // 创建时间
	UpdatedAt      time.Time          `bson:"updated_at"`               // 更新时间
}

// IsMedia 是否为媒体消息
func (m *Message) IsMedia() bool {
	return m.MessageType == MessageTypePhoto ||
		m.MessageType == MessageTypeVideo ||
		m.MessageType == MessageTypeDocument ||
		m.MessageType == MessageTypeVoice ||
		m.MessageType == MessageTypeAudio ||
		m.MessageType == MessageTypeSticker ||
		m.MessageType == MessageTypeAnimation
}

// GetFileID 获取文件 ID（如果是媒体消息）
func (m *Message) GetFileID() string {
	if m.IsMedia() {
		return m.FileID
	}
	return ""
}

// GetContent 获取消息内容（文本或 Caption）
func (m *Message) GetContent() string {
	if m.Text != "" {
		return m.Text
	}
	return m.Caption
}
