package upstream

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

var (
	interfaceIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	ratePattern            = regexp.MustCompile(`^\d+(\.\d+)?%?$`)
	upstreamCommandPattern = regexp.MustCompile(`^(绑定接口\s+\S+.*|解绑接口(\s+\S+)?|接口ID|接口状态)$`)
)

const bindCommandGuide = "绑定接口 [接口名称] [接口ID] [接口费率]\n例如: 绑定接口 支付宝8888 123 7%"

// Feature 处理接口 ID 绑定逻辑
type Feature struct {
	groupService service.GroupService
	userService  service.UserService
}

// New 创建 Upstream 功能
func New(groupService service.GroupService, userService service.UserService) *Feature {
	return &Feature{
		groupService: groupService,
		userService:  userService,
	}
}

// Name 功能名称
func (f *Feature) Name() string {
	return "upstream"
}

// AllowedGroupTiers 限定接口管理功能可用的群等级
func (f *Feature) AllowedGroupTiers() []models.GroupTier {
	return []models.GroupTier{
		models.GroupTierBasic,
		models.GroupTierUpstream,
	}
}

// Enabled 功能是否启用
func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
	return true
}

// Match 判断是否命中命令
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Text == "" {
		return false
	}
	text := strings.TrimSpace(msg.Text)
	return upstreamCommandPattern.MatchString(text)
}

// Process 处理命令
func (f *Feature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Failed to check admin permission: user_id=%d, err=%v", msg.From.ID, err)
		return respond("❌ 权限检查失败"), true, nil
	}
	if !isAdmin {
		return respond("❌ 仅管理员可以操作接口绑定"), true, nil
	}

	text := strings.TrimSpace(msg.Text)

	switch {
	case strings.HasPrefix(text, "绑定接口 "):
		respText, handled, handlerErr := f.handleBind(ctx, msg, text)
		return respond(respText), handled, handlerErr
	case text == "解绑接口":
		respText, handled, handlerErr := f.handleUnbind(ctx, msg)
		return respond(respText), handled, handlerErr
	case text == "接口ID" || text == "接口状态":
		respText, handled, handlerErr := f.handleQuery(ctx, msg)
		return respond(respText), handled, handlerErr
	default:
		return nil, false, nil
	}
}

// Priority 功能优先级
func (f *Feature) Priority() int {
	// 紧随商户绑定命令之后
	return 16
}

func (f *Feature) handleBind(ctx context.Context, msg *botModels.Message, text string) (string, bool, error) {
	name, interfaceID, rate, errMsg := parseBindArguments(text)
	if errMsg != "" {
		return errMsg, true, nil
	}

	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	if group.Settings.MerchantID != 0 {
		return fmt.Sprintf("❌ 当前已绑定商户号: %d\n如需绑定接口，请先「解绑」商户号。", group.Settings.MerchantID), true, nil
	}

	settings := group.Settings
	settings.MerchantID = 0

	newBinding := models.InterfaceBinding{
		Name: name,
		ID:   interfaceID,
		Rate: rate,
	}

	currentBindings := group.Settings.InterfaceBindings
	action := "绑定成功"
	if idx := findBindingIndex(currentBindings, interfaceID); idx >= 0 {
		settings.InterfaceBindings[idx] = newBinding
		action = "信息已更新"
	} else {
		settings.InterfaceBindings = append(currentBindings, newBinding)
	}

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to bind interface ID: chat_id=%d, interface_id=%s, err=%v", msg.Chat.ID, interfaceID, err)
		return "❌ 绑定失败，请稍后重试", true, nil
	}

	logger.L().Infof("Interface binding saved: chat_id=%d, interface_id=%s, name=%s, rate=%s, operator=%d",
		msg.Chat.ID, interfaceID, name, rate, msg.From.ID)
	return fmt.Sprintf("✅ 接口%s：%s", action, formatInterfaceBindingSummary(newBinding)), true, nil
}

func (f *Feature) handleUnbind(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	current := group.Settings.InterfaceBindings
	if len(current) == 0 {
		return "ℹ️ 当前群组未绑定接口 ID", true, nil
	}

	parts := strings.Fields(strings.TrimSpace(msg.Text))
	settings := group.Settings
	if len(parts) == 1 {
		settings.InterfaceBindings = nil
		if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
			logger.L().Errorf("Failed to unbind all interface IDs: chat_id=%d, err=%v", msg.Chat.ID, err)
			return "❌ 解绑失败，请稍后重试", true, nil
		}
		logger.L().Infof("All interface IDs unbound: chat_id=%d, operator=%d", msg.Chat.ID, msg.From.ID)
		return "✅ 已解绑所有接口 ID", true, nil
	}

	target := parts[1]
	newList, removed := removeInterfaceBinding(current, target)
	if removed == nil {
		return fmt.Sprintf("ℹ️ 未找到接口 ID: %s", target), true, nil
	}

	settings.InterfaceBindings = newList

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to unbind interface ID: chat_id=%d, interface_id=%s, err=%v", msg.Chat.ID, target, err)
		return "❌ 解绑失败，请稍后重试", true, nil
	}

	logger.L().Infof("Interface ID unbound: chat_id=%d, interface_id=%s, operator=%d", msg.Chat.ID, target, msg.From.ID)
	return fmt.Sprintf("✅ 已解绑接口：%s", formatInterfaceBindingSummary(*removed)), true, nil
}

func (f *Feature) handleQuery(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	if len(group.Settings.InterfaceBindings) == 0 {
		return fmt.Sprintf("ℹ️ 当前群组未绑定接口 ID\n\n使用「%s」进行绑定", bindCommandGuide), true, nil
	}

	builder := strings.Builder{}
	builder.WriteString("✅ 当前绑定接口：\n")
	for _, binding := range group.Settings.InterfaceBindings {
		builder.WriteString(fmt.Sprintf("• %s\n", formatInterfaceBindingSummary(binding)))
	}
	builder.WriteString("\n使用「解绑接口 [接口ID]」解除单个接口，或直接发送「解绑接口」清空全部")

	return builder.String(), true, nil
}

func respond(text string) *types.Response {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return &types.Response{Text: text}
}

func parseBindArguments(text string) (name, interfaceID, rate, errMsg string) {
	parts := strings.Fields(text)
	if len(parts) < 4 {
		return "", "", "", fmt.Sprintf("❌ 绑定格式错误，请使用: %s", bindCommandGuide)
	}

	rawName := strings.Join(parts[1:len(parts)-2], " ")
	name = strings.TrimSpace(rawName)
	if name == "" {
		return "", "", "", "❌ 接口名称不能为空"
	}

	interfaceID = strings.TrimSpace(parts[len(parts)-2])
	if interfaceID == "" || !interfaceIDPattern.MatchString(interfaceID) {
		return "", "", "", "❌ 接口 ID 仅支持字母、数字、下划线或中划线"
	}

	rawRate := parts[len(parts)-1]
	normalizedRate, ok := normalizeRateInput(rawRate)
	if !ok {
		return "", "", "", "❌ 费率仅支持数字，可选结尾 % 符号\n例如: 7%"
	}

	return name, interfaceID, normalizedRate, ""
}

func normalizeRateInput(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !ratePattern.MatchString(trimmed) {
		return "", false
	}

	if strings.HasSuffix(trimmed, "%") {
		withoutPercent := strings.TrimSpace(strings.TrimSuffix(trimmed, "%"))
		if withoutPercent == "" {
			return "", false
		}
		return fmt.Sprintf("%s%%", withoutPercent), true
	}
	return trimmed, true
}

func findBindingIndex(bindings []models.InterfaceBinding, target string) int {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	if targetLower == "" {
		return -1
	}
	for idx, binding := range bindings {
		if strings.ToLower(binding.ID) == targetLower {
			return idx
		}
	}
	return -1
}

func removeInterfaceBinding(bindings []models.InterfaceBinding, target string) ([]models.InterfaceBinding, *models.InterfaceBinding) {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	if targetLower == "" {
		return bindings, nil
	}

	result := make([]models.InterfaceBinding, 0, len(bindings))
	var removed *models.InterfaceBinding
	for _, binding := range bindings {
		if removed == nil && strings.ToLower(binding.ID) == targetLower {
			copyBinding := binding
			removed = &copyBinding
			continue
		}
		result = append(result, binding)
	}
	if removed == nil {
		return bindings, nil
	}
	return result, removed
}

func formatInterfaceBindingSummary(binding models.InterfaceBinding) string {
	name := bindingDisplayName(binding.Name)
	rate := strings.TrimSpace(binding.Rate)
	rateSummary := ""
	if rate != "" {
		rateSummary = fmt.Sprintf("，费率: %s", html.EscapeString(rate))
	}
	return fmt.Sprintf("%s (ID: <code>%s</code>%s)",
		html.EscapeString(name),
		html.EscapeString(binding.ID),
		rateSummary)
}

func bindingDisplayName(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "(未命名接口)"
	}
	return clean
}
