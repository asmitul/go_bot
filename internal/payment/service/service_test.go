package service

import (
	"encoding/json"
	"testing"
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
