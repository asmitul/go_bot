.PHONY: local-up local-down local-logs local-restart local-clean local-mongo test-ttl help

# 默认目标：显示帮助信息
help:
	@echo "📦 Go Bot 本地测试环境命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make <command>"
	@echo ""
	@echo "可用命令:"
	@echo "  local-up       启动本地测试环境（MongoDB + Bot）"
	@echo "  local-down     停止本地测试环境"
	@echo "  local-logs     查看 Bot 实时日志"
	@echo "  local-restart  重启 Bot（保留数据库）"
	@echo "  local-clean    清理所有数据（包括数据库）"
	@echo "  local-mongo    连接到本地 MongoDB"
	@echo "  test-ttl       检查 TTL 索引配置"
	@echo ""
	@echo "首次使用:"
	@echo "  1. cp .env.local.example .env.local"
	@echo "  2. 编辑 .env.local 填入 Bot Token 和 Owner ID"
	@echo "  3. make local-up"

# 启动本地测试环境
local-up:
	@echo "🚀 启动本地测试环境..."
	@if [ ! -f .env.local ]; then \
		echo "❌ 错误: .env.local 文件不存在"; \
		echo "请先运行: cp .env.local.example .env.local"; \
		echo "然后编辑 .env.local 填入你的配置"; \
		exit 1; \
	fi
	docker-compose -f docker-compose.local.yml --env-file .env.local up -d
	@echo "✅ 环境已启动！"
	@echo "📝 查看日志: make local-logs"

# 停止本地测试环境
local-down:
	@echo "🛑 停止本地测试环境..."
	docker-compose -f docker-compose.local.yml down
	@echo "✅ 环境已停止"

# 查看实时日志
local-logs:
	@echo "📝 查看 Bot 实时日志（Ctrl+C 退出）..."
	docker-compose -f docker-compose.local.yml logs -f bot

# 重启 Bot（保留数据库）
local-restart:
	@echo "♻️  重启 Bot..."
	docker-compose -f docker-compose.local.yml restart bot
	@echo "✅ Bot 已重启"
	@echo "📝 查看日志: make local-logs"

# 清理所有数据（包括数据库）
local-clean:
	@echo "🧹 清理所有本地数据..."
	@read -p "确认删除所有数据？(y/N) " confirm && [ "$$confirm" = "y" ] || exit 1
	docker-compose -f docker-compose.local.yml down -v
	rm -rf data/
	@echo "✅ 已清理所有本地数据"

# 连接到 MongoDB 查看数据
local-mongo:
	@echo "🔗 连接到本地 MongoDB..."
	@echo "提示: 数据库名称为 go_bot_local"
	@echo "退出: 输入 exit 或按 Ctrl+D"
	@echo ""
	docker exec -it go_bot_mongodb_local mongosh -u admin -p password123

# 测试 TTL 索引
test-ttl:
	@echo "📊 检查 TTL 索引配置..."
	@echo ""
	@docker exec go_bot_mongodb_local mongosh -u admin -p password123 --quiet --eval \
		"use go_bot_local; \
		 var indexes = db.messages.getIndexes(); \
		 var hasTTL = false; \
		 indexes.forEach(function(idx) { \
		   if (idx.expireAfterSeconds !== undefined) { \
		     print('✅ TTL 索引已配置'); \
		     print('   索引名称:', idx.name); \
		     print('   过期时间:', idx.expireAfterSeconds, '秒'); \
		     print('   等于:', (idx.expireAfterSeconds / 86400).toFixed(1), '天'); \
		     hasTTL = true; \
		   } \
		 }); \
		 if (!hasTTL) print('❌ 未找到 TTL 索引');"
	@echo ""
	@echo "💡 提示: 修改 MESSAGE_RETENTION_DAYS 后需要重启 bot 生效"
