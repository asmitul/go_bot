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
	}

	b := decodeBalance(raw)
	if b.MerchantID != "1001" || b.Balance != "123.45" || b.Currency != "CNY" {
		t.Fatalf("unexpected balance decode: %#v", b)
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
