package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"

	"go_bot/internal/logger"
)

// sendMessage 发送消息（统一错误处理，使用 HTML 格式）
func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string, replyTo ...int) {
	b.sendMessageWithMarkup(ctx, chatID, text, nil, replyTo...)
}

// sendMessageWithMarkup 发送带自定义 ReplyMarkup 的消息
func (b *Bot) sendMessageWithMarkup(ctx context.Context, chatID int64, text string, markup botModels.ReplyMarkup, replyTo ...int) {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: botModels.ParseModeHTML,
	}

	if len(replyTo) > 0 && replyTo[0] > 0 {
		params.ReplyParameters = &botModels.ReplyParameters{
			MessageID: replyTo[0],
		}
	}

	if markup != nil {
		params.ReplyMarkup = markup
	}

	if _, err := b.bot.SendMessage(ctx, params); err != nil {
		logger.L().Errorf("Failed to send message to chat %d: %v", chatID, err)
	}
}

// sendErrorMessage 发送错误消息
func (b *Bot) sendErrorMessage(ctx context.Context, chatID int64, message string, replyTo ...int) {
	b.sendMessage(ctx, chatID, "❌ "+message, replyTo...)
}

// sendSuccessMessage 发送成功消息
func (b *Bot) sendSuccessMessage(ctx context.Context, chatID int64, message string, replyTo ...int) {
	b.sendMessage(ctx, chatID, "✅ "+message, replyTo...)
}

func (b *Bot) editMessage(ctx context.Context, chatID int64, messageID int, text string, markup botModels.ReplyMarkup) {
	params := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: botModels.ParseModeHTML,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := b.bot.EditMessageText(ctx, params); err != nil {
		logger.L().Errorf("Failed to edit message %d in chat %d: %v", messageID, chatID, err)
	}
}
