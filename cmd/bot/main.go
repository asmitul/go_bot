package main

import (
	"go_bot/internal/logger"
)

func main() {
	// 初始化logger
	logger.Init()
	logger.L().Info("bot 启动")
}
