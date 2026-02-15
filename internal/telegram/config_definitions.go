package telegram

import (
	"fmt"
	"strconv"

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
//	{
//	    ID:       "feature_enabled",              // 唯一标识
//	    Name:     "功能名称",                      // 显示在菜单中的名称
//	    Icon:     "🎯",                            // 功能图标
//	    Type:     models.ConfigTypeToggle,        // 类型：开关
//	    Category: "功能管理",                      // 分类（可用于分组）
//	    ToggleGetter: func(g *models.Group) bool {
//	        return g.Settings.FeatureEnabled      // 从 GroupSettings 读取当前状态
//	    },
//	    ToggleSetter: func(s *models.GroupSettings, val bool) {
//	        s.FeatureEnabled = val                // 更新 GroupSettings
//	    },
//	    RequireAdmin: true,                       // 需要管理员权限
//	}
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

		// 加密货币价格查询功能开关
		{
			ID:       "crypto_enabled",
			Name:     "USDT价格查询",
			Icon:     "💰",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.CryptoEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.CryptoEnabled = val
			},
			RequireAdmin: true,
		},

		// 加密货币浮动费率选择
		{
			ID:       "crypto_float_rate",
			Name:     "USDT浮动费率",
			Icon:     "📊",
			Type:     models.ConfigTypeSelect,
			Category: "功能管理",
			SelectGetter: func(g *models.Group) string {
				// 将 float64 转换为字符串
				return fmt.Sprintf("%.2f", g.Settings.CryptoFloatRate)
			},
			SelectOptions: []models.SelectOption{
				{Value: "0.00", Label: "无浮动", Icon: "⭕"},
				{Value: "0.08", Label: "0.08", Icon: "0️⃣·0️⃣8️⃣"},
				{Value: "0.09", Label: "0.09", Icon: "0️⃣·0️⃣9️⃣"},
				{Value: "0.10", Label: "0.10", Icon: "0️⃣·1️⃣0️⃣"},
				{Value: "0.11", Label: "0.11", Icon: "0️⃣·1️⃣1️⃣"},
				{Value: "0.12", Label: "0.12", Icon: "0️⃣·1️⃣2️⃣"},
				{Value: "0.13", Label: "0.13", Icon: "0️⃣·1️⃣3️⃣"},
			},
			SelectSetter: func(s *models.GroupSettings, val string) {
				// 将字符串转换为 float64
				rate, _ := strconv.ParseFloat(val, 64)
				s.CryptoFloatRate = rate
			},
			RequireAdmin: true,
		},

		// 接收频道转发开关
		{
			ID:       "forward_enabled",
			Name:     "接收频道转发",
			Icon:     "📢",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.ForwardEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.ForwardEnabled = val
			},
			RequireAdmin: true,
		},

		// 收支记账功能开关
		{
			ID:       "accounting_enabled",
			Name:     "收支记账",
			Icon:     "💳",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.AccountingEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.AccountingEnabled = val
			},
			RequireAdmin: true,
		},

		// 四方支付功能开关
		{
			ID:       "sifang_enabled",
			Name:     "四方支付查询",
			Icon:     "🏦",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			AllowedTiers: []models.GroupTier{
				models.GroupTierMerchant,
			},
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.SifangEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.SifangEnabled = val
			},
			RequireAdmin: true,
		},

		// 四方支付自动查单开关
		{
			ID:       "sifang_auto_lookup_enabled",
			Name:     "四方自动查单",
			Icon:     "🔍",
			Type:     models.ConfigTypeToggle,
			Category: "功能管理",
			AllowedTiers: []models.GroupTier{
				models.GroupTierMerchant,
			},
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.SifangAutoLookupEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.SifangAutoLookupEnabled = val
			},
			ToggleDisabled: func(g *models.Group) (bool, string) {
				if !g.Settings.SifangEnabled {
					return true, "需先开启四方支付"
				}
				return false, ""
			},
			RequireAdmin: true,
		},

		// 订单联动回传引用开关（仅商户群）
		{
			ID:       "cascade_reply_enabled",
			Name:     "回传引用消息",
			Icon:     "💬",
			Type:     models.ConfigTypeToggle,
			Category: "订单联动",
			AllowedTiers: []models.GroupTier{
				models.GroupTierMerchant,
			},
			ToggleGetter: func(g *models.Group) bool {
				return models.IsCascadeReplyEnabled(g.Settings)
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.CascadeReplyEnabled = val
				s.CascadeReplyConfigured = true
			},
			RequireAdmin: true,
		},

		// 订单联动转发开关（仅上游群）
		{
			ID:       "cascade_forward_enabled",
			Name:     "转单开关",
			Icon:     "🔁",
			Type:     models.ConfigTypeToggle,
			Category: "订单联动",
			AllowedTiers: []models.GroupTier{
				models.GroupTierUpstream,
			},
			ToggleGetter: func(g *models.Group) bool {
				return g.Settings.CascadeForwardEnabled
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.CascadeForwardEnabled = val
				s.CascadeForwardConfigured = true
			},
			RequireAdmin: true,
		},

		// 上游余额轮询告警开关（仅上游群）
		{
			ID:       "balance_monitor_enabled",
			Name:     "上游余额轮询告警",
			Icon:     "🚨",
			Type:     models.ConfigTypeToggle,
			Category: "监控告警",
			AllowedTiers: []models.GroupTier{
				models.GroupTierUpstream,
			},
			ToggleGetter: func(g *models.Group) bool {
				return models.IsBalanceMonitorEnabled(g.Settings)
			},
			ToggleSetter: func(s *models.GroupSettings, val bool) {
				s.BalanceMonitorEnabled = val
				s.BalanceMonitorConfigured = true
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
