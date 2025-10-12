package telegram

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/telegram/models"
)

// getConfigItems 获取所有配置项定义
//
// ==================== 如何添加新配置项 ====================
//
// 这是添加新配置项的地方！只需在下方的 configItems 数组中添加新的配置项定义即可。
//
// 支持 4 种配置类型：
// 1. Toggle（开关型）- 开启/关闭某个功能
// 2. Select（选择型）- 从多个选项中选择一个
// 3. Input（输入型）- 用户输入文本
// 4. Action（动作型）- 执行一次性操作（如测试、重置）
//
// ==================== 配置类型示例 ====================
//
// 【1. Toggle 开关型示例】
// {
//     ID:       "feature_enabled",           // 唯一标识
//     Name:     "功能名称",                    // 显示名称
//     Icon:     "🎯",                         // 图标
//     Type:     models.ConfigTypeToggle,     // 类型：开关
//     Category: "功能分类",                    // 分类（用于分组显示）
//     ToggleGetter: func(g *models.Group) bool {
//         return g.Settings.FeatureEnabled    // 从群组配置中读取当前状态
//     },
//     ToggleSetter: func(s *models.GroupSettings, val bool) {
//         s.FeatureEnabled = val              // 更新群组配置
//     },
//     RequireAdmin: true,                    // 是否需要管理员权限
// }
//
// 【2. Select 选择型示例】
// {
//     ID:       "theme",
//     Name:     "主题",
//     Icon:     "🎨",
//     Type:     models.ConfigTypeSelect,
//     Category: "外观设置",
//     SelectGetter: func(g *models.Group) string {
//         return g.Settings.Theme             // 返回当前选项值
//     },
//     SelectOptions: []models.SelectOption{
//         {Value: "light", Label: "浅色", Icon: "☀️"},
//         {Value: "dark", Label: "深色", Icon: "🌙"},
//     },
//     SelectSetter: func(s *models.GroupSettings, val string) {
//         s.Theme = val                       // 设置新选项
//     },
//     RequireAdmin: true,
// }
//
// 【3. Input 输入型示例】
// {
//     ID:       "custom_message",
//     Name:     "自定义消息",
//     Icon:     "✏️",
//     Type:     models.ConfigTypeInput,
//     Category: "消息管理",
//     InputGetter: func(g *models.Group) string {
//         return g.Settings.CustomMessage     // 返回当前值
//     },
//     InputSetter: func(s *models.GroupSettings, val string) {
//         s.CustomMessage = val               // 设置新值
//     },
//     InputPrompt: "📝 请输入自定义消息内容",  // 输入提示文本
//     InputValidator: func(text string) error {
//         if len(text) > 200 {
//             return fmt.Errorf("内容不能超过 200 字符")
//         }
//         return nil                          // 验证通过
//     },
//     RequireAdmin: true,
// }
//
// 【4. Action 动作型示例】
// {
//     ID:       "reset_stats",
//     Name:     "重置统计",
//     Icon:     "🔄",
//     Type:     models.ConfigTypeAction,
//     Category: "数据管理",
//     ActionHandler: func(ctx context.Context, chatID, userID int64) error {
//         // 执行自定义操作
//         // 注意：可以访问 b.groupService 等服务
//         return nil                          // 返回 nil 表示成功
//     },
//     RequireAdmin: true,
// }
//
// ==================== 添加步骤 ====================
//
// 1. 在 models/group.go 的 GroupSettings 结构中添加新字段（如果需要持久化）
// 2. 在下方的 configItems 数组中添加配置项定义
// 3. 测试配置菜单功能
//
// 注意事项：
// - ID 必须唯一
// - Category 相同的配置项会分组显示
// - RequireAdmin 设置为 true 时，只有管理员可以修改
// - InputValidator 验证失败最多允许重试 3 次
//
func (b *Bot) getConfigItems() []models.ConfigItem {
	return []models.ConfigItem{
		// ========== 消息管理 ==========

		// 欢迎消息开关
		{
			ID:       "welcome_enabled",
			Name:     "欢迎消息",
			Icon:     "🎉",
			Type:     models.ConfigTypeToggle,
			Category: "消息管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.WelcomeEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.WelcomeEnabled = val
			},
			RequireAdmin: true,
		},

		// 欢迎文本编辑
		{
			ID:       "welcome_text",
			Name:     "欢迎文本",
			Icon:     "✏️",
			Type:     models.ConfigTypeInput,
			Category: "消息管理",
			InputGetter: func(g *models.Group) string {
				if g.Settings.WelcomeText == "" {
					return "欢迎 {name} 加入群组！"
				}
				return g.Settings.WelcomeText
			},
			InputSetter: func(s *models.GroupSettings, val string) {
				s.WelcomeText = val
			},
			InputPrompt: "📝 请输入欢迎文本\n\n" +
				"💡 可用占位符：\n" +
				"• {name} - 用户的名字\n" +
				"• {username} - 用户的 @用户名\n\n" +
				"示例：欢迎 {name} 加入我们！",
			InputValidator: func(text string) error {
				if len(text) > 500 {
					return fmt.Errorf("欢迎文本不能超过 500 字符")
				}
				if len(text) == 0 {
					return fmt.Errorf("欢迎文本不能为空")
				}
				return nil
			},
			RequireAdmin: true,
		},

		// 测试欢迎消息
		{
			ID:       "test_welcome",
			Name:     "测试欢迎消息",
			Icon:     "🧪",
			Type:     models.ConfigTypeAction,
			Category: "消息管理",
			ActionHandler: func(ctx context.Context, chatID, userID int64) error {
				// 获取群组配置
				group, err := b.groupService.GetGroupInfo(ctx, chatID)
				if err != nil {
					return err
				}

				// 获取用户信息
				user, err := b.userService.GetUserInfo(ctx, userID)
				if err != nil {
					return err
				}

				// 构造测试消息
				welcomeText := group.Settings.WelcomeText
				if welcomeText == "" {
					welcomeText = "欢迎 {name} 加入群组！"
				}
				welcomeText = strings.ReplaceAll(welcomeText, "{name}", user.FirstName)
				welcomeText = strings.ReplaceAll(welcomeText, "{username}", "@"+user.Username)

				// 发送测试消息
				b.sendMessage(ctx, chatID, "🧪 测试欢迎消息：\n\n"+welcomeText)
				return nil
			},
			RequireAdmin: true,
		},

		// ========== 安全管理 ==========

		// 反垃圾开关
		{
			ID:       "antispam_enabled",
			Name:     "反垃圾",
			Icon:     "🛡️",
			Type:     models.ConfigTypeToggle,
			Category: "安全管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.AntiSpam
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.AntiSpam = val
			},
			RequireAdmin: true,
		},

		// ========== 基础设置 ==========

		// 语言选择
		{
			ID:       "language",
			Name:     "语言",
			Icon:     "🌐",
			Type:     models.ConfigTypeSelect,
			Category: "基础设置",
			SelectGetter: func(g *models.Group) string {
				if g.Settings.Language == "" {
					return "zh"
				}
				return g.Settings.Language
			},
			SelectOptions: []models.SelectOption{
				{Value: "zh", Label: "中文", Icon: "🇨🇳"},
				{Value: "en", Label: "English", Icon: "🇺🇸"},
			},
			SelectSetter: func(s *models.GroupSettings, val string) {
				s.Language = val
			},
			RequireAdmin: true,
		},
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
