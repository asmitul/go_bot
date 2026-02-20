.PHONY: local-up local-down local-logs local-restart local-clean local-mongo local-build build test-ttl version bump-major bump-minor bump-patch help
.DEFAULT_GOAL := help

COMPOSE_FILE := docker-compose.local.yml
DOCKER_COMPOSE := $(if $(shell command -v docker-compose >/dev/null 2>&1 && echo yes),docker-compose,docker compose)
COMPOSE := $(DOCKER_COMPOSE) -f $(COMPOSE_FILE)
VERSION_FILE := VERSION
VERSION := $(strip $(shell cat $(VERSION_FILE) 2>/dev/null || echo dev))
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -w -s \
	-X go_bot/internal/version.Version=$(VERSION) \
	-X go_bot/internal/version.GitCommit=$(GIT_COMMIT) \
	-X go_bot/internal/version.BuildTime=$(BUILD_TIME)

# 默认目标：显示帮助信息
help:
	@echo "📦 Go Bot 本地测试环境命令"
	@echo ""
	@echo "使用方法:"
	@echo "  make <command>"
	@echo ""
	@echo "可用命令:"
	@echo "  version        查看当前版本号"
	@echo "  bump-patch     升级补丁版本 (x.y.z -> x.y.z+1)"
	@echo "  bump-minor     升级次版本 (x.y.z -> x.y+1.0)"
	@echo "  bump-major     升级主版本 (x.y.z -> x+1.0.0)"
	@echo "  build          构建带版本信息的二进制 (bin/bot)"
	@echo "  local-up       启动本地测试环境（MongoDB + Bot）"
	@echo "  local-build    仅重建本地 bot 镜像（带版本信息）"
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

# 查看版本
version:
	@echo "$(VERSION)"

# 升级版本号
bump-major:
	@./scripts/bump_version.sh major $(VERSION_FILE)

bump-minor:
	@./scripts/bump_version.sh minor $(VERSION_FILE)

bump-patch:
	@./scripts/bump_version.sh patch $(VERSION_FILE)

# 构建二进制（包含版本、commit、构建时间）
build:
	@echo "🔨 构建 bot 二进制..."
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/bot ./cmd/bot
	@echo "✅ 构建完成: bin/bot (version=$(VERSION), commit=$(GIT_COMMIT))"

# 启动本地测试环境
local-up:
	@echo "🚀 启动本地测试环境..."
	@if [ ! -f .env.local ]; then \
		echo "❌ 错误: .env.local 文件不存在"; \
		echo "请先运行: cp .env.local.example .env.local"; \
		echo "然后编辑 .env.local 填入你的配置"; \
		exit 1; \
	fi
	APP_VERSION=$(VERSION) GIT_COMMIT=$(GIT_COMMIT) BUILD_TIME=$(BUILD_TIME) \
	$(COMPOSE) --env-file .env.local up -d
	@echo "✅ 环境已启动！"
	@echo "📝 查看日志: make local-logs"

# 重建本地 bot 镜像（带版本信息）
local-build:
	@echo "🔨 重建本地 bot 镜像..."
	APP_VERSION=$(VERSION) GIT_COMMIT=$(GIT_COMMIT) BUILD_TIME=$(BUILD_TIME) \
	$(COMPOSE) --env-file .env.local build bot
	@echo "✅ 镜像重建完成 (version=$(VERSION), commit=$(GIT_COMMIT))"

# 停止本地测试环境
local-down:
	@echo "🛑 停止本地测试环境..."
	$(COMPOSE) down
	@echo "✅ 环境已停止"

# 查看实时日志
local-logs:
	@echo "📝 查看 Bot 实时日志（Ctrl+C 退出）..."
	$(COMPOSE) logs -f bot

# 重启 Bot（保留数据库）
local-restart:
	@echo "♻️  重启 Bot..."
	$(COMPOSE) restart bot
	@echo "✅ Bot 已重启"
	@echo "📝 查看日志: make local-logs"

# 清理所有数据（包括数据库）
local-clean:
	@echo "🧹 清理所有本地数据..."
	@read -p "确认删除所有数据？(y/N) " confirm && [ "$$confirm" = "y" ] || exit 1
	$(COMPOSE) down -v
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
