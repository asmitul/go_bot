package service

import (
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
