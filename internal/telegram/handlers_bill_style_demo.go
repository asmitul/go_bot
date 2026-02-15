package telegram

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/logger"

	"github.com/go-telegram/bot"
	botModels "github.com/go-telegram/bot/models"
)

const (
	billStyleDemoCommandSlash    = "/bill_style_demo"
	billStyleDemoCommandCN       = "账单样式示例"
	billStyleDemoCommandCNSimple = "账单样式"
	billStyleDemoCallbackPrefix  = "billdemo:"
)

type billStyleDemoExample struct {
	text            string
	parseMode       botModels.ParseMode
	replyMarkup     botModels.ReplyMarkup
	messageEffectID string
}

func (b *Bot) handleBillStyleDemo(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	if update.Message == nil {
		return
	}

	chatID := update.Message.Chat.ID
	replyTo := update.Message.ID

	b.sendMessage(ctx, chatID,
		"🧪 账单样式预览（临时命令）\n下面每条都是独立示例，方便你在线逐条对比效果。", replyTo)

	examples := buildBillStyleDemoExamples()
	successCount := 0
	failedCount := 0

	for idx, example := range examples {
		if err := sendBillStyleDemoExample(ctx, botInstance, chatID, replyTo, example); err != nil {
			failedCount++
			logger.L().Errorf("Bill style demo send failed: chat_id=%d, index=%d, err=%v", chatID, idx, err)
			continue
		}
		successCount++
	}

	result := fmt.Sprintf("✅ 账单样式预览发送完成：成功 %d 条，失败 %d 条。", successCount, failedCount)
	if failedCount > 0 {
		result += "\n如果有失败，通常是客户端/Bot API 对某些新样式支持不一致。"
	}
	b.sendMessage(ctx, chatID, result, replyTo)
}

func (b *Bot) handleBillStyleDemoCallback(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
	query := update.CallbackQuery
	if query == nil {
		return
	}

	data := strings.TrimPrefix(query.Data, billStyleDemoCallbackPrefix)
	if data == "" {
		b.answerCallback(ctx, botInstance, query.ID, "无效示例按钮", true)
		return
	}

	switch data {
	case "page:prev":
		b.answerCallback(ctx, botInstance, query.ID, "演示：切到上一页", false)
	case "page:today":
		b.answerCallback(ctx, botInstance, query.ID, "演示：切到今日", false)
	case "page:next":
		b.answerCallback(ctx, botInstance, query.ID, "演示：切到下一页", false)
	case "action:export":
		b.answerCallback(ctx, botInstance, query.ID, "演示：这里可接 CSV/PDF 导出", false)
	case "view:compact":
		b.updateBillStyleToggleMessage(ctx, query, true)
		b.answerCallback(ctx, botInstance, query.ID, "已切换为简洁视图", false)
	case "view:detail":
		b.updateBillStyleToggleMessage(ctx, query, false)
		b.answerCallback(ctx, botInstance, query.ID, "已切换为详细视图", false)
	default:
		b.answerCallback(ctx, botInstance, query.ID, "未知示例按钮", true)
	}
}

func (b *Bot) updateBillStyleToggleMessage(ctx context.Context, query *botModels.CallbackQuery, compact bool) {
	if query == nil || query.Message.Message == nil {
		return
	}

	msg := query.Message.Message
	b.editMessage(ctx, msg.Chat.ID, msg.ID, buildBillStyleToggleText(compact), buildBillStyleToggleKeyboard())
}

func sendBillStyleDemoExample(ctx context.Context, botInstance *bot.Bot, chatID int64, replyTo int, example billStyleDemoExample) error {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   example.text,
	}

	if example.parseMode != "" {
		params.ParseMode = example.parseMode
	}

	if example.replyMarkup != nil {
		params.ReplyMarkup = example.replyMarkup
	}

	if example.messageEffectID != "" {
		params.MessageEffectID = example.messageEffectID
	}

	if replyTo > 0 {
		params.ReplyParameters = &botModels.ReplyParameters{MessageID: replyTo}
	}

	_, err := botInstance.SendMessage(ctx, params)
	return err
}

func buildBillStyleDemoExamples() []billStyleDemoExample {
	return []billStyleDemoExample{
		{
			text: "[示例 01] 原样文本\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"16:21:29      694.00\n" +
				"16:20:49      694.00",
		},
		{
			text: "[示例 02] HTML 强调\n" +
				"💸 <b>提款明细</b>（总计 <b>1388</b>｜<b>2</b> 笔）\n" +
				"16:21:29      <b>694.00</b>\n" +
				"16:20:49      <b>694.00</b>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 03] HTML code\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"<code>16:21:29</code>      <code>694.00</code>\n" +
				"<code>16:20:49</code>      <code>694.00</code>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 04] HTML pre 等宽对齐\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"<pre>时间       金额\n16:21:29   694.00\n16:20:49   694.00</pre>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 05] HTML blockquote\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"<blockquote>16:21:29      694.00\n16:20:49      694.00</blockquote>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 06] 可展开 blockquote\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"<blockquote expandable>16:21:29      694.00\n16:20:49      694.00\n（点击展开或收起）</blockquote>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 07] spoiler 金额折叠\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"16:21:29      <tg-spoiler>694.00</tg-spoiler>\n" +
				"16:20:49      <tg-spoiler>694.00</tg-spoiler>",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "*示例 08 MarkdownV2*\n" +
				"💸 *提款明细* 总计 *1388* 共 *2* 笔\n" +
				"```text\n16:21:29   694.00\n16:20:49   694.00\n```",
			parseMode: botModels.ParseModeMarkdown,
		},
		{
			text: "[示例 09-A] 拆分发送：摘要\n" +
				"📑 账单 - 2026-02-15\n" +
				"跑量：0\n成交：0\n笔数：0",
		},
		{
			text: "[示例 09-B] 拆分发送：明细\n" +
				"💸 提款明细（总计 1388｜2 笔）\n" +
				"<pre>16:21:29   694.00\n16:20:49   694.00</pre>\n" +
				"余额：-17945.20",
			parseMode: botModels.ParseModeHTML,
		},
		{
			text: "[示例 10] InlineKeyboard 分页按钮（演示）\n" +
				"💸 提款明细（总计 1388｜2 笔）",
			replyMarkup: buildBillStylePaginationKeyboard(),
		},
		{
			text:        "[示例 11] copy_text 一键复制\n点击下方按钮复制首行或全部明细。",
			replyMarkup: buildBillStyleCopyKeyboard(),
		},
		{
			text:        buildBillStyleToggleText(false),
			parseMode:   botModels.ParseModeHTML,
			replyMarkup: buildBillStyleToggleKeyboard(),
		},
		{
			text: "[示例 13] ReplyKeyboard 快捷筛选\n" +
				"点击输入框可看到“今日账单/昨日账单/提款明细”等快捷按钮。",
			replyMarkup: &botModels.ReplyKeyboardMarkup{
				Keyboard: [][]botModels.KeyboardButton{
					{{Text: "今日账单"}, {Text: "昨日账单"}, {Text: "提款明细"}},
					{{Text: "通道账单"}, {Text: "导出账单"}},
				},
				ResizeKeyboard:        true,
				OneTimeKeyboard:       true,
				InputFieldPlaceholder: "选择一个筛选动作",
			},
		},
		{
			text:        "[示例 13-B] 清理 ReplyKeyboard（恢复默认输入）",
			replyMarkup: &botModels.ReplyKeyboardRemove{RemoveKeyboard: true},
		},
		{
			text: "[示例 14] 图片账单卡片（sendPhoto）\n" +
				"说明：需要先把账单渲染成图片，再通过 sendPhoto 发送。",
		},
		{
			text: "[示例 15] 文档导出（sendDocument）\n" +
				"说明：可导出 CSV/PDF，对账和留档更方便。",
		},
		{
			text: "[示例 16] Mini App（WebApp）\n" +
				"说明：可做完整表格筛选、分页、图表，这个需要单独前端页面。",
		},
		{
			text: "[示例 17] 自定义 emoji / 按钮颜色 / 消息动效\n" +
				"说明：这些能力需要额外资源或特定客户端支持，当前只做说明示例。",
		},
	}
}

func buildBillStylePaginationKeyboard() *botModels.InlineKeyboardMarkup {
	return &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{
				{Text: "⬅️ 上一页", CallbackData: billStyleDemoCallbackPrefix + "page:prev"},
				{Text: "📅 今日", CallbackData: billStyleDemoCallbackPrefix + "page:today"},
				{Text: "➡️ 下一页", CallbackData: billStyleDemoCallbackPrefix + "page:next"},
			},
			{
				{Text: "📤 导出", CallbackData: billStyleDemoCallbackPrefix + "action:export"},
			},
		},
	}
}

func buildBillStyleCopyKeyboard() *botModels.InlineKeyboardMarkup {
	return &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{
				{
					Text:     "📋 复制首行",
					CopyText: botModels.CopyTextButton{Text: "16:21:29      694.00"},
				},
				{
					Text:     "📋 复制全部",
					CopyText: botModels.CopyTextButton{Text: "16:21:29      694.00\n16:20:49      694.00"},
				},
			},
		},
	}
}

func buildBillStyleToggleKeyboard() *botModels.InlineKeyboardMarkup {
	return &botModels.InlineKeyboardMarkup{
		InlineKeyboard: [][]botModels.InlineKeyboardButton{
			{
				{Text: "简洁视图", CallbackData: billStyleDemoCallbackPrefix + "view:compact"},
				{Text: "详细视图", CallbackData: billStyleDemoCallbackPrefix + "view:detail"},
			},
		},
	}
}

func buildBillStyleToggleText(compact bool) string {
	if compact {
		return "[示例 12] editMessage 切换视图（简洁）\n" +
			"💸 提款明细（2 笔）\n" +
			"16:21:29  694.00\n" +
			"16:20:49  694.00"
	}

	return "[示例 12] editMessage 切换视图（详细）\n" +
		"💸 提款明细（总计 1388｜2 笔）\n" +
		"<pre>时间       金额\n16:21:29   694.00\n16:20:49   694.00</pre>\n" +
		"余额：-17945.20"
}
