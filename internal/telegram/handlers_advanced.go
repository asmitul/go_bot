package telegram

import (
	"context"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// handleInlineQuery 处理内联查询
func (b *Bot) handleInlineQuery(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	query := update.InlineQuery
	if query == nil {
		return
	}

	// 构建查询日志
	queryLog := &models.InlineQueryLog{
		QueryID:  query.ID,
		UserID:   query.From.ID,
		Username: query.From.Username,
		Query:    query.Query,
		Offset:   query.Offset,
		Location: nil, // 如果有位置信息可以处理
	}

	// 处理查询
	if err := b.inlineService.HandleInlineQuery(ctx, queryLog); err != nil {
		logger.L().WithError(err).Error("处理内联查询失败")
		return
	}

	// 注意：这里应该返回查询结果给 Telegram
	// 实际业务中需要根据 query.Query 生成结果并调用 AnswerInlineQuery
	// 这里仅记录日志
	logger.L().WithFields(map[string]interface{}{
		"query_id": query.ID,
		"query":    query.Query,
	}).Info("内联查询已记录")
}

// handleChosenInlineResult 处理内联结果选择
func (b *Bot) handleChosenInlineResult(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	result := update.ChosenInlineResult
	if result == nil {
		return
	}

	// 构建结果日志
	resultLog := &models.ChosenInlineResultLog{
		ResultID:        result.ResultID,
		UserID:          result.From.ID,
		Username:        result.From.Username,
		Query:           result.Query,
		InlineMessageID: result.InlineMessageID,
	}

	// 处理结果选择
	if err := b.inlineService.HandleChosenResult(ctx, resultLog); err != nil {
		logger.L().WithError(err).Error("处理内联结果选择失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"result_id": result.ResultID,
		"user_id":   result.From.ID,
	}).Info("内联结果选择已记录")
}

// handlePoll 处理新投票
func (b *Bot) handlePoll(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	msg := update.Message
	if msg == nil || msg.Poll == nil {
		return
	}

	poll := msg.Poll

	// 构建投票记录
	pollRecord := &models.PollRecord{
		PollID:                poll.ID,
		ChatID:                msg.Chat.ID,
		MessageID:             int64(msg.ID),
		Question:              poll.Question,
		Type:                  string(poll.Type),
		AllowsMultipleAnswers: poll.AllowsMultipleAnswers,
		IsAnonymous:           poll.IsAnonymous,
		IsClosed:              poll.IsClosed,
		TotalVoterCount:       poll.TotalVoterCount,
	}

	// 转换选项
	pollRecord.Options = make([]models.PollOption, len(poll.Options))
	for i, opt := range poll.Options {
		pollRecord.Options[i] = models.PollOption{
			Text:       opt.Text,
			VoterCount: opt.VoterCount,
		}
	}

	// 如果是测验，记录正确答案（CorrectOptionID 可以是 0，表示第一个选项）
	if string(poll.Type) == "quiz" {
		pollRecord.CorrectOptionID = poll.CorrectOptionID
	}

	// 记录创建者
	if msg.From != nil {
		pollRecord.CreatedBy = msg.From.ID
	}

	// 处理投票创建
	if err := b.pollService.HandlePollCreation(ctx, pollRecord); err != nil {
		logger.L().WithError(err).Error("处理投票创建失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":  poll.ID,
		"question": poll.Question,
		"chat_id":  msg.Chat.ID,
	}).Info("投票创建已记录")
}

// handlePollAnswer 处理投票回答
func (b *Bot) handlePollAnswer(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	answer := update.PollAnswer
	if answer == nil {
		return
	}

	// 如果是匿名投票，可能没有用户信息
	if answer.User == nil {
		logger.L().Warn("收到匿名投票回答，跳过记录")
		return
	}

	// 构建回答记录
	pollAnswer := &models.PollAnswer{
		PollID:    answer.PollID,
		UserID:    answer.User.ID,
		Username:  answer.User.Username,
		OptionIDs: answer.OptionIDs,
	}

	// 处理投票回答
	if err := b.pollService.HandlePollAnswer(ctx, pollAnswer); err != nil {
		logger.L().WithError(err).Error("处理投票回答失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"poll_id":    answer.PollID,
		"user_id":    answer.User.ID,
		"option_ids": answer.OptionIDs,
	}).Info("投票回答已记录")
}

// handleMessageReaction 处理消息反应
func (b *Bot) handleMessageReaction(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	reaction := update.MessageReaction
	if reaction == nil {
		return
	}

	// 检查是否为匿名反应
	if reaction.User == nil {
		logger.L().Debug("Received anonymous message reaction, skipping")
		return
	}

	// 构建反应记录
	reactionRecord := &models.MessageReactionRecord{
		ChatID:    reaction.Chat.ID,
		MessageID: int64(reaction.MessageID),
		UserID:    reaction.User.ID,
		Username:  reaction.User.Username,
	}

	// 转换新反应列表
	reactionRecord.Reactions = make([]models.Reaction, len(reaction.NewReaction))
	for i, r := range reaction.NewReaction {
		reactionRecord.Reactions[i] = models.Reaction{
			Type: string(r.Type),
		}
		// 如果是 emoji 类型
		if r.ReactionTypeEmoji != nil {
			reactionRecord.Reactions[i].Emoji = r.ReactionTypeEmoji.Emoji
		}
	}

	// 处理消息反应
	if err := b.reactionService.HandleReaction(ctx, reactionRecord); err != nil {
		logger.L().WithError(err).Error("处理消息反应失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":    reaction.Chat.ID,
		"message_id": reaction.MessageID,
		"user_id":    reaction.User.ID,
		"reactions":  len(reaction.NewReaction),
	}).Info("消息反应已记录")
}

// handleMessageReactionCount 处理反应统计
func (b *Bot) handleMessageReactionCount(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	reactionCount := update.MessageReactionCount
	if reactionCount == nil {
		return
	}

	// 构建统计记录
	countRecord := &models.MessageReactionCountRecord{
		ChatID:    reactionCount.Chat.ID,
		MessageID: int64(reactionCount.MessageID),
	}

	// 转换反应统计
	countRecord.ReactionCounts = make([]models.ReactionCount, len(reactionCount.Reactions))
	totalCount := 0
	for i, rc := range reactionCount.Reactions {
		countRecord.ReactionCounts[i] = models.ReactionCount{
			Reaction: models.Reaction{
				Type: string(rc.Type.Type),
			},
			Count: rc.TotalCount,
		}
		// 如果是 emoji 类型
		if rc.Type.ReactionTypeEmoji != nil {
			countRecord.ReactionCounts[i].Reaction.Emoji = rc.Type.ReactionTypeEmoji.Emoji
		}
		totalCount += rc.TotalCount
	}
	countRecord.TotalCount = totalCount

	// 处理反应统计
	if err := b.reactionService.HandleReactionCount(ctx, countRecord); err != nil {
		logger.L().WithError(err).Error("处理反应统计失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":     reactionCount.Chat.ID,
		"message_id":  reactionCount.MessageID,
		"total_count": totalCount,
	}).Info("反应统计已更新")
}

// handleEditedChannelPost 处理编辑的频道消息
func (b *Bot) handleEditedChannelPost(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	msg := update.EditedChannelPost
	if msg == nil {
		return
	}

	// 构建消息记录
	message := &models.Message{
		TelegramID: int64(msg.ID),
		ChatID:     msg.Chat.ID,
		Text:       msg.Text,
		IsEdited:   true,
	}

	// 如果有发送者信息
	if msg.From != nil {
		message.UserID = msg.From.ID
		message.Username = msg.From.Username
	}

	// 设置编辑时间
	var editTime time.Time
	if msg.EditDate != 0 {
		editTime = time.Unix(int64(msg.EditDate), 0)
	} else {
		editTime = time.Now().UTC()
	}
	message.EditedAt = &editTime

	// 记录消息编辑
	if err := b.messageService.RecordEdit(ctx, message); err != nil {
		logger.L().WithError(err).Error("记录频道消息编辑失败")
		return
	}

	logger.L().WithFields(map[string]interface{}{
		"chat_id":    msg.Chat.ID,
		"message_id": msg.ID,
	}).Info("频道消息编辑已记录")
}
