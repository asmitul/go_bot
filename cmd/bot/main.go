package main

import (
	"context"
	"os"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/mongo"
)

func main() {
	// 初始化logger
	logger.Init()

	// 配置 MongoDB 连接
	cfg := mongo.Config{
		URI:      os.Getenv("MONGO_URI"),
		Database: os.Getenv("DATABASE_NAME"),
		Timeout:  5 * time.Second,
	}

	// 初始化 MongoDB 客户端
	client, err := mongo.NewClient(cfg)
	if err != nil {
		logger.L().Fatalf("Failed to create MongoDB client: %v", err)
	}
	defer client.Close(context.Background())

	// 验证连接
	if err := client.Ping(context.Background()); err != nil {
		logger.L().Fatalf("Failed to ping MongoDB: %v", err)
	}
	logger.L().Info("MongoDB connected successfully")

	// 使用数据库
	db := client.Database()
	logger.L().Infof("Using database: %s", db.Name())

}
