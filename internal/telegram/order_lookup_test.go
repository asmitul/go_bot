package telegram

import (
	"testing"

	paymentservice "go_bot/internal/payment/service"
)

func TestBuildAutoOrderMessage_NotFound(t *testing.T) {
	merchantID := int64(2024164)
	original := "210127021330008786236"
	composed := "2024164210127021330008786236"

	got := buildAutoOrderMessage(merchantID, original, composed, nil)
	want := "📦 <b>订单自动查询</b>\n" +
		"商户号：<code>2024164</code>\n" +
		"检测到订单号：<code>210127021330008786236</code>\n" +
		"查询订单号：<code>2024164210127021330008786236</code>\n" +
		"❌ 未查询到相关订单，请核对后重试。"

	if got != want {
		t.Fatalf("unexpected message when not found:\nwant:\n%s\n---\ngot:\n%s", want, got)
	}
}

func TestBuildAutoOrderMessage_WithOrder(t *testing.T) {
	merchantID := int64(2024164)
	original := "210127021330008786236"
	composed := "2024164210127021330008786236"
	order := &paymentservice.Order{
		MerchantOrderNo: "2024164210127021330008786236",
		PlatformOrderNo: "P1234567890",
		Amount:          "100.00",
		RealAmount:      "98.88",
		StatusText:      "已支付",
		PayStatus:       "SUCCESS",
		NotifyStatus:    "SENT",
		Channel:         "USDT",
		CreatedAt:       "2025-11-03 02:15:00",
		PaidAt:          "2025-11-03 02:16:00",
	}

	got := buildAutoOrderMessage(merchantID, original, composed, order)
	want := "📦 <b>订单自动查询</b>\n" +
		"商户号：<code>2024164</code>\n" +
		"检测到订单号：<code>210127021330008786236</code>\n" +
		"查询订单号：<code>2024164210127021330008786236</code>\n" +
		"平台订单号：<code>P1234567890</code>\n" +
		"金额：<code>100.00</code>\n" +
		"实收金额：<code>98.88</code>\n" +
		"状态：<b>已支付</b>\n" +
		"支付状态：<code>SUCCESS</code>\n" +
		"通知状态：<code>SENT</code>\n" +
		"通道：<code>USDT</code>\n" +
		"创建时间：<code>2025-11-03 02:15:00</code>\n" +
		"支付时间：<code>2025-11-03 02:16:00</code>"

	if got != want {
		t.Fatalf("unexpected message when order found:\nwant:\n%s\n---\ngot:\n%s", want, got)
	}
}
