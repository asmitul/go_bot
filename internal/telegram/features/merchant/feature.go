package merchant

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/features/types"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

// Feature 商户号绑定功能
type Feature struct {
	groupService service.GroupService
	userService  service.UserService
}

// New 创建商户号绑定功能实例
func New(groupService service.GroupService, userService service.UserService) *Feature {
	return &Feature{
		groupService: groupService,
		userService:  userService,
	}
}

// Name 返回功能名称
func (f *Feature) Name() string {
	return "merchant"
}

// AllowedGroupTiers 指定允许操作商户号的群等级
func (f *Feature) AllowedGroupTiers() []models.GroupTier {
	return []models.GroupTier{
		models.GroupTierBasic,
		models.GroupTierMerchant,
	}
}

// Enabled 检查功能是否启用
// 商户号绑定功能始终启用（不需要群组配置开关）
func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
	return true
}

// Match 检查消息是否匹配商户号命令
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Text == "" {
		return false
	}

	// 匹配: "绑定 123456", "解绑", "商户号", "绑定状态"
	pattern := `^(绑定\s+\d+|解绑|商户号|绑定状态)$`
	matched, _ := regexp.MatchString(pattern, strings.TrimSpace(msg.Text))
	return matched
}

// Process 处理商户号命令
func (f *Feature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (*types.Response, bool, error) {
	// 权限检查: 仅 Admin+ 可操作
	isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
	if err != nil {
		logger.L().Errorf("Failed to check admin permission: user_id=%d, err=%v", msg.From.ID, err)
		return resp("❌ 权限检查失败"), true, nil
	}

	if !isAdmin {
		logger.L().Warnf("Unauthorized merchant operation attempt: user_id=%d, chat_id=%d", msg.From.ID, msg.Chat.ID)
		return resp("❌ 仅管理员可以操作商户号绑定"), true, nil
	}

	text := strings.TrimSpace(msg.Text)

	// 绑定命令
	if strings.HasPrefix(text, "绑定 ") {
		respText, handled, err := f.handleBind(ctx, msg, text)
		return resp(respText), handled, err
	}

	// 解绑命令
	if text == "解绑" {
		respText, handled, err := f.handleUnbind(ctx, msg)
		return resp(respText), handled, err
	}

	// 查询命令
	if text == "商户号" || text == "绑定状态" {
		respText, handled, err := f.handleQuery(ctx, msg)
		return resp(respText), handled, err
	}

	return nil, false, nil
}

// Priority 返回功能优先级 (1-100，数字越小优先级越高)
// 商户号绑定属于高优先级命令，设置为 15
func (f *Feature) Priority() int {
	return 15
}

// handleBind 处理绑定命令
func (f *Feature) handleBind(ctx context.Context, msg *botModels.Message, text string) (string, bool, error) {
	// 提取商户号
	parts := strings.Fields(text)
	if len(parts) != 2 {
		return "❌ 绑定格式错误，请使用: 绑定 [商户号]\n例如: 绑定 2025100", true, nil
	}

	merchantIDStr := parts[1]

	// 验证商户号格式 (纯数字)
	if !regexp.MustCompile(`^\d+$`).MatchString(merchantIDStr) {
		return "❌ 商户号必须为纯数字", true, nil
	}

	// 解析为数字
	merchantID, err := strconv.Atoi(merchantIDStr)
	if err != nil || merchantID <= 0 {
		return "❌ 商户号格式错误", true, nil
	}

	// 获取当前群组信息
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	if len(group.Settings.InterfaceIDs) > 0 {
		return "❌ 当前群组已绑定接口 ID，请先使用「解绑接口」解除全部接口后再操作商户号", true, nil
	}

	// 检查是否已绑定其他商户号
	if group.Settings.MerchantID != 0 && group.Settings.MerchantID != int32(merchantID) {
		return fmt.Sprintf("❌ 当前已绑定商户号: %d\n请先使用「解绑」命令解绑后再绑定新的商户号", group.Settings.MerchantID), true, nil
	}

	// 检查是否已绑定相同商户号
	if group.Settings.MerchantID == int32(merchantID) {
		return fmt.Sprintf("✅ 当前群组已绑定商户号: %d", merchantID), true, nil
	}

	// 执行绑定
	settings := group.Settings
	settings.MerchantID = int32(merchantID)
	settings.InterfaceIDs = nil

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to bind merchant ID: chat_id=%d, merchant_id=%d, err=%v", msg.Chat.ID, merchantID, err)
		return "❌ 绑定失败，请稍后重试", true, nil
	}

	logger.L().Infof("Merchant ID bound: chat_id=%d, merchant_id=%d, operator=%d", msg.Chat.ID, merchantID, msg.From.ID)
	return fmt.Sprintf("✅ 商户号绑定成功: %d", merchantID), true, nil
}

// handleUnbind 处理解绑命令
func (f *Feature) handleUnbind(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	// 获取当前群组信息
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	// 检查是否已绑定
	if group.Settings.MerchantID == 0 {
		return "ℹ️ 当前群组未绑定任何商户号", true, nil
	}

	oldMerchantID := group.Settings.MerchantID

	// 执行解绑
	settings := group.Settings
	settings.MerchantID = 0

	if err := f.groupService.UpdateGroupSettings(ctx, msg.Chat.ID, settings); err != nil {
		logger.L().Errorf("Failed to unbind merchant ID: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 解绑失败，请稍后重试", true, nil
	}

	logger.L().Infof("Merchant ID unbound: chat_id=%d, old_merchant_id=%d, operator=%d", msg.Chat.ID, oldMerchantID, msg.From.ID)
	return fmt.Sprintf("✅ 已解绑商户号: %d", oldMerchantID), true, nil
}

// handleQuery 处理查询命令
func (f *Feature) handleQuery(ctx context.Context, msg *botModels.Message) (string, bool, error) {
	// 获取当前群组信息
	group, err := f.groupService.GetGroupInfo(ctx, msg.Chat.ID)
	if err != nil {
		logger.L().Errorf("Failed to get group info: chat_id=%d, err=%v", msg.Chat.ID, err)
		return "❌ 获取群组信息失败", true, nil
	}

	// 返回绑定状态
	if group.Settings.MerchantID == 0 {
		return "ℹ️ 当前群组未绑定商户号\n\n使用「绑定 [商户号]」进行绑定\n例如: 绑定 2025100", true, nil
	}

	return fmt.Sprintf("✅ 当前绑定商户号: %d\n\n使用「解绑」可以解除绑定", group.Settings.MerchantID), true, nil
}

func resp(text string) *types.Response {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return &types.Response{Text: text}
}
