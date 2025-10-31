package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go_bot/internal/payment/sifang"
)

// Service 定义四方支付相关操作
type Service interface {
	GetBalance(ctx context.Context, merchantID int64) (*Balance, error)
	GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*SummaryByDay, error)
	GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*SummaryByDayChannel, error)
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

func (s *sifangService) GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*SummaryByDay, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24*time.Hour - time.Second)

	business := map[string]string{
		"start_time": start.Format("2006-01-02 15:04:05"),
		"end_time":   end.Format("2006-01-02 15:04:05"),
	}

	var raw json.RawMessage
	if err := s.client.Post(ctx, "summarybyday", merchantID, business, &raw); err != nil {
		return nil, err
	}

	summary, err := decodeSummaryByDay(raw)
	if err != nil {
		return nil, err
	}

	dateStr := date.Format("2006-01-02")
	if summary == nil {
		summary = &SummaryByDay{
			Date:           dateStr,
			OrderCount:     "0",
			SuccessCount:   "0",
			TotalAmount:    "0",
			MerchantIncome: "0",
			AgentIncome:    "0",
		}
	} else if strings.TrimSpace(summary.Date) == "" {
		summary.Date = dateStr
	}

	return summary, nil
}

func (s *sifangService) GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*SummaryByDayChannel, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24*time.Hour - time.Second)

	business := map[string]string{
		"start_time":    start.Format("2006-01-02 15:04:05"),
		"end_time":      end.Format("2006-01-02 15:04:05"),
		"channel_codes": "",
	}

	var raw json.RawMessage
	if err := s.client.Post(ctx, "summarybydaychannel", merchantID, business, &raw); err != nil {
		return nil, err
	}

	summaries, err := decodeSummaryByDayChannel(raw)
	if err != nil {
		return nil, err
	}

	if len(summaries) == 0 {
		return []*SummaryByDayChannel{
			{
				Date:           date.Format("2006-01-02"),
				ChannelCode:    "-",
				OrderCount:     "0",
				SuccessCount:   "0",
				TotalAmount:    "0",
				MerchantIncome: "0",
				AgentIncome:    "0",
			},
		}, nil
	}

	return summaries, nil
}

// Balance 表示账户余额信息
type Balance struct {
	MerchantID      string
	Balance         string
	PendingWithdraw string
	Currency        string
	UpdatedAt       string
}

// SummaryByDay 表示按日汇总数据
type SummaryByDay struct {
	Date           string
	OrderCount     string
	SuccessCount   string
	TotalAmount    string
	MerchantIncome string
	AgentIncome    string
}

// SummaryByDayChannel 表示按日按通道汇总数据
type SummaryByDayChannel struct {
	Date           string
	ChannelCode    string
	ChannelName    string
	OrderCount     string
	SuccessCount   string
	TotalAmount    string
	MerchantIncome string
	AgentIncome    string
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

func decodeSummaryByDay(data json.RawMessage) (*SummaryByDay, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal summary data failed: %w", err)
	}

	summary, ok := extractSummaryFromAny(payload)
	if !ok {
		return nil, nil
	}

	return summary, nil
}

func extractSummaryFromAny(value interface{}) (*SummaryByDay, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		if summary, ok := buildSummaryFromMap(v); ok {
			return summary, true
		}

		keys := []string{"summary", "list", "items", "data", "rows", "result", "records"}
		for _, key := range keys {
			if nested, exists := v[key]; exists {
				if summary, ok := extractSummaryFromAny(nested); ok {
					return summary, true
				}
			}
		}

		for key, nested := range v {
			if summary, ok := extractSummaryFromAny(nested); ok {
				if summary.Date == "" && looksLikeDate(key) {
					summary.Date = key
				}
				return summary, true
			}
		}
	case []interface{}:
		for _, item := range v {
			if summary, ok := extractSummaryFromAny(item); ok {
				return summary, true
			}
		}
	}
	return nil, false
}

func buildSummaryFromMap(m map[string]interface{}) (*SummaryByDay, bool) {
	if len(m) == 0 {
		return nil, false
	}

	summary := &SummaryByDay{
		Date: pickString(m,
			"date", "day", "summary_date", "stat_date", "daytime", "date_str", "stat_day", "settle_date", "date_time"),
		OrderCount: pickString(m,
			"order_count", "order_num", "orders", "total_orders", "trade_count", "order_total", "total_count", "order_total_num", "order_all", "count", "pay_count", "order_quantity"),
		SuccessCount: pickString(m,
			"success_count", "success_num", "success_orders", "success_order_num", "success_total", "success", "success_total_count", "success_order_count", "pay_success", "pay_success_num", "pay_success_count", "success_quantity"),
		TotalAmount: pickString(m,
			"total_amount", "amount", "total_money", "sum_amount", "order_amount", "success_amount", "success_money", "trade_amount", "total_order_amount", "total_order_money", "sum_money", "order_money", "amount_total", "pay_amount", "money", "money_total", "success_price", "order_price", "gross_amount"),
		MerchantIncome: pickString(m,
			"merchant_income", "merchant_amount", "merchant_money", "merchant", "merchant_real", "merchant_real_amount", "merchant_settle_amount", "merchant_real_money", "real_amount", "real_money", "success_income", "merchant_profit"),
		AgentIncome: pickString(m,
			"agent_income", "agent_amount", "agent_profit", "agent_money", "agent", "share_profit", "profit", "commission", "agent_commission", "agent_fee", "agent_share"),
	}

	if summary.OrderCount == "" && summary.SuccessCount == "" &&
		summary.TotalAmount == "" && summary.MerchantIncome == "" && summary.AgentIncome == "" {
		return nil, false
	}

	if summary.Date == "" {
		summary.Date = pickString(m, "start_time", "stat_time")
	}

	return summary, true
}

func pickString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			str := strings.TrimSpace(stringify(val))
			if str != "" {
				return str
			}
		}
	}
	return ""
}

func decodeSummaryByDayChannel(data json.RawMessage) ([]*SummaryByDayChannel, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal channel summary data failed: %w", err)
	}

	items := extractChannelSummaries(payload)
	return items, nil
}

func extractChannelSummaries(value interface{}) []*SummaryByDayChannel {
	switch v := value.(type) {
	case map[string]interface{}:
		if list, ok := extractChannelSummariesFromMap(v); ok {
			return list
		}

		keys := []string{"items", "list", "data", "rows", "channels", "result"}
		for _, key := range keys {
			if nested, exists := v[key]; exists {
				if list := extractChannelSummaries(nested); len(list) > 0 {
					return list
				}
			}
		}

		var collected []*SummaryByDayChannel
		for key, nested := range v {
			if list := extractChannelSummaries(nested); len(list) > 0 {
				for _, item := range list {
					if item.Date == "" && looksLikeDate(key) {
						item.Date = key
					}
					collected = append(collected, item)
				}
			}
		}
		return collected
	case []interface{}:
		result := make([]*SummaryByDayChannel, 0, len(v))
		for _, elem := range v {
			if elem == nil {
				continue
			}
			switch entry := elem.(type) {
			case map[string]interface{}:
				if summary := buildChannelSummary(entry); summary != nil {
					result = append(result, summary)
				}
			default:
				result = append(result, extractChannelSummaries(entry)...)
			}
		}
		return result
	default:
		return nil
	}
}

func extractChannelSummariesFromMap(m map[string]interface{}) ([]*SummaryByDayChannel, bool) {
	if summary := buildChannelSummary(m); summary != nil {
		return []*SummaryByDayChannel{summary}, true
	}

	if raw, ok := m["items"]; ok {
		if list := extractChannelSummaries(raw); len(list) > 0 {
			return list, true
		}
	}

	return nil, false
}

func buildChannelSummary(m map[string]interface{}) *SummaryByDayChannel {
	if len(m) == 0 {
		return nil
	}

	summary := &SummaryByDayChannel{
		Date:           pickString(m, "date", "day", "summary_date", "stat_date", "daytime", "date_str", "stat_day", "settle_date", "date_time"),
		ChannelCode:    pickString(m, "channel_code", "channel", "channel_id", "code"),
		ChannelName:    pickString(m, "channel_name", "channelTitle", "channel_display", "name"),
		OrderCount:     pickString(m, "order_count", "order_num", "orders", "total_orders", "trade_count", "order_total", "total_count", "order_total_num", "order_all", "count", "pay_count", "order_quantity"),
		SuccessCount:   pickString(m, "success_count", "success_num", "success_orders", "success_order_num", "success_total", "success", "success_total_count", "success_order_count", "pay_success", "pay_success_num", "pay_success_count", "success_quantity"),
		TotalAmount:    pickString(m, "total_amount", "amount", "total_money", "sum_amount", "order_amount", "success_amount", "success_money", "trade_amount", "total_order_amount", "total_order_money", "sum_money", "order_money", "amount_total", "pay_amount", "money", "money_total", "success_price", "order_price", "gross_amount"),
		MerchantIncome: pickString(m, "merchant_income", "merchant_amount", "merchant_money", "merchant", "merchant_real", "merchant_real_amount", "merchant_settle_amount", "merchant_real_money", "real_amount", "real_money", "success_income", "merchant_profit"),
		AgentIncome:    pickString(m, "agent_income", "agent_amount", "agent_profit", "agent_money", "agent", "share_profit", "profit", "commission", "agent_commission", "agent_fee", "agent_share"),
	}

	if summary.ChannelCode == "" && summary.OrderCount == "" && summary.TotalAmount == "" && summary.MerchantIncome == "" && summary.AgentIncome == "" {
		return nil
	}

	return summary
}
func looksLikeDate(val string) bool {
	s := strings.TrimSpace(val)
	if len(s) != len("2006-01-02") {
		return false
	}
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return true
	}
	return false
}

func stringify(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
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
	default:
		return fmt.Sprintf("%v", v)
	}
}
