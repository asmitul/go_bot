package upstream

import (
	"context"
	"fmt"
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
	upstreamCommandPattern = regexp.MustCompile(`^(绑定接口\s+\S+|解绑接口(\s+\S+)?|接口ID|接口状态)$`)
)

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
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return "❌ 绑定格式错误，请使用: 绑定接口 [接口ID]\n例如: 绑定接口 upstream_01", true, nil
	}

	interfaceID := strings.TrimSpace(parts[1])
	if !interfaceIDPattern.MatchString(interfaceID) {
		return "❌ 接口 ID 仅支持字母、数字、下划线或中划线", true, nil
	}

	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	if group.Settings.MerchantID != 0 {
		return fmt.Sprintf("❌ 当前已绑定商户号: %d\n如需绑定接口，请先「解绑」商户号。", group.Settings.MerchantID), true, nil
	}

	currentIDs := group.Settings.InterfaceIDs
	if containsInterfaceID(currentIDs, interfaceID) {
		return fmt.Sprintf("✅ 当前群组已绑定接口 ID: %s", interfaceID), true, nil
	}

	settings := group.Settings
	settings.MerchantID = 0
	settings.InterfaceIDs = append(currentIDs, interfaceID)

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to bind interface ID: chat_id=%d, interface_id=%s, err=%v", msg.Chat.ID, interfaceID, err)
		return "❌ 绑定失败，请稍后重试", true, nil
	}

	logger.L().Infof("Interface ID bound: chat_id=%d, interface_id=%s, operator=%d", msg.Chat.ID, interfaceID, msg.From.ID)
	return fmt.Sprintf("✅ 接口 ID 绑定成功: %s", interfaceID), true, nil
}

func (f *Feature) handleUnbind(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	current := group.Settings.InterfaceIDs
	if len(current) == 0 {
		return "ℹ️ 当前群组未绑定接口 ID", true, nil
	}

	parts := strings.Fields(strings.TrimSpace(msg.Text))
	settings := group.Settings
	if len(parts) == 1 {
		settings.InterfaceIDs = nil
		if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
			logger.L().Errorf("Failed to unbind all interface IDs: chat_id=%d, err=%v", msg.Chat.ID, err)
			return "❌ 解绑失败，请稍后重试", true, nil
		}
		logger.L().Infof("All interface IDs unbound: chat_id=%d, operator=%d", msg.Chat.ID, msg.From.ID)
		return "✅ 已解绑所有接口 ID", true, nil
	}

	target := parts[1]
	newList, removed := removeInterfaceID(current, target)
	if !removed {
		return fmt.Sprintf("ℹ️ 未找到接口 ID: %s", target), true, nil
	}

	settings.InterfaceIDs = newList

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to unbind interface ID: chat_id=%d, interface_id=%s, err=%v", msg.Chat.ID, target, err)
		return "❌ 解绑失败，请稍后重试", true, nil
	}

	logger.L().Infof("Interface ID unbound: chat_id=%d, interface_id=%s, operator=%d", msg.Chat.ID, target, msg.From.ID)
	return fmt.Sprintf("✅ 已解绑接口 ID: %s", target), true, nil
}

func (f *Feature) handleQuery(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	if len(group.Settings.InterfaceIDs) == 0 {
		return "ℹ️ 当前群组未绑定接口 ID\n\n使用「绑定接口 [接口ID]」进行绑定", true, nil
	}

	builder := strings.Builder{}
	builder.WriteString("✅ 当前绑定接口 ID：\n")
	for _, id := range group.Settings.InterfaceIDs {
		builder.WriteString(fmt.Sprintf("• %s\n", id))
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

func containsInterfaceID(ids []string, target string) bool {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	for _, id := range ids {
		if strings.ToLower(id) == targetLower {
			return true
		}
	}
	return false
}

func removeInterfaceID(ids []string, target string) ([]string, bool) {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	if targetLower == "" {
		return ids, false
	}

	result := make([]string, 0, len(ids))
	removed := false
	for _, id := range ids {
		if !removed && strings.ToLower(id) == targetLower {
			removed = true
			continue
		}
		result = append(result, id)
	}
	if !removed {
		return ids, false
	}
	return result, true
}
