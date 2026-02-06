package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"go_bot/internal/logger"
)

const (
	okxC2CAPI = "https://www.okx.com/v3/c2c/tradingOrders/books"
)

// C2COrder OKX C2C 订单结构
type C2COrder struct {
	Price           string `json:"price"`           // 价格
	NickName        string `json:"nickName"`        // 商家昵称
	AvailableAmount string `json:"availableAmount"` // 可用数量
}

// C2CData OKX API 数据结构
type C2CData struct {
	Buy  []C2COrder `json:"buy"`  // 买单列表
	Sell []C2COrder `json:"sell"` // 卖单列表
}

// C2CResponse OKX API 响应结构
type C2CResponse struct {
	Code int     `json:"code"` // 响应码（0 表示成功）
	Data C2CData `json:"data"` // 数据对象（包含 buy/sell 列表）
	Msg  string  `json:"msg"`  // 消息
}

// FetchC2COrders 从 OKX 获取 C2C 订单列表
func FetchC2COrders(ctx context.Context, paymentMethod string) ([]C2COrder, error) {
	// 构建请求参数
	params := url.Values{
		"quoteCurrency": {"CNY"},
		"baseCurrency":  {"USDT"},
		"side":          {"sell"},
		"paymentMethod": {paymentMethod},
		"userType":      {"all"},
	}

	// 构建完整 URL
	fullURL := fmt.Sprintf("%s?%s", okxC2CAPI, params.Encode())

	// 创建 HTTP 客户端（5 秒超时）
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 User-Agent（模拟浏览器）
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	// 发送请求
	logger.L().Debugf("Fetching OKX C2C orders: payment_method=%s", paymentMethod)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OKX API: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		logger.L().Errorf("OKX API HTTP error: status=%d, url=%s", resp.StatusCode, fullURL)
		return nil, fmt.Errorf("OKX API returned non-200 status: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.L().Errorf("Failed to read OKX API response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 打印响应体大小和截断的内容（调试用）
	bodyStr := string(body)
	bodyLen := len(bodyStr)
	if bodyLen > 500 {
		logger.L().Debugf("OKX API response (size=%d bytes, truncated): %s...", bodyLen, bodyStr[:500])
	} else {
		logger.L().Debugf("OKX API response (size=%d bytes): %s", bodyLen, bodyStr)
	}

	// 解析 JSON
	var apiResp C2CResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		// 解析失败时打印完整响应（用于调试）
		logger.L().Errorf("Failed to parse JSON response: %s", bodyStr)
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 检查业务响应码
	if apiResp.Code != 0 {
		logger.L().Errorf("OKX API business error: code=%d, msg=%s, payment_method=%s",
			apiResp.Code, apiResp.Msg, paymentMethod)
		return nil, fmt.Errorf("OKX API error: code=%d, msg=%s", apiResp.Code, apiResp.Msg)
	}

	// 检查数据（从 sell 列表中提取订单）
	orders := apiResp.Data.Sell
	if len(orders) == 0 {
		logger.L().Warnf("OKX API returned empty order list: payment_method=%s", paymentMethod)
		return nil, fmt.Errorf("no orders available")
	}

	logger.L().Infof("Fetched %d orders from OKX: payment_method=%s", len(orders), paymentMethod)
	return orders, nil
}
