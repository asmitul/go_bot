package types

import botModels "github.com/go-telegram/bot/models"

// Response 表示功能输出内容。
// Text 按 HTML 解析，ReplyMarkup 用于附加按钮等交互组件。
type Response struct {
	Text        string
	ReplyMarkup botModels.ReplyMarkup
	Temporary   bool // 标记为临时消息时由 handler 发送后自动删除
}
