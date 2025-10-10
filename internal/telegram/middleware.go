package telegram

import (
	"context"

	"go_bot/internal/logger"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// RequireOwner 中间件：仅允许 Owner 执行
func (b *Bot) RequireOwner(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		user, err := b.userRepo.GetByTelegramID(ctx, update.Message.From.ID)
		if err != nil || !user.IsOwner() {
			logger.L().Warnf("Non-owner user %d attempted to use owner command", update.Message.From.ID)
			_, _ = botInstance.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ 此命令仅限 Bot Owner 使用",
			})
			return
		}

		next(ctx, botInstance, update)
	}
}

// RequireAdmin 中间件：需要管理员权限（Admin 或 Owner）
func (b *Bot) RequireAdmin(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		user, err := b.userRepo.GetByTelegramID(ctx, update.Message.From.ID)
		if err != nil || !user.IsAdmin() {
			logger.L().Warnf("Non-admin user %d attempted to use admin command", update.Message.From.ID)
			_, _ = botInstance.SendMessage(ctx, &bot.SendMessageParams{
				ChatID: update.Message.Chat.ID,
				Text:   "❌ 此命令需要管理员权限",
			})
			return
		}

		next(ctx, botInstance, update)
	}
}
