package translator

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	botModels "github.com/go-telegram/bot/models"
)

// TranslatorFeature 翻译功能插件
type TranslatorFeature struct{}

func New() *TranslatorFeature {
	return &TranslatorFeature{}
}

func (f *TranslatorFeature) Name() string {
	return "translator"
}

func (f *TranslatorFeature) Enabled(ctx context.Context, group *models.Group) bool {
	// 从群组配置读取(需要先在 models.GroupSettings 中添加 TranslatorEnabled 字段)
	return group.Settings.TranslatorEnabled
}

func (f *TranslatorFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	// 匹配 "翻译 xxx" 或 "/translate xxx"
	text := strings.TrimSpace(msg.Text)
	return strings.HasPrefix(text, "翻译 ") || strings.HasPrefix(text, "/translate ")
}

func (f *TranslatorFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (string, bool, error) {
	// 提取待翻译文本
	text := strings.TrimSpace(msg.Text)
	text = strings.TrimPrefix(text, "翻译 ")
	text = strings.TrimPrefix(text, "/translate ")
	text = strings.TrimSpace(text)

	if text == "" {
		return "❌ 请提供要翻译的文本\n\n用法: 翻译 hello world", true, nil
	}

	// 调用翻译 API(这里是示例,需要替换为真实的翻译 API)
	translated, err := translate(text)
	if err != nil {
		logger.L().Errorf("Translation failed: %v", err)
		return fmt.Sprintf("❌ 翻译失败: %v", err), true, nil
	}

	logger.L().Infof("Translated: %s -> %s (chat_id=%d)", text, translated, msg.Chat.ID)
	return fmt.Sprintf("📖 翻译结果:\n\n原文: %s\n译文: %s", text, translated), true, nil
}

func (f *TranslatorFeature) Priority() int {
	return 30 // 中等优先级
}

// translate 调用翻译 API(示例实现)
func translate(text string) (string, error) {
	// TODO: 替换为真实的翻译 API 调用
	// 例如: Google Translate API、DeepL API、百度翻译 API 等

	// 示例: 简单的中英互译检测
	isChinese := containsChinese(text)
	if isChinese {
		return "Hello World (Demo Translation)", nil
	}
	return "你好世界 (演示翻译)", nil
}

// containsChinese 检测文本是否包含中文字符
func containsChinese(text string) bool {
	for _, r := range text {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}
