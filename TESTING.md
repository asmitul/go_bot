# 🧪 在线测试指南

本文档提供完整的在线测试指南，适用于通过 GitHub Actions 部署到 VPS 后的 Telegram Bot 测试。

## 目录

- [准备工作](#准备工作)
- [部署验证](#部署验证)
- [Telegram 功能测试](#telegram-功能测试)
- [数据库验证](#数据库验证)
- [故障排查](#故障排查)
- [更新与重新测试](#更新与重新测试)
- [日志分析](#日志分析)
- [性能监控](#性能监控)
- [测试检查清单](#测试检查清单)
- [快速命令参考](#快速命令参考)

---

## 准备工作

### 1. 获取 Telegram Bot Token

1. 在 Telegram 搜索 `@BotFather`
2. 发送 `/newbot` 命令
3. 按提示输入 bot 名称（显示名）和 username（必须以 `bot` 结尾）
4. 创建成功后会得到一个 **Token**（格式：`1234567890:ABCdefGHIjklMNOpqrsTUVwxyz`）
5. 保存此 Token，配置到 GitHub Secrets 的 `TELEGRAM_TOKEN` 中

### 2. 获取你的 Telegram User ID

**方法一（推荐）：**
1. 在 Telegram 搜索 `@userinfobot`
2. 点击 Start 或发送任意消息
3. Bot 会返回你的 **User ID**（一串数字，例如 `123456789`）

**方法二：**
1. 在 Telegram 搜索 `@raw_data_bot`
2. 发送任意消息
3. 在返回的 JSON 中找到 `"id"` 字段

### 3. 配置 MongoDB

本项目支持 MongoDB Atlas（云端）或自建 MongoDB。推荐使用 MongoDB Atlas 免费版（M0）：

1. 访问 https://www.mongodb.com/cloud/atlas
2. 注册并创建免费的 M0 集群
3. 创建数据库用户（设置用户名和密码）
4. 在 Network Access 中允许所有 IP 访问（`0.0.0.0/0`）
5. 获取连接字符串（格式：`mongodb+srv://username:password@cluster0.xxxxx.mongodb.net/`）
6. 配置到 GitHub Secrets 的 `MONGO_URI` 中

### 4. 配置 GitHub Secrets

在 GitHub 仓库设置中配置以下 Secrets：

| Secret Name | 说明 | 示例 |
|-------------|------|------|
| `TELEGRAM_TOKEN` | Telegram Bot API Token | `123:ABCdef...` |
| `BOT_OWNER_IDS` | 管理员 ID（逗号分隔） | `123456789` 或 `123,456` |
| `MONGO_URI` | MongoDB 连接字符串 | `mongodb+srv://user:pass@...` |
| `VPS_HOST` | VPS 服务器地址 | `1.2.3.4` |
| `VPS_USER` | SSH 用户名 | `root` 或 `ubuntu` |
| `VPS_PORT` | SSH 端口 | `22` |
| `SSH_KEY` | SSH 私钥 | 完整的私钥内容 |

---

## 部署验证

### 步骤 1：检查 GitHub Actions 部署状态

1. 进入 GitHub 仓库主页
2. 点击 **Actions** 标签
3. 查看最新的 workflow run
4. 确认 **CD (Continuous Deployment)** workflow 显示绿色 ✅
5. 如果显示红色 ❌，点击进入查看失败原因

### 步骤 2：SSH 连接 VPS 检查容器

```bash
# 连接到 VPS
ssh your_user@your_vps_host

# 查看 bot 容器是否运行
docker ps | grep go_bot
```

**预期输出：**
```
abc123def  ghcr.io/yourname/go_bot:latest  "bot"  Up 2 minutes  go_bot
```

如果没有输出，说明容器未运行，需要检查部署日志。

### 步骤 3：查看 Bot 日志

```bash
# 查看实时日志
docker logs -f go_bot

# 查看最近 100 行日志
docker logs --tail 100 go_bot
```

**成功启动的日志标志：**
```
INFO[0000] Application started successfully
INFO[0000] Using database: go_bot
INFO[0000] Starting Telegram bot...
INFO[0001] Telegram bot initialized successfully
```

如果看到 `ERROR` 级别日志，需要检查配置是否正确。

---

## Telegram 功能测试

### 基础命令测试

在 Telegram 中找到你的 bot（搜索创建时设置的 username），依次测试以下命令：

#### 测试 1：启动 Bot

```
发送：/start
```

**预期结果：**
- 收到欢迎消息
- 你的用户信息被自动注册到数据库（role: owner）

#### 测试 2：Ping 测试

```
发送：/ping
```

**预期结果：**
```
🏓 Pong!
```

#### 测试 3：查看管理员列表

```
发送：/admins
```

**预期结果：**
```
📋 管理员列表:
1. @your_username (123456789) - Owner
```

#### 测试 4：查看用户信息

```
发送：/userinfo 你的_User_ID
例如：/userinfo 123456789
```

**预期结果：**
```
👤 用户信息:
- ID: 123456789
- 用户名: @your_username
- 姓名: Your Name
- 角色: owner
- 授予人: 系统初始化
- 最后活跃: 刚刚
```

### 权限管理测试（需要第二个测试账号）

如果你有第二个 Telegram 账号，可以测试权限管理功能：

#### 测试 5：授予管理员权限

```
步骤 1：用第二个账号向 bot 发送 /start
步骤 2：获取第二个账号的 User ID（通过 @userinfobot）
步骤 3：回到主账号，发送：/grant <第二个账号的ID>
```

**预期结果：**
```
✅ 已成功授予用户 <ID> 管理员权限
```

验证：发送 `/admins`，应该看到管理员列表中出现第二个用户。

#### 测试 6：撤销管理员权限

```
发送：/revoke <第二个账号的ID>
```

**预期结果：**
```
✅ 已成功撤销用户 <ID> 的管理员权限
```

验证：再次发送 `/admins`，第二个用户应该已从列表中移除。

#### 测试 7：权限检查

用第二个普通用户账号（非 owner）测试：

```
发送：/grant 123456
```

**预期结果：**
```
❌ 你没有权限执行此操作
```

### 群组功能测试

#### 测试 8：添加 Bot 到群组

```
步骤 1：在 Telegram 创建一个测试群组
步骤 2：将你的 Bot 添加到群组（作为成员或管理员）
```

**预期结果：**
- Bot 自动发送欢迎消息到群组
- 群组信息被记录到数据库的 `groups` collection
- VPS 日志显示 `"Bot added to group: <群组名>"`

#### 测试 9：群组中的命令

在群组中发送命令：

```
发送：/ping
```

**预期结果：**
```
🏓 Pong!
```

#### 测试 10：消息记录

```
步骤 1：在群组中发送普通文本消息
步骤 2：在群组中发送图片
步骤 3：在群组中发送文件
```

**预期结果：**
- 所有消息都被记录到数据库的 `messages` collection
- 群组统计信息更新（total_messages, last_message_at）

#### 测试 11：Bot 离开群组

在群组中发送（仅 Admin+ 权限）：

```
发送：/leave
```

**预期结果：**
- Bot 发送告别消息
- Bot 自动退出群组
- 数据库中群组记录被标记为 `left` 状态

### 错误处理测试

#### 测试 12：无效命令

```
发送：/invalidcommand
```

**预期结果：**
Bot 可能不回复或返回 "未知命令"（取决于实现）

#### 测试 13：无效参数

```
测试场景 A：发送 /grant（不带参数）
测试场景 B：发送 /grant abc（非数字参数）
测试场景 C：发送 /userinfo 99999999999999（不存在的用户）
```

**预期结果：**
每个场景都应收到相应的错误提示消息

---

## 数据库验证

### 使用 MongoDB Atlas Web UI

如果使用的是 MongoDB Atlas：

1. 登录 https://cloud.mongodb.com
2. 选择你的项目和集群
3. 点击 **Browse Collections** 按钮
4. 在 Web 界面中浏览数据

**检查项：**
- users collection 有你的用户记录，role 为 `owner`
- groups collection 有测试群组记录（如果测试了群组功能）
- messages collection 有消息记录（如果发送了消息）

---

## 故障排查

### 问题 1：Bot 不回复消息

**排查步骤：**

```bash
# 1. 检查容器是否运行
docker ps | grep go_bot
```

如果没有输出，容器未运行：

```bash
# 2. 查看容器日志查找错误
docker logs go_bot

# 3. 尝试重启容器
docker restart go_bot

# 4. 查看实时日志监控启动过程
docker logs -f go_bot
```

**常见原因：**
- ❌ `TELEGRAM_TOKEN` 配置错误
- ❌ 网络连接问题
- ❌ MongoDB 连接失败导致启动失败

### 问题 2：权限命令无效（如 /grant 不工作）

**排查步骤：**

```bash
# 检查 BOT_OWNER_IDS 环境变量是否正确设置
docker exec go_bot env | grep BOT_OWNER_IDS
```

**预期输出：**
```
BOT_OWNER_IDS=123456789
```

如果输出不正确或为空：
1. 检查 GitHub Secrets 中的 `BOT_OWNER_IDS` 配置
2. 重新部署项目

**验证你的权限：**

```bash
# 在 Telegram 向 bot 发送
/userinfo 你的_User_ID

# 确认返回的 role 是 "owner"
```

### 问题 3：数据库连接失败

**排查步骤：**

```bash
# 1. 查看日志中的数据库相关错误
docker logs go_bot | grep -i "mongo\|database\|error"

# 2. 检查 MONGO_URI 环境变量
docker exec go_bot env | grep MONGO_URI
```

**常见原因：**

❌ **MongoDB Atlas Network Access 未配置**
- 解决方法：在 Atlas 的 Network Access 中添加 `0.0.0.0/0`（允许所有 IP）

❌ **连接字符串格式错误**
- 正确格式：`mongodb+srv://username:password@cluster.mongodb.net/`
- 注意：密码中的特殊字符需要 URL 编码

❌ **数据库用户权限不足**
- 解决方法：确保数据库用户有读写权限

**测试连接：**

```bash
# 使用 mongosh 测试连接
mongosh "你的_MONGO_URI" --eval "db.adminCommand('ping')"

# 预期输出：{ ok: 1 }
```

### 问题 4：GitHub Actions 部署失败

**排查步骤：**

1. 进入 GitHub Actions 页面
2. 点击失败的 workflow run
3. 查看失败的 step

**常见失败原因：**

❌ **SSH 连接失败**
- 检查 `VPS_HOST`、`VPS_USER`、`VPS_PORT`、`SSH_KEY` 是否正确
- 确保 SSH_KEY 是完整的私钥（包括 `-----BEGIN...-----` 和 `-----END...-----`）

❌ **Docker 镜像拉取失败**
- 检查 VPS 网络连接
- 检查 GitHub Container Registry 权限

❌ **容器启动健康检查超时**
- 查看 VPS 上的容器日志：`docker logs go_bot`
- 检查环境变量配置是否完整

---

## 更新与重新测试

### 标准更新流程

```bash
# 1. 修改代码
vim internal/telegram/handlers.go

# 2. 提交代码
git add .
git commit -m "feat: 添加新功能"

# 3. 推送到 main 分支
git push origin main
```

### GitHub Actions 自动执行

推送后，GitHub Actions 会自动：
1. ✅ **CI Pipeline** - 运行 lint、测试、安全扫描
2. ✅ **CD Pipeline** - 构建 Docker 镜像并部署到 VPS

### 验证部署

```bash
# 等待 2-3 分钟后，SSH 到 VPS
ssh your_user@your_vps_host

# 查看容器更新时间（STATUS 列）
docker ps | grep go_bot

# 查看最新日志
docker logs --tail 50 go_bot

# 确认新功能生效
```

### 在 Telegram 测试新功能

根据修改的内容测试相应功能，确保：
- ✅ 新命令正常响应
- ✅ 修复的 bug 不再出现
- ✅ 现有功能未受影响

---

## 日志分析

### 查看不同级别的日志

```bash
# 查看所有 ERROR 级别日志
docker logs go_bot | grep ERROR

# 查看所有 WARN 级别日志
docker logs go_bot | grep WARN

# 查看 INFO 级别日志（正常操作）
docker logs go_bot | grep INFO

# 查看 DEBUG 级别日志（如果启用了 debug 模式）
docker logs go_bot | grep DEBUG
```

### 查看特定功能的日志

```bash
# 查看权限相关日志
docker logs go_bot | grep -i "permission\|grant\|revoke"

# 查看数据库操作日志
docker logs go_bot | grep -i "mongodb\|database\|collection"

# 查看消息处理日志
docker logs go_bot | grep -i "message\|handler"

# 查看群组相关日志
docker logs go_bot | grep -i "group\|chat"
```

### 导出日志文件

```bash
# 导出最近的日志到文件
docker logs go_bot > bot_logs.txt

# 导出最近 1000 行日志
docker logs --tail 1000 go_bot > bot_logs_1000.txt

# 下载日志到本地分析
scp your_user@your_vps_host:~/bot_logs.txt ./
```

### 实时监控日志

```bash
# 实时查看日志（持续输出）
docker logs -f go_bot

# 实时查看并过滤 ERROR
docker logs -f go_bot | grep ERROR

# 按 Ctrl+C 停止监控
```

---

## 性能监控

### 查看容器资源使用

```bash
# 查看当前资源使用情况
docker stats go_bot --no-stream

# 持续监控资源使用
docker stats go_bot
```

**预期指标：**
- **CPU 使用率：** < 10%（空闲时），< 50%（高负载）
- **内存使用：** < 100MB（取决于消息量和数据库缓存）
- **网络 I/O：** 根据消息频率变化

### 查看容器运行时间

```bash
docker ps | grep go_bot
```

**STATUS 列显示：**
- `Up 3 hours` - 容器已稳定运行 3 小时
- `Up 2 days` - 容器已稳定运行 2 天

如果频繁重启（显示 `Up 2 minutes` 等较短时间），需要检查日志查找原因。

### 检查容器健康状态

```bash
# 查看容器详细信息
docker inspect go_bot | grep -A 10 "Health"

# 查看容器退出代码（如果容器停止）
docker inspect go_bot | grep "ExitCode"
```

### 长期运行稳定性测试

```bash
# 设置定时任务，每小时检查一次容器状态
# 在 VPS 上添加 cron job
crontab -e

# 添加以下行：
0 * * * * docker ps | grep go_bot || echo "Bot is down!" | mail -s "Alert" your@email.com
```

---

## 测试检查清单

### 部署验证
- [ ] GitHub Actions CD workflow 成功 ✅
- [ ] VPS 上 `docker ps` 显示容器运行中
- [ ] `docker logs` 显示 "Application started successfully"
- [ ] 无 ERROR 级别的启动日志

### 基础功能测试
- [ ] `/start` 命令响应正常
- [ ] `/ping` 命令响应正常
- [ ] 用户信息自动注册到数据库
- [ ] `last_active_at` 自动更新

### 权限管理测试
- [ ] `/admins` 显示管理员列表
- [ ] `/userinfo <id>` 显示用户详细信息
- [ ] `/grant <id>` 成功授予管理员权限（Owner only）
- [ ] `/revoke <id>` 成功撤销管理员权限（Owner only）
- [ ] 普通用户无法使用 Owner 专属命令
- [ ] 权限检查日志正常

### 群组功能测试
- [ ] Bot 加入群组时发送欢迎消息
- [ ] 群组信息正确记录到数据库
- [ ] 群组中的命令正常响应
- [ ] `/leave` 命令让 bot 正确退出群组
- [ ] 群组统计信息正常更新

### 消息记录测试
- [ ] 文本消息正确记录
- [ ] 图片消息正确记录（含 media_file_id）
- [ ] 其他媒体消息正确记录
- [ ] 消息编辑历史正确记录
- [ ] 群组消息统计更新

### 数据库验证
- [ ] `users` collection 数据结构正确
- [ ] `groups` collection 数据结构正确
- [ ] `messages` collection 数据结构正确
- [ ] 用户 role 正确（owner/admin/user）
- [ ] 索引已自动创建

### 错误处理
- [ ] 无效命令有适当处理
- [ ] 无效参数返回错误提示
- [ ] 权限不足返回错误消息
- [ ] 用户不存在返回友好提示

### 日志检查
- [ ] 无未处理的 ERROR 日志
- [ ] 请求处理流程日志完整
- [ ] 权限检查日志正常
- [ ] 数据库操作日志正常

### 性能检查
- [ ] 容器稳定运行（无频繁重启）
- [ ] 内存使用在合理范围内
- [ ] CPU 使用在合理范围内
- [ ] 并发请求处理正常

---

## 快速命令参考

### SSH 连接与基础检查

```bash
# 连接到 VPS
ssh your_user@your_vps_host

# 一键健康检查
echo "=== 容器状态 ===" && \
docker ps | grep go_bot && \
echo -e "\n=== 最近日志 ===" && \
docker logs --tail 20 go_bot && \
echo -e "\n=== 资源使用 ===" && \
docker stats go_bot --no-stream
```

### 容器管理

```bash
# 查看容器状态
docker ps | grep go_bot

# 查看容器日志（最近 100 行）
docker logs --tail 100 go_bot

# 实时查看日志
docker logs -f go_bot

# 重启容器
docker restart go_bot

# 停止容器
docker stop go_bot

# 启动容器
docker start go_bot

# 删除容器（谨慎使用）
docker rm -f go_bot
```

### 日志分析

```bash
# 查看错误日志
docker logs go_bot | grep ERROR

# 查看警告日志
docker logs go_bot | grep WARN

# 查看权限相关日志
docker logs go_bot | grep -i "permission\|grant\|revoke"

# 查看数据库操作日志
docker logs go_bot | grep -i "mongo\|database"

# 导出日志
docker logs go_bot > bot_logs.txt
```

### 环境变量检查

```bash
# 查看所有环境变量
docker exec go_bot env

# 查看特定环境变量
docker exec go_bot env | grep -E "TELEGRAM_TOKEN|BOT_OWNER_IDS|MONGO"
```

### 数据库操作（如果安装了 mongosh）

```bash
# 连接到 MongoDB
mongosh "你的_MONGO_URI"

# 常用数据库命令
use go_bot
show collections
db.users.find().pretty()
db.groups.find().pretty()
db.messages.find().sort({sent_at: -1}).limit(10).pretty()
db.users.countDocuments()
db.messages.countDocuments()
```

### 容器资源监控

```bash
# 查看资源使用（单次）
docker stats go_bot --no-stream

# 持续监控资源
docker stats go_bot

# 查看容器详细信息
docker inspect go_bot
```

### 快速重启并监控

```bash
# 重启容器并立即查看日志
docker restart go_bot && sleep 3 && docker logs -f go_bot
```

### 紧急问题排查

```bash
# 完整的问题排查流程
echo "=== 1. 检查容器是否运行 ===" && \
docker ps | grep go_bot && \
echo -e "\n=== 2. 查看最近错误 ===" && \
docker logs go_bot | grep ERROR | tail -20 && \
echo -e "\n=== 3. 检查环境变量 ===" && \
docker exec go_bot env | grep -E "TELEGRAM|MONGO|BOT" && \
echo -e "\n=== 4. 资源使用情况 ===" && \
docker stats go_bot --no-stream
```

---

## 常见测试场景

### 场景 1：代码更新后验证部署

```bash
# 1. 确认 GitHub Actions 完成
# 访问 GitHub Actions 页面，确认绿色 ✅

# 2. SSH 到 VPS
ssh your_user@your_vps_host

# 3. 查看容器是否已更新（检查 CREATED 时间）
docker ps | grep go_bot

# 4. 查看最新日志
docker logs --tail 50 go_bot

# 5. 在 Telegram 测试新功能
```

### 场景 2：Bot 突然不响应

```bash
# 1. 检查容器状态
docker ps | grep go_bot

# 2. 如果容器未运行，查看日志
docker logs go_bot | tail -100

# 3. 重启容器
docker restart go_bot

# 4. 监控启动过程
docker logs -f go_bot

# 5. 在 Telegram 测试 /ping
```

### 场景 3：检查某个功能是否正常工作

```bash
# 例如：检查消息记录功能

# 1. 在 Telegram 发送测试消息

# 2. 查看处理日志
docker logs go_bot | grep -i "message" | tail -20

# 3. 连接 MongoDB 查看数据
mongosh "你的_MONGO_URI"
use go_bot
db.messages.find().sort({sent_at: -1}).limit(5).pretty()
```

### 场景 4：定期健康检查（每周一次）

```bash
# 1. 检查容器运行时间（应该是连续的）
docker ps | grep go_bot

# 2. 检查资源使用（确保无内存泄漏）
docker stats go_bot --no-stream

# 3. 查看是否有错误日志累积
docker logs go_bot | grep ERROR | wc -l

# 4. 在 Telegram 测试基础功能
/ping
/admins

# 5. 检查数据库增长情况
mongosh "你的_MONGO_URI" --eval "
  use go_bot
  printjson({
    users: db.users.countDocuments(),
    groups: db.groups.countDocuments(),
    messages: db.messages.countDocuments()
  })
"
```

---

## 总结

本测试指南涵盖了 Telegram Bot 的所有主要功能测试。建议：

1. **首次部署后** - 完整执行一遍所有测试项
2. **代码更新后** - 重点测试修改相关的功能
3. **定期检查** - 每周执行一次健康检查
4. **遇到问题** - 参考故障排查章节

如有问题，请检查：
- GitHub Actions 日志
- VPS 容器日志
- MongoDB 连接状态
- Telegram Bot Token 有效性

祝测试顺利！🎉
