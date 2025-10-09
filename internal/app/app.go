package app

import (
	"context"
	"fmt"

	"go_bot/internal/config"
	"go_bot/internal/logger"
	"go_bot/internal/mongo"
)

// App 应用服务容器
// 负责管理所有服务的生命周期（初始化、运行、关闭）
type App struct {
	MongoDB *mongo.Client
	// 未来扩展其他服务：
	// TelegramBot *telegram.Bot
	// RedisClient *redis.Client
}

// New 初始化应用及其所有服务
// 按顺序初始化各个服务，任何服务初始化失败都会返回错误
func New(cfg *config.Config) (*App, error) {
	app := &App{}

	// 初始化 MongoDB
	mongoClient, err := mongo.InitFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("init MongoDB failed: %w", err)
	}
	app.MongoDB = mongoClient
	logger.L().Info("MongoDB initialized successfully")

	// 未来在这里初始化其他服务
	// 示例：
	// app.TelegramBot, err = telegram.New(cfg.TelegramToken)
	// if err != nil {
	//     app.Close(context.Background()) // 清理已初始化的服务
	//     return nil, fmt.Errorf("init Telegram bot failed: %w", err)
	// }

	return app, nil
}

// Close 优雅关闭所有服务
// 应该在应用退出时调用，确保资源正确释放
func (a *App) Close(ctx context.Context) error {
	if a.MongoDB != nil {
		if err := a.MongoDB.Close(ctx); err != nil {
			return fmt.Errorf("close MongoDB failed: %w", err)
		}
	}
	// 未来添加其他服务的关闭逻辑
	return nil
}
