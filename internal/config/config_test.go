package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadMerchantRateMonitorDefaults(t *testing.T) {
	t.Setenv("MERCHANT_RATE_MONITOR_ENABLED", "")
	t.Setenv("MERCHANT_RATE_MONITOR_INTERVAL_MINUTES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.MerchantRateMonitorEnabled {
		t.Fatal("expected merchant rate monitor to be enabled by default")
	}
	if cfg.MerchantRateMonitorInterval != time.Minute {
		t.Fatalf("expected one-minute interval, got %s", cfg.MerchantRateMonitorInterval)
	}
}

func TestLoadMerchantRateMonitorOverrides(t *testing.T) {
	t.Setenv("MERCHANT_RATE_MONITOR_ENABLED", "false")
	t.Setenv("MERCHANT_RATE_MONITOR_INTERVAL_MINUTES", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.MerchantRateMonitorEnabled {
		t.Fatal("expected merchant rate monitor to be disabled")
	}
	if cfg.MerchantRateMonitorInterval != 5*time.Minute {
		t.Fatalf("expected five-minute interval, got %s", cfg.MerchantRateMonitorInterval)
	}
}

func TestLoadMerchantRateMonitorInvalidValues(t *testing.T) {
	t.Run("invalid enabled", func(t *testing.T) {
		t.Setenv("MERCHANT_RATE_MONITOR_ENABLED", "sometimes")
		t.Setenv("MERCHANT_RATE_MONITOR_INTERVAL_MINUTES", "")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "MERCHANT_RATE_MONITOR_ENABLED") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid interval", func(t *testing.T) {
		t.Setenv("MERCHANT_RATE_MONITOR_ENABLED", "true")
		t.Setenv("MERCHANT_RATE_MONITOR_INTERVAL_MINUTES", "0")
		_, err := Load()
		if err == nil || !strings.Contains(err.Error(), "MERCHANT_RATE_MONITOR_INTERVAL_MINUTES") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
