package service

import (
	"errors"
	"strings"

	"go_bot/internal/payment/sifang"
)

// IsOrderNotFoundError reports whether err means the order does not exist on Sifang.
func IsOrderNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *sifang.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	if apiErr.Code == 404 {
		return true
	}

	message := strings.TrimSpace(apiErr.Message)
	if message == "" {
		return false
	}

	messageLower := strings.ToLower(message)
	if messageLower == "not found" || strings.Contains(messageLower, "order not found") {
		return true
	}

	message = strings.TrimSpace(strings.TrimSuffix(message, "。"))
	if strings.Contains(message, "订单不存在") || strings.Contains(message, "查无订单") || strings.Contains(message, "无此订单") {
		return true
	}

	return false
}
