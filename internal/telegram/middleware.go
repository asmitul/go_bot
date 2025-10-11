package telegram

import (
	"context"

	"go_bot/internal/logger"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// extractUserAndChatID 从 update 中提取用户 ID 和聊天 ID（支持多种 update 类型）
// 注意：ChannelPost 和 EditedChannelPost 不包含在此函数中，因为频道消息没有 From 字段
func extractUserAndChatID(update *models.Update) (userID int64, chatID int64, ok bool) {
	switch {
	case update.Message != nil && update.Message.From != nil:
		return update.Message.From.ID, update.Message.Chat.ID, true
	case update.CallbackQuery != nil && update.CallbackQuery.From.ID != 0:
		// CallbackQuery 的 chatID 可能来自 Message 或 InlineMessage
		chatID := int64(0)
		if update.CallbackQuery.Message.Message != nil {
			chatID = update.CallbackQuery.Message.Message.Chat.ID
		}
		return update.CallbackQuery.From.ID, chatID, true
	case update.EditedMessage != nil && update.EditedMessage.From != nil:
		return update.EditedMessage.From.ID, update.EditedMessage.Chat.ID, true
	case update.InlineQuery != nil:
		return update.InlineQuery.From.ID, 0, true
	default:
		// ChannelPost 和 EditedChannelPost 等频道相关更新不包含用户信息
		return 0, 0, false
	}
}

// RequireOwner 中间件：仅允许 Owner 执行
func (b *Bot) RequireOwner(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		userID, chatID, ok := extractUserAndChatID(update)
		if !ok {
			logger.L().Warn("Unable to extract user ID from update for owner check")
			return
		}

		// 使用 Service 检查权限
		isOwner, err := b.userService.CheckOwnerPermission(ctx, userID)
		if err != nil || !isOwner {
			logger.L().Warnf("Non-owner user %d attempted to use owner command", userID)
			if chatID != 0 {
				b.sendErrorMessage(ctx, chatID, "此命令仅限 Bot Owner 使用")
			}
			return
		}

		next(ctx, botInstance, update)
	}
}

// RequireAdmin 中间件：需要管理员权限（Admin 或 Owner）
func (b *Bot) RequireAdmin(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *models.Update) {
		userID, chatID, ok := extractUserAndChatID(update)
		if !ok {
			logger.L().Warn("Unable to extract user ID from update for admin check")
			return
		}

		// 使用 Service 检查权限
		isAdmin, err := b.userService.CheckAdminPermission(ctx, userID)
		if err != nil || !isAdmin {
			logger.L().Warnf("Non-admin user %d attempted to use admin command", userID)
			if chatID != 0 {
				b.sendErrorMessage(ctx, chatID, "此命令需要管理员权限")
			}
			return
		}

		next(ctx, botInstance, update)
	}
}
