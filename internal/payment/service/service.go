package service

import (
	"context"
	"encoding/json"
	"errors"
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
	GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*SummaryByPZID, error)
	GetChannelStatus(ctx context.Context, merchantID int64) ([]*ChannelStatus, error)
	GetWithdrawList(ctx context.Context, merchantID int64, start, end time.Time, page, pageSize int) (*WithdrawList, error)
	SendMoney(ctx context.Context, merchantID int64, amount float64, opts SendMoneyOptions) (*SendMoneyResult, error)
	GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType OrderNumberType) (*OrderDetail, error)
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

// OrderDetail 订单详情结构
type OrderDetail struct {
	Order      *Order
	Extended   *OrderExtended
	NotifyLogs []*NotifyLog
}

// Order 订单基础信息
type Order struct {
	MerchantOrderNo  string
	PlatformOrderNo  string
	Amount           string
	RealAmount       string
	Status           string
	StatusText       string
	NotifyStatus     string
	NotifyStatusText string
	NotifyTimes      string
	NotifyLastError  string
	ChannelCode      string
	ChannelName      string
	CreatedAt        string
	PaidAt           string
	CompletedAt      string
	ExpiredAt        string
	NotifyURL        string
	ReturnURL        string
	Description      string
	Attach           string
	ClientIP         string
	Currency         string
	UserID           string
	PaymentURL       string
	BankCode         string
	BankAccount      string
	BankAccountName  string
	BankBranch       string
	BuyerName        string
	BuyerID          string
	Extra            map[string]string
}

// OrderExtended 订单扩展信息
type OrderExtended struct {
	OrderID          string
	MerchantID       string
	ChannelID        string
	ChannelFee       string
	ChannelFeeRate   string
	ChannelCost      string
	DeductStatus     string
	DeductStatusText string
	DeductAmount     string
	DeductReason     string
	RiskFlag         bool
	Manual           bool
	Remark           string
	CreatedAt        string
	UpdatedAt        string
}

// NotifyLog 订单回调日志
type NotifyLog struct {
	Status      string
	StatusText  string
	Request     string
	Response    string
	URL         string
	AttemptedAt string
	Duration    string
	Retry       string
}

// OrderNumberType 标识订单号类型
type OrderNumberType string

const (
	// OrderNumberTypeMerchant 使用商户订单号查询
	OrderNumberTypeMerchant OrderNumberType = "merchant"
	// OrderNumberTypePlatform 使用平台订单号查询
	OrderNumberTypePlatform OrderNumberType = "platform"
	// OrderNumberTypeAuto 优先使用商户订单号，失败时回退到平台订单号
	OrderNumberTypeAuto OrderNumberType = "auto"
)

const orderDetailTimeout = 8 * time.Second

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

func (s *sifangService) GetSummaryByDayByPZID(ctx context.Context, pzid string, start, end time.Time) (*SummaryByPZID, error) {
	pzid = strings.TrimSpace(pzid)
	if pzid == "" {
		return nil, fmt.Errorf("pzid is required")
	}
	if end.Before(start) {
		return nil, fmt.Errorf("end time must not be before start time")
	}
	business := map[string]string{
		"pzid":       pzid,
		"start_time": start.Format("2006-01-02 15:04:05"),
		"end_time":   end.Format("2006-01-02 15:04:05"),
	}

	var raw json.RawMessage
	if err := s.client.Post(ctx, "summarybydaypzid", 0, business, &raw); err != nil {
		return nil, err
	}

	return decodeSummaryByPZID(raw)
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

func (s *sifangService) GetOrderDetail(ctx context.Context, merchantID int64, orderNo string, numberType OrderNumberType) (*OrderDetail, error) {
	if merchantID == 0 {
		return nil, fmt.Errorf("merchant id is required")
	}

	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return nil, fmt.Errorf("order number is required")
	}

	if numberType == "" {
		numberType = OrderNumberTypeAuto
	}

	lookupOrder := resolveOrderNumberTypes(numberType)
	var lastErr error

	for idx, kind := range lookupOrder {
		business := map[string]string{
			"with_notify_logs": "1",
		}

		switch kind {
		case OrderNumberTypeMerchant:
			business["merchant_order_no"] = orderNo
		case OrderNumberTypePlatform:
			business["platform_order_no"] = orderNo
		default:
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, orderDetailTimeout)
		raw := make(map[string]interface{})
		err := s.client.Post(reqCtx, "orderdetail", merchantID, business, &raw)
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("get order detail timed out (%s number)", describeOrderNumberType(kind))
			}

			var apiErr *sifang.APIError
			if errors.As(err, &apiErr) {
				lastErr = fmt.Errorf("get order detail failed with sifang error (%s number): %w", describeOrderNumberType(kind), err)
			} else {
				lastErr = fmt.Errorf("get order detail failed (%s number): %w", describeOrderNumberType(kind), err)
			}

			if idx < len(lookupOrder)-1 {
				continue
			}

			return nil, lastErr
		}

		detail := decodeOrderDetail(raw)
		if detail == nil || detail.Order == nil {
			lastErr = fmt.Errorf("order detail is empty (%s number)", describeOrderNumberType(kind))
			if idx < len(lookupOrder)-1 {
				continue
			}

			return nil, lastErr
		}

		return detail, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return nil, fmt.Errorf("order detail lookup failed")
}

func resolveOrderNumberTypes(numberType OrderNumberType) []OrderNumberType {
	switch numberType {
	case OrderNumberTypeMerchant:
		return []OrderNumberType{OrderNumberTypeMerchant}
	case OrderNumberTypePlatform:
		return []OrderNumberType{OrderNumberTypePlatform}
	case OrderNumberTypeAuto:
		fallthrough
	default:
		return []OrderNumberType{OrderNumberTypeMerchant, OrderNumberTypePlatform}
	}
}

func describeOrderNumberType(numberType OrderNumberType) string {
	switch numberType {
	case OrderNumberTypeMerchant:
		return "merchant"
	case OrderNumberTypePlatform:
		return "platform"
	default:
		return string(numberType)
	}
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

// SummaryByPZID 表示按日按上游配置 ID 汇总数据
type SummaryByPZID struct {
	PZID      string
	PZName    string
	StartDate string
	EndDate   string
	Items     []*SummaryByPZIDItem
}

// SummaryByPZIDItem 为单日统计
type SummaryByPZIDItem struct {
	Date           string
	OrderCount     string
	GrossAmount    string
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

func decodeOrderDetail(raw map[string]interface{}) *OrderDetail {
	if len(raw) == 0 {
		return nil
	}

	detail := &OrderDetail{
		NotifyLogs: make([]*NotifyLog, 0),
	}

	if orderVal, ok := raw["order"]; ok {
		if order := buildOrder(orderVal); order != nil {
			detail.Order = order
		}
	}

	if extendedVal, ok := raw["extended"]; ok {
		if extended := buildOrderExtended(extendedVal); extended != nil {
			detail.Extended = extended
		}
	}

	if logsVal, ok := raw["notify_logs"]; ok {
		detail.NotifyLogs = append(detail.NotifyLogs, buildNotifyLogs(logsVal)...)
	}

	if detail.Order == nil && detail.Extended == nil && len(detail.NotifyLogs) == 0 {
		return nil
	}

	return detail
}

func buildOrder(value interface{}) *Order {
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	order := &Order{
		MerchantOrderNo:  pickString(m, "merchant_order_no", "order_no", "merchant_no", "orderid"),
		PlatformOrderNo:  pickString(m, "platform_order_no", "platform_no", "sys_order_no", "trade_no", "upstream_order_no"),
		Amount:           pickString(m, "amount", "order_amount", "money", "total_amount", "price"),
		RealAmount:       pickString(m, "real_amount", "merchant_amount", "success_amount", "real_money", "merchant_real"),
		Status:           pickString(m, "status", "order_status", "pay_status", "state"),
		StatusText:       pickString(m, "status_text", "status_desc", "status_name", "order_status_name"),
		NotifyStatus:     pickString(m, "notify_status", "notify_state", "notify_result"),
		NotifyStatusText: pickString(m, "notify_status_text", "notify_desc", "notify_status_name"),
		NotifyTimes:      pickString(m, "notify_times", "notify_count", "notify_num"),
		NotifyLastError:  pickString(m, "notify_last_error", "notify_error", "notify_message"),
		ChannelCode:      pickString(m, "channel_code", "channel", "pay_channel", "pay_type"),
		ChannelName:      pickString(m, "channel_name", "channel_display", "channel_title", "pay_channel_name"),
		CreatedAt:        pickString(m, "created_at", "create_time", "created_time", "ctime", "order_time"),
		PaidAt:           pickString(m, "paid_at", "pay_time", "payment_time", "success_time"),
		CompletedAt:      pickString(m, "completed_at", "finish_time", "complete_time"),
		ExpiredAt:        pickString(m, "expired_at", "expire_time", "overdue_time"),
		NotifyURL:        pickString(m, "notify_url", "callback_url", "notify"),
		ReturnURL:        pickString(m, "return_url", "back_url"),
		Description:      pickString(m, "description", "body", "subject", "product_name", "goods_name"),
		Attach:           pickString(m, "attach", "remark", "extra", "metadata"),
		ClientIP:         pickString(m, "client_ip", "ip", "customer_ip"),
		Currency:         pickString(m, "currency", "money_type"),
		UserID:           pickString(m, "user_id", "uid", "member_id"),
		PaymentURL:       pickString(m, "payment_url", "pay_url", "url", "cashier_url"),
		BankCode:         pickString(m, "bank_code", "bank"),
		BankAccount:      pickString(m, "bank_account", "account", "card_number"),
		BankAccountName:  pickString(m, "bank_account_name", "account_name", "card_name"),
		BankBranch:       pickString(m, "bank_branch", "branch"),
		BuyerName:        pickString(m, "buyer_name", "customer_name", "payer_name"),
		BuyerID:          pickString(m, "buyer_id", "customer_id"),
		Extra:            make(map[string]string),
	}

	knownKeys := map[string]struct{}{
		"merchant_order_no":  {},
		"order_no":           {},
		"merchant_no":        {},
		"orderid":            {},
		"platform_order_no":  {},
		"platform_no":        {},
		"sys_order_no":       {},
		"trade_no":           {},
		"upstream_order_no":  {},
		"amount":             {},
		"order_amount":       {},
		"money":              {},
		"total_amount":       {},
		"price":              {},
		"real_amount":        {},
		"merchant_amount":    {},
		"success_amount":     {},
		"real_money":         {},
		"merchant_real":      {},
		"status":             {},
		"order_status":       {},
		"pay_status":         {},
		"state":              {},
		"status_text":        {},
		"status_desc":        {},
		"status_name":        {},
		"order_status_name":  {},
		"notify_status":      {},
		"notify_state":       {},
		"notify_result":      {},
		"notify_status_text": {},
		"notify_desc":        {},
		"notify_status_name": {},
		"notify_times":       {},
		"notify_count":       {},
		"notify_num":         {},
		"notify_last_error":  {},
		"notify_error":       {},
		"notify_message":     {},
		"channel_code":       {},
		"channel":            {},
		"pay_channel":        {},
		"pay_type":           {},
		"channel_name":       {},
		"channel_display":    {},
		"channel_title":      {},
		"pay_channel_name":   {},
		"created_at":         {},
		"create_time":        {},
		"created_time":       {},
		"ctime":              {},
		"order_time":         {},
		"paid_at":            {},
		"pay_time":           {},
		"payment_time":       {},
		"success_time":       {},
		"completed_at":       {},
		"finish_time":        {},
		"complete_time":      {},
		"expired_at":         {},
		"expire_time":        {},
		"overdue_time":       {},
		"notify_url":         {},
		"callback_url":       {},
		"notify":             {},
		"return_url":         {},
		"back_url":           {},
		"description":        {},
		"body":               {},
		"subject":            {},
		"product_name":       {},
		"goods_name":         {},
		"attach":             {},
		"remark":             {},
		"extra":              {},
		"metadata":           {},
		"client_ip":          {},
		"ip":                 {},
		"customer_ip":        {},
		"currency":           {},
		"money_type":         {},
		"user_id":            {},
		"uid":                {},
		"member_id":          {},
		"payment_url":        {},
		"pay_url":            {},
		"url":                {},
		"cashier_url":        {},
		"bank_code":          {},
		"bank":               {},
		"bank_account":       {},
		"account":            {},
		"card_number":        {},
		"bank_account_name":  {},
		"account_name":       {},
		"card_name":          {},
		"bank_branch":        {},
		"branch":             {},
		"buyer_name":         {},
		"customer_name":      {},
		"payer_name":         {},
		"buyer_id":           {},
		"customer_id":        {},
	}

	for key, val := range m {
		if _, exists := knownKeys[key]; exists {
			continue
		}
		str := strings.TrimSpace(stringify(val))
		if str == "" {
			continue
		}
		order.Extra[key] = str
	}

	if len(order.Extra) == 0 {
		order.Extra = nil
	}

	if order.MerchantOrderNo == "" && order.PlatformOrderNo == "" && order.Amount == "" && order.Status == "" && order.ChannelCode == "" && order.ChannelName == "" && order.RealAmount == "" && order.PaymentURL == "" {
		return nil
	}

	return order
}

func buildOrderExtended(value interface{}) *OrderExtended {
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	extended := &OrderExtended{
		OrderID:          pickString(m, "order_id", "id"),
		MerchantID:       pickString(m, "merchant_id", "mid", "merchant"),
		ChannelID:        pickString(m, "channel_id", "cid"),
		ChannelFee:       pickString(m, "channel_fee", "fee", "poundage"),
		ChannelFeeRate:   pickString(m, "channel_fee_rate", "fee_rate", "channel_rate"),
		ChannelCost:      pickString(m, "channel_cost", "cost"),
		DeductStatus:     pickString(m, "deduct_status", "deduct_state", "deduct"),
		DeductStatusText: pickString(m, "deduct_status_text", "deduct_desc", "deduct_status_name"),
		DeductAmount:     pickString(m, "deduct_amount", "deduct_money", "deduct_fee"),
		DeductReason:     pickString(m, "deduct_reason", "deduct_remark", "deduct_msg"),
		RiskFlag:         pickBool(m, "risk_flag", "is_risk", "risk"),
		Manual:           pickBool(m, "manual", "is_manual", "manual_flag"),
		Remark:           pickString(m, "remark", "memo", "note"),
		CreatedAt:        pickString(m, "created_at", "create_time", "ctime"),
		UpdatedAt:        pickString(m, "updated_at", "update_time", "utime"),
	}

	if extended.OrderID == "" && extended.MerchantID == "" && extended.ChannelFee == "" && extended.DeductStatus == "" && extended.DeductAmount == "" && !extended.RiskFlag && !extended.Manual {
		return nil
	}

	return extended
}

func buildNotifyLogs(value interface{}) []*NotifyLog {
	if value == nil {
		return nil
	}

	logs := make([]*NotifyLog, 0)

	switch v := value.(type) {
	case []interface{}:
		for _, item := range v {
			if log := buildNotifyLog(item); log != nil {
				logs = append(logs, log)
			}
		}
	case []map[string]interface{}:
		for _, item := range v {
			if log := buildNotifyLog(item); log != nil {
				logs = append(logs, log)
			}
		}
	case map[string]interface{}:
		for _, item := range v {
			if log := buildNotifyLog(item); log != nil {
				logs = append(logs, log)
			}
		}
	default:
		// ignore unsupported types
	}

	return logs
}

func buildNotifyLog(value interface{}) *NotifyLog {
	m, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	log := &NotifyLog{
		Status:      pickString(m, "status", "state", "result"),
		StatusText:  pickString(m, "status_text", "status_desc", "result_desc"),
		Request:     pickString(m, "request", "request_body", "payload"),
		Response:    pickString(m, "response", "response_body", "reply"),
		URL:         pickString(m, "url", "notify_url", "callback_url"),
		AttemptedAt: pickString(m, "attempted_at", "created_at", "notify_time", "time"),
		Duration:    pickString(m, "duration", "cost", "elapsed"),
		Retry:       pickString(m, "retry", "retry_count", "times"),
	}

	if log.Status == "" && log.StatusText == "" && log.Request == "" && log.Response == "" && log.URL == "" && log.AttemptedAt == "" {
		return nil
	}

	return log
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

func decodeSummaryByPZID(data json.RawMessage) (*SummaryByPZID, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal pzid summary data failed: %w", err)
	}

	summary := &SummaryByPZID{
		Items: make([]*SummaryByPZIDItem, 0),
	}

	switch v := payload.(type) {
	case map[string]interface{}:
		summary.PZID = pickString(v, "pzid", "upstream_id", "channel_id", "interface_id")
		summary.PZName = pickString(v, "pz_name", "pzname", "interface_name", "channel_name")
		summary.StartDate = pickString(v, "start_date", "start", "start_date_str")
		summary.EndDate = pickString(v, "end_date", "end", "end_date_str")

		keys := []string{"items", "list", "data", "rows", "result"}
		for _, key := range keys {
			if nested, exists := v[key]; exists {
				summary.Items = append(summary.Items, buildPZIDSummaries(nested)...)
			}
		}

		// 有些实现直接以日期为键
		if len(summary.Items) == 0 {
			for key, nested := range v {
				list := buildPZIDSummaries(nested)
				for _, item := range list {
					if item.Date == "" && looksLikeDate(key) {
						item.Date = key
					}
					summary.Items = append(summary.Items, item)
				}
			}
		}
	case []interface{}:
		summary.Items = append(summary.Items, buildPZIDSummaries(v)...)
	default:
		// ignore unsupported structures
	}

	return summary, nil
}

func buildPZIDSummaries(value interface{}) []*SummaryByPZIDItem {
	items := make([]*SummaryByPZIDItem, 0)
	switch v := value.(type) {
	case []interface{}:
		for _, elem := range v {
			if elem == nil {
				continue
			}
			if item := buildPZIDSummaryItem(elem); item != nil {
				items = append(items, item)
			}
		}
	case map[string]interface{}:
		for key, elem := range v {
			if elem == nil {
				continue
			}
			if item := buildPZIDSummaryItem(elem); item != nil {
				if item.Date == "" && looksLikeDate(key) {
					item.Date = key
				}
				items = append(items, item)
			}
		}
	}
	return items
}

func buildPZIDSummaryItem(value interface{}) *SummaryByPZIDItem {
	m, ok := value.(map[string]interface{})
	if !ok || len(m) == 0 {
		return nil
	}

	item := &SummaryByPZIDItem{
		Date: pickString(m,
			"date", "day", "summary_date", "stat_date", "date_str", "settle_date", "daytime"),
		OrderCount: pickString(m,
			"order_count", "order_num", "orders", "count", "total_orders", "success_count", "total_count"),
		GrossAmount: pickString(m,
			"gross_amount", "total_amount", "amount", "total_money", "sum_amount", "money", "order_amount", "success_amount"),
		MerchantIncome: pickString(m,
			"merchant_income", "merchant_amount", "merchant_money", "merchant", "merchant_real", "merchant_real_amount", "real_amount"),
		AgentIncome: pickString(m,
			"agent_income", "agent_amount", "agent_profit", "agent_money", "profit", "commission"),
	}

	if item.Date == "" {
		item.Date = pickString(m, "start_time")
	}

	if item.OrderCount == "" && item.GrossAmount == "" && item.MerchantIncome == "" && item.AgentIncome == "" {
		return nil
	}

	return item
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
