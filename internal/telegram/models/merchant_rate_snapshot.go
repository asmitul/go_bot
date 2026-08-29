package models

import (
	"slices"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MerchantChannelRate 保存单个商户通道的费率快照。
type MerchantChannelRate struct {
	ChannelCode string `bson:"channel_code"`
	ChannelName string `bson:"channel_name,omitempty"`
	Rate        string `bson:"rate"`
}

// MerchantRateSnapshot 保存商户群最近一次成功处理的费率基线。
type MerchantRateSnapshot struct {
	ID            primitive.ObjectID    `bson:"_id,omitempty"`
	ChatID        int64                 `bson:"chat_id"`
	MerchantID    int64                 `bson:"merchant_id"`
	Rates         []MerchantChannelRate `bson:"rates"`
	LastCheckedAt time.Time             `bson:"last_checked_at"`
	CreatedAt     time.Time             `bson:"created_at"`
	UpdatedAt     time.Time             `bson:"updated_at"`
}

// NormalizeMerchantChannelRates 清理、去重并按通道代码排序。
func NormalizeMerchantChannelRates(rates []MerchantChannelRate) []MerchantChannelRate {
	if len(rates) == 0 {
		return nil
	}

	byCode := make(map[string]MerchantChannelRate, len(rates))
	for _, item := range rates {
		code := strings.TrimSpace(item.ChannelCode)
		if code == "" {
			continue
		}
		byCode[strings.ToLower(code)] = MerchantChannelRate{
			ChannelCode: code,
			ChannelName: strings.TrimSpace(item.ChannelName),
			Rate:        strings.TrimSpace(item.Rate),
		}
	}

	keys := make([]string, 0, len(byCode))
	for key := range byCode {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	normalized := make([]MerchantChannelRate, 0, len(keys))
	for _, key := range keys {
		normalized = append(normalized, byCode[key])
	}
	return normalized
}
