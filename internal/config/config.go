package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 应用程序配置
type Config struct {
	TelegramToken        string  // Telegram Bot API Token
	BotOwnerIDs          []int64 // Bot管理员ID列表
	MongoURI             string  // MongoDB连接URI
	MongoDBName          string  // MongoDB数据库名称
	MessageRetentionDays int     // 消息保留天数（过期自动删除）
	ChannelID            int64   // 源频道 ID（用于转发功能）
	Payment              PaymentConfig
	AI                   AIConfig
}

// PaymentConfig 支付相关配置
type PaymentConfig struct {
	Sifang SifangConfig
}

// AIConfig AI 相关配置
type AIConfig struct {
	XAI XAIConfig
}

// XAIConfig xAI API 配置
type XAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// SifangConfig 四方支付配置
type SifangConfig struct {
	BaseURL            string
	AccessKey          string
	MasterKey          string
	DefaultMerchantKey string
	MerchantKeys       map[int64]string
	Timeout            time.Duration
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	mongoDBName := os.Getenv("MONGO_DB_NAME")
	if mongoDBName == "" {
		mongoDBName = "go_bot"
	}

	cfg := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		MongoURI:      os.Getenv("MONGO_URI"),
		MongoDBName:   mongoDBName,
	}

	// 解析BOT_OWNER_IDS
	ownerIDsStr := os.Getenv("BOT_OWNER_IDS")
	if ownerIDsStr != "" {
		var err error
		cfg.BotOwnerIDs, err = parseOwnerIDs(ownerIDsStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse BOT_OWNER_IDS: %w", err)
		}
	}

	// 解析MESSAGE_RETENTION_DAYS（默认7天）
	retentionDaysStr := os.Getenv("MESSAGE_RETENTION_DAYS")
	if retentionDaysStr == "" {
		cfg.MessageRetentionDays = 7 // 默认保留7天
	} else {
		days, err := strconv.Atoi(retentionDaysStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MESSAGE_RETENTION_DAYS: %w", err)
		}
		if days < 1 {
			return nil, fmt.Errorf("MESSAGE_RETENTION_DAYS must be >= 1, got %d", days)
		}
		cfg.MessageRetentionDays = days
	}

	// 解析CHANNEL_ID（可选，用于转发功能）
	channelIDStr := os.Getenv("CHANNEL_ID")
	if channelIDStr != "" {
		channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CHANNEL_ID: %w", err)
		}
		cfg.ChannelID = channelID
	}

	// 加载四方支付配置
	sifangCfg, err := loadSifangConfig()
	if err != nil {
		return nil, err
	}
	cfg.Payment.Sifang = sifangCfg

	xaiCfg, err := loadXAIConfig()
	if err != nil {
		return nil, err
	}
	cfg.AI.XAI = xaiCfg

	return cfg, nil
}

// parseOwnerIDs 解析逗号分隔的用户ID字符串
// 支持格式: "123456789" 或 "123456789,987654321"
func parseOwnerIDs(s string) ([]int64, error) {
	parts := strings.Split(s, ",")
	ids := make([]int64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid owner ID %q: %w", part, err)
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func loadSifangConfig() (SifangConfig, error) {
	var cfg SifangConfig

	cfg.BaseURL = strings.TrimSpace(os.Getenv("SIFANG_BASE_URL"))
	cfg.AccessKey = strings.TrimSpace(os.Getenv("SIFANG_ACCESS_KEY"))
	cfg.MasterKey = strings.TrimSpace(os.Getenv("SIFANG_MASTER_KEY"))
	cfg.DefaultMerchantKey = strings.TrimSpace(os.Getenv("SIFANG_DEFAULT_MERCHANT_KEY"))

	if timeoutStr := strings.TrimSpace(os.Getenv("SIFANG_TIMEOUT_SECONDS")); timeoutStr != "" {
		seconds, err := strconv.Atoi(timeoutStr)
		if err != nil || seconds <= 0 {
			return SifangConfig{}, fmt.Errorf("invalid SIFANG_TIMEOUT_SECONDS: %s", timeoutStr)
		}
		cfg.Timeout = time.Duration(seconds) * time.Second
	} else {
		cfg.Timeout = 10 * time.Second
	}

	merchantKeyStr := strings.TrimSpace(os.Getenv("SIFANG_MERCHANT_KEYS"))
	if merchantKeyStr != "" {
		parsed, err := parseMerchantKeys(merchantKeyStr)
		if err != nil {
			return SifangConfig{}, err
		}
		cfg.MerchantKeys = parsed
	} else {
		cfg.MerchantKeys = map[int64]string{}
	}

	return cfg, nil
}

func loadXAIConfig() (XAIConfig, error) {
	var cfg XAIConfig

	cfg.APIKey = strings.TrimSpace(os.Getenv("XAI_API_KEY"))
	cfg.BaseURL = strings.TrimSpace(os.Getenv("XAI_BASE_URL"))
	cfg.Model = strings.TrimSpace(os.Getenv("XAI_MODEL"))

	if timeoutStr := strings.TrimSpace(os.Getenv("XAI_TIMEOUT_SECONDS")); timeoutStr != "" {
		seconds, err := strconv.Atoi(timeoutStr)
		if err != nil || seconds <= 0 {
			return XAIConfig{}, fmt.Errorf("invalid XAI_TIMEOUT_SECONDS: %s", timeoutStr)
		}
		cfg.Timeout = time.Duration(seconds) * time.Second
	} else {
		cfg.Timeout = 15 * time.Second
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.x.ai/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "grok-beta"
	}

	return cfg, nil
}

// parseMerchantKeys 解析格式为 "1001:secret,1002:secret2" 的字符串
func parseMerchantKeys(input string) (map[int64]string, error) {
	pairs := strings.Split(input, ",")
	result := make(map[int64]string, len(pairs))

	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid SIFANG_MERCHANT_KEYS entry: %s", pair)
		}

		idStr := strings.TrimSpace(parts[0])
		key := strings.TrimSpace(parts[1])
		if idStr == "" || key == "" {
			return nil, fmt.Errorf("invalid SIFANG_MERCHANT_KEYS entry: %s", pair)
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid merchant id in SIFANG_MERCHANT_KEYS: %s", idStr)
		}

		result[id] = key
	}

	return result, nil
}
