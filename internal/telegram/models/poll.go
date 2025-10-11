package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 投票类型常量
const (
	PollTypeRegular = "regular" // 普通投票
	PollTypeQuiz    = "quiz"    // 测验
)

// PollRecord 投票记录
type PollRecord struct {
	ID                    primitive.ObjectID `bson:"_id,omitempty"`
	PollID                string             `bson:"poll_id"`                        // Telegram Poll ID（唯一）
	ChatID                int64              `bson:"chat_id,omitempty"`              // 投票所在聊天
	MessageID             int64              `bson:"message_id,omitempty"`           // 投票消息 ID
	Question              string             `bson:"question"`                       // 投票问题
	Options               []PollOption       `bson:"options"`                        // 投票选项
	Type                  string             `bson:"type"`                           // 投票类型（regular/quiz）
	AllowsMultipleAnswers bool               `bson:"allows_multiple_answers"`        // 是否允许多选
	IsAnonymous           bool               `bson:"is_anonymous"`                   // 是否匿名
	IsClosed              bool               `bson:"is_closed"`                      // 是否已关闭
	CorrectOptionID       int                `bson:"correct_option_id,omitempty"`    // 正确答案 ID（quiz）
	TotalVoterCount       int                `bson:"total_voter_count"`              // 总投票人数
	CreatedBy             int64              `bson:"created_by,omitempty"`           // 创建者 ID
	CreatedAt             time.Time          `bson:"created_at"`                     // 创建时间
	UpdatedAt             time.Time          `bson:"updated_at"`                     // 更新时间
	ClosedAt              *time.Time         `bson:"closed_at,omitempty"`            // 关闭时间
}

// PollOption 投票选项
type PollOption struct {
	Text       string `bson:"text"`        // 选项文本
	VoterCount int    `bson:"voter_count"` // 得票数
}

// PollAnswer 投票回答
type PollAnswer struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	PollID     string             `bson:"poll_id"`             // 投票 ID
	UserID     int64              `bson:"user_id"`             // 投票用户 ID
	Username   string             `bson:"username,omitempty"`  // 用户名
	OptionIDs  []int              `bson:"option_ids"`          // 选择的选项 ID 列表
	IsCorrect  *bool              `bson:"is_correct,omitempty"` // 是否正确（quiz）
	CreatedAt  time.Time          `bson:"created_at"`          // 投票时间
}

// IsQuiz 是否为测验
func (p *PollRecord) IsQuiz() bool {
	return p.Type == PollTypeQuiz
}

// GetWinningOption 获取得票最多的选项
func (p *PollRecord) GetWinningOption() *PollOption {
	if len(p.Options) == 0 {
		return nil
	}

	maxVotes := p.Options[0].VoterCount
	winningIdx := 0

	for i, opt := range p.Options {
		if opt.VoterCount > maxVotes {
			maxVotes = opt.VoterCount
			winningIdx = i
		}
	}

	return &p.Options[winningIdx]
}
