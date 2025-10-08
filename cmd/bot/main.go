package main

import (
	"context"
	"time"

	"go_bot/internal/logger"
	"go_bot/internal/mongo"
)

func main() {
	// 初始化logger
	logger.Init()

	// 配置 MongoDB 连接
	cfg := mongo.Config{
		URI:      "mongodb://localhost:27017",
		Database: "mydb",
		Timeout:  5 * time.Second,
	}

	// 初始化 MongoDB 客户端
	client, err := mongo.NewClient(cfg)
	if err != nil {
		logger.L().Fatalf("Failed to create MongoDB client: %v", err)
	}
	defer client.Close(context.Background())

	// 使用数据库
	db := client.Database()

	logger.L().Info("MongoDB 连接状态: ", client.Ping(context.Background(), nil))

	logger.L().Info("MongoDB 数据库: ", db)

}
