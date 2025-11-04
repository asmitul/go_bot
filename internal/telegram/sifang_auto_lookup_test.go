package telegram

import (
	"strings"
	"testing"

	paymentservice "go_bot/internal/payment/service"
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

	result := formatLookupSuccess("M-123", detail)

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

	result := formatLookupSuccess("M-456", detail)

	if strings.Contains(result, "通知失败详情") {
		t.Fatalf("did not expect failure section in result: %s", result)
	}
}
