package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"

	"go_bot/internal/logger"
)

// sendMessage 发送消息（统一错误处理，使用 HTML 格式）
func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string) {
	if _, err := b.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: botModels.ParseModeHTML,
	}); err != nil {
		logger.L().Errorf("Failed to send message to chat %d: %v", chatID, err)
	}
}

// sendErrorMessage 发送错误消息
func (b *Bot) sendErrorMessage(ctx context.Context, chatID int64, message string) {
	b.sendMessage(ctx, chatID, "❌ "+message)
}

// sendSuccessMessage 发送成功消息
func (b *Bot) sendSuccessMessage(ctx context.Context, chatID int64, message string) {
	b.sendMessage(ctx, chatID, "✅ "+message)
}
