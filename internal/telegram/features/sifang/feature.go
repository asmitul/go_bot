package sifang

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go_bot/internal/logger"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
)

// Feature 四方支付功能
type Feature struct {
	paymentService paymentservice.Service
}

// New 创建四方支付功能实例
func New(paymentSvc paymentservice.Service) *Feature {
	return &Feature{
		paymentService: paymentSvc,
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
func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return false
	}

	return text == "余额"
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

	if strings.TrimSpace(msg.Text) != "余额" {
		return "", false, nil
	}

	return f.handleBalance(ctx, merchantID)
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
	// sb.WriteString("🏦 四方支付余额\n")
	// sb.WriteString(fmt.Sprintf("商户号：%s\n", merchant))
	// sb.WriteString(fmt.Sprintf("可用余额：%s\n", emptyFallback(balance.Balance, "未知")))
	// sb.WriteString(fmt.Sprintf("待提现：%s\n", emptyFallback(balance.PendingWithdraw, "0")))
	// if balance.Currency != "" {
	// 	sb.WriteString(fmt.Sprintf("币种：%s\n", balance.Currency))
	// }
	// if balance.UpdatedAt != "" {
	// 	sb.WriteString(fmt.Sprintf("更新时间：%s\n", balance.UpdatedAt))
	// }

	sb.WriteString(fmt.Sprintf("%s", emptyFallback(balance.Balance, "未知")))

	logger.L().Infof("Sifang balance queried: merchant_id=%s", merchant)
	return sb.String(), true, nil
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
