package main

import (
	"context"

	"go_bot/internal/app"
	"go_bot/internal/config"
	"go_bot/internal/logger"
)

func main() {
	// 初始化 logger
	logger.Init()

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.L().Fatalf("Failed to load config: %v", err)
	}

	// 初始化应用（包含所有服务）
	application, err := app.New(cfg)
	if err != nil {
		logger.L().Fatalf("Failed to initialize app: %v", err)
	}
	defer application.Close(context.Background())

	logger.L().Info("Application started successfully")

	// 使用数据库
	db := application.MongoDB.Database()
	logger.L().Infof("Using database: %s", db.Name())

	// TODO: 启动 Telegram Bot
	// TODO: 阻塞等待信号（优雅关闭）
}
