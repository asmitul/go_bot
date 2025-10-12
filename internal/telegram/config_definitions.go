package telegram

import (
	"context"
	"fmt"
	"strings"

	"go_bot/internal/telegram/models"
)

// getConfigItems 获取所有配置项定义
//
// 这是添加新配置项的地方！
// 只需在 configItems 数组中添加新的配置项定义即可
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
