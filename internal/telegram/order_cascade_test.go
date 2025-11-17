package telegram

import (
	"strings"
	"testing"
	"time"

	botModels "github.com/go-telegram/bot/models"
)

func TestBuildOrderCascadeMessageIncludesFields(t *testing.T) {
	payload := orderCascadeMessagePayload{
		MerchantOrderNoFull: "FULL-123",
		OrderNo:             "ORD-1",
		StatusText:          "未支付",
	}

	msg := buildOrderCascadeMessage(payload)
	if !strings.Contains(msg, "订单号：<code>FULL-123</code>") {
		t.Fatalf("expected order number, got %s", msg)
	}
	if !strings.Contains(msg, "订单状态：未支付") {
		t.Fatalf("expected status, got %s", msg)
	}
	if !strings.Contains(msg, "🤖 Bot 自动转单") {
		t.Fatalf("expected bot signature, got %s", msg)
	}
}

func TestBuildOrderCascadeFeedbackMessage(t *testing.T) {
	state := &orderCascadeState{
		SourceGroupTitle:   "商户群",
		UpstreamGroupTitle: "上游群",
		InterfaceID:        "123",
		InterfaceName:      "接口X",
		OrderNo:            "ORD-2",
		MerchantOrderFull:  "FULL-2",
		ChannelName:        "USDT",
	}
	user := &botModels.User{Username: "tester"}
	when := time.Date(2024, 11, 20, 10, 30, 0, 0, time.UTC)

	text := buildOrderCascadeFeedbackMessage(state, orderCascadeActionManual, user, when)
	if text != "🛠 人工处理" {
		t.Fatalf("unexpected feedback text: %s", text)
	}
}
