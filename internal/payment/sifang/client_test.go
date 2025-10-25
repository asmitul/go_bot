package sifang

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"go_bot/internal/config"
)

func TestComputeSign(t *testing.T) {
	params := map[string]string{
		"merchant_id": "1001",
		"amount":      "100",
		"timestamp":   "1700000000",
		"sign":        "",
		"empty":       "   ",
	}

	sign := computeSign(params, "secret")
	expected := "A7336862EB54F9EC16FCC93AA2B1004D"
	if sign != expected {
		t.Fatalf("unexpected sign: got %s, want %s", sign, expected)
	}
}

func TestPostSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content-type: %s", ct)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse body: %v", err)
		}

		if got := values.Get("merchant_id"); got != "1001" {
			t.Fatalf("unexpected merchant_id: %s", got)
		}

		if got := values.Get("access_key"); got != "master-access" {
			t.Fatalf("access_key missing: %v", got)
		}

		if got := values.Get("sign"); got != "C233FA67AB751F994EF85439B6944BE7" {
			t.Fatalf("unexpected sign: %s", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"balance":"123.45"}}`))
	}))
	defer server.Close()

	cfg := config.SifangConfig{
		BaseURL:   server.URL,
		AccessKey: "master-access",
		MasterKey: "MASTERSECRET",
		Timeout:   3 * time.Second,
		MerchantKeys: map[int64]string{
			1001: "merchant-secret",
		},
	}

	client, err := NewClient(cfg, WithNowFunc(func() time.Time {
		return time.Unix(1700000000, 0)
	}))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var resp struct {
		Balance string `json:"balance"`
	}

	err = client.Post(context.Background(), "balance", 1001, map[string]string{"order_no": "abc"}, &resp)
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	if resp.Balance != "123.45" {
		t.Fatalf("unexpected balance: %s", resp.Balance)
	}
}

func TestPostBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":100,"message":"签名错误","data":null}`))
	}))
	defer server.Close()

	cfg := config.SifangConfig{
		BaseURL:            server.URL,
		DefaultMerchantKey: "merchant-secret",
		Timeout:            3 * time.Second,
	}

	client, err := NewClient(cfg, WithNowFunc(func() time.Time {
		return time.Unix(1700000000, 0)
	}))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.Post(context.Background(), "balance", 1001, nil, nil)
	if err == nil {
		t.Fatalf("expected error but got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != 100 {
		t.Fatalf("unexpected code: %d", apiErr.Code)
	}
}

func TestPostMissingMerchantKey(t *testing.T) {
	cfg := config.SifangConfig{
		BaseURL: "https://example.com",
		Timeout: 3 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.Post(context.Background(), "balance", 1001, nil, nil)
	if err == nil {
		t.Fatalf("expected error when merchant key missing")
	}
}
