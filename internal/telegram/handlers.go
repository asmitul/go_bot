package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	// ========== 阶段 1: 新增 Handler 注册 ==========

	// CallbackQuery - 内联按钮回调
	b.bot.RegisterHandler(bot.HandlerTypeCallbackQueryData, "", bot.MatchTypePrefix,
		b.asyncHandler(b.handleCallback))

	// EditedMessage - 消息编辑事件
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.EditedMessage != nil
	}, b.asyncHandler(b.handleEditedMessage))

	// MyChatMember - Bot 状态变化事件
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.MyChatMember != nil
	}, b.asyncHandler(b.handleMyChatMember))

	// MediaMessage - 媒体消息（图片、视频、文件等）
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		if update.Message == nil {
			return false
		}
		msg := update.Message
		return msg.Photo != nil || msg.Video != nil || msg.Document != nil ||
			msg.Voice != nil || msg.Audio != nil || msg.Sticker != nil || msg.Animation != nil
	}, b.asyncHandler(b.handleMediaMessage))

	// ChannelPost - 频道消息
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.ChannelPost != nil
	}, b.asyncHandler(b.handleChannelPost))

	// ========== 阶段 2: 群组管理 Handler 注册 ==========

	// ChatMember - 成员状态变化事件
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.ChatMember != nil
	}, b.asyncHandler(b.handleChatMember))

	// ChatJoinRequest - 入群申请
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.ChatJoinRequest != nil
	}, b.asyncHandler(b.handleChatJoinRequest))

	// NewChatMembers - 新成员加入（系统消息）
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.Message != nil && update.Message.NewChatMembers != nil
	}, b.asyncHandler(b.handleNewChatMembers))

	// LeftChatMember - 成员离开（系统消息）
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.Message != nil && update.Message.LeftChatMember != nil
	}, b.asyncHandler(b.handleLeftChatMember))

	// 群组管理命令 - Admin+
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/welcome", bot.MatchTypeExact,
		b.asyncHandler(b.RequireAdmin(b.handleWelcome)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/setwelcome", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireAdmin(b.handleSetWelcome)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/approve", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireAdmin(b.handleApproveJoinRequest)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/reject", bot.MatchTypePrefix,
		b.asyncHandler(b.RequireAdmin(b.handleRejectJoinRequest)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/members", bot.MatchTypeExact,
		b.asyncHandler(b.RequireAdmin(b.handleMembers)))

	// ========== 阶段 3: 高级特性 Handler 注册 ==========

	// InlineQuery - 内联查询
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.InlineQuery != nil
	}, b.asyncHandler(b.handleInlineQuery))

	// ChosenInlineResult - 内联结果选择
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.ChosenInlineResult != nil
	}, b.asyncHandler(b.handleChosenInlineResult))

	// Poll - 新投票
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.Message != nil && update.Message.Poll != nil
	}, b.asyncHandler(b.handlePoll))

	// PollAnswer - 投票回答
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.PollAnswer != nil
	}, b.asyncHandler(b.handlePollAnswer))

	// MessageReaction - 消息反应
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.MessageReaction != nil
	}, b.asyncHandler(b.handleMessageReaction))

	// MessageReactionCount - 反应统计
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.MessageReactionCount != nil
	}, b.asyncHandler(b.handleMessageReactionCount))

	// EditedChannelPost - 编辑的频道消息
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.EditedChannelPost != nil
	}, b.asyncHandler(b.handleEditedChannelPost))

	logger.L().Info("All handlers registered (including Stage 1, Stage 2, and Stage 3 handlers)")
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
		// 显示用户名或仅显示名字
		userName := admin.FirstName
		if admin.Username != "" {
			userName = admin.FirstName + " (@" + admin.Username + ")"
		}
		text.WriteString(fmt.Sprintf("%d. %s %s - ID: %d\n",
			i+1,
			roleEmoji,
			userName,
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

// ========== 阶段 1: 新增 Handler（核心交互功能） ==========

// handleCallback 处理 CallbackQuery（内联按钮回调）
func (b *Bot) handleCallback(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.CallbackQuery == nil || update.CallbackQuery.From.ID == 0 {
		return
	}

	startTime := time.Now().UTC()
	callbackLog := &models.CallbackLog{
		CallbackQueryID: update.CallbackQuery.ID,
		UserID:          update.CallbackQuery.From.ID,
		Username:        update.CallbackQuery.From.Username,
		Data:            update.CallbackQuery.Data,
		Answered:        false,
	}

	// 必须先应答 Telegram，否则会显示加载状态并重试发送
	_, err := botInstance.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	})
	if err != nil {
		logger.L().Errorf("Failed to answer callback query: %v", err)
		callbackLog.Error = err.Error()
		_ = b.callbackService.LogCallback(ctx, callbackLog)
		return
	}
	callbackLog.Answered = true

	// 解析 callback_data
	callbackData, err := b.callbackService.ParseAndHandle(ctx, update.CallbackQuery.Data)
	if err != nil {
		logger.L().Warnf("Invalid callback data: %s, error: %v", update.CallbackQuery.Data, err)
		callbackLog.Error = err.Error()
		_ = b.callbackService.LogCallback(ctx, callbackLog)
		return
	}

	callbackLog.Action = callbackData.Action
	callbackLog.Params = callbackData.Params

	// 提取 chatID 和 messageID（如果可用）
	// 注意：来自 inline message 的 callback 没有 Message 字段
	if update.CallbackQuery.Message.Message != nil {
		callbackLog.ChatID = update.CallbackQuery.Message.Message.Chat.ID
		callbackLog.MessageID = int64(update.CallbackQuery.Message.Message.ID)
	}

	// 路由到具体的处理函数
	switch callbackData.Action {
	case models.CallbackActionAdminPage:
		b.handleAdminListPagination(ctx, botInstance, update, callbackData)
	case models.CallbackActionConfirmDelete:
		b.handleConfirmDelete(ctx, botInstance, update, callbackData)
	case models.CallbackActionGroupSettings:
		b.handleGroupSettings(ctx, botInstance, update, callbackData)
	default:
		logger.L().Warnf("Unhandled callback action: %s", callbackData.Action)
		callbackLog.Error = "unhandled action"
	}

	// 记录处理耗时
	callbackLog.ProcessingTime = time.Since(startTime).Milliseconds()
	_ = b.callbackService.LogCallback(ctx, callbackLog)
}

// handleAdminListPagination 处理管理员列表翻页
func (b *Bot) handleAdminListPagination(ctx context.Context, botInstance *bot.Bot, update *botModels.Update, data *models.CallbackData) {
	// TODO: 实现分页逻辑（暂时只刷新列表）
	if update.CallbackQuery.Message.Message == nil {
		logger.L().Warn("Admin list pagination callback from inline message, skipping")
		return
	}

	admins, err := b.userService.ListAllAdmins(ctx)
	if err != nil {
		logger.L().Errorf("Failed to get admins for pagination: %v", err)
		return
	}

	var text strings.Builder
	text.WriteString("👥 管理员列表:\n\n")
	for i, admin := range admins {
		roleEmoji := "👑"
		if admin.Role == models.RoleAdmin {
			roleEmoji = "⭐"
		}
		// 显示用户名或仅显示名字
		userName := admin.FirstName
		if admin.Username != "" {
			userName = admin.FirstName + " (@" + admin.Username + ")"
		}
		text.WriteString(fmt.Sprintf("%d. %s %s\n", i+1, roleEmoji, userName))
	}

	// 更新消息内容
	_, _ = botInstance.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text:      text.String(),
	})
}

// handleConfirmDelete 处理删除确认对话框
func (b *Bot) handleConfirmDelete(ctx context.Context, botInstance *bot.Bot, update *botModels.Update, data *models.CallbackData) {
	// TODO: 实现删除确认逻辑
	logger.L().Infof("Confirm delete callback: params=%v", data.Params)

	if update.CallbackQuery.Message.Message != nil {
		_, err := botInstance.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      "✅ 操作已确认",
		})
		if err != nil {
			logger.L().Errorf("Failed to edit message for confirm delete: %v", err)
		}
	}
}

// handleGroupSettings 处理群组设置面板
func (b *Bot) handleGroupSettings(ctx context.Context, botInstance *bot.Bot, update *botModels.Update, data *models.CallbackData) {
	// TODO: 实现群组设置面板逻辑
	logger.L().Infof("Group settings callback: params=%v", data.Params)

	if update.CallbackQuery.Message.Message != nil {
		_, err := botInstance.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
			MessageID: update.CallbackQuery.Message.Message.ID,
			Text:      "⚙️ 群组设置面板（开发中）",
		})
		if err != nil {
			logger.L().Errorf("Failed to edit message for group settings: %v", err)
		}
	}
}

// handleEditedMessage 处理消息编辑事件
func (b *Bot) handleEditedMessage(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.EditedMessage == nil || update.EditedMessage.From == nil {
		return
	}

	// 构造消息对象
	message := &models.Message{
		TelegramID:  int64(update.EditedMessage.ID),
		ChatID:      update.EditedMessage.Chat.ID,
		UserID:      update.EditedMessage.From.ID,
		Username:    update.EditedMessage.From.Username,
		MessageType: models.MessageTypeText,
		Text:        update.EditedMessage.Text,
		Caption:     update.EditedMessage.Caption,
	}

	// 记录编辑
	if err := b.messageService.RecordEdit(ctx, message); err != nil {
		logger.L().Errorf("Failed to record edited message: %v", err)
		return
	}

	logger.L().Infof("Message edited: chat_id=%d, message_id=%d, user_id=%d",
		message.ChatID, message.TelegramID, message.UserID)
}

// handleMyChatMember 处理 Bot 在群组中的状态变化
func (b *Bot) handleMyChatMember(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.MyChatMember == nil {
		return
	}

	chatID := update.MyChatMember.Chat.ID
	newMemberType := update.MyChatMember.NewChatMember.Type
	oldMemberType := update.MyChatMember.OldChatMember.Type

	logger.L().Infof("Bot status changed: chat_id=%d, old_status=%s, new_status=%s",
		chatID, oldMemberType, newMemberType)

	// Bot 被添加到群组
	if newMemberType == "member" || newMemberType == "administrator" {
		group := &models.Group{
			TelegramID: chatID,
			Type:       string(update.MyChatMember.Chat.Type),
			Title:      update.MyChatMember.Chat.Title,
			Username:   update.MyChatMember.Chat.Username,
			BotStatus:  models.BotStatusActive,
			// BotJoinedAt 由 Repository 的 $setOnInsert 自动设置（记录首次加入时间）
			// 注意：如果 Bot 被移除后重新加入，BotJoinedAt 保持首次加入的时间不变
			// 如果需要追踪重新加入，应该在 model 中添加 BotLastJoinedAt 字段
			Settings: models.GroupSettings{
				WelcomeEnabled: true,
				Language:       "zh",
			},
			// Stats 由 Repository 的 $setOnInsert 自动设置默认值
			// 这里不设置 Stats，让 repository 层处理
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := b.groupService.CreateOrUpdateGroup(ctx, group); err != nil {
			logger.L().Errorf("Failed to create/update group: %v", err)
			return
		}

		logger.L().Infof("Bot added to group: chat_id=%d, title=%s", chatID, group.Title)
	}

	// Bot 被踢出或离开群组
	if newMemberType == "kicked" || newMemberType == "left" {
		if err := b.groupService.MarkBotLeft(ctx, chatID); err != nil {
			logger.L().Errorf("Failed to mark bot left: %v", err)
			return
		}

		logger.L().Infof("Bot removed from group: chat_id=%d, status=%s", chatID, newMemberType)
	}
}

// handleMediaMessage 处理媒体消息（图片、视频、文件等）
func (b *Bot) handleMediaMessage(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	msg := update.Message
	message := &models.Message{
		TelegramID: int64(msg.ID),
		ChatID:     msg.Chat.ID,
		UserID:     msg.From.ID,
		Username:   msg.From.Username,
		Caption:    msg.Caption,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	// 判断消息类型并提取文件信息
	switch {
	case msg.Photo != nil && len(msg.Photo) > 0:
		// 取最大尺寸的图片
		photo := msg.Photo[len(msg.Photo)-1]
		message.MessageType = models.MessageTypePhoto
		message.FileID = photo.FileID
		message.FileUniqueID = photo.FileUniqueID
		message.FileSize = int64(photo.FileSize)
		message.Width = photo.Width
		message.Height = photo.Height

	case msg.Video != nil:
		message.MessageType = models.MessageTypeVideo
		message.FileID = msg.Video.FileID
		message.FileUniqueID = msg.Video.FileUniqueID
		message.FileSize = int64(msg.Video.FileSize)
		message.Width = msg.Video.Width
		message.Height = msg.Video.Height
		message.Duration = msg.Video.Duration
		message.MimeType = msg.Video.MimeType

	case msg.Document != nil:
		message.MessageType = models.MessageTypeDocument
		message.FileID = msg.Document.FileID
		message.FileUniqueID = msg.Document.FileUniqueID
		message.FileSize = int64(msg.Document.FileSize)
		message.FileName = msg.Document.FileName
		message.MimeType = msg.Document.MimeType

	case msg.Voice != nil:
		message.MessageType = models.MessageTypeVoice
		message.FileID = msg.Voice.FileID
		message.FileUniqueID = msg.Voice.FileUniqueID
		message.FileSize = int64(msg.Voice.FileSize)
		message.Duration = msg.Voice.Duration
		message.MimeType = msg.Voice.MimeType

	case msg.Audio != nil:
		message.MessageType = models.MessageTypeAudio
		message.FileID = msg.Audio.FileID
		message.FileUniqueID = msg.Audio.FileUniqueID
		message.FileSize = int64(msg.Audio.FileSize)
		message.Duration = msg.Audio.Duration
		message.MimeType = msg.Audio.MimeType
		message.FileName = msg.Audio.FileName

	case msg.Sticker != nil:
		message.MessageType = models.MessageTypeSticker
		message.FileID = msg.Sticker.FileID
		message.FileUniqueID = msg.Sticker.FileUniqueID
		message.FileSize = int64(msg.Sticker.FileSize)
		message.Width = msg.Sticker.Width
		message.Height = msg.Sticker.Height

	case msg.Animation != nil:
		message.MessageType = models.MessageTypeAnimation
		message.FileID = msg.Animation.FileID
		message.FileUniqueID = msg.Animation.FileUniqueID
		message.FileSize = int64(msg.Animation.FileSize)
		message.Width = msg.Animation.Width
		message.Height = msg.Animation.Height
		message.Duration = msg.Animation.Duration
		message.MimeType = msg.Animation.MimeType

	default:
		// 不是媒体消息
		return
	}

	// 记录媒体消息
	if err := b.messageService.HandleMediaMessage(ctx, message); err != nil {
		logger.L().Errorf("Failed to handle media message: %v", err)
		return
	}

	logger.L().Infof("Media message handled: type=%s, chat_id=%d, user_id=%d, file_size=%d",
		message.MessageType, message.ChatID, message.UserID, message.FileSize)
}

// handleChannelPost 处理频道消息
func (b *Bot) handleChannelPost(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.ChannelPost == nil {
		return
	}

	post := update.ChannelPost
	message := &models.Message{
		TelegramID:    int64(post.ID),
		ChatID:        post.Chat.ID,
		MessageType:   models.MessageTypeText,
		Text:          post.Text,
		Caption:       post.Caption,
		IsChannelPost: true,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	// 如果是媒体消息，提取文件信息
	if post.Photo != nil && len(post.Photo) > 0 {
		photo := post.Photo[len(post.Photo)-1]
		message.MessageType = models.MessageTypePhoto
		message.FileID = photo.FileID
		message.FileUniqueID = photo.FileUniqueID
	} else if post.Video != nil {
		message.MessageType = models.MessageTypeVideo
		message.FileID = post.Video.FileID
		message.FileUniqueID = post.Video.FileUniqueID
	}

	// 记录频道消息
	if err := b.messageService.RecordMessage(ctx, message); err != nil {
		logger.L().Errorf("Failed to record channel post: %v", err)
		return
	}

	logger.L().Infof("Channel post recorded: chat_id=%d, message_id=%d, type=%s",
		message.ChatID, message.TelegramID, message.MessageType)
}
