package telegram

import (
	"strings"
	"testing"
	"time"

	paymentservice "go_bot/internal/payment/service"

	botModels "github.com/go-telegram/bot/models"
)

func TestFindOrderCascadeStateByUpstreamMessage(t *testing.T) {
	now := time.Now()
	b := &Bot{
		orderCascadeStates: map[string]*orderCascadeState{
			"active": {
				Token:             "active",
				UpstreamChatID:    -10001,
				UpstreamMessageID: 101,
				MerchantChatID:    -20001,
				ExpiresAt:         now.Add(time.Hour),
			},
			"expired": {
				Token:             "expired",
				UpstreamChatID:    -10002,
				UpstreamMessageID: 202,
				MerchantChatID:    -20002,
				ExpiresAt:         now.Add(-time.Minute),
			},
		},
	}

	state, ok := b.findOrderCascadeStateByUpstreamMessage(-10001, 101)
	if !ok || state == nil {
		t.Fatal("expected active state to be found")
	}
	if state.Token != "active" {
		t.Fatalf("unexpected state token: got %s, want active", state.Token)
	}
	if _, exists := b.orderCascadeStates["expired"]; exists {
		t.Fatal("expected expired state to be cleaned up")
	}
}

func TestFindOrderCascadeStateByUpstreamMessageNotFound(t *testing.T) {
	b := &Bot{
		orderCascadeStates: map[string]*orderCascadeState{
			"active": {
				Token:             "active",
				UpstreamChatID:    -10001,
				UpstreamMessageID: 101,
				MerchantChatID:    -20001,
				ExpiresAt:         time.Now().Add(time.Hour),
			},
		},
	}

	state, ok := b.findOrderCascadeStateByUpstreamMessage(-10001, 999)
	if ok || state != nil {
		t.Fatalf("expected no state, got ok=%v state=%+v", ok, state)
	}
}

func TestIsOrderCascadeRelayContent(t *testing.T) {
	t.Run("text", func(t *testing.T) {
		msg := &botModels.Message{Text: "已处理"}
		if !isOrderCascadeRelayContent(msg) {
			t.Fatal("expected text message to be relayable")
		}
	})

	t.Run("photo", func(t *testing.T) {
		msg := &botModels.Message{Photo: []botModels.PhotoSize{{FileID: "photo-id"}}}
		if !isOrderCascadeRelayContent(msg) {
			t.Fatal("expected photo message to be relayable")
		}
	})

	t.Run("video", func(t *testing.T) {
		msg := &botModels.Message{Video: &botModels.Video{FileID: "video-id"}}
		if !isOrderCascadeRelayContent(msg) {
			t.Fatal("expected video message to be relayable")
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		msg := &botModels.Message{Document: &botModels.Document{FileID: "doc-id"}}
		if isOrderCascadeRelayContent(msg) {
			t.Fatal("expected document message to be non-relayable")
		}
	})
}

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
	user := &botModels.User{Username: "tester"}
	when := time.Date(2024, 11, 20, 10, 30, 0, 0, time.UTC)

	t.Run("reply mode", func(t *testing.T) {
		state := &orderCascadeState{
			MerchantReplyOn:    true,
			SourceGroupTitle:   "商户群",
			UpstreamGroupTitle: "上游群",
			InterfaceID:        "123",
			InterfaceName:      "接口X",
			OrderNo:            "ORD-2",
			MerchantOrderFull:  "FULL-2",
			ChannelName:        "USDT",
		}

		text := buildOrderCascadeFeedbackMessage(state, orderCascadeActionManual, user, when)
		if text != "🛠 人工处理" {
			t.Fatalf("unexpected feedback text: %s", text)
		}
	})

	t.Run("direct mode includes order info", func(t *testing.T) {
		state := &orderCascadeState{
			MerchantReplyOn:    false,
			SourceGroupTitle:   "商户群",
			UpstreamGroupTitle: "上游群",
			InterfaceID:        "123",
			InterfaceName:      "接口X",
			OrderNo:            "ORD-2",
			MerchantOrderFull:  "FULL-2",
			ChannelName:        "USDT",
		}

		text := buildOrderCascadeFeedbackMessage(state, orderCascadeActionManual, user, when)
		if !strings.Contains(text, "<pre><code>FULL-2</code></pre>") {
			t.Fatalf("expected order code block in feedback, got %s", text)
		}
		if !strings.Contains(text, "结果：🛠 人工处理") {
			t.Fatalf("expected action in feedback, got %s", text)
		}
		if strings.Contains(text, "接口：") || strings.Contains(text, "通道：") || strings.Contains(text, "反馈人：") || strings.Contains(text, "时间：") {
			t.Fatalf("expected compact feedback format, got %s", text)
		}
	})
}

func TestResolveCascadeMerchantOrderNo(t *testing.T) {
	t.Run("prefer merchant order no", func(t *testing.T) {
		binding := &paymentservice.OrderChannelBinding{
			MerchantOrderNo:     "UR863638992959049681",
			MerchantOrderNoFull: "2023173UR863638992959049681",
		}

		got := resolveCascadeMerchantOrderNo(2023173, binding, "fallback")
		if got != "UR863638992959049681" {
			t.Fatalf("expected raw merchant order no, got %s", got)
		}
	})

	t.Run("strip merchant prefix when only full available", func(t *testing.T) {
		binding := &paymentservice.OrderChannelBinding{
			MerchantOrderNoFull: "2023173UR863638992959049681",
		}

		got := resolveCascadeMerchantOrderNo(2023173, binding, "fallback")
		if got != "UR863638992959049681" {
			t.Fatalf("expected merchant prefix stripped, got %s", got)
		}
	})
}
