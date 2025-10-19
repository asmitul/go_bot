# 🚀 项目使用说明

## 🧩 1. 下载项目

你可以直接从 GitHub 克隆或下载本项目到本地：

```bash
git clone https://github.com/asmitul/go_bot.git
cd go_bot
```

---

## 🔐 2. 配置 GitHub Actions Secrets

在项目的 **GitHub 仓库** 中，依次进入：
`Settings` → `Secrets and variables` → `Actions` → `New repository secret`

### 必需 Secrets

添加以下密钥信息（Secrets）：

| 名称               | 说明                         |
| ---------------- | -------------------------- |
| `TELEGRAM_TOKEN` | 你的 Telegram 机器人令牌          |
| `MONGO_URI`      | MongoDB 数据库连接字符串           |
| `BOT_OWNER_IDS`  | 机器人管理员 ID 列表，使用英文逗号 `,` 分隔 |
| `VPS_HOST`       | 远程服务器地址                    |
| `VPS_USER`       | 远程服务器用户名                   |
| `VPS_PORT`       | SSH 端口（默认：`22`）            |
| `SSH_KEY`        | 用于连接 VPS 的私钥               |

### 可选 Secrets

| 名称 | 说明 |
| ---- | ---- |
| `CHANNEL_ID` | 源频道 ID，用于自动转发消息到群组。格式：`-100` 开头的 13 位数字（例如 `-1001234567890`）。不设置时转发功能不启用 |

**如何获取频道 ID**：
1. 在频道中转发一条消息到 [@userinfobot](https://t.me/userinfobot)
2. Bot 会返回频道的 ID（显示为 Origin Chat）
3. 复制该 ID 并配置到 GitHub Secrets

---

## ⚙️ 3. 可选环境变量（Variables）

在同一页面下的 **Variables** 部分，你可以根据需要添加可选变量：

| 名称             | 说明                                                            | 默认值     |
| ---------------- | --------------------------------------------------------------- | ---------- |
| `LOG_LEVEL`      | 日志级别（支持：`debug`、`info`、`warn`、`error`）                | `info`     |
| `MONGO_DB_NAME`  | MongoDB 数据库名称。未设置时默认使用仓库名（如 `go_bot`） | 仓库名 |
| `MESSAGE_RETENTION_DAYS` | 消息保留天数，过期后自动删除（最小值：1） | `7` |


---

## 🪵 4. 日志模块

本项目日志模块位于 `internal/logger/` 目录，使用 [**logrus**](https://github.com/sirupsen/logrus) 作为日志记录库。
其支持结构化日志输出、日志级别控制、文件输出等特性，适用于开发与生产环境。


## 🗄️ 5. 数据库模块

本项目数据库模块位于 `internal/mongo/` 目录，使用 [**MongoDB 官方 Go 驱动**](https://github.com/mongodb/mongo-go-driver) 实现。

- **连接配置**：通过环境变量 `MONGO_URI` 配置数据库连接字符串（如：`mongodb+srv://<user>:<password>@cluster0.mongodb.net/<dbname>?retryWrites=true&w=majority`）

## ⚙️ 6. 配置模块

本项目配置模块位于 `internal/config/` 目录，负责集中管理应用程序的环境变量配置。

- **功能**：统一加载和解析所有环境变量配置，避免在代码中直接读取环境变量
- **配置项**：
  - `TELEGRAM_TOKEN` - Telegram Bot API 令牌
  - `BOT_OWNER_IDS` - 机器人管理员 ID 列表（支持单个 ID 如 `123456789`，或逗号分隔多个 ID 如 `123456789,987654321`）
  - `MONGO_URI` - MongoDB 数据库连接字符串
  - `MONGO_DB_NAME` - MongoDB 数据库名称

---

## 🏗️ 7. 应用层模块

本项目应用层模块位于 `internal/app/` 目录，作为统一的服务初始化和生命周期管理层。

- **核心职责**：
  - 统一管理所有服务的初始化（MongoDB、Telegram Bot、Redis 等）
  - 提供优雅的资源关闭机制
  - 简化 `main.go` 入口逻辑，保持代码整洁

- **添加新服务**：当需要添加新服务时，只需在 `internal/app/app.go` 中：
  1. 在 `App` 结构体中添加服务字段（如 `TelegramBot *telegram.Bot`）
  2. 在 `New()` 函数中初始化该服务
  3. 在 `Close()` 函数中添加关闭逻辑
  4. `main.go` 无需任何改动

---

## 🤖 8. Telegram Bot 模块

本项目 Telegram Bot 模块位于 `internal/telegram/` 目录，使用 [**go-telegram/bot**](https://github.com/go-telegram/bot) 库实现。

- **架构设计**：采用 Repository + Service 模式的分层架构，包含以下子模块：
  - `models/` - 数据模型（User、Group）
  - `repository/` - 数据访问层（UserRepository、GroupRepository），负责数据库 CRUD 操作
  - `service/` - 业务逻辑层（UserService、GroupService），封装业务验证和权限检查逻辑
  - `telegram.go` - Bot 核心服务
  - `handlers.go` - 命令处理器，调用 service 层处理业务逻辑
  - `middleware.go` - 权限中间件
  - `worker_pool.go` - Worker Pool 实现，并发处理 handler 任务，带 panic recovery 和队列管理
  - `helpers.go` - 辅助函数，统一封装消息发送和错误处理

- **权限系统**：三级权限管理
  - **Owner** - 最高权限，由 `BOT_OWNER_IDS` 环境变量配置，可管理 Admin
  - **Admin** - 管理员权限，可查看用户信息、管理群组
  - **User** - 普通用户，可使用基础命令

- **支持的命令**：

| 命令 | 权限要求 | 功能说明 |
|------|----------|----------|
| `/start` | 所有用户 | 欢迎消息，自动注册用户到数据库 |
| `/ping` | 所有用户 | 测试 Bot 连接状态 |
| `/grant <user_id>` | Owner | 授予指定用户管理员权限 |
| `/revoke <user_id>` | Owner | 撤销指定用户的管理员权限 |
| `/admins` | Admin+ | 查看所有管理员列表 |
| `/userinfo <user_id>` | Admin+ | 查看指定用户的详细信息 |

- **数据库设计**：

  **users Collection**（用户信息表）
  - `telegram_id` - Telegram 用户 ID（唯一索引）
  - `username` - 用户名
  - `first_name` / `last_name` - 姓名
  - `role` - 角色（owner/admin/user）
  - `permissions` - 自定义权限列表（预留扩展）
  - `granted_by` / `granted_at` - 权限授予信息
  - `last_active_at` - 最后活跃时间

  **groups Collection**（群组信息表）
  - `telegram_id` - Telegram Chat ID（唯一索引）
  - `type` - 群组类型（group/supergroup/channel）
  - `title` - 群组名称
  - `bot_status` - Bot 状态（active/kicked/left）
  - `settings` - 群组配置（欢迎消息、反垃圾等）
  - `stats` - 群组统计信息（消息数、最后消息时间等）

- **使用示例**：

  1. **获取 Bot Token**：访问 [@BotFather](https://t.me/BotFather)，发送 `/newbot` 创建机器人，获取 Token
  2. **获取用户 ID**：向 [@userinfobot](https://t.me/userinfobot) 发送任意消息，即可获取自己的 Telegram ID
  3. **配置 Owner**：在 GitHub Secrets 中设置 `BOT_OWNER_IDS=123456789`（你的 Telegram ID）
  4. **授予管理员**：向 Bot 发送 `/grant 987654321` 即可授予其他用户管理员权限
