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
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/leave", bot.MatchTypeExact,
		b.asyncHandler(b.RequireAdmin(b.handleLeave)))
	b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/configs", bot.MatchTypeExact,
		b.asyncHandler(b.RequireAdmin(b.handleConfigs)))

	// 配置菜单回调查询处理器
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.CallbackQuery != nil && strings.HasPrefix(update.CallbackQuery.Data, "config:")
	}, b.asyncHandler(b.handleConfigCallback))

	// Bot 状态变化事件 (MyChatMember)
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.MyChatMember != nil
	}, b.asyncHandler(b.handleMyChatMember))

	// 消息编辑事件
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.EditedMessage != nil
	}, b.asyncHandler(b.handleEditedMessage))

	// 频道消息
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.ChannelPost != nil
	}, b.asyncHandler(b.handleChannelPost))

	// 编辑的频道消息
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.EditedChannelPost != nil
	}, b.asyncHandler(b.handleEditedChannelPost))

	// 媒体消息处理（照片、视频等）
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		if update.Message == nil {
			return false
		}
		msg := update.Message
		return msg.Photo != nil || msg.Video != nil || msg.Document != nil ||
			msg.Voice != nil || msg.Audio != nil || msg.Sticker != nil || msg.Animation != nil
	}, b.asyncHandler(b.handleMediaMessage))

	// 成员离开
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		return update.Message != nil && update.Message.LeftChatMember != nil
	}, b.asyncHandler(b.handleLeftChatMember))

	// 普通文本消息（放在最后，作为 fallback）
	b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
		if update.Message == nil || update.Message.Text == "" {
			return false
		}
		msg := update.Message
		// 排除命令、系统消息、媒体消息
		return !strings.HasPrefix(msg.Text, "/") &&
			msg.NewChatMembers == nil &&
			msg.LeftChatMember == nil &&
			msg.Photo == nil && msg.Video == nil && msg.Document == nil &&
			msg.Voice == nil && msg.Audio == nil && msg.Sticker == nil && msg.Animation == nil
	}, b.asyncHandler(b.handleTextMessage))

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

// handleLeave 处理 /leave 命令（让 Bot 离开群组）
func (b *Bot) handleLeave(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// 只能在群组中使用
	if update.Message.Chat.Type != "group" && update.Message.Chat.Type != "supergroup" {
		b.sendErrorMessage(ctx, chatID, "此命令只能在群组中使用")
		return
	}

	// 发送离别消息
	b.sendMessage(ctx, chatID, "👋 再见！我将离开这个群组。")

	// 标记 Bot 离开并删除群组记录
	if err := b.groupService.LeaveGroup(ctx, chatID); err != nil {
		logger.L().Errorf("Failed to mark group as left: chat_id=%d, error=%v", chatID, err)
	}

	// 让 Bot 离开群组
	_, err := botInstance.LeaveChat(ctx, &bot.LeaveChatParams{
		ChatID: chatID,
	})
	if err != nil {
		logger.L().Errorf("Failed to leave chat: chat_id=%d, error=%v", chatID, err)
	}
}

// handleMyChatMember 处理 Bot 状态变化（被添加到群组/被踢出群组）
func (b *Bot) handleMyChatMember(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.MyChatMember == nil {
		return
	}

	chatMember := update.MyChatMember
	chat := chatMember.Chat
	oldStatus := chatMember.OldChatMember.Type
	newStatus := chatMember.NewChatMember.Type

	logger.L().Infof("Bot status change: chat_id=%d, old=%s, new=%s", chat.ID, oldStatus, newStatus)

	// Bot 被添加到群组
	if (oldStatus == botModels.ChatMemberTypeLeft || oldStatus == botModels.ChatMemberTypeBanned) &&
		(newStatus == botModels.ChatMemberTypeMember || newStatus == botModels.ChatMemberTypeAdministrator) {
		group := &models.Group{
			TelegramID: chat.ID,
			Type:       string(chat.Type),
			Title:      chat.Title,
			Username:   chat.Username,
			BotStatus:  models.BotStatusActive,
		}

		if err := b.groupService.HandleBotAddedToGroup(ctx, group); err != nil {
			logger.L().Errorf("Failed to handle bot added to group: %v", err)
			return
		}

		// 发送欢迎消息
		welcomeText := fmt.Sprintf(
			"👋 你好！我是 Bot，感谢邀请我加入 %s！\n\n"+
				"使用 /help 查看可用命令。",
			chat.Title,
		)
		b.sendMessage(ctx, chat.ID, welcomeText)
	}

	// Bot 被踢出或离开群组
	if (oldStatus == botModels.ChatMemberTypeMember || oldStatus == botModels.ChatMemberTypeAdministrator) &&
		(newStatus == botModels.ChatMemberTypeLeft || newStatus == botModels.ChatMemberTypeBanned) {
		reason := "left"
		if newStatus == botModels.ChatMemberTypeBanned {
			reason = "kicked"
		}

		if err := b.groupService.HandleBotRemovedFromGroup(ctx, chat.ID, reason); err != nil {
			logger.L().Errorf("Failed to handle bot removed from group: %v", err)
		}
	}
}

// handleTextMessage 处理普通文本消息
func (b *Bot) handleTextMessage(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	msg := update.Message

	// 排除命令消息（以 / 开头）
	if strings.HasPrefix(msg.Text, "/") {
		return
	}

	// 排除系统消息（NewChatMembers、LeftChatMember 等）
	if msg.NewChatMembers != nil || msg.LeftChatMember != nil {
		return
	}

	// 优先检查用户输入状态（用于配置菜单输入）
	if msg.From != nil && b.configMenuService != nil {
		items := b.getConfigItems()
		responseMsg, err := b.configMenuService.ProcessUserInput(ctx, msg.Chat.ID, msg.From.ID, msg.Text, items)

		// 如果有响应消息（无论成功或失败），说明这是配置输入
		if responseMsg != "" {
			if err != nil {
				b.sendErrorMessage(ctx, msg.Chat.ID, responseMsg)
			} else {
				b.sendSuccessMessage(ctx, msg.Chat.ID, responseMsg)
			}
			return // 处理完配置输入，不再记录为普通消息
		}
	}

	// 检查计算器功能（仅群组）
	if msg.Chat.Type == "group" || msg.Chat.Type == "supergroup" {
		// 获取群组配置
		group, err := b.groupService.GetGroupInfo(ctx, msg.Chat.ID)
		if err == nil && group.Settings.CalculatorEnabled {
			// 判断是否为数学表达式
			if IsMathExpression(msg.Text) {
				// 尝试计算
				result, err := Calculate(msg.Text)
				if err != nil {
					// 计算失败，发送错误提示
					logger.L().Warnf("Calculator failed: chat_id=%d, text=%s, error=%v", msg.Chat.ID, msg.Text, err)
					b.sendErrorMessage(ctx, msg.Chat.ID, fmt.Sprintf("计算错误: %v", err))
				} else {
					// 计算成功，发送结果
					logger.L().Infof("Calculator: %s = %g (chat_id=%d)", msg.Text, result, msg.Chat.ID)
					resultText := fmt.Sprintf("🧮 %s = %g", msg.Text, result)
					b.sendMessage(ctx, msg.Chat.ID, resultText)
				}
				return // 已处理，不再记录为普通消息
			}
		}
	}

	// 构造消息信息
	replyToID := int64(0)
	if msg.ReplyToMessage != nil {
		replyToID = int64(msg.ReplyToMessage.ID)
	}

	textMsg := &service.TextMessageInfo{
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		UserID:            msg.From.ID,
		Text:              msg.Text,
		ReplyToMessageID:  replyToID,
		SentAt:            time.Unix(int64(msg.Date), 0),
	}

	// 记录消息
	if err := b.messageService.HandleTextMessage(ctx, textMsg); err != nil {
		logger.L().Errorf("Failed to handle text message: %v", err)
	}
}

// handleMediaMessage 处理媒体消息
func (b *Bot) handleMediaMessage(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	var messageType, fileID, mimeType string
	var fileSize int64

	// 判断媒体类型并提取信息
	if msg.Photo != nil && len(msg.Photo) > 0 {
		messageType = models.MessageTypePhoto
		photo := msg.Photo[len(msg.Photo)-1] // 取最大尺寸
		fileID = photo.FileID
		fileSize = int64(photo.FileSize)
	} else if msg.Video != nil {
		messageType = models.MessageTypeVideo
		fileID = msg.Video.FileID
		fileSize = int64(msg.Video.FileSize)
		mimeType = msg.Video.MimeType
	} else if msg.Document != nil {
		messageType = models.MessageTypeDocument
		fileID = msg.Document.FileID
		fileSize = int64(msg.Document.FileSize)
		mimeType = msg.Document.MimeType
	} else if msg.Voice != nil {
		messageType = models.MessageTypeVoice
		fileID = msg.Voice.FileID
		fileSize = int64(msg.Voice.FileSize)
		mimeType = msg.Voice.MimeType
	} else if msg.Audio != nil {
		messageType = models.MessageTypeAudio
		fileID = msg.Audio.FileID
		fileSize = int64(msg.Audio.FileSize)
		mimeType = msg.Audio.MimeType
	} else if msg.Sticker != nil {
		messageType = models.MessageTypeSticker
		fileID = msg.Sticker.FileID
		fileSize = int64(msg.Sticker.FileSize)
	} else if msg.Animation != nil {
		messageType = models.MessageTypeAnimation
		fileID = msg.Animation.FileID
		fileSize = int64(msg.Animation.FileSize)
		mimeType = msg.Animation.MimeType
	} else {
		return // 不是支持的媒体类型
	}

	// 构造媒体消息信息
	mediaMsg := &service.MediaMessageInfo{
		TelegramMessageID: int64(msg.ID),
		ChatID:            msg.Chat.ID,
		UserID:            msg.From.ID,
		MessageType:       messageType,
		Caption:           msg.Caption,
		MediaFileID:       fileID,
		MediaFileSize:     fileSize,
		MediaMimeType:     mimeType,
		SentAt:            time.Unix(int64(msg.Date), 0),
	}

	// 记录消息
	if err := b.messageService.HandleMediaMessage(ctx, mediaMsg); err != nil {
		logger.L().Errorf("Failed to handle media message: %v", err)
	}
}

// handleEditedMessage 处理消息编辑事件
func (b *Bot) handleEditedMessage(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.EditedMessage == nil || update.EditedMessage.Text == "" {
		return
	}

	msg := update.EditedMessage
	editedAt := time.Unix(int64(msg.EditDate), 0)

	// 更新消息编辑信息
	if err := b.messageService.HandleEditedMessage(ctx, int64(msg.ID), msg.Chat.ID, msg.Text, editedAt); err != nil {
		logger.L().Errorf("Failed to handle edited message: %v", err)
	}
}

// handleChannelPost 处理频道消息
func (b *Bot) handleChannelPost(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.ChannelPost == nil {
		return
	}

	post := update.ChannelPost
	messageType := models.MessageTypeChannelPost
	text := post.Text
	fileID := ""

	// 如果是媒体消息，提取 file_id
	if post.Photo != nil && len(post.Photo) > 0 {
		fileID = post.Photo[len(post.Photo)-1].FileID
	} else if post.Video != nil {
		fileID = post.Video.FileID
	} else if post.Document != nil {
		fileID = post.Document.FileID
	}

	channelPost := &service.ChannelPostInfo{
		TelegramMessageID: int64(post.ID),
		ChatID:            post.Chat.ID,
		MessageType:       messageType,
		Text:              text,
		MediaFileID:       fileID,
		SentAt:            time.Unix(int64(post.Date), 0),
	}

	// 记录频道消息
	if err := b.messageService.RecordChannelPost(ctx, channelPost); err != nil {
		logger.L().Errorf("Failed to handle channel post: %v", err)
	}
}

// handleEditedChannelPost 处理编辑的频道消息
func (b *Bot) handleEditedChannelPost(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.EditedChannelPost == nil || update.EditedChannelPost.Text == "" {
		return
	}

	post := update.EditedChannelPost
	editedAt := time.Unix(int64(post.EditDate), 0)

	// 更新频道消息编辑信息
	if err := b.messageService.HandleEditedMessage(ctx, int64(post.ID), post.Chat.ID, post.Text, editedAt); err != nil {
		logger.L().Errorf("Failed to handle edited channel post: %v", err)
	}
}

// handleLeftChatMember 处理成员离开系统消息
func (b *Bot) handleLeftChatMember(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.LeftChatMember == nil {
		return
	}

	msg := update.Message
	leftMember := msg.LeftChatMember

	// 记录日志
	logger.L().Infof("Member left: chat_id=%d, user_id=%d, username=%s",
		msg.Chat.ID, leftMember.ID, leftMember.Username)

	// 这里可以添加更多逻辑，例如：
	// - 发送离别消息
	// - 更新成员统计
	// - 记录离开事件到数据库
}
