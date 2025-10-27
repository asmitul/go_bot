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

// Balance 表示账户余额信息
type Balance struct {
	MerchantID      string
	Balance         string
	PendingWithdraw string
	Currency        string
	UpdatedAt       string
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


