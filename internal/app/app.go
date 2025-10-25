package app

import (
	"context"
	"fmt"

	"go_bot/internal/config"
	"go_bot/internal/logger"
	"go_bot/internal/mongo"
	paymentservice "go_bot/internal/payment/service"
	"go_bot/internal/payment/sifang"
	"go_bot/internal/telegram"
)

// App 应用服务容器
// 负责管理所有服务的生命周期（初始化、运行、关闭）
type App struct {
	MongoDB        *mongo.Client
	TelegramBot    *telegram.Bot
	PaymentService paymentservice.Service
	// 未来扩展其他服务：
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

	// 初始化四方支付服务（可选）
	if cfg.Payment.Sifang.BaseURL != "" {
		sifangClient, err := sifang.NewClient(cfg.Payment.Sifang)
		if err != nil {
			app.Close(context.Background())
			return nil, fmt.Errorf("init Sifang client failed: %w", err)
		}
		app.PaymentService = paymentservice.NewSifangService(sifangClient)
		logger.L().Info("Sifang payment service initialized successfully")
	} else {
		logger.L().Warn("Sifang payment service not initialized: SIFANG_BASE_URL is empty")
	}

	// 初始化 Telegram Bot
	app.TelegramBot, err = telegram.InitFromConfig(cfg, app.MongoDB.Database(), app.PaymentService)
	if err != nil {
		app.Close(context.Background()) // 清理已初始化的服务
		return nil, fmt.Errorf("init Telegram bot failed: %w", err)
	}
	logger.L().Info("Telegram bot initialized successfully")

	return app, nil
}

// Close 优雅关闭所有服务
// 应该在应用退出时调用，确保资源正确释放
func (a *App) Close(ctx context.Context) error {
	// 关闭 Telegram Bot
	if a.TelegramBot != nil {
		if err := a.TelegramBot.Stop(ctx); err != nil {
			logger.L().Warnf("Failed to stop Telegram bot: %v", err)
		}
	}

	// 关闭 MongoDB
	if a.MongoDB != nil {
		if err := a.MongoDB.Close(ctx); err != nil {
			return fmt.Errorf("close MongoDB failed: %w", err)
		}
	}

	return nil
}
