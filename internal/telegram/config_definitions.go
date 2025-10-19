package telegram

import (
	"go_bot/internal/telegram/models"
)

// getConfigItems 获取所有配置项定义
//
// ==================== 配置系统说明 ====================
//
// 当前使用：Toggle（开关型）配置 - 简单直观的功能开关
// 保留框架：支持 4 种配置类型（Toggle/Select/Input/Action），未来可随时扩展
//
// ==================== 如何添加新的开关配置 ====================
//
// 在下方数组中添加新的 ConfigItem 即可：
//
// {
//     ID:       "feature_enabled",              // 唯一标识
//     Name:     "功能名称",                      // 显示在菜单中的名称
//     Icon:     "🎯",                            // 功能图标
//     Type:     models.ConfigTypeToggle,        // 类型：开关
//     Category: "功能管理",                      // 分类（可用于分组）
//     ToggleGetter: func(g *models.Group) bool {
//         return g.Settings.FeatureEnabled      // 从 GroupSettings 读取当前状态
//     },
//     ToggleSetter: func(s *models.GroupSettings, val bool) {
//         s.FeatureEnabled = val                // 更新 GroupSettings
//     },
//     RequireAdmin: true,                       // 需要管理员权限
// }
//
// ==================== 高级配置类型（已支持，按需启用）====================
//
// 框架已支持以下类型，需要时参考 models/config_item.go 的完整文档：
//
// 1. Toggle（开关型）- 当前使用中
// 2. Select（选择型）- 例如：语言选择、主题选择
// 3. Input（输入型）  - 例如：自定义欢迎文本、自定义命令前缀
// 4. Action（动作型） - 例如：测试功能、重置统计、清理缓存
//
// 详细示例请查看 Git 历史记录中的完整注释，或参考 models/config_item.go
//
// ==================== 添加步骤 ====================
//
// 1. 如果需要持久化新配置，先在 models/group.go 的 GroupSettings 结构中添加字段
// 2. 在下方数组中添加配置项定义
// 3. 测试功能（发送 /configs 命令查看菜单）
//
func (b *Bot) getConfigItems() []models.ConfigItem {
	return []models.ConfigItem{
		// ========== 功能管理 ==========

		// 计算器功能开关
		{
			ID:       "calculator_enabled",
			Name:     "计算器功能",
			Icon:     "🧮",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.CalculatorEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.CalculatorEnabled = val
			},
			RequireAdmin: true,
		},

		// 翻译功能开关
		{
			ID:       "translator_enabled",
			Name:     "翻译功能",
			Icon:     "📖",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.TranslatorEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.TranslatorEnabled = val
			},
			RequireAdmin: true,
		},

		// ========== 扩展示例（已注释）==========
		//
		// 需要更多配置？取消注释或添加新配置项即可：
		//
		// 【Input 类型示例 - 自定义欢迎文本】
		// {
		//     ID:       "welcome_text",
		//     Name:     "欢迎文本",
		//     Icon:     "✏️",
		//     Type:     models.ConfigTypeInput,
		//     Category: "功能管理",
		//     InputGetter: func(g *models.Group) string {
		//         if g.Settings.WelcomeText == "" {
		//             return "欢迎 {name} 加入群组！"
		//         }
		//         return g.Settings.WelcomeText
		//     },
		//     InputSetter: func(s *models.GroupSettings, val string) {
		//         s.WelcomeText = val
		//     },
		//     InputPrompt: "📝 请输入欢迎文本\n\n可用占位符：{name}, {username}",
		//     InputValidator: func(text string) error {
		//         if len(text) > 500 {
		//             return fmt.Errorf("不能超过 500 字符")
		//         }
		//         return nil
		//     },
		//     RequireAdmin: true,
		// },
		//
		// 【Select 类型示例 - 语言选择】
		// {
		//     ID:       "language",
		//     Name:     "语言",
		//     Icon:     "🌐",
		//     Type:     models.ConfigTypeSelect,
		//     Category: "功能管理",
		//     SelectGetter: func(g *models.Group) string {
		//         if g.Settings.Language == "" {
		//             return "zh"
		//         }
		//         return g.Settings.Language
		//     },
		//     SelectOptions: []models.SelectOption{
		//         {Value: "zh", Label: "中文", Icon: "🇨🇳"},
		//         {Value: "en", Label: "English", Icon: "🇺🇸"},
		//     },
		//     SelectSetter: func(s *models.GroupSettings, val string) {
		//         s.Language = val
		//     },
		//     RequireAdmin: true,
		// },
	}
}

// getConfigItemByID 根据 ID 获取配置项
func (b *Bot) getConfigItemByID(id string) *models.ConfigItem {
	items := b.getConfigItems()
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// getConfigItemsByCategory 按分类分组配置项
func getConfigItemsByCategory(items []models.ConfigItem) map[string][]models.ConfigItem {
	result := make(map[string][]models.ConfigItem)
	for _, item := range items {
		result[item.Category] = append(result[item.Category], item)
	}
	return result
}
