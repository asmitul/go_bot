package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
)

const (
	// MaxInputRetries 最大输入验证失败重试次数
	MaxInputRetries = 3
)

// ConfigMenuService 配置菜单服务
// 负责构建 InlineKeyboard 菜单和处理用户交互
type ConfigMenuService struct {
	groupService GroupService
	userStates   sync.Map // map[string]*models.UserState (key: "chatID:userID")
}

// NewConfigMenuService 创建配置菜单服务
func NewConfigMenuService(groupService GroupService) *ConfigMenuService {
	return &ConfigMenuService{
		groupService: groupService,
		userStates:   sync.Map{},
	}
}

// BuildMainMenu 构建主配置菜单
// 根据 ConfigItem 定义生成 InlineKeyboard
// 注意：调用方需要先调用 GetOrCreateGroup 确保群组存在
func (s *ConfigMenuService) BuildMainMenu(ctx context.Context, group *models.Group, items []models.ConfigItem) (*botModels.InlineKeyboardMarkup, error) {
	var keyboard [][]botModels.InlineKeyboardButton

	// 直接添加所有配置项按钮（不分类、不分组，简洁平铺）
	for _, item := range items {
		button := s.buildButtonForItem(item, group)
		keyboard = append(keyboard, []botModels.InlineKeyboardButton{button})
	}

	// 添加底部操作按钮
	keyboard = append(keyboard, []botModels.InlineKeyboardButton{
		{Text: "🔄 刷新", CallbackData: "config:refresh"},
		{Text: "❌ 关闭", CallbackData: "config:close"},
	})

	return &botModels.InlineKeyboardMarkup{InlineKeyboard: keyboard}, nil
}

// buildButtonForItem 为单个配置项构建按钮
func (s *ConfigMenuService) buildButtonForItem(item models.ConfigItem, group *models.Group) botModels.InlineKeyboardButton {
	var statusText string
	var disabled bool
	var disabledReason string

	switch item.Type {
	case models.ConfigTypeToggle:
		// 开关型：显示当前状态 ON/OFF
		enabled := item.ToggleGetter(group)
		if item.ToggleDisabled != nil {
			disabled, disabledReason = item.ToggleDisabled(group)
		}
		if enabled {
			statusText = "✅"
		} else {
			statusText = "❌"
		}

	case models.ConfigTypeSelect:
		// 选择型：显示当前选项
		currentValue := item.SelectGetter(group)
		for _, opt := range item.SelectOptions {
			if opt.Value == currentValue {
				statusText = opt.Icon
				break
			}
		}

	case models.ConfigTypeInput:
		// 输入型：显示编辑图标
		statusText = "✏️"

	case models.ConfigTypeAction:
		// 动作型：显示动作图标
		statusText = "▶️"
	}

	// 按钮文本格式：图标 + 名称 + 状态
	buttonText := fmt.Sprintf("%s %s %s", item.Icon, item.Name, statusText)
	if disabled && disabledReason != "" {
		buttonText = fmt.Sprintf("%s %s（%s） %s", item.Icon, item.Name, disabledReason, statusText)
	}
	callbackData := fmt.Sprintf("config:%s:%s", item.Type, item.ID)

	return botModels.InlineKeyboardButton{
		Text:         buttonText,
		CallbackData: callbackData,
	}
}

// HandleCallback 处理回调查询（用户点击按钮）
// 注意：调用方需要先调用 GetOrCreateGroup 确保群组存在
func (s *ConfigMenuService) HandleCallback(
	ctx context.Context,
	group *models.Group,
	userID int64,
	data string,
	items []models.ConfigItem,
) (message string, shouldUpdateMenu bool, err error) {
	chatID := group.TelegramID
	// 解析 callback data: "config:type:id" 或 "config:action"
	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return "❌ 无效的回调数据", false, fmt.Errorf("invalid callback data: %s", data)
	}

	action := parts[1]

	switch action {
	case "refresh":
		return "🔄 菜单已刷新", true, nil

	case "close":
		return "✅ 配置菜单已关闭", false, nil

	case "noop":
		// 不可点击的按钮（如分类标题）
		return "", false, nil

	case string(models.ConfigTypeToggle):
		if len(parts) < 3 {
			return "❌ 缺少配置项 ID", false, fmt.Errorf("missing config ID")
		}
		return s.handleToggle(ctx, group, parts[2], items)

	case string(models.ConfigTypeSelect):
		if len(parts) < 3 {
			return "❌ 缺少配置项 ID", false, fmt.Errorf("missing config ID")
		}
		return s.handleSelect(ctx, group, userID, parts[2], items)

	case string(models.ConfigTypeInput):
		if len(parts) < 3 {
			return "❌ 缺少配置项 ID", false, fmt.Errorf("missing config ID")
		}
		return s.handleInput(ctx, chatID, userID, parts[2], items)

	case string(models.ConfigTypeAction):
		if len(parts) < 3 {
			return "❌ 缺少配置项 ID", false, fmt.Errorf("missing config ID")
		}
		return s.handleAction(ctx, chatID, userID, parts[2], items)

	default:
		return "❌ 未知的操作", false, fmt.Errorf("unknown action: %s", action)
	}
}

// handleToggle 处理开关型配置
func (s *ConfigMenuService) handleToggle(ctx context.Context, group *models.Group, configID string, items []models.ConfigItem) (string, bool, error) {
	// 查找配置项
	item := findItemByID(items, configID)
	if item == nil {
		return "❌ 配置项不存在", false, fmt.Errorf("config item not found: %s", configID)
	}

	if item.ToggleDisabled != nil {
		if disabled, reason := item.ToggleDisabled(group); disabled {
			if reason == "" {
				reason = "当前功能不可用"
			}
			return fmt.Sprintf("⚠️ %s", reason), false, nil
		}
	}

	// 切换状态
	currentValue := item.ToggleGetter(group)
	newValue := !currentValue

	// 更新配置
	item.ToggleSetter(&group.Settings, newValue)
	if err := s.groupService.UpdateGroupSettings(ctx, group.TelegramID, group.Settings); err != nil {
		return "❌ 更新配置失败", false, err
	}

	statusText := "关闭"
	if newValue {
		statusText = "开启"
	}

	logger.L().Infof("Config toggle updated: chat_id=%d, config=%s, value=%v", group.TelegramID, configID, newValue)
	return fmt.Sprintf("✅ %s 已%s", item.Name, statusText), true, nil
}

// handleSelect 处理选择型配置（暂不实现多选框，直接切换到下一个选项）
func (s *ConfigMenuService) handleSelect(ctx context.Context, group *models.Group, userID int64, configID string, items []models.ConfigItem) (string, bool, error) {
	// 查找配置项
	item := findItemByID(items, configID)
	if item == nil {
		return "❌ 配置项不存在", false, fmt.Errorf("config item not found: %s", configID)
	}

	// 获取当前选项
	currentValue := item.SelectGetter(group)

	// 找到下一个选项（循环）
	currentIndex := -1
	for i, opt := range item.SelectOptions {
		if opt.Value == currentValue {
			currentIndex = i
			break
		}
	}

	nextIndex := (currentIndex + 1) % len(item.SelectOptions)
	nextOption := item.SelectOptions[nextIndex]

	// 更新配置
	item.SelectSetter(&group.Settings, nextOption.Value)
	if err := s.groupService.UpdateGroupSettings(ctx, group.TelegramID, group.Settings); err != nil {
		return "❌ 更新配置失败", false, err
	}

	logger.L().Infof("Config select updated: chat_id=%d, config=%s, value=%s", group.TelegramID, configID, nextOption.Value)
	return fmt.Sprintf("✅ %s 已设置为：%s %s", item.Name, nextOption.Icon, nextOption.Label), true, nil
}

// handleInput 处理输入型配置（设置用户状态，等待用户输入）
func (s *ConfigMenuService) handleInput(ctx context.Context, chatID, userID int64, configID string, items []models.ConfigItem) (string, bool, error) {
	// 查找配置项
	item := findItemByID(items, configID)
	if item == nil {
		return "❌ 配置项不存在", false, fmt.Errorf("config item not found: %s", configID)
	}

	// 设置用户状态
	state := &models.UserState{
		UserID:     userID,
		ChatID:     chatID,
		Action:     fmt.Sprintf("input:%s", configID),
		ExpiresAt:  time.Now().Add(5 * time.Minute).Unix(), // 5分钟过期
		RetryCount: 0,                                      // 初始化重试次数
		Context:    ctx,
	}
	s.SetUserState(chatID, userID, state)

	logger.L().Infof("User state set: chat_id=%d, user_id=%d, action=%s", chatID, userID, state.Action)
	return fmt.Sprintf("📝 %s\n\n请在 5 分钟内发送文本消息：", item.InputPrompt), false, nil
}

// handleAction 处理动作型配置（执行自定义操作）
func (s *ConfigMenuService) handleAction(ctx context.Context, chatID, userID int64, configID string, items []models.ConfigItem) (string, bool, error) {
	// 查找配置项
	item := findItemByID(items, configID)
	if item == nil {
		return "❌ 配置项不存在", false, fmt.Errorf("config item not found: %s", configID)
	}

	// 执行 ActionHandler
	if item.ActionHandler == nil {
		return "❌ 未配置操作处理器", false, fmt.Errorf("action handler not configured")
	}

	// 类型断言为正确的函数签名
	handler, ok := item.ActionHandler.(func(context.Context, int64, int64) error)
	if !ok {
		return "❌ 操作处理器类型错误", false, fmt.Errorf("invalid action handler type")
	}

	// 执行操作
	if err := handler(ctx, chatID, userID); err != nil {
		logger.L().Errorf("Action handler failed: config=%s, error=%v", configID, err)
		return fmt.Sprintf("❌ 操作失败: %v", err), false, err
	}

	logger.L().Infof("Action executed: chat_id=%d, config=%s", chatID, configID)
	return fmt.Sprintf("✅ %s 执行成功", item.Name), true, nil
}

// ProcessUserInput 处理用户输入（当用户处于输入状态时）
// 注意：调用方需要先调用 GetOrCreateGroup 确保群组存在
func (s *ConfigMenuService) ProcessUserInput(
	ctx context.Context,
	group *models.Group,
	userID int64,
	text string,
	items []models.ConfigItem,
) (message string, err error) {
	chatID := group.TelegramID
	// 获取用户状态
	state := s.GetUserState(chatID, userID)
	if state == nil {
		return "", nil // 用户没有待处理状态
	}

	// 检查是否过期
	if time.Now().Unix() > state.ExpiresAt {
		s.ClearUserState(chatID, userID)
		return "⏰ 输入超时，请重新打开配置菜单", fmt.Errorf("user state expired")
	}

	// 解析状态：input:config_id
	parts := strings.Split(state.Action, ":")
	if len(parts) != 2 || parts[0] != "input" {
		s.ClearUserState(chatID, userID)
		return "❌ 无效的用户状态", fmt.Errorf("invalid user state: %s", state.Action)
	}

	configID := parts[1]

	// 查找配置项
	item := findItemByID(items, configID)
	if item == nil {
		s.ClearUserState(chatID, userID)
		return "❌ 配置项不存在", fmt.Errorf("config item not found: %s", configID)
	}

	// 验证输入
	if item.InputValidator != nil {
		if err := item.InputValidator(text); err != nil {
			// 验证失败，检查重试次数
			state.RetryCount++

			if state.RetryCount >= MaxInputRetries {
				// 超过最大重试次数，清除状态
				s.ClearUserState(chatID, userID)
				logger.L().Warnf("User exceeded max input retries: chat_id=%d, user_id=%d, config=%s", chatID, userID, configID)
				return fmt.Sprintf("❌ 输入验证失败次数过多\n\n错误: %v\n\n请重新打开配置菜单", err), fmt.Errorf("max retries exceeded")
			}

			// 未超过限制，更新状态并允许重新输入
			s.SetUserState(chatID, userID, state)
			remainingRetries := MaxInputRetries - state.RetryCount
			return fmt.Sprintf("❌ 输入验证失败: %v\n\n剩余尝试次数: %d\n请重新输入：", err, remainingRetries), nil
		}
	}

	// 更新配置
	item.InputSetter(&group.Settings, text)
	if err := s.groupService.UpdateGroupSettings(ctx, chatID, group.Settings); err != nil {
		s.ClearUserState(chatID, userID)
		return "❌ 更新配置失败", err
	}

	// 清除用户状态
	s.ClearUserState(chatID, userID)

	logger.L().Infof("Config input updated: chat_id=%d, config=%s", chatID, configID)
	return fmt.Sprintf("✅ %s 已更新", item.Name), nil
}

// SetUserState 设置用户状态
func (s *ConfigMenuService) SetUserState(chatID, userID int64, state *models.UserState) {
	key := fmt.Sprintf("%d:%d", chatID, userID)
	s.userStates.Store(key, state)
}

// GetUserState 获取用户状态
func (s *ConfigMenuService) GetUserState(chatID, userID int64) *models.UserState {
	key := fmt.Sprintf("%d:%d", chatID, userID)
	val, ok := s.userStates.Load(key)
	if !ok {
		return nil
	}
	return val.(*models.UserState)
}

// ClearUserState 清除用户状态
func (s *ConfigMenuService) ClearUserState(chatID, userID int64) {
	key := fmt.Sprintf("%d:%d", chatID, userID)
	s.userStates.Delete(key)
}

// findItemByID 根据 ID 查找配置项
func findItemByID(items []models.ConfigItem, id string) *models.ConfigItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
