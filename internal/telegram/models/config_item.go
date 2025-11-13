package models

import (
	"context"
)

// ConfigItemType 配置项类型
type ConfigItemType string

const (
	ConfigTypeToggle ConfigItemType = "toggle" // 开关型
	ConfigTypeSelect ConfigItemType = "select" // 选择型
	ConfigTypeInput  ConfigItemType = "input"  // 输入型
	ConfigTypeAction ConfigItemType = "action" // 动作型（测试、重置等）
)

// ConfigItem 配置项定义
//
// 这是一个通用的配置项元数据结构，支持4种类型：
// - Toggle: 开关型配置（如：启用/禁用欢迎消息）
// - Select: 选择型配置（如：语言选择 zh/en）
// - Input: 输入型配置（如：自定义欢迎文本）
// - Action: 动作型配置（如：测试欢迎消息）
type ConfigItem struct {
	ID       string         // 唯一标识，如 "welcome_enabled"
	Name     string         // 显示名称，如 "欢迎消息"
	Icon     string         // 图标，如 "🎉"
	Type     ConfigItemType // 配置类型
	Category string         // 分类（用于分组显示）
	// AllowedTiers 限定哪些群组等级可见此配置；为空表示所有等级
	AllowedTiers []GroupTier

	// Toggle 类型专用
	ToggleGetter   func(*Group) bool           // 获取当前状态
	ToggleSetter   func(*GroupSettings, bool)  // 设置状态
	ToggleDisabled func(*Group) (bool, string) // 是否禁用开关及原因（返回 true 表示禁用）

	// Select 类型专用
	SelectGetter  func(*Group) string          // 获取当前选项
	SelectOptions []SelectOption               // 可选项
	SelectSetter  func(*GroupSettings, string) // 设置选项

	// Input 类型专用
	InputGetter    func(*Group) string          // 获取当前值
	InputSetter    func(*GroupSettings, string) // 设置值
	InputPrompt    string                       // 输入提示文本
	InputValidator func(string) error           // 输入验证器

	// Action 类型专用
	// ActionHandler 的参数：(ctx, chatID, userID)
	// 由于 ActionHandler 需要访问 Bot 实例，我们使用 interface{} 避免循环依赖
	// 实际使用时会传入 func(context.Context, interface{}, int64, int64) error
	ActionHandler interface{}

	// 权限控制
	RequireAdmin bool // 是否需要管理员权限
}

// SelectOption 选择项
type SelectOption struct {
	Value string // 内部值
	Label string // 显示标签
	Icon  string // 图标
}

// UserState 用户状态（用于管理多步交互）
//
// 当用户点击"编辑文本"等需要输入的配置时，
// 会设置用户状态，等待用户发送文本消息
type UserState struct {
	UserID     int64           // 用户 ID
	ChatID     int64           // 聊天 ID
	Action     string          // 动作标识，如 "input:welcome_text"
	MessageID  int             // 菜单消息 ID
	ExpiresAt  int64           // 过期时间（Unix 时间戳）
	RetryCount int             // 重试次数（用于限制验证失败重试）
	Context    context.Context // 上下文（用于取消操作）
}
