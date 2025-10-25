package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go_bot/internal/payment/sifang"
)

// Service 定义四方支付相关操作
type Service interface {
	GetBalance(ctx context.Context, merchantID int64) (*Balance, error)
	ListOrders(ctx context.Context, merchantID int64, filter OrdersFilter) (*OrdersResult, error)
}

type sifangService struct {
	client *sifang.Client
}

// NewSifangService 创建基于四方支付的服务实现
func NewSifangService(client *sifang.Client) Service {
	return &sifangService{client: client}
}

func (s *sifangService) GetBalance(ctx context.Context, merchantID int64) (*Balance, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	raw := make(map[string]interface{})
	if err := s.client.Post(ctx, "balance", merchantID, nil, &raw); err != nil {
		return nil, err
	}

	return decodeBalance(raw), nil
}

func (s *sifangService) ListOrders(ctx context.Context, merchantID int64, filter OrdersFilter) (*OrdersResult, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	params := filter.toParams()

	raw := make(map[string]interface{})
	if err := s.client.Post(ctx, "orders", merchantID, params, &raw); err != nil {
		return nil, err
	}

	return decodeOrdersResult(raw), nil
}

// Balance 表示账户余额信息
type Balance struct {
	MerchantID      string
	Balance         string
	PendingWithdraw string
	Currency        string
	UpdatedAt       string
}

// OrdersResult 表示订单列表结果
type OrdersResult struct {
	Items    []Order
	Summary  map[string]string
	Total    int
	Page     int
	PageSize int
}

// Order 表示订单简要信息
type Order struct {
	MerchantOrderNo string
	PlatformOrderNo string
	Amount          string
	Status          string
	NotifyStatus    string
	ChannelCode     string
	CreatedAt       string
	PaidAt          string
}

// OrdersFilter 订单列表查询条件
type OrdersFilter struct {
	Status          string
	MerchantOrderNo string
	PlatformOrderNo string
	ChannelCode     string
	StartTime       string
	EndTime         string
	PayStart        string
	PayEnd          string
	MinAmount       string
	MaxAmount       string
	Page            int
	PageSize        int
}

func (f OrdersFilter) toParams() map[string]string {
	params := map[string]string{}

	setIfNotEmpty := func(key, value string) {
		if value != "" {
			params[key] = value
		}
	}

	setIfNotEmpty("status", f.Status)
	setIfNotEmpty("merchant_order_no", f.MerchantOrderNo)
	setIfNotEmpty("platform_order_no", f.PlatformOrderNo)
	setIfNotEmpty("channel_code", f.ChannelCode)
	setIfNotEmpty("start_time", f.StartTime)
	setIfNotEmpty("end_time", f.EndTime)
	setIfNotEmpty("pay_start", f.PayStart)
	setIfNotEmpty("pay_end", f.PayEnd)
	setIfNotEmpty("min_amount", f.MinAmount)
	setIfNotEmpty("max_amount", f.MaxAmount)

	if f.Page > 0 {
		params["page"] = strconv.Itoa(f.Page)
	}
	if f.PageSize > 0 {
		params["page_size"] = strconv.Itoa(f.PageSize)
	}

	return params
}

func decodeBalance(raw map[string]interface{}) *Balance {
	return &Balance{
		MerchantID:      stringify(raw["merchant_id"]),
		Balance:         stringify(raw["balance"]),
		PendingWithdraw: stringify(raw["pending_withdraw"]),
		Currency:        stringify(raw["currency"]),
		UpdatedAt:       stringify(raw["updated_at"]),
	}
}

func decodeOrdersResult(raw map[string]interface{}) *OrdersResult {
	result := &OrdersResult{
		Items:   make([]Order, 0),
		Summary: map[string]string{},
	}

	if total, ok := raw["total"]; ok {
		result.Total = parseInt(total)
	}
	if page, ok := raw["page"]; ok {
		result.Page = parseInt(page)
	}
	if pageSize, ok := raw["page_size"]; ok {
		result.PageSize = parseInt(pageSize)
	}

	if summary, ok := raw["summary"].(map[string]interface{}); ok {
		for k, v := range summary {
			result.Summary[k] = stringify(v)
		}
	}

	if items, ok := raw["items"].([]interface{}); ok {
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			result.Items = append(result.Items, Order{
				MerchantOrderNo: stringify(m["merchant_order_no"]),
				PlatformOrderNo: stringify(m["platform_order_no"]),
				Amount:          stringify(m["amount"]),
				Status:          stringify(m["status"]),
				NotifyStatus:    stringify(m["notify_status"]),
				ChannelCode:     stringify(m["channel_code"]),
				CreatedAt:       stringify(m["created_at"]),
				PaidAt:          stringify(m["pay_at"]),
			})
		}
	}

	return result
}

func stringify(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		i, err := strconv.Atoi(v)
		if err != nil {
			return 0
		}
		return i
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0
		}
		return int(i)
	default:
		return 0
	}
}
