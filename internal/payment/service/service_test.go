package service

import (
	"reflect"
	"testing"
)

func TestOrdersFilterToParams(t *testing.T) {
	filter := OrdersFilter{
		Status:          "1",
		MerchantOrderNo: "M123",
		Page:            2,
		PageSize:        50,
	}

	params := filter.toParams()

	expected := map[string]string{
		"status":            "1",
		"merchant_order_no": "M123",
		"page":              "2",
		"page_size":         "50",
	}

	if !reflect.DeepEqual(params, expected) {
		t.Fatalf("unexpected params: %#v", params)
	}
}

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
