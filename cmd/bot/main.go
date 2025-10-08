package main

import (
	"go_bot/internal/logger"
)

// 初始化logger
func init() { logger.Init() }

func main() {
	logger.L().Info("bot 启动")
	// 打印logger的级别
	logger.L().Info("logger的级别是", logger.L().GetLevel())
}
