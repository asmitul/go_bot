package telegram

import (
	"context"
	"time"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"

	"go_bot/internal/logger"
)

const (
	temporaryMessageLifetime = 5 * time.Second
	temporaryDeleteTimeout   = 5 * time.Second
)

// sendMessage 发送消息（统一错误处理，使用 HTML 格式）
func (b *Bot) sendMessage(ctx context.Context, chatID int64, text string, replyTo ...int) {
	_, _ = b.sendMessageWithMarkupAndMessage(ctx, chatID, text, nil, replyTo...)
}

// sendMessageWithMarkupAndMessage 发送消息并返回 Telegram Message
func (b *Bot) sendMessageWithMarkupAndMessage(ctx context.Context, chatID int64, text string, markup botModels.ReplyMarkup, replyTo ...int) (*botModels.Message, error) {
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

	msg, err := b.bot.SendMessage(ctx, params)
	if err != nil {
		logger.L().Errorf("Failed to send message to chat %d: %v", chatID, err)
		return nil, err
	}

	return msg, nil
}

// sendErrorMessage 发送错误消息
func (b *Bot) sendErrorMessage(ctx context.Context, chatID int64, message string, replyTo ...int) {
	b.sendMessage(ctx, chatID, "❌ "+message, replyTo...)
}

// sendTemporaryMessage 发送临时消息，会在短时间后自动删除
func (b *Bot) sendTemporaryMessage(ctx context.Context, chatID int64, text string, replyTo ...int) (*botModels.Message, error) {
	return b.sendTemporaryMessageWithMarkup(ctx, chatID, text, nil, replyTo...)
}

// sendTemporaryErrorMessage 发送临时错误消息，自动添加错误前缀
func (b *Bot) sendTemporaryErrorMessage(ctx context.Context, chatID int64, message string, replyTo ...int) {
	_, _ = b.sendTemporaryMessage(ctx, chatID, "❌ "+message, replyTo...)
}

// sendSuccessMessage 发送成功消息
func (b *Bot) sendSuccessMessage(ctx context.Context, chatID int64, message string, replyTo ...int) {
	b.sendMessage(ctx, chatID, "✅ "+message, replyTo...)
}

// sendTemporaryMessageWithMarkup 发送临时消息（支持自定义 Markup）
func (b *Bot) sendTemporaryMessageWithMarkup(ctx context.Context, chatID int64, text string, markup botModels.ReplyMarkup, replyTo ...int) (*botModels.Message, error) {
	msg, err := b.sendMessageWithMarkupAndMessage(ctx, chatID, text, markup, replyTo...)
	if err != nil || msg == nil {
		return msg, err
	}

	deleteCtx := b.tempMessageCtx
	if deleteCtx == nil {
		deleteCtx = context.Background()
	}

	go func(chatID int64, messageID int) {
		timer := time.NewTimer(temporaryMessageLifetime)
		defer timer.Stop()

		select {
		case <-timer.C:
			logger.L().Infof("Attempting to delete temporary message: chat_id=%d message_id=%d", chatID, messageID)

			ctx, cancel := context.WithTimeout(deleteCtx, temporaryDeleteTimeout)
			defer cancel()

			if _, err := b.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
				ChatID:    chatID,
				MessageID: messageID,
			}); err != nil {
				logger.L().Errorf("Failed to delete temporary message: chat_id=%d message_id=%d err=%v", chatID, messageID, err)
			}
		case <-deleteCtx.Done():
			return
		}
	}(chatID, msg.ID)

	return msg, nil
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
