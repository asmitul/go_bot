package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// 创建可取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.L().Info("Application started successfully")

	// 使用数据库
	db := application.MongoDB.Database()
	logger.L().Infof("Using database: %s", db.Name())

	// 启动 Telegram Bot（在 goroutine 中运行，因为 Start 是阻塞式的）
	go func() {
		logger.L().Info("Starting Telegram bot...")
		if err := application.TelegramBot.Start(ctx); err != nil {
			logger.L().Errorf("Telegram bot error: %v", err)
		}
	}()

	// 等待中断信号（优雅关闭）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan // 阻塞等待信号
	logger.L().Info("Received shutdown signal, gracefully shutting down...")

	// 取消 context，通知 bot 停止
	cancel()

	// 等待 bot 停止（给一些时间让 bot 完成当前处理）
	time.Sleep(2 * time.Second)

	// 关闭所有服务
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := application.Close(shutdownCtx); err != nil {
		logger.L().Errorf("Error during shutdown: %v", err)
	}

	logger.L().Info("Application stopped successfully")
}
