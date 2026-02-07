package telegram

import (
	"strings"
	"testing"

	paymentservice "go_bot/internal/payment/service"

	botModels "github.com/go-telegram/bot/models"
)

func TestFormatLookupSuccess_WithNotifyFailure(t *testing.T) {
	detail := &paymentservice.OrderDetail{
		Order: &paymentservice.Order{
			Status:           "paid",
			StatusText:       "已支付",
			NotifyStatus:     "failed",
			NotifyStatusText: "通知失败",
			NotifyLastError:  "HTTP 500 Internal Server Error",
			PlatformOrderNo:  "PF-123",
		},
		NotifyLogs: []*paymentservice.NotifyLog{
			{
				Status:      "failed",
				StatusText:  "失败",
				URL:         "https://callback.example.com/v1/notify",
				Response:    "{\"code\":500,\"msg\":\"error\"}",
				AttemptedAt: "2024-03-18 12:00:00",
			},
		},
	}

	result := formatLookupSuccess(2023111, "M-123", detail)

	if !strings.Contains(result, "<code>2023111M-123</code>") {
		t.Fatalf("expected merchant-prefixed order number in result: %s", result)
	}

	if !strings.Contains(result, "<b>通知失败详情</b>") {
		t.Fatalf("expected failure section in result: %s", result)
	}
	if !strings.Contains(result, "最后错误：HTTP 500 Internal Server Error") {
		t.Fatalf("expected last error in result: %s", result)
	}
	if !strings.Contains(result, "响应：{&#34;code&#34;:500,&#34;msg&#34;:&#34;error&#34;}") {
		t.Fatalf("expected response snippet in result: %s", result)
	}
}

func TestFormatLookupSuccess_WithoutNotifyFailure(t *testing.T) {
	detail := &paymentservice.OrderDetail{
		Order: &paymentservice.Order{
			Status:           "paid",
			StatusText:       "已支付",
			NotifyStatus:     "success",
			NotifyStatusText: "通知成功",
			PlatformOrderNo:  "PF-456",
		},
		NotifyLogs: []*paymentservice.NotifyLog{
			{
				Status:      "success",
				StatusText:  "成功",
				AttemptedAt: "2024-03-18 12:00:00",
			},
		},
	}

	result := formatLookupSuccess(2023111, "M-456", detail)

	if !strings.Contains(result, "<code>2023111M-456</code>") {
		t.Fatalf("expected merchant-prefixed order number in result: %s", result)
	}

	if strings.Contains(result, "通知失败详情") {
		t.Fatalf("did not expect failure section in result: %s", result)
	}
}

func TestBuildLookupCopyKeyboard(t *testing.T) {
	markup := buildLookupCopyKeyboard([]string{"2023111P-1", "2023111P-2"})
	keyboard, ok := markup.(*botModels.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected inline keyboard markup, got %T", markup)
	}

	if len(keyboard.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 keyboard rows, got %d", len(keyboard.InlineKeyboard))
	}

	if got := keyboard.InlineKeyboard[0][0].CopyText.Text; got != "2023111P-1" {
		t.Fatalf("unexpected first copy text: %s", got)
	}
	if got := keyboard.InlineKeyboard[0][0].Text; got != "复制订单号 1" {
		t.Fatalf("unexpected first button label: %s", got)
	}
	if got := keyboard.InlineKeyboard[1][0].CopyText.Text; got != "2023111P-2" {
		t.Fatalf("unexpected second copy text: %s", got)
	}
}

func TestBuildLookupCopyKeyboard_SingleOrder(t *testing.T) {
	markup := buildLookupCopyKeyboard([]string{"2023111P-single"})
	keyboard, ok := markup.(*botModels.InlineKeyboardMarkup)
	if !ok {
		t.Fatalf("expected inline keyboard markup, got %T", markup)
	}

	if got := keyboard.InlineKeyboard[0][0].Text; got != "点击复制订单号" {
		t.Fatalf("unexpected single button label: %s", got)
	}
}
