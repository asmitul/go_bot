package crypto

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go_bot/internal/logger"
	"go_bot/internal/telegram/models"

	botModels "github.com/go-telegram/bot/models"
)

const (
	// DefaultFloatRate 默认浮动费率
	DefaultFloatRate = 0.12
)

// CryptoFeature 加密货币价格查询功能
type CryptoFeature struct{}

// New 创建加密货币价格查询功能实例
func New() *CryptoFeature {
	return &CryptoFeature{}
}

// Name 返回功能名称
func (f *CryptoFeature) Name() string {
	return "crypto"
}

// Enabled 检查功能是否启用
func (f *CryptoFeature) Enabled(ctx context.Context, group *models.Group) bool {
	return group.Settings.CryptoEnabled
}

// Match 检查消息是否匹配（只处理群组中的特定命令）
func (f *CryptoFeature) Match(ctx context.Context, msg *botModels.Message) bool {
	// 只处理群组消息
	if msg.Chat.Type != "group" && msg.Chat.Type != "supergroup" {
		return false
	}

	// 检查是否匹配命令格式
	_, err := ParseCommand(msg.Text)
	return err == nil
}

// Process 处理价格查询请求
func (f *CryptoFeature) Process(ctx context.Context, msg *botModels.Message, group *models.Group) (string, bool, error) {
	// 解析命令
	cmdInfo, err := ParseCommand(msg.Text)
	if err != nil {
		logger.L().Warnf("Crypto command parse failed: chat_id=%d, text=%s, error=%v", msg.Chat.ID, msg.Text, err)
		return "❌ 命令格式错误", true, nil
	}

	// 从 OKX 获取订单列表
	orders, err := FetchC2COrders(ctx, cmdInfo.PaymentMethod)
	if err != nil {
		logger.L().Errorf("Failed to fetch OKX orders: payment_method=%s, error=%v", cmdInfo.PaymentMethod, err)
		return "❌ 获取价格失败，请稍后重试", true, nil
	}

	// 检查订单数量
	if len(orders) == 0 {
		return "❌ 暂无可用订单", true, nil
	}

	// 检查序号是否超出范围
	if cmdInfo.SerialNum > len(orders) {
		return fmt.Sprintf("❌ 商家序号超出范围（最多 %d 个）", len(orders)), true, nil
	}

	// 获取选中的订单（序号从 1 开始，数组从 0 开始）
	selectedOrder := orders[cmdInfo.SerialNum-1]
	selectedPrice, err := strconv.ParseFloat(selectedOrder.Price, 64)
	if err != nil {
		logger.L().Errorf("Failed to parse selected price: price=%s, error=%v", selectedOrder.Price, err)
		return "❌ 价格解析失败", true, nil
	}

	// 从群组配置读取浮动费率
	floatRate := group.Settings.CryptoFloatRate

	// 计算最终价格
	finalPrice := selectedPrice + floatRate

	// 构建响应消息（使用 HTML 格式）
	var response strings.Builder
	response.WriteString("<b>OTC商家实时价格</b>\n\n")
	response.WriteString(fmt.Sprintf("信息来源: 欧易 <b>%s</b>\n\n", cmdInfo.PaymentMethodName))

	// 显示订单列表（最多 10 个）
	maxDisplay := 10
	if len(orders) < maxDisplay {
		maxDisplay = len(orders)
	}

	for i := 0; i < maxDisplay; i++ {
		order := orders[i]
		price, _ := strconv.ParseFloat(order.Price, 64)

		// 如果是选中的订单，高亮显示
		if i == cmdInfo.SerialNum-1 {
			// 根据浮动费率决定显示格式
			if floatRate > 0 {
				// 有浮动：显示完整格式
				response.WriteString(fmt.Sprintf("✅<b>%.2f        %s</b>___➕<b>%.2f</b>🟰<code>%.2f</code>⬅️\n",
					price, order.NickName, floatRate, finalPrice))
			} else {
				// 无浮动：不显示加号部分
				response.WriteString(fmt.Sprintf("✅<b>%.2f        %s</b> 🟰 <code>%.2f</code>⬅️\n",
					price, order.NickName, finalPrice))
			}
		} else {
			response.WriteString(fmt.Sprintf("     <code>%.2f   %s</code>\n", price, order.NickName))
		}
	}

	// 如果提供了金额，计算总价
	if cmdInfo.HasAmount {
		totalPrice := finalPrice * cmdInfo.Amount
		response.WriteString(fmt.Sprintf("\n<code>%.2f</code> ✖️ <code>%.0f</code> <b>U</b> 🟰 <code>%.2f</code> <b>¥</b>",
			finalPrice, cmdInfo.Amount, totalPrice))
	}

	logger.L().Infof("Crypto query: chat_id=%d, payment=%s, serial=%d, amount=%.0f, price=%.2f",
		msg.Chat.ID, cmdInfo.PaymentMethod, cmdInfo.SerialNum, cmdInfo.Amount, finalPrice)

	return response.String(), true, nil
}

// Priority 返回优先级（30 = 中优先级）
func (f *CryptoFeature) Priority() int {
	return 30
}
