package telegram

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// RequireOwner 中间件：仅允许 Owner 执行
func (b *Bot) RequireOwner(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		// 使用 Service 检查权限
		isOwner, err := b.userService.CheckOwnerPermission(ctx, update.Message.From.ID)
		if err != nil || !isOwner {
			logger.L().Warnf("Non-owner user %d attempted to use owner command", update.Message.From.ID)
			b.sendErrorMessage(ctx, update.Message.Chat.ID, "此命令仅限 Bot Owner 使用")
			return
		}

		next(ctx, botInstance, update)
	}
}

// RequireAdmin 中间件：需要管理员权限（Admin 或 Owner）
func (b *Bot) RequireAdmin(next bot.HandlerFunc) bot.HandlerFunc {
	return func(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		// 使用 Service 检查权限
		isAdmin, err := b.userService.CheckAdminPermission(ctx, update.Message.From.ID)
		if err != nil || !isAdmin {
			logger.L().Warnf("Non-admin user %d attempted to use admin command", update.Message.From.ID)
			b.sendErrorMessage(ctx, update.Message.Chat.ID, "此命令需要管理员权限")
			return
		}

		next(ctx, botInstance, update)
	}
}

// RequireGroupTier 中间件：限制命令只能在指定群等级执行
func (b *Bot) RequireGroupTier(allowed []models.GroupTier, next bot.HandlerFunc) bot.HandlerFunc {
	allowedCopy := append([]models.GroupTier(nil), allowed...)

	return func(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
		if update.Message == nil {
			next(ctx, botInstance, update)
			return
		}

		chatID := update.Message.Chat.ID
		group, err := b.groupService.GetGroupInfo(ctx, chatID)
		if err != nil {
			logger.L().Warnf("Failed to load group for tier guard: chat_id=%d err=%v", chatID, err)
			b.sendTemporaryErrorMessage(ctx, chatID, "获取群组信息失败，请稍后再试")
			return
		}

		tier := models.NormalizeGroupTier(group.Tier)
		if !models.IsTierAllowed(tier, allowedCopy) {
			logger.L().Infof("Command blocked due to tier mismatch: chat_id=%d tier=%s text=%q allowed=%v",
				chatID, tier, update.Message.Text, allowedCopy)
			notice := fmt.Sprintf("⚠️ 此命令仅适用于：%s\n当前群类型：%s",
				models.FormatAllowedTierList(allowedCopy), models.GroupTierDisplayName(tier))
			b.sendTemporaryMessageWithMarkup(ctx, chatID, notice, nil)
			return
		}

		next(ctx, botInstance, update)
	}
}
