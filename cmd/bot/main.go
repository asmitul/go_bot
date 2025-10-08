package main

import (
	"context"
	"go_bot/internal/logger"
	"go_bot/internal/mongo"
)

func main() {
	// 初始化logger
	logger.Init()

	// 初始化mongo
	if err := mongo.Init(); err != nil {
		logger.L().Fatalf("MongoDB 初始化失败: %v", err)
	}

	logger.L().Info("MongoDB 连接状态: ", mongo.Client().Ping(context.Background(), nil))

}
