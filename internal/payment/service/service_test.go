package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_bot/internal/config"
	"go_bot/internal/payment/sifang"
)

func TestDecodeBalance(t *testing.T) {
	raw := map[string]interface{}{
		"merchant_id":      "1001",
		"balance":          123.45,
		"pending_withdraw": "10.00",
		"currency":         "CNY",
		"updated_at":       "2024-01-01 12:00:00",
		"history_days":     7,
		"history_balance":  "100.00",
	}

	b := decodeBalance(raw)
	if b.MerchantID != "1001" || b.Balance != "123.45" || b.Currency != "CNY" {
		t.Fatalf("unexpected balance decode: %#v", b)
	}
	if b.HistoryDays != 7 || b.HistoryBalance != "100.00" {
		t.Fatalf("unexpected history fields: %#v", b)
	}
}

func TestDecodeSummaryByDay_List(t *testing.T) {
	payload := map[string]interface{}{
		"list": []map[string]interface{}{
			{
				"day":             "2024-10-26",
				"order_num":       12,
				"success_num":     11,
				"gross_amount":    "1000.50",
				"merchant_amount": "950.40",
				"agent_profit":    "50.10",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	summary, err := decodeSummaryByDay(data)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}

	if summary.Date != "2024-10-26" {
		t.Fatalf("unexpected date: %s", summary.Date)
	}
	if summary.OrderCount != "12" || summary.SuccessCount != "11" {
		t.Fatalf("unexpected counts: %#v", summary)
	}
	if summary.TotalAmount != "1000.50" || summary.MerchantIncome != "950.40" || summary.AgentIncome != "50.10" {
		t.Fatalf("unexpected amounts: %#v", summary)
	}
}

func TestDecodeSummaryByDay_SummaryObject(t *testing.T) {
	payload := map[string]interface{}{
		"summary": map[string]interface{}{
			"summary_date":    "2024-10-25",
			"order_total":     "8",
			"success_total":   "7",
			"total_money":     888.0,
			"merchant_income": "800.00",
			"share_profit":    "88.00",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	summary, err := decodeSummaryByDay(data)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}

	if summary.Date != "2024-10-25" {
		t.Fatalf("unexpected date: %s", summary.Date)
	}
	if summary.OrderCount != "8" || summary.SuccessCount != "7" {
		t.Fatalf("unexpected counts: %#v", summary)
	}
	if summary.TotalAmount != "888" || summary.MerchantIncome != "800.00" || summary.AgentIncome != "88.00" {
		t.Fatalf("unexpected amounts: %#v", summary)
	}
}

func TestDecodeSummaryByDay_Empty(t *testing.T) {
	summary, err := decodeSummaryByDay([]byte("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != nil {
		t.Fatalf("expected nil summary, got %#v", summary)
	}
}

func TestDecodeSummaryByDay_DynamicKeys(t *testing.T) {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"2025-10-31": map[string]interface{}{
				"count":               15,
				"success_count":       12,
				"total_order_money":   "12345.67",
				"merchant_real_money": "11000.00",
				"agent_commission":    "1345.67",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	summary, err := decodeSummaryByDay(data)
	if err != nil {
		t.Fatalf("decode summary: %v", err)
	}

	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}

	if summary.Date != "2025-10-31" {
		t.Fatalf("unexpected date: %s", summary.Date)
	}
	if summary.OrderCount != "15" || summary.SuccessCount != "12" {
		t.Fatalf("unexpected counts: %#v", summary)
	}
	if summary.TotalAmount != "12345.67" || summary.MerchantIncome != "11000.00" || summary.AgentIncome != "1345.67" {
		t.Fatalf("unexpected amounts: %#v", summary)
	}
}

func TestDecodeSummaryByDayChannel_Items(t *testing.T) {
	payload := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"date":            "2025-10-30",
				"channel_code":    "USDT",
				"channel_name":    "USDT通道",
				"order_count":     10,
				"success_count":   9,
				"gross_amount":    "5000.00",
				"merchant_income": "4500.00",
				"agent_income":    "200.00",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	items, err := decodeSummaryByDayChannel(data)
	if err != nil {
		t.Fatalf("decode channel summary: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Date != "2025-10-30" || item.ChannelCode != "USDT" || item.ChannelName != "USDT通道" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.TotalAmount != "5000.00" || item.MerchantIncome != "4500.00" || item.AgentIncome != "200.00" {
		t.Fatalf("unexpected amounts: %#v", item)
	}
}

func TestDecodeSummaryByDayChannel_DynamicKeys(t *testing.T) {
	payload := map[string]interface{}{
		"2025-10-30": []map[string]interface{}{
			{
				"code":              "ALIPAY",
				"channel_name":      "支付宝",
				"count":             "5",
				"success_total":     "4",
				"total_order_money": "1234.56",
				"merchant_profit":   "1200.00",
				"agent_commission":  "34.56",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	items, err := decodeSummaryByDayChannel(data)
	if err != nil {
		t.Fatalf("decode channel summary: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	item := items[0]
	if item.Date != "2025-10-30" || item.ChannelCode != "ALIPAY" || item.ChannelName != "支付宝" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.TotalAmount != "1234.56" || item.MerchantIncome != "1200.00" || item.AgentIncome != "34.56" {
		t.Fatalf("unexpected amounts: %#v", item)
	}
}

func TestDecodeSummaryByPZID_Items(t *testing.T) {
	payload := map[string]interface{}{
		"pzid":       "1024",
		"start_date": "2024-10-20",
		"end_date":   "2024-10-26",
		"items": []map[string]interface{}{
			{
				"date":            "2024-10-26",
				"order_count":     25,
				"gross_amount":    "10000.00",
				"merchant_income": "9800.00",
				"agent_income":    "200.00",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	summary, err := decodeSummaryByPZID(data)
	if err != nil {
		t.Fatalf("decode pzid summary: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}
	if summary.PZID != "1024" || summary.StartDate != "2024-10-20" || summary.EndDate != "2024-10-26" {
		t.Fatalf("unexpected meta: %#v", summary)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Date != "2024-10-26" || item.OrderCount != "25" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.GrossAmount != "10000.00" || item.MerchantIncome != "9800.00" || item.AgentIncome != "200.00" {
		t.Fatalf("unexpected amounts: %#v", item)
	}
}

func TestDecodeSummaryByPZID_DynamicKeys(t *testing.T) {
	payload := map[string]interface{}{
		"result": map[string]interface{}{
			"2024-11-01": map[string]interface{}{
				"count":            10,
				"sum_amount":       "1234.56",
				"merchant_real":    "1100.00",
				"agent_profit":     "134.56",
				"success_count":    9,
				"success_amount":   "1234.56",
				"merchant_income":  "1100.00",
				"agent_income":     "134.56",
				"gross_amount":     "1234.56",
				"merchant_amount":  "1100.00",
				"agent_commission": "134.56",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	summary, err := decodeSummaryByPZID(data)
	if err != nil {
		t.Fatalf("decode pzid summary: %v", err)
	}
	if summary == nil {
		t.Fatalf("expected summary, got nil")
	}
	if len(summary.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(summary.Items))
	}
	item := summary.Items[0]
	if item.Date != "2024-11-01" || item.OrderCount != "10" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if item.GrossAmount != "1234.56" || item.MerchantIncome != "1100.00" || item.AgentIncome != "134.56" {
		t.Fatalf("unexpected amounts: %#v", item)
	}
}

func TestDecodeOrderDetail(t *testing.T) {
	raw := map[string]interface{}{
		"order": map[string]interface{}{
			"merchant_order_no": "M1001",
			"platform_order_no": "P2002",
			"amount":            "100.00",
			"status":            "1",
			"custom_field":      "custom-value",
		},
		"extended": map[string]interface{}{
			"order_id":    "OID-1",
			"channel_fee": "1.23",
			"risk_flag":   1,
		},
		"notify_logs": []map[string]interface{}{
			{
				"status":      "success",
				"notify_url":  "https://callback",
				"notify_time": "2024-10-26 12:00:00",
			},
		},
	}

	detail := decodeOrderDetail(raw)
	if detail == nil {
		t.Fatalf("expected detail, got nil")
	}

	if detail.Order == nil {
		t.Fatalf("expected order, got nil")
	}

	if detail.Order.MerchantOrderNo != "M1001" || detail.Order.PlatformOrderNo != "P2002" {
		t.Fatalf("unexpected order numbers: %#v", detail.Order)
	}

	if detail.Order.Extra == nil || detail.Order.Extra["custom_field"] != "custom-value" {
		t.Fatalf("expected extra field, got %#v", detail.Order.Extra)
	}

	if detail.Extended == nil || detail.Extended.OrderID != "OID-1" || detail.Extended.ChannelFee != "1.23" || !detail.Extended.RiskFlag {
		t.Fatalf("unexpected extended: %#v", detail.Extended)
	}

	if len(detail.NotifyLogs) != 1 {
		t.Fatalf("expected 1 notify log, got %d", len(detail.NotifyLogs))
	}
}

func TestDecodeOrderDetail_MapNotifyLogs(t *testing.T) {
	raw := map[string]interface{}{
		"notify_logs": map[string]interface{}{
			"1": map[string]interface{}{
				"result": "ok",
				"time":   "2024-10-26 12:00:00",
			},
			"2": map[string]interface{}{
				"result": "fail",
				"time":   "2024-10-26 12:05:00",
			},
		},
	}

	detail := decodeOrderDetail(raw)
	if detail == nil {
		t.Fatalf("expected detail, got nil")
	}
	if detail.Order != nil {
		t.Fatalf("expected no order, got %#v", detail.Order)
	}
	if len(detail.NotifyLogs) != 2 {
		t.Fatalf("expected 2 notify logs, got %d", len(detail.NotifyLogs))
	}
}

func TestDecodeOrderDetail_Empty(t *testing.T) {
	if detail := decodeOrderDetail(map[string]interface{}{}); detail != nil {
		t.Fatalf("expected nil detail, got %#v", detail)
	}
}

func TestSifangService_GetOrderDetail_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("merchant_order_no"); got != "MER-1" {
			t.Fatalf("unexpected merchant order no: %s", got)
		}

		fmt.Fprintf(w, `{"code":0,"message":"ok","data":{"order":{"merchant_order_no":"MER-1","platform_order_no":"PF-9","amount":"10.00","status":"1"},"extended":{"order_id":"OID-9","channel_fee":"0.50"},"notify_logs":[{"status":"success","notify_url":"https://callback","notify_time":"2024-10-26 12:00:00"}]}}`)
	}))
	defer ts.Close()

	cfg := config.SifangConfig{
		BaseURL:            ts.URL,
		DefaultMerchantKey: "secret",
		Timeout:            2 * time.Second,
	}
	client, err := sifang.NewClient(cfg, sifang.WithHTTPClient(ts.Client()), sifang.WithNowFunc(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewSifangService(client)
	detail, err := svc.GetOrderDetail(context.Background(), 1001, "MER-1", OrderNumberTypeMerchant)
	if err != nil {
		t.Fatalf("GetOrderDetail returned error: %v", err)
	}

	if detail.Order == nil || detail.Order.PlatformOrderNo != "PF-9" {
		t.Fatalf("unexpected order detail: %#v", detail.Order)
	}

	if detail.Extended == nil || detail.Extended.ChannelFee != "0.50" {
		t.Fatalf("unexpected extended: %#v", detail.Extended)
	}
}

func TestSifangService_GetOrderDetail_Fallback(t *testing.T) {
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}

		if r.Form.Get("merchant_order_no") != "" {
			fmt.Fprintf(w, `{"code":404,"message":"not found","data":null}`)
			return
		}

		if r.Form.Get("platform_order_no") != "PLAT-1" {
			t.Fatalf("unexpected platform order number: %s", r.Form.Get("platform_order_no"))
		}

		fmt.Fprintf(w, `{"code":0,"message":"ok","data":{"order":{"platform_order_no":"PLAT-1","status":"1"}}}`)
	}))
	defer ts.Close()

	cfg := config.SifangConfig{
		BaseURL:            ts.URL,
		DefaultMerchantKey: "secret",
		Timeout:            2 * time.Second,
	}
	client, err := sifang.NewClient(cfg, sifang.WithHTTPClient(ts.Client()), sifang.WithNowFunc(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewSifangService(client)
	detail, err := svc.GetOrderDetail(context.Background(), 1001, "PLAT-1", OrderNumberTypeAuto)
	if err != nil {
		t.Fatalf("GetOrderDetail returned error: %v", err)
	}

	if detail.Order == nil || detail.Order.PlatformOrderNo != "PLAT-1" {
		t.Fatalf("unexpected order: %#v", detail.Order)
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
}

func TestSifangService_GetOrderDetail_NoData(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"code":0,"message":"ok","data":{}}`)
	}))
	defer ts.Close()

	cfg := config.SifangConfig{
		BaseURL:            ts.URL,
		DefaultMerchantKey: "secret",
		Timeout:            2 * time.Second,
	}
	client, err := sifang.NewClient(cfg, sifang.WithHTTPClient(ts.Client()), sifang.WithNowFunc(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewSifangService(client)
	if _, err := svc.GetOrderDetail(context.Background(), 1001, "MER-1", OrderNumberTypeMerchant); err == nil {
		t.Fatalf("expected error for empty detail")
	}
}

func TestSifangService_GetOrderDetail_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"code":500,"message":"server error","data":null}`)
	}))
	defer ts.Close()

	cfg := config.SifangConfig{
		BaseURL:            ts.URL,
		DefaultMerchantKey: "secret",
		Timeout:            2 * time.Second,
	}
	client, err := sifang.NewClient(cfg, sifang.WithHTTPClient(ts.Client()), sifang.WithNowFunc(func() time.Time { return time.Unix(1700000000, 0) }))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	svc := NewSifangService(client)
	if _, err := svc.GetOrderDetail(context.Background(), 1001, "MER-1", OrderNumberTypeMerchant); err == nil {
		t.Fatalf("expected api error")
	}
}

func TestDecodeChannelStatus(t *testing.T) {
	payload := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"channel_code":     "cjwxhf",
				"channel_name":     "微信话费慢充",
				"system_enabled":   1,
				"merchant_enabled": "1",
				"rate":             "0.10",
				"min_amount":       "10",
				"max_amount":       "5000",
			},
			{
				"code":            "tbsqhf",
				"name":            "淘宝授权话费",
				"system_status":   true,
				"merchant_status": 0,
				"fee_rate":        "10%",
				"daily_quota":     "10000",
				"daily_used":      "2000",
				"last_used_at":    "2025-10-31 10:00:00",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	items, err := decodeChannelStatus(data)
	if err != nil {
		t.Fatalf("decode channel status: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	first := items[0]
	if first.ChannelCode != "cjwxhf" || first.ChannelName != "微信话费慢充" {
		t.Fatalf("unexpected first item: %#v", first)
	}
	if !first.SystemEnabled || !first.MerchantEnabled {
		t.Fatalf("expected first item enabled: %#v", first)
	}
	if first.Rate != "0.10" || first.MinAmount != "10" || first.MaxAmount != "5000" {
		t.Fatalf("unexpected first item amounts: %#v", first)
	}

	second := items[1]
	if second.ChannelCode != "tbsqhf" || second.ChannelName != "淘宝授权话费" {
		t.Fatalf("unexpected second item: %#v", second)
	}
	if !second.SystemEnabled || second.MerchantEnabled {
		t.Fatalf("unexpected second item enabled flags: %#v", second)
	}
	if second.Rate != "10%" || second.DailyQuota != "10000" || second.DailyUsed != "2000" {
		t.Fatalf("unexpected second item limits: %#v", second)
	}
	if second.LastUsedAt != "2025-10-31 10:00:00" {
		t.Fatalf("unexpected second item last used: %#v", second)
	}
}

func TestDecodeWithdrawList_Items(t *testing.T) {
	payload := map[string]interface{}{
		"page":      1,
		"page_size": 10,
		"total":     2,
		"items": []map[string]interface{}{
			{
				"withdraw_no": "W2025",
				"order_no":    "O1",
				"amount":      "100.00",
				"fee":         "1.00",
				"status":      "paid",
				"create_time": "2025-10-31 10:00:00",
				"pay_time":    "2025-10-31 11:00:00",
				"channel":     "ALIPAY",
			},
			{
				"id":                "W2024",
				"merchant_order_no": "O2",
				"withdraw_amount":   "200",
				"charge":            "2.00",
				"state":             "processing",
				"apply_time":        "2025-10-30 09:00:00",
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	list, err := decodeWithdrawList(data)
	if err != nil {
		t.Fatalf("decode withdraw list: %v", err)
	}

	if list.Page != 1 || list.PageSize != 10 || list.Total != 2 {
		t.Fatalf("unexpected pagination: %#v", list)
	}

	if len(list.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list.Items))
	}

	first := list.Items[0]
	if first.WithdrawNo != "W2025" || first.Amount != "100.00" || first.Status != "paid" {
		t.Fatalf("unexpected first item: %#v", first)
	}
	if first.Channel != "ALIPAY" || first.PaidAt != "2025-10-31 11:00:00" {
		t.Fatalf("unexpected first item channel/time: %#v", first)
	}

	second := list.Items[1]
	if second.WithdrawNo != "W2024" || second.Amount != "200" || second.Fee != "2.00" {
		t.Fatalf("unexpected second item: %#v", second)
	}
	if second.Status != "processing" || second.PaidAt != "" {
		t.Fatalf("unexpected second item status/time: %#v", second)
	}
}

func TestDecodeSendMoney(t *testing.T) {
	raw := map[string]interface{}{
		"merchant_id":      "1001",
		"balance_after":    "900.50",
		"pending_withdraw": "100.00",
		"frozen_today":     "20.00",
		"fee":              "1.00",
		"withdraw": map[string]interface{}{
			"withdraw_no": "W2025",
			"amount":      "100.00",
			"status":      "processing",
			"channel":     "ALIPAY",
		},
	}

	result := decodeSendMoney(raw)
	if result == nil {
		t.Fatalf("expected result, got nil")
	}
	if result.MerchantID != "1001" || result.BalanceAfter != "900.50" {
		t.Fatalf("unexpected basic fields: %#v", result)
	}
	if result.PendingWithdraw != "100.00" || result.FrozenToday != "20.00" || result.Fee != "1.00" {
		t.Fatalf("unexpected amount fields: %#v", result)
	}
	if result.Withdraw == nil || result.Withdraw.WithdrawNo != "W2025" || result.Withdraw.Channel != "ALIPAY" {
		t.Fatalf("unexpected withdraw: %#v", result.Withdraw)
	}
}

func TestDecodeSendMoney_Empty(t *testing.T) {
	if decodeSendMoney(map[string]interface{}{}) != nil {
		t.Fatalf("expected nil for empty map")
	}
}
