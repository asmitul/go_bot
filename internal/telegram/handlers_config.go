package telegram

import (
	"context"

	"go_bot/internal/logger"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// handleConfigs 处理 /configs 命令
// 显示交互式配置菜单
// 注意：权限检查由 RequireAdmin 中间件完成
func (b *Bot) handleConfigs(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// 获取配置项定义
	items := b.getConfigItems()

	// 构建菜单
	keyboard, err := b.configMenuService.BuildMainMenu(ctx, chatID, items)
	if err != nil {
		logger.L().Errorf("Failed to build config menu: chat_id=%d, error=%v", chatID, err)
		b.sendErrorMessage(ctx, chatID, "❌ 构建配置菜单失败，请稍后重试")
		return
	}

	// 发送菜单
	menuText := "⚙️ **群组配置菜单**\n\n" +
		"点击下方按钮进行配置：\n" +
		"• ✅/❌ = 开关状态\n" +
		"• ✏️ = 点击后输入\n" +
		"• ▶️ = 执行操作"

	_, err = botInstance.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        menuText,
		ParseMode:   botModels.ParseModeMarkdown,
		ReplyMarkup: keyboard,
	})

	if err != nil {
		logger.L().Errorf("Failed to send config menu: %v", err)
		b.sendErrorMessage(ctx, chatID, "❌ 发送配置菜单失败")
	} else {
		logger.L().Infof("Config menu sent: chat_id=%d, user_id=%d", chatID, update.Message.From.ID)
	}
}

// handleConfigCallback 处理配置菜单的回调查询
// 处理用户点击 InlineKeyboard 按钮
func (b *Bot) handleConfigCallback(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.CallbackQuery == nil {
		return
	}

	query := update.CallbackQuery

	// 检查消息是否可访问
	if query.Message.Message == nil {
		logger.L().Warn("Callback query message is inaccessible")
		return
	}

	chatID := query.Message.Message.Chat.ID
	userID := query.From.ID
	messageID := query.Message.Message.ID
	callbackData := query.Data

	// 权限检查：只有管理员可以操作
	user, err := b.userService.GetUserInfo(ctx, userID)
	if err != nil || !user.IsAdmin() {
		b.answerCallback(ctx, botInstance, query.ID, "⚠️ 只有管理员可以操作配置")
		logger.L().Warnf("Non-admin user %d attempted to use config callback in chat %d", userID, chatID)
		return
	}

	// 获取配置项定义
	items := b.getConfigItems()

	// 处理回调
	message, shouldUpdateMenu, err := b.configMenuService.HandleCallback(ctx, chatID, userID, callbackData, items)

	if err != nil {
		logger.L().Errorf("Failed to handle config callback: data=%s, error=%v", callbackData, err)
		b.answerCallback(ctx, botInstance, query.ID, "❌ 操作失败")
		return
	}

	// 回应回调查询（显示提示消息）
	if message != "" {
		b.answerCallback(ctx, botInstance, query.ID, message)
	}

	// 如果需要更新菜单，重新构建并编辑消息
	if shouldUpdateMenu {
		keyboard, err := b.configMenuService.BuildMainMenu(ctx, chatID, items)
		if err != nil {
			logger.L().Errorf("Failed to rebuild config menu: %v", err)
			return
		}

		menuText := "⚙️ **群组配置菜单**\n\n" +
			"点击下方按钮进行配置：\n" +
			"• ✅/❌ = 开关状态\n" +
			"• ✏️ = 点击后输入\n" +
			"• ▶️ = 执行操作"

		_, err = botInstance.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      chatID,
			MessageID:   messageID,
			Text:        menuText,
			ParseMode:   botModels.ParseModeMarkdown,
			ReplyMarkup: keyboard,
		})

		if err != nil {
			logger.L().Errorf("Failed to update config menu: %v", err)
		}
	}

	// 处理特殊操作：关闭菜单
	if callbackData == "config:close" {
		_, err := botInstance.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    chatID,
			MessageID: messageID,
		})
		if err != nil {
			logger.L().Errorf("Failed to delete config menu: %v", err)
		}
	}
}

// answerCallback 回应 callback query（显示顶部提示）
func (b *Bot) answerCallback(ctx context.Context, botInstance *bot.Bot, callbackQueryID, text string) {
	_, err := botInstance.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackQueryID,
		Text:            text,
		ShowAlert:       false, // 显示为顶部提示，不弹窗
	})
	if err != nil {
		logger.L().Errorf("Failed to answer callback query: %v", err)
	}
}
