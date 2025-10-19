package forward

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/logger"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

// HandleRecallCallback 处理撤回按钮点击（显示二次确认）
func (s *Service) HandleRecallCallback(ctx context.Context, botInstance *bot.Bot, query *botModels.CallbackQuery) {
	taskID := strings.TrimPrefix(query.Data, "recall:")

	// 显示二次确认按钮
	keyboard := &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{
				{Text: "✅ 确认撤回", CallbackData: fmt.Sprintf("recall_confirm:%s", taskID)},
				{Text: "❌ 取消", CallbackData: "recall_cancel"},
			},
		},
	}

	// 回复确认提示（显示在顶部警告框）
	_, err := botInstance.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            "⚠️ 确认撤回所有已转发的消息？",
		ShowAlert:       true,
	})
	if err != nil {
		logger.L().Errorf("Failed to answer callback query: %v", err)
	}

	// 更新消息按钮为确认界面
	if query.Message.Message != nil {
		_, err = botInstance.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      query.Message.Message.Chat.ID,
			MessageID:   query.Message.Message.ID,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			logger.L().Errorf("Failed to edit message markup: %v", err)
		}
	}

	logger.L().Infof("User %d requested recall confirmation for task %s", query.From.ID, taskID)
}

// HandleRecallConfirmCallback 处理确认撤回
func (s *Service) HandleRecallConfirmCallback(ctx context.Context, botInstance *bot.Bot, query *botModels.CallbackQuery) {
	taskID := strings.TrimPrefix(query.Data, "recall_confirm:")

	logger.L().Infof("User %d confirmed recall for task %s", query.From.ID, taskID)

	// 执行撤回
	successCount, failedCount, err := s.RecallForwardedMessages(ctx, botInstance, taskID, query.From.ID)

	var resultText string
	if err != nil {
		resultText = fmt.Sprintf("❌ 撤回失败: %v", err)
		logger.L().Errorf("Recall failed for task %s: %v", taskID, err)
	} else {
		resultText = fmt.Sprintf("✅ 撤回完成\n\n成功: %d 条\n失败: %d 条", successCount, failedCount)
		logger.L().Infof("Recall completed for task %s: success=%d, failed=%d", taskID, successCount, failedCount)
	}

	// 回复用户撤回结果
	_, err = botInstance.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            resultText,
		ShowAlert:       true,
	})
	if err != nil {
		logger.L().Errorf("Failed to answer callback query: %v", err)
	}

	// 禁用按钮，显示"已撤回"
	keyboard := &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{{Text: "🗑️ 已撤回", CallbackData: "noop"}},
		},
	}
	if query.Message.Message != nil {
		_, err = botInstance.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      query.Message.Message.Chat.ID,
			MessageID:   query.Message.Message.ID,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			logger.L().Errorf("Failed to edit message markup: %v", err)
		}
	}
}

// HandleRecallCancelCallback 处理取消撤回
func (s *Service) HandleRecallCancelCallback(ctx context.Context, botInstance *bot.Bot, query *botModels.CallbackQuery) {
	logger.L().Infof("User %d canceled recall", query.From.ID)

	// 回复用户
	_, err := botInstance.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
		Text:            "操作已取消",
	})
	if err != nil {
		logger.L().Errorf("Failed to answer callback query: %v", err)
	}

	// 恢复原始按钮（从消息中解析 taskID）
	// 由于无法轻松获取原始 taskID，这里直接关闭按钮
	keyboard := &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{{Text: "操作已取消", CallbackData: "noop"}},
		},
	}
	if query.Message.Message != nil {
		_, err = botInstance.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
			ChatID:      query.Message.Message.Chat.ID,
			MessageID:   query.Message.Message.ID,
			ReplyMarkup: keyboard,
		})
		if err != nil {
			logger.L().Errorf("Failed to edit message markup: %v", err)
		}
	}
}
