package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 应用程序配置
type Config struct {
	TelegramToken string  // Telegram Bot API Token
	BotOwnerIDs   []int64 // Bot管理员ID列表
	MongoURI      string  // MongoDB连接URI
	MongoDBName   string  // MongoDB数据库名称
}

// Load 从环境变量加载配置
func Load() (*Config, error) {
	cfg := &Config{
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
		MongoURI:      os.Getenv("MONGO_URI"),
		MongoDBName:   os.Getenv("MONGO_DB_NAME"),
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
