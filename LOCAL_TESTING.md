# 🧪 本地测试指南

本地测试环境使用 Docker Compose 一键启动完整的测试环境（MongoDB + Bot），支持快速迭代开发和功能验证。

---

## 🚀 快速开始

### 1. 准备工作

#### 步骤 1: 复制环境变量模板

```bash
cp .env.local.example .env.local
```

#### 步骤 2: 编辑配置文件

编辑 `.env.local`，填入你的配置：

```bash
# 必填项
TELEGRAM_TOKEN=your_test_bot_token_here    # 从 @BotFather 获取
BOT_OWNER_IDS=123456789                     # 从 @userinfobot 获取你的 User ID

# 可选项
MESSAGE_RETENTION_DAYS=7                    # 消息保留天数（默认 7 天）
```

**获取 Bot Token 和 User ID:**
- **Bot Token**: 在 Telegram 搜索 [@BotFather](https://t.me/BotFather)，发送 `/newbot` 创建测试 bot
- **User ID**: 在 Telegram 搜索 [@userinfobot](https://t.me/userinfobot)，发送任意消息获取你的 ID

---

### 2. 启动测试环境

```bash
make local-up
```

**预期输出:**
```
🚀 启动本地测试环境...
✅ 环境已启动！
📝 查看日志: make local-logs
```

---

### 3. 查看日志

```bash
make local-logs
```

**成功启动标志:**
```
INFO[0000] Application started successfully
INFO[0000] Using database: go_bot_local
INFO[0000] Starting Telegram bot...
INFO[0001] Telegram bot initialized successfully
INFO[0001] Message indexes ensured (TTL: 7 days = 604800 seconds)
```

按 `Ctrl+C` 退出日志查看（不会停止服务）。

---

### 4. 在 Telegram 测试

向你的测试 bot 发送消息测试基本功能：

| 命令 | 说明 | 预期结果 |
|------|------|----------|
| `/start` | 测试 bot 启动和用户注册 | 收到欢迎消息 |
| `/ping` | 测试连接状态 | 收到 "🏓 Pong!" |
| `/admins` | 查看管理员列表 | 显示你的用户信息（Owner） |
| `/userinfo <your_id>` | 查看用户详情 | 显示完整的用户信息 |
| 发送文本消息 | 测试消息记录功能 | 消息被记录到数据库 |
| 发送图片/文件 | 测试媒体消息记录 | 媒体消息被记录到数据库 |

---

## 🔍 验证 TTL 功能

### 方式 1: 使用 Makefile 命令（推荐）

```bash
make test-ttl
```

**预期输出:**
```
📊 检查 TTL 索引配置...

✅ TTL 索引已配置
   索引名称: sent_at_1
   过期时间: 604800 秒
   等于: 7.0 天

💡 提示: 修改 MESSAGE_RETENTION_DAYS 后需要重启 bot 生效
```

---

### 方式 2: 连接 MongoDB 手动查看

```bash
make local-mongo
```

在 `mongosh` 中执行：

```javascript
// 切换到测试数据库
use go_bot_local

// 查看所有索引
db.messages.getIndexes()

// 查看最近的消息
db.messages.find().sort({sent_at: -1}).limit(5).pretty()

// 统计消息总数
db.messages.countDocuments()

// 退出
exit
```

---

## ⚡ 快速验证 TTL 自动删除（1 分钟测试）

如果想快速验证 TTL 是否真的会自动删除消息，可以临时设置极短的保留期：

### 步骤 1: 修改保留期

编辑 `.env.local`，设置为约 1 分钟：

```bash
MESSAGE_RETENTION_DAYS=0.0007  # 约 60 秒
```

### 步骤 2: 重启 Bot

```bash
make local-restart
make local-logs  # 确认日志显示新的 TTL 时间: 约 60 seconds
```

### 步骤 3: 发送测试消息

向 bot 发送几条测试消息。

### 步骤 4: 等待 1-2 分钟后检查

```bash
make local-mongo
```

在 mongosh 中执行：

```javascript
use go_bot_local
db.messages.countDocuments()  // 应该为 0 或很少
db.messages.find().pretty()   // 应该看不到刚才的消息
```

⚠️ **注意**：
- MongoDB TTL 后台任务每 60 秒运行一次，所以实际删除可能有最多 1 分钟延迟
- 测试完成后记得改回正常值：`MESSAGE_RETENTION_DAYS=7`

---

## 🔄 切换分支测试

### 测试其他功能分支

```bash
# 1. 切换到其他分支
git checkout another-feature-branch

# 2. 停止当前环境
make local-down

# 3. 重新构建并启动（会使用新分支代码）
make local-up

# 4. 查看日志确认启动成功
make local-logs
```

### 回到主分支

```bash
git checkout feature/message-ttl
make local-down
make local-up
```

---

## 📋 常用命令速查

| 命令 | 说明 |
|------|------|
| `make help` | 显示所有可用命令 |
| `make local-up` | 启动测试环境 |
| `make local-down` | 停止测试环境 |
| `make local-logs` | 查看实时日志 |
| `make local-restart` | 重启 Bot（保留数据） |
| `make local-clean` | 清理所有数据（需确认） |
| `make local-mongo` | 连接 MongoDB |
| `make test-ttl` | 检查 TTL 索引 |

---

## 🛠️ 高级操作

### 查看 Docker 容器状态

```bash
docker ps | grep go_bot
```

### 手动重启 MongoDB

```bash
docker restart go_bot_mongodb_local
```

### 查看 MongoDB 日志

```bash
docker logs go_bot_mongodb_local
```

### 直接使用 docker-compose 命令

```bash
# 启动（前台运行，查看所有日志）
docker-compose -f docker-compose.local.yml --env-file .env.local up

# 查看所有服务状态
docker-compose -f docker-compose.local.yml ps

# 重新构建镜像
docker-compose -f docker-compose.local.yml build --no-cache
```

---

## 🗄️ 数据管理

### 数据存储位置

本地 MongoDB 数据存储在：
```
./data/mongodb/
```

### 备份数据

```bash
# 复制整个数据目录
cp -r data/mongodb data/mongodb_backup_$(date +%Y%m%d)
```

### 恢复数据

```bash
# 停止环境
make local-down

# 删除当前数据
rm -rf data/mongodb

# 恢复备份
cp -r data/mongodb_backup_20250117 data/mongodb

# 重启环境
make local-up
```

### 完全清理并重新开始

```bash
make local-clean  # 会提示确认
```

---

## 🐛 故障排查

### 问题 1: Bot 启动失败

**现象**: `make local-up` 后 bot 容器一直重启

**排查步骤**:
```bash
# 查看详细日志
make local-logs

# 检查环境变量是否正确
cat .env.local

# 检查容器状态
docker ps -a | grep go_bot
```

**常见原因**:
- ❌ `TELEGRAM_TOKEN` 不正确或为空
- ❌ `BOT_OWNER_IDS` 格式错误
- ❌ MongoDB 未启动或连接失败

---

### 问题 2: 端口被占用

**现象**: 启动时报错 "port 27017 already in use"

**解决方法**:
```bash
# 检查是否有其他 MongoDB 在运行
lsof -i :27017

# 停止其他 MongoDB 服务
brew services stop mongodb-community  # 如果用 Homebrew 安装过

# 或者修改 docker-compose.local.yml 使用其他端口
# ports:
#   - "27018:27017"
```

---

### 问题 3: 数据库连接失败

**现象**: 日志显示 "failed to connect to MongoDB"

**排查步骤**:
```bash
# 检查 MongoDB 容器是否健康
docker ps | grep mongodb

# 查看 MongoDB 日志
docker logs go_bot_mongodb_local

# 测试连接
docker exec go_bot_mongodb_local mongosh -u admin -p password123 --eval "db.adminCommand('ping')"
```

---

### 问题 4: TTL 索引未生效

**现象**: 消息没有自动删除

**排查步骤**:
```bash
# 1. 检查索引是否存在
make test-ttl

# 2. 检查 bot 启动日志
make local-logs | grep "TTL"
# 应该看到: Message indexes ensured (TTL: X days = Y seconds)

# 3. 等待 MongoDB TTL 后台任务运行（最多 60 秒）

# 4. 确认消息的 sent_at 时间已过期
make local-mongo
# use go_bot_local
# db.messages.find({}, {sent_at: 1, _id: 0}).sort({sent_at: -1}).limit(5)
```

---

## ✅ 测试检查清单

完整测试一个功能分支时，请确认以下项目：

- [ ] Bot 成功启动，日志无 ERROR
- [ ] `/start` 命令响应正常
- [ ] `/ping` 命令响应正常
- [ ] 用户信息自动注册到数据库
- [ ] 消息成功记录到 messages collection
- [ ] TTL 索引存在且配置正确（`make test-ttl`）
- [ ] MongoDB 数据正常（`make local-mongo` 查看）
- [ ] 权限系统正常工作（Owner 可以执行管理命令）
- [ ] 切换分支后环境重启成功

---

## 📚 相关文档

- [README.md](./README.md) - 项目总体说明
- [AGENTS.md](./AGENTS.md) - 仓库贡献指南
- [TESTING.md](./TESTING.md) - 线上环境测试指南
- [GitHub Actions 配置](./.github/workflows/) - CI/CD 流程

---

## 💡 提示

1. **本地测试完成后推送到 main 分支即可自动部署到生产环境**
2. **本地数据与生产环境完全隔离，可以放心测试**
3. **建议为不同的测试场景创建不同的测试 bot**
4. **定期执行 `make local-clean` 清理测试数据**

---

## 🎉 开始测试

现在你已经掌握了本地测试环境的所有操作，开始愉快地开发和测试吧！

```bash
# 一键启动
make local-up

# 开始测试
# ... 在 Telegram 中测试你的 bot ...

# 完成后停止
make local-down
```

有问题？查看 [故障排查](#-故障排查) 章节或检查日志 `make local-logs`。
