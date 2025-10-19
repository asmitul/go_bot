package calculator

import (
	"context"
	"fmt"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"
	botModels "github.com/go-telegram/bot/models"
)

// CalculatorFeature 计算器功能插件
type CalculatorFeature struct{}

// New 创建计算器功能实例
func New() *CalculatorFeature {
	return &CalculatorFeature{}
}

// Name 返回功能名称
func (f *CalculatorFeature) Name() string {
	return "calculator"
}

// Enabled 检查功能是否启用
func (f *CalculatorFeature) Enabled(ctx context.Context, group *models.Group) bool {
	return group.Settings.CalculatorEnabled
}

// Match 检查消息是否匹配(只处理群组中的数学表达式)
func (f *CalculatorFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	// 只处理群组消息
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	// 检查是否为数学表达式
	return IsMathExpression(msg.Text)
}

// Process 处理计算请求
func (f *CalculatorFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (string, bool, error) {
	// 执行计算
	result, err := Calculate(msg.Text)
	if err != nil {
		// 计算失败
		logger.L().Warnf("Calculator failed: chat_id=%d, text=%s, error=%v", msg.Chat.ID, msg.Text, err)
		return fmt.Sprintf("❌ 计算错误: %v", err), true, nil
	}

	// 计算成功
	logger.L().Infof("Calculator: %s = %g (chat_id=%d)", msg.Text, result, msg.Chat.ID)
	return fmt.Sprintf("🧮 %s = %g", msg.Text, result), true, nil
}

// Priority 返回优先级(20 = 高优先级)
func (f *CalculatorFeature) Priority() int {
	return 20
}
