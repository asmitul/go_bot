package telegram

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// registerHandlers 注册所有命令处理器（异步执行）
func (b *Bot) registerHandlers() {
	// 普通命令 - 异步执行
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact,
		b.asyncHandler(b.handleStart))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/ping", bot.MatchTypeExact,
		b.asyncHandler(b.handlePing))

	// 管理员命令（仅 Owner） - 异步执行
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/grant", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireOwner(b.handleGrantAdmin)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/revoke", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireOwner(b.handleRevokeAdmin)))

	// 管理员命令（Admin+） - 异步执行
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/admins", bot.MatchTypeExact,
		b.asyncHandler(b.RequireAdmin(b.handleListAdmins)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/userinfo", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireAdmin(b.handleUserInfo)))

	logger.L().Debug("All handlers registered with async execution")
}

// handleStart 处理 /start 命令
func (b *Bot) handleStart(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	// 使用 Service 注册/更新用户
	userInfo := &service.TelegramUserInfo{
		TelegramID:   update.Message.From.ID,
		Username:     update.Message.From.Username,
		FirstName:    update.Message.From.FirstName,
		LastName:     update.Message.From.LastName,
		LanguageCode: update.Message.From.LanguageCode,
		IsPremium:    update.Message.From.IsPremium,
	}

	if err := b.userService.RegisterOrUpdateUser(ctx, userInfo); err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "注册失败，请稍后重试")
		return
	}

	welcomeText := fmt.Sprintf(
		"👋 你好, %s!\n\n欢迎使用本 Bot。\n\n可用命令:\n/start - 开始\n/ping - 测试连接\n/admins - 查看管理员列表（需要管理员权限）",
		update.Message.From.FirstName,
	)

	b.sendMessage(ctx, update.Message.Chat.ID, welcomeText)
}

// handlePing 处理 /ping 命令
func (b *Bot) handlePing(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	// 更新用户活跃时间
	if update.Message.From != nil {
		_ = b.userService.UpdateUserActivity(ctx, update.Message.From.ID)
	}

	b.sendMessage(ctx, update.Message.Chat.ID, "🏓 Pong!")
}

// handleGrantAdmin 处理 /grant 命令（授予管理员权限）
func (b *Bot) handleGrantAdmin(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	// 解析命令参数
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, update.Message.Chat.ID,
			"用法: /grant <user_id>\n例如: /grant 123456789")
		return
	}

	var targetID int64
	_, err := fmt.Sscanf(parts[1], "%d", &targetID)
	if err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "无效的用户 ID")
		return
	}

	// 使用 Service 授予管理员权限（包含业务验证）
	if err := b.userService.GrantAdminPermission(ctx, targetID, update.Message.From.ID); err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, err.Error())
		return
	}

	b.sendSuccessMessage(ctx, update.Message.Chat.ID,
		fmt.Sprintf("已授予用户 %d 管理员权限", targetID))
}

// handleRevokeAdmin 处理 /revoke 命令（撤销管理员权限）
func (b *Bot) handleRevokeAdmin(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	// 解析命令参数
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, update.Message.Chat.ID,
			"用法: /revoke <user_id>\n例如: /revoke 123456789")
		return
	}

	var targetID int64
	_, err := fmt.Sscanf(parts[1], "%d", &targetID)
	if err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "无效的用户 ID")
		return
	}

	// 使用 Service 撤销管理员权限（包含业务验证）
	if err := b.userService.RevokeAdminPermission(ctx, targetID, update.Message.From.ID); err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, err.Error())
		return
	}

	b.sendSuccessMessage(ctx, update.Message.Chat.ID,
		fmt.Sprintf("已撤销用户 %d 的管理员权限", targetID))
}

// handleListAdmins 处理 /admins 命令（列出所有管理员）
func (b *Bot) handleListAdmins(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	// 使用 Service 获取管理员列表
	admins, err := b.userService.ListAllAdmins(ctx)
	if err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "查询失败")
		return
	}

	if len(admins) == 0 {
		b.sendMessage(ctx, update.Message.Chat.ID, "📝 暂无管理员")
		return
	}

	var text strings.Builder
	text.WriteString("👥 管理员列表:\n\n")
	for i, admin := range admins {
		roleEmoji := "👤"
		if admin.Role == models.RoleOwner {
			roleEmoji = "👑"
		}
		text.WriteString(fmt.Sprintf("%d. %s %s (@%s) - ID: %d\n",
			i+1,
			roleEmoji,
			admin.FirstName,
			admin.Username,
			admin.TelegramID,
		))
	}

	b.sendMessage(ctx, update.Message.Chat.ID, text.String())
}

// handleUserInfo 处理 /userinfo 命令（查看用户信息）
func (b *Bot) handleUserInfo(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	// 解析命令参数
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, update.Message.Chat.ID,
			"用法: /userinfo <user_id>\n例如: /userinfo 123456789")
		return
	}

	var targetID int64
	_, err := fmt.Sscanf(parts[1], "%d", &targetID)
	if err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "无效的用户 ID")
		return
	}

	// 使用 Service 查询用户信息
	user, err := b.userService.GetUserInfo(ctx, targetID)
	if err != nil {
		b.sendErrorMessage(ctx, update.Message.Chat.ID, "用户不存在或查询失败")
		return
	}

	roleEmoji := "👤"
	if user.Role == models.RoleOwner {
		roleEmoji = "👑"
	} else if user.Role == models.RoleAdmin {
		roleEmoji = "⭐"
	}

	premiumBadge := ""
	if user.IsPremium {
		premiumBadge = " 💎"
	}

	text := fmt.Sprintf(
		"👤 用户信息\n\n"+
			"ID: %d\n"+
			"姓名: %s %s%s\n"+
			"用户名: @%s\n"+
			"角色: %s %s\n"+
			"语言: %s\n"+
			"创建时间: %s\n"+
			"最后活跃: %s",
		user.TelegramID,
		user.FirstName,
		user.LastName,
		premiumBadge,
		user.Username,
		roleEmoji,
		user.Role,
		user.LanguageCode,
		user.CreatedAt.Format("2006-01-02 15:04:05"),
		user.LastActiveAt.Format("2006-01-02 15:04:05"),
	)

	b.sendMessage(ctx, update.Message.Chat.ID, text)
}
