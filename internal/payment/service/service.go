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
	GetBalance(ctx context.Context, merchantID int64, historyDays int) (*Balance, error)
	GetSummaryByDay(ctx context.Context, merchantID int64, date time.Time) (*SummaryByDay, error)
	GetSummaryByDayByChannel(ctx context.Context, merchantID int64, date time.Time) ([]*SummaryByDayChannel, error)
	GetChannelStatus(ctx context.Context, merchantID int64) ([]*ChannelStatus, error)
	GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*WithdrawList, error)
	SendMoney(ctx context.Context, merchantID int64, amount float64, opts SendMoneyOptions) (*SendMoneyResult, error)
	GetOrders(ctx context.Context, merchantID int64, filter OrderFilter) (*OrderList, error)
}

type sifangService struct {
	client *sifang.Client
}

// SendMoneyOptions 下发请求的可选参数
type SendMoneyOptions struct {
	BankID     string
	GoogleCode string
}

// SendMoneyResult 表示下发接口的返回结果
type SendMoneyResult struct {
	MerchantID      string
	Withdraw        *Withdraw
	BalanceAfter    string
	PendingWithdraw string
	FrozenToday     string
	Fee             string
}

// OrderFilter 订单查询条件
type OrderFilter struct {
	MerchantOrderNo string
	PlatformOrderNo string
	Status          string
	Page            int
	PageSize        int
}

// NewSifangService 创建基于四方支付的服务实现
func NewSifangService(client *sifang.Client) Service {
	return &sifangService{client: client}
}

func (s *sifangService) GetBalance(ctx context.Context, merchantID int64, historyDays int) (*Balance, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	if historyDays < 0 {
		historyDays = 0
	}
	if historyDays > 365 {
		historyDays = 365
	}

	business := map[string]string{
		"history_days": strconv.Itoa(historyDays),
	}

	raw := make(map[string]interface{})
	if err := s.client.Post(ctx, "balance", merchantID, business, &raw); err != nil {
		return nil, err
	}

	balance := decodeBalance(raw)
	if balance != nil && balance.HistoryDays == 0 {
		balance.HistoryDays = historyDays
	}

	return balance, nil
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

func (s *sifangService) GetChannelStatus(ctx context.Context, merchantID int64) ([]*ChannelStatus, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	var raw json.RawMessage
	if err := s.client.Post(ctx, "channelstatus", merchantID, nil, &raw); err != nil {
		return nil, err
	}

	statuses, err := decodeChannelStatus(raw)
	if err != nil {
		return nil, err
	}

	return statuses, nil
}

func (s *sifangService) GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*WithdrawList, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	business := map[string]string{
		"start_time": start.Format("2006-01-02 15:04:05"),
		"end_time":   end.Format("2006-01-02 15:04:05"),
		"page":       strconv.Itoa(page),
		"page_size":  strconv.Itoa(pageSize),
	}

	var raw json.RawMessage
	if err := s.client.Post(ctx, "withdrawlist", merchantID, business, &raw); err != nil {
		return nil, err
	}

	return decodeWithdrawList(raw)
}

func (s *sifangService) GetOrders(ctx context.Context, merchantID int64, filter OrderFilter) (*OrderList, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	business := make(map[string]string)
	if strings.TrimSpace(filter.MerchantOrderNo) != "" {
		business["merchant_order_no"] = strings.TrimSpace(filter.MerchantOrderNo)
	}
	if strings.TrimSpace(filter.PlatformOrderNo) != "" {
		business["platform_order_no"] = strings.TrimSpace(filter.PlatformOrderNo)
	}
	if strings.TrimSpace(filter.Status) != "" {
		business["status"] = strings.TrimSpace(filter.Status)
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	business["page"] = strconv.Itoa(page)
	business["page_size"] = strconv.Itoa(pageSize)

	var raw json.RawMessage
	if err := s.client.Post(ctx, "orders", merchantID, business, &raw); err != nil {
		return nil, err
	}

	return decodeOrderList(raw)
}

func (s *sifangService) SendMoney(ctx context.Context, merchantID int64, amount float64, opts SendMoneyOptions) (*SendMoneyResult, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	formattedAmount := fmt.Sprintf("%.2f", amount)

	business := map[string]string{
		"amount": formattedAmount,
		"money":  formattedAmount,
	}

	if strings.TrimSpace(opts.BankID) != "" {
		business["bank_id"] = strings.TrimSpace(opts.BankID)
	}
	if strings.TrimSpace(opts.GoogleCode) != "" {
		business["google_code"] = strings.TrimSpace(opts.GoogleCode)
	}

	raw := make(map[string]interface{})
	if err := s.client.Post(ctx, "sendmoney", merchantID, business, &raw); err != nil {
		return nil, err
	}

	result := decodeSendMoney(raw)
	if result != nil && strings.TrimSpace(result.MerchantID) == "" {
		result.MerchantID = strconv.FormatInt(merchantID, 10)
	}

	return result, nil
}

// Balance 表示账户余额信息
type Balance struct {
	MerchantID      string
	Balance         string
	PendingWithdraw string
	Currency        string
	UpdatedAt       string
	HistoryDays     int
	HistoryBalance  string
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

// ChannelStatus 表示通道状态
type ChannelStatus struct {
	ChannelCode     string
	ChannelName     string
	SystemEnabled   bool
	MerchantEnabled bool
	Rate            string
	MinAmount       string
	MaxAmount       string
	DailyQuota      string
	DailyUsed       string
	LastUsedAt      string
}

// Withdraw 表示提现记录
type Withdraw struct {
	WithdrawNo string
	OrderNo    string
	Amount     string
	Fee        string
	Status     string
	CreatedAt  string
	PaidAt     string
	Channel    string
}

// WithdrawList 表示提现列表及分页信息
type WithdrawList struct {
	Page       int
	PageSize   int
	Total      int
	TotalPages int
	Items      []*Withdraw
}

// Order 表示订单信息
type Order struct {
	MerchantOrderNo string
	PlatformOrderNo string
	Amount          string
	RealAmount      string
	Status          string
	StatusText      string
	PayStatus       string
	NotifyStatus    string
	Channel         string
	CreatedAt       string
	PaidAt          string
}

// OrderSummary 表示订单汇总信息
type OrderSummary struct {
	TotalCount     string
	TotalAmount    string
	SuccessAmount  string
	MerchantIncome string
}

// OrderList 表示订单列表数据
type OrderList struct {
	Page       int
	PageSize   int
	Total      int
	TotalPages int
	Items      []*Order
	Summary    *OrderSummary
}

func decodeBalance(raw map[string]interface{}) *Balance {
	return &Balance{
		MerchantID:      stringify(raw["merchant_id"]),
		Balance:         stringify(raw["balance"]),
		PendingWithdraw: stringify(raw["pending_withdraw"]),
		Currency:        stringify(raw["currency"]),
		UpdatedAt:       stringify(raw["updated_at"]),
		HistoryDays:     parseInt(raw["history_days"], 0),
		HistoryBalance:  stringify(raw["history_balance"]),
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

func decodeChannelStatus(data json.RawMessage) ([]*ChannelStatus, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal channel status failed: %w", err)
	}

	return extractChannelStatus(payload), nil
}

func decodeWithdrawList(data json.RawMessage) (*WithdrawList, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return &WithdrawList{Items: []*Withdraw{}}, nil
	}

	var payload struct {
		Page       int         `json:"page"`
		PageSize   int         `json:"page_size"`
		Total      int         `json:"total"`
		TotalPages int         `json:"total_pages"`
		Items      interface{} `json:"items"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal withdraw list failed: %w", err)
	}

	list := &WithdrawList{
		Page:       payload.Page,
		PageSize:   payload.PageSize,
		Total:      payload.Total,
		TotalPages: payload.TotalPages,
		Items:      make([]*Withdraw, 0),
	}

	if payload.Items == nil {
		return list, nil
	}

	switch v := payload.Items.(type) {
	case []interface{}:
		for _, elem := range v {
			if elem == nil {
				continue
			}
			if summary := buildWithdraw(elem); summary != nil {
				list.Items = append(list.Items, summary)
			}
		}
	case map[string]interface{}:
		for _, elem := range v {
			if elem == nil {
				continue
			}
			if summary := buildWithdraw(elem); summary != nil {
				list.Items = append(list.Items, summary)
			}
		}
	default:
		// ignore unrecognized structure
	}

	return list, nil
}

func buildWithdraw(value interface{}) *Withdraw {
	item, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	withdraw := &Withdraw{
		WithdrawNo: pickString(item, "withdraw_no", "id", "withdraw_id"),
		OrderNo:    pickString(item, "order_no", "merchant_order_no", "orderid"),
		Amount:     pickString(item, "amount", "money", "withdraw_amount", "apply_amount"),
		Fee:        pickString(item, "fee", "charge", "service_fee"),
		Status:     pickString(item, "status", "state"),
		CreatedAt:  pickString(item, "created_at", "create_time", "created_time", "apply_time", "ctime"),
		PaidAt:     pickString(item, "paid_at", "pay_time", "payed_at"),
		Channel:    pickString(item, "channel", "channel_name", "channel_code"),
	}

	if withdraw.WithdrawNo == "" && withdraw.OrderNo == "" && withdraw.Amount == "" {
		return nil
	}

	return withdraw
}

func decodeOrderList(data json.RawMessage) (*OrderList, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return &OrderList{Items: []*Order{}}, nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal order list failed: %w", err)
	}

	list := &OrderList{
		Items: make([]*Order, 0),
	}

	populateOrderList(payload, list)

	if len(list.Items) == 0 {
		if order := buildOrder(payload); order != nil {
			list.Items = append(list.Items, order)
		}
	}

	return list, nil
}

func populateOrderList(value interface{}, list *OrderList) {
	switch v := value.(type) {
	case map[string]interface{}:
		if page := parseInt(v["page"], 0); page > 0 && list.Page == 0 {
			list.Page = page
		}
		if size := parseInt(v["page_size"], 0); size > 0 && list.PageSize == 0 {
			list.PageSize = size
		}
		if total := parseInt(v["total"], 0); total > 0 && list.Total == 0 {
			list.Total = total
		}
		if pages := parseInt(v["total_pages"], 0); pages > 0 && list.TotalPages == 0 {
			list.TotalPages = pages
		}

		keys := []string{"items", "list", "data", "rows", "orders", "result"}
		handled := make(map[string]struct{}, len(keys))
		hasNested := false
		for _, key := range keys {
			nested, ok := v[key]
			if !ok || nested == nil {
				continue
			}
			hasNested = true
			handled[key] = struct{}{}
			appendOrders(nested, list)
		}

		if summary := buildOrderSummary(v["summary"]); summary != nil {
			list.Summary = summary
		} else if list.Summary == nil {
			if summary := buildOrderSummary(v); summary != nil {
				list.Summary = summary
			}
		}

		for key, nested := range v {
			if _, skip := handled[key]; skip || key == "summary" || nested == nil {
				continue
			}
			appendOrders(nested, list)
		}

		if !hasNested {
			if order := buildOrder(v); order != nil {
				list.Items = append(list.Items, order)
			}
		}

	case []interface{}:
		for _, elem := range v {
			appendOrders(elem, list)
		}
	}
}

func appendOrders(value interface{}, list *OrderList) {
	switch v := value.(type) {
	case []interface{}:
		for _, elem := range v {
			if order := buildOrder(elem); order != nil {
				list.Items = append(list.Items, order)
			} else {
				populateOrderList(elem, list)
			}
		}
	case map[string]interface{}:
		if order := buildOrder(v); order != nil {
			list.Items = append(list.Items, order)
		} else {
			populateOrderList(v, list)
		}
	}
}

func buildOrder(value interface{}) *Order {
	item, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	order := &Order{
		MerchantOrderNo: pickString(item, "merchant_order_no", "order_no", "mer_order_no", "merchant_order", "merchantno", "orderid"),
		PlatformOrderNo: pickString(item, "platform_order_no", "platform_no", "trade_no", "sys_order_no", "order_id", "system_order_no", "transaction_id"),
		Amount:          pickString(item, "amount", "money", "order_amount", "total_amount", "price", "pay_amount"),
		RealAmount:      pickString(item, "real_amount", "merchant_amount", "merchant_money", "success_amount", "merchant_income", "real_money", "paid_amount"),
		Status:          pickString(item, "status", "order_status"),
		StatusText:      pickString(item, "status_text", "status_desc", "status_label", "state_text", "status_name"),
		PayStatus:       pickString(item, "pay_status", "paystate", "payment_status"),
		NotifyStatus:    pickString(item, "notify_status", "notify_state", "callback_status"),
		Channel:         pickString(item, "channel", "channel_name", "channel_code"),
		CreatedAt:       pickString(item, "created_at", "create_time", "order_time", "ctime", "created_time"),
		PaidAt:          pickString(item, "paid_at", "pay_time", "payment_time", "payed_at"),
	}

	if order.StatusText == "" {
		order.StatusText = pickString(item, "status_message", "statusmsg")
	}
	if order.StatusText == "" {
		order.StatusText = order.Status
	}

	if order.MerchantOrderNo == "" && order.PlatformOrderNo == "" && order.Amount == "" && order.StatusText == "" && order.Status == "" {
		return nil
	}

	return order
}

func buildOrderSummary(value interface{}) *OrderSummary {
	item, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	summary := &OrderSummary{
		TotalCount:     pickString(item, "total_count", "order_count", "count", "total_orders", "sum_count"),
		TotalAmount:    pickString(item, "total_amount", "amount_total", "sum_amount", "order_amount_total"),
		SuccessAmount:  pickString(item, "success_amount", "success_money", "paid_amount", "success_total_amount"),
		MerchantIncome: pickString(item, "merchant_income", "merchant_amount", "merchant_money", "merchant_total"),
	}

	if summary.TotalCount == "" && summary.TotalAmount == "" && summary.SuccessAmount == "" && summary.MerchantIncome == "" {
		return nil
	}

	return summary
}

func decodeSendMoney(raw map[string]interface{}) *SendMoneyResult {
	if len(raw) == 0 {
		return nil
	}

	result := &SendMoneyResult{
		MerchantID:      stringify(raw["merchant_id"]),
		BalanceAfter:    stringify(raw["balance_after"]),
		PendingWithdraw: stringify(raw["pending_withdraw"]),
		FrozenToday:     stringify(raw["frozen_today"]),
		Fee:             stringify(raw["fee"]),
	}

	if withdrawRaw, ok := raw["withdraw"]; ok && withdrawRaw != nil {
		if withdraw := buildWithdraw(withdrawRaw); withdraw != nil {
			result.Withdraw = withdraw
		}
	}

	if result.MerchantID == "" && result.Withdraw == nil && result.BalanceAfter == "" && result.PendingWithdraw == "" && result.Fee == "" && result.FrozenToday == "" {
		return nil
	}

	return result
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

func extractChannelStatus(value interface{}) []*ChannelStatus {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		result := make([]*ChannelStatus, 0, len(v))
		for _, elem := range v {
			if elem == nil {
				continue
			}
			switch entry := elem.(type) {
			case map[string]interface{}:
				if status := buildChannelStatus(entry); status != nil {
					result = append(result, status)
				} else {
					if nested := extractChannelStatus(entry); len(nested) > 0 {
						result = append(result, nested...)
					}
				}
			default:
				if nested := extractChannelStatus(entry); len(nested) > 0 {
					result = append(result, nested...)
				}
			}
		}
		return result
	case map[string]interface{}:
		if status := buildChannelStatus(v); status != nil {
			return []*ChannelStatus{status}
		}

		keys := []string{"items", "list", "channels", "data", "rows", "result"}
		for _, key := range keys {
			if nested, exists := v[key]; exists {
				if list := extractChannelStatus(nested); len(list) > 0 {
					return list
				}
			}
		}

		var result []*ChannelStatus
		for _, nested := range v {
			if list := extractChannelStatus(nested); len(list) > 0 {
				result = append(result, list...)
			}
		}
		return result
	default:
		return nil
	}
}

func buildChannelStatus(m map[string]interface{}) *ChannelStatus {
	if len(m) == 0 {
		return nil
	}

	status := &ChannelStatus{
		ChannelCode:     pickString(m, "channel_code", "code", "channel", "channel_id"),
		ChannelName:     pickString(m, "channel_name", "name", "channelTitle", "channel_display"),
		Rate:            pickString(m, "rate", "channel_rate", "fee_rate"),
		MinAmount:       pickString(m, "min_amount", "min", "min_money", "min_limit"),
		MaxAmount:       pickString(m, "max_amount", "max", "max_money", "max_limit"),
		DailyQuota:      pickString(m, "daily_quota", "day_quota", "quota", "daily_limit"),
		DailyUsed:       pickString(m, "daily_used", "day_used", "used", "used_quota"),
		LastUsedAt:      pickString(m, "last_used_at", "last_time", "updated_at", "last_pay_time"),
		SystemEnabled:   pickBool(m, "system_enabled", "system_open", "system_status", "enabled", "is_enabled"),
		MerchantEnabled: pickBool(m, "merchant_enabled", "merchant_open", "merchant_status", "merchant_enable"),
	}

	if status.ChannelCode == "" && status.ChannelName == "" && status.Rate == "" && !status.SystemEnabled && !status.MerchantEnabled {
		return nil
	}

	return status
}

func pickBool(m map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if parsed, ok := parseBoolValue(val); ok {
				return parsed
			}
		}
	}
	return false
}

func parseBoolValue(value interface{}) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		return parseBoolString(v)
	case json.Number:
		if num, err := v.Float64(); err == nil {
			return num != 0, true
		}
	case float64:
		return v != 0, true
	case float32:
		return v != 0, true
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case int32:
		return v != 0, true
	default:
		str := strings.TrimSpace(stringify(v))
		if str == "" {
			return false, false
		}
		return parseBoolString(str)
	}
	return false, false
}

func parseBoolString(input string) (bool, bool) {
	s := strings.TrimSpace(strings.ToLower(input))
	if s == "" {
		return false, false
	}

	switch s {
	case "1", "true", "t", "yes", "y", "on", "open", "enabled", "enable":
		return true, true
	case "0", "false", "f", "no", "n", "off", "close", "closed", "disabled", "disable":
		return false, true
	default:
		if num, err := strconv.ParseFloat(s, 64); err == nil {
			return num != 0, true
		}
	}

	return false, false
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

func parseInt(value interface{}, fallback int) int {
	str := strings.TrimSpace(stringify(value))
	if str == "" {
		return fallback
	}
	if n, err := strconv.Atoi(str); err == nil {
		return n
	}
	return fallback
}
