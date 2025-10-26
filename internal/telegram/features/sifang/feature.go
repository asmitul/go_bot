package sifang

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"
	"go_bot/internal/telegram/service"

	botModels "github.com/go-telegram/bot/models"
)

var (
	orderCmdRegex = regexp.MustCompile(`^四方订单(\s+\d+)?$`)
	statusMap     = map[string]string{
		"0": "未支付",
		"1": "成功",
		"2": "扣量",
	}
	notifyStatusMap = map[string]string{
		"0": "未回调",
		"1": "成功",
		"2": "失败",
	}
)

// Feature 四方支付功能
type Feature struct {
	paymentService paymentservice.Service
	userService    service.UserService
}

// New 创建四方支付功能实例
func New(paymentSvc paymentservice.Service, userSvc service.UserService) *Feature {
	return &Feature{
		paymentService: paymentSvc,
		userService:    userSvc,
	}
}

// Name 功能名称
func (f *Feature) Name() string {
	return "sifang_payment"
}

// Enabled 仅在群组启用且服务已配置时生效
func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
	return group.Settings.SifangEnabled
}

// Match 支持命令：
//   - 余额
//   - 四方订单 [页码]
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return false
	}

	if text == "余额" {
		return true
	}

	return orderCmdRegex.MatchString(text)
}

// Process 执行四方支付查询
func (f *Feature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (string, bool, error) {
	if f.paymentService == nil {
		return "❌ 未配置四方支付服务，请联系管理员", true, nil
	}

	if msg.From == nil {
		return "", false, nil
	}

	merchantID := int64(group.Settings.MerchantID)
	if merchantID == 0 {
		return "ℹ️ 当前群组未绑定商户号，请先使用「绑定 [商户号]」命令", true, nil
	}

	text := strings.TrimSpace(msg.Text)
	isBalanceCmd := text == "余额"

	if !isBalanceCmd {
		// 权限校验仅针对订单命令
		isAdmin, err := f.userService.CheckAdminPermission(ctx, msg.From.ID)
		if err != nil {
			logger.L().Errorf("Sifang feature admin check failed: user_id=%d, err=%v", msg.From.ID, err)
			return "❌ 权限检查失败，请稍后重试", true, nil
		}
		if !isAdmin {
			return "❌ 仅管理员可查询四方订单", true, nil
		}
	}

	switch {
	case isBalanceCmd:
		return f.handleBalance(ctx, merchantID)
	case orderCmdRegex.MatchString(text):
		page := parsePage(text)
		return f.handleOrders(ctx, merchantID, page)
	default:
		return "", false, nil
	}
}

// Priority 设置为 25，介于商户绑定与行情功能之间
func (f *Feature) Priority() int {
	return 25
}

func (f *Feature) handleBalance(ctx context.Context, merchantID int64) (string, bool, error) {
	balance, err := f.paymentService.GetBalance(ctx, merchantID)
	if err != nil {
		logger.L().Errorf("Sifang balance query failed: merchant_id=%d, err=%v", merchantID, err)
		return fmt.Sprintf("❌ 查询余额失败：%v", err), true, nil
	}

	merchant := balance.MerchantID
	if merchant == "" {
		merchant = strconv.FormatInt(merchantID, 10)
	}

	var sb strings.Builder
	sb.WriteString("🏦 四方支付余额\n")
	sb.WriteString(fmt.Sprintf("商户号：%s\n", merchant))
	sb.WriteString(fmt.Sprintf("可用余额：%s\n", emptyFallback(balance.Balance, "未知")))
	sb.WriteString(fmt.Sprintf("待提现：%s\n", emptyFallback(balance.PendingWithdraw, "0")))
	if balance.Currency != "" {
		sb.WriteString(fmt.Sprintf("币种：%s\n", balance.Currency))
	}
	if balance.UpdatedAt != "" {
		sb.WriteString(fmt.Sprintf("更新时间：%s\n", balance.UpdatedAt))
	}

	logger.L().Infof("Sifang balance queried: merchant_id=%s", merchant)
	return sb.String(), true, nil
}

func (f *Feature) handleOrders(ctx context.Context, merchantID int64, page int) (string, bool, error) {
	filter := paymentservice.OrdersFilter{
		Page:     page,
		PageSize: 5,
	}

	result, err := f.paymentService.ListOrders(ctx, merchantID, filter)
	if err != nil {
		logger.L().Errorf("Sifang orders query failed: merchant_id=%d, page=%d, err=%v", merchantID, page, err)
		return fmt.Sprintf("❌ 查询订单失败：%v", err), true, nil
	}

	if len(result.Items) == 0 {
		return fmt.Sprintf("ℹ️ 第 %d 页暂无订单记录", page), true, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📄 四方支付订单（第 %d 页）\n\n", page))

	for i, order := range result.Items {
		sb.WriteString(fmt.Sprintf("%d) 平台单号：%s\n", i+1, emptyFallback(order.PlatformOrderNo, "无")))
		if order.MerchantOrderNo != "" {
			sb.WriteString(fmt.Sprintf("   商户单号：%s\n", order.MerchantOrderNo))
		}
		sb.WriteString(fmt.Sprintf("   金额：%s\n", emptyFallback(order.Amount, "未知")))
		status := emptyFallback(statusMap[order.Status], order.Status)
		notify := emptyFallback(notifyStatusMap[order.NotifyStatus], order.NotifyStatus)
		sb.WriteString(fmt.Sprintf("   状态：%s | 回调：%s\n", status, notify))
		if order.ChannelCode != "" {
			sb.WriteString(fmt.Sprintf("   通道：%s\n", order.ChannelCode))
		}
		if order.PaidAt != "" {
			sb.WriteString(fmt.Sprintf("   支付时间：%s\n", order.PaidAt))
		}
		if order.CreatedAt != "" {
			sb.WriteString(fmt.Sprintf("   创建时间：%s\n", order.CreatedAt))
		}
		if i < len(result.Items)-1 {
			sb.WriteString("\n")
		}
	}

	if len(result.Summary) > 0 {
		sb.WriteString("\n📊 汇总：\n")
		for k, v := range result.Summary {
			sb.WriteString(fmt.Sprintf("   %s：%s\n", k, v))
		}
	}

	logger.L().Infof("Sifang orders queried: merchant_id=%d, page=%d, items=%d", merchantID, page, len(result.Items))
	return sb.String(), true, nil
}

func parsePage(text string) int {
	matches := orderCmdRegex.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 1
	}
	pageStr := strings.TrimSpace(matches[1])
	if pageStr == "" {
		return 1
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		return 1
	}
	return page
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
