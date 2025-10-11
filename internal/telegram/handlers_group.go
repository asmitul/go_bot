package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// ========== 阶段 2: 群组管理 Handler ==========

// handleChatMember 处理成员状态变化事件
func (b *Bot) handleChatMember(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.ChatMember == nil {
		return
	}

	chatMember := update.ChatMember
	newMemberType := chatMember.NewChatMember.Type
	oldMemberType := chatMember.OldChatMember.Type

	// 判断事件类型
	var eventType string
	switch {
	case oldMemberType == "left" && (newMemberType == "member" || newMemberType == "administrator"):
		eventType = models.MemberEventJoined
	case (oldMemberType == "member" || oldMemberType == "administrator") && newMemberType == "left":
		eventType = models.MemberEventLeft
	case (oldMemberType == "member" || oldMemberType == "administrator") && newMemberType == "kicked":
		eventType = models.MemberEventBanned
	case oldMemberType == "member" && newMemberType == "administrator":
		eventType = models.MemberEventPromoted
	case oldMemberType == "administrator" && newMemberType == "member":
		eventType = models.MemberEventDemoted
	default:
		eventType = "status_changed"
	}

	// 构造事件记录
	event := &models.ChatMemberEvent{
		ChatID:    chatMember.Chat.ID,
		ChatTitle: chatMember.Chat.Title,
		UserID:    chatMember.NewChatMember.Member.User.ID,
		Username:  chatMember.NewChatMember.Member.User.Username,
		FirstName: chatMember.NewChatMember.Member.User.FirstName,
		LastName:  chatMember.NewChatMember.Member.User.LastName,
		EventType: eventType,
		OldStatus: string(oldMemberType),
		NewStatus: string(newMemberType),
		CreatedAt: time.Now(),
	}

	// 如果有操作者信息
	if chatMember.From.ID != 0 {
		event.ChangedBy = chatMember.From.ID
		event.ChangedByUsername = chatMember.From.Username
	}

	// 记录事件
	if err := b.memberService.HandleMemberChange(ctx, event); err != nil {
		logger.L().Errorf("Failed to handle member change: %v", err)
		return
	}

	// 如果是新成员加入，发送欢迎消息
	if eventType == models.MemberEventJoined {
		shouldSend, welcomeText, err := b.memberService.SendWelcomeMessage(ctx, event.ChatID, event.UserID)
		if err != nil {
			logger.L().Errorf("Failed to check welcome message: %v", err)
			return
		}

		if shouldSend {
			// 构造欢迎消息（可以使用用户名）
			message := fmt.Sprintf("%s @%s", welcomeText, event.Username)
			b.sendMessage(ctx, event.ChatID, message)
		}
	}

	logger.L().Infof("Chat member event handled: chat_id=%d, user_id=%d, event=%s",
		event.ChatID, event.UserID, eventType)
}

// handleChatJoinRequest 处理入群申请
func (b *Bot) handleChatJoinRequest(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.ChatJoinRequest == nil {
		return
	}

	joinReq := update.ChatJoinRequest

	// 构造入群请求记录
	request := &models.JoinRequest{
		ChatID:    joinReq.Chat.ID,
		ChatTitle: joinReq.Chat.Title,
		UserID:    joinReq.From.ID,
		Username:  joinReq.From.Username,
		FirstName: joinReq.From.FirstName,
		LastName:  joinReq.From.LastName,
		Bio:       joinReq.Bio,
		Status:    models.JoinRequestStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if joinReq.InviteLink != nil {
		request.InviteLink = joinReq.InviteLink.InviteLink
	}

	// 记录入群请求
	if err := b.memberService.HandleJoinRequest(ctx, request); err != nil {
		logger.L().Errorf("Failed to handle join request: %v", err)
		return
	}

	// TODO: 通知管理员有新的入群申请（可以发送带审批按钮的消息）
	logger.L().Infof("Join request received: chat_id=%d, user_id=%d, username=%s",
		request.ChatID, request.UserID, request.Username)
}

// handleNewChatMembers 处理新成员加入消息事件（系统消息）
func (b *Bot) handleNewChatMembers(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.NewChatMembers == nil {
		return
	}

	chatID := update.Message.Chat.ID
	chatTitle := update.Message.Chat.Title

	for _, newMember := range update.Message.NewChatMembers {
		// 记录成员加入事件
		event := &models.ChatMemberEvent{
			ChatID:    chatID,
			ChatTitle: chatTitle,
			UserID:    newMember.ID,
			Username:  newMember.Username,
			FirstName: newMember.FirstName,
			LastName:  newMember.LastName,
			EventType: models.MemberEventJoined,
			OldStatus: models.MemberStatusLeft,
			NewStatus: models.MemberStatusMember,
			CreatedAt: time.Now(),
		}

		if err := b.memberService.HandleMemberChange(ctx, event); err != nil {
			logger.L().Errorf("Failed to handle new member: %v", err)
			continue
		}

		// 检查是否发送欢迎消息
		shouldSend, welcomeText, err := b.memberService.SendWelcomeMessage(ctx, chatID, newMember.ID)
		if err != nil {
			logger.L().Errorf("Failed to check welcome message: %v", err)
			continue
		}

		if shouldSend {
			message := fmt.Sprintf("%s @%s", welcomeText, newMember.Username)
			b.sendMessage(ctx, chatID, message)
		}
	}

	logger.L().Infof("New members handled: chat_id=%d, count=%d", chatID, len(update.Message.NewChatMembers))
}

// handleLeftChatMember 处理成员离开消息事件（系统消息）
func (b *Bot) handleLeftChatMember(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.LeftChatMember == nil {
		return
	}

	leftMember := update.Message.LeftChatMember
	chatID := update.Message.Chat.ID
	chatTitle := update.Message.Chat.Title

	// 记录成员离开事件
	event := &models.ChatMemberEvent{
		ChatID:    chatID,
		ChatTitle: chatTitle,
		UserID:    leftMember.ID,
		Username:  leftMember.Username,
		FirstName: leftMember.FirstName,
		LastName:  leftMember.LastName,
		EventType: models.MemberEventLeft,
		OldStatus: models.MemberStatusMember,
		NewStatus: models.MemberStatusLeft,
		CreatedAt: time.Now(),
	}

	if err := b.memberService.HandleMemberChange(ctx, event); err != nil {
		logger.L().Errorf("Failed to handle left member: %v", err)
		return
	}

	logger.L().Infof("Member left: chat_id=%d, user_id=%d, username=%s",
		chatID, leftMember.ID, leftMember.Username)
}

// ========== 群组管理命令 Handler ==========

// handleWelcome 查看欢迎消息设置
func (b *Bot) handleWelcome(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// 获取群组设置
	group, err := b.groupService.GetGroupInfo(ctx, chatID)
	if err != nil {
		b.sendErrorMessage(ctx, chatID, "获取群组设置失败")
		return
	}

	var status string
	if group.Settings.WelcomeEnabled {
		status = "✅ 已启用"
	} else {
		status = "❌ 已禁用"
	}

	text := fmt.Sprintf(
		"⚙️ 欢迎消息设置\n\n"+
			"状态: %s\n"+
			"消息内容: %s\n\n"+
			"使用 /setwelcome <消息> 修改欢迎消息",
		status,
		group.Settings.WelcomeText,
	)

	b.sendMessage(ctx, chatID, text)
}

// handleSetWelcome 设置欢迎消息
func (b *Bot) handleSetWelcome(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// 解析命令参数
	parts := strings.SplitN(update.Message.Text, " ", 2)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, chatID,
			"用法: /setwelcome <欢迎消息>\n例如: /setwelcome 欢迎新成员加入！")
		return
	}

	welcomeText := strings.TrimSpace(parts[1])
	if welcomeText == "" {
		b.sendErrorMessage(ctx, chatID, "欢迎消息不能为空")
		return
	}

	// 更新设置
	if err := b.memberService.UpdateWelcomeSettings(ctx, chatID, true, welcomeText); err != nil {
		b.sendErrorMessage(ctx, chatID, err.Error())
		return
	}

	b.sendSuccessMessage(ctx, chatID,
		fmt.Sprintf("已更新欢迎消息:\n%s", welcomeText))
}

// handleApproveJoinRequest 批准入群申请
func (b *Bot) handleApproveJoinRequest(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	reviewerID := update.Message.From.ID
	reviewerUsername := update.Message.From.Username

	// 解析命令参数
	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, chatID,
			"用法: /approve <user_id>\n例如: /approve 123456789")
		return
	}

	var userID int64
	_, err := fmt.Sscanf(parts[1], "%d", &userID)
	if err != nil {
		b.sendErrorMessage(ctx, chatID, "无效的用户 ID")
		return
	}

	// 批准请求
	if err := b.memberService.ApproveJoinRequest(ctx, chatID, userID, reviewerID, reviewerUsername); err != nil {
		b.sendErrorMessage(ctx, chatID, err.Error())
		return
	}

	// 实际批准入群（调用 Telegram API）
	_, err = botInstance.ApproveChatJoinRequest(ctx, &bot.ApproveChatJoinRequestParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		logger.L().Errorf("Failed to approve join request via API: %v", err)
		b.sendErrorMessage(ctx, chatID, "批准失败，请稍后重试")
		return
	}

	b.sendSuccessMessage(ctx, chatID,
		fmt.Sprintf("已批准用户 %d 的入群申请", userID))
}

// handleRejectJoinRequest 拒绝入群申请
func (b *Bot) handleRejectJoinRequest(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	chatID := update.Message.Chat.ID
	reviewerID := update.Message.From.ID
	reviewerUsername := update.Message.From.Username

	// 解析命令参数
	parts := strings.SplitN(update.Message.Text, " ", 3)
	if len(parts) < 2 {
		b.sendErrorMessage(ctx, chatID,
			"用法: /reject <user_id> [原因]\n例如: /reject 123456789 不符合群规")
		return
	}

	var userID int64
	_, err := fmt.Sscanf(parts[1], "%d", &userID)
	if err != nil {
		b.sendErrorMessage(ctx, chatID, "无效的用户 ID")
		return
	}

	reason := ""
	if len(parts) == 3 {
		reason = strings.TrimSpace(parts[2])
	}

	// 拒绝请求
	if err := b.memberService.RejectJoinRequest(ctx, chatID, userID, reviewerID, reviewerUsername, reason); err != nil {
		b.sendErrorMessage(ctx, chatID, err.Error())
		return
	}

	// 实际拒绝入群（调用 Telegram API）
	_, err = botInstance.DeclineChatJoinRequest(ctx, &bot.DeclineChatJoinRequestParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		logger.L().Errorf("Failed to decline join request via API: %v", err)
		b.sendErrorMessage(ctx, chatID, "拒绝失败，请稍后重试")
		return
	}

	b.sendSuccessMessage(ctx, chatID,
		fmt.Sprintf("已拒绝用户 %d 的入群申请", userID))
}

// handleMembers 查看成员列表（简化版，显示最近事件）
func (b *Bot) handleMembers(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID

	// 获取最近的成员事件
	events, err := b.memberService.GetChatMemberHistory(ctx, chatID, 10)
	if err != nil {
		b.sendErrorMessage(ctx, chatID, "获取成员信息失败")
		return
	}

	if len(events) == 0 {
		b.sendMessage(ctx, chatID, "📝 暂无成员事件记录")
		return
	}

	var text strings.Builder
	text.WriteString("👥 最近成员事件 (最多10条):\n\n")
	for i, event := range events {
		eventEmoji := "📍"
		switch event.EventType {
		case models.MemberEventJoined:
			eventEmoji = "✅"
		case models.MemberEventLeft:
			eventEmoji = "❌"
		case models.MemberEventPromoted:
			eventEmoji = "⬆️"
		case models.MemberEventBanned:
			eventEmoji = "🚫"
		}

		text.WriteString(fmt.Sprintf("%d. %s %s (@%s) - %s\n   时间: %s\n\n",
			i+1,
			eventEmoji,
			event.FirstName,
			event.Username,
			event.EventType,
			event.CreatedAt.Format("2006-01-02 15:04"),
		))
	}

	b.sendMessage(ctx, chatID, text.String())
}
