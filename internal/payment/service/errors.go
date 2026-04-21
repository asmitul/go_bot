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

	if errors.Is(err, errOrderDetailEmpty) || errors.Is(err, errOrderPayDetailEmpty) {
		return true
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

// IsMerchantNotFoundOrDisabledError reports whether err means merchant does not exist or is disabled on Sifang.
func IsMerchantNotFoundOrDisabledError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *sifang.APIError
	if !errors.As(err, &apiErr) {
		return false
	}

	message := strings.TrimSpace(strings.TrimSuffix(apiErr.Message, "。"))
	if message == "" {
		return false
	}

	messageLower := strings.ToLower(message)
	if strings.Contains(message, "商户号不存在或已停用") {
		return true
	}
	if strings.Contains(message, "商户号不存在") && strings.Contains(message, "停用") {
		return true
	}
	if strings.Contains(messageLower, "merchant") &&
		(strings.Contains(messageLower, "not exist") || strings.Contains(messageLower, "disabled")) {
		return true
	}

	return false
}
