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

---

## ⚙️ 3. 可选环境变量（Variables）

在同一页面下的 **Variables** 部分，你可以根据需要添加可选变量：

| 名称             | 说明                                                            | 默认值     |
| ---------------- | --------------------------------------------------------------- | ---------- |
| `LOG_LEVEL`      | 日志级别（支持：`debug`、`info`、`warn`、`error`）                | `info`     |
| `MONGO_DB_NAME`  | MongoDB 数据库名称。未设置时默认使用仓库名（如 `go_bot`） | 仓库名 |


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
  - 统一管理所有服务的初始化（MongoDB、未来的 Telegram Bot、Redis 等）
  - 提供优雅的资源关闭机制
  - 简化 `main.go` 入口逻辑，保持代码整洁

- **添加新服务**：当需要添加新服务时，只需在 `internal/app/app.go` 中：
  1. 在 `App` 结构体中添加服务字段（如 `TelegramBot *telegram.Bot`）
  2. 在 `New()` 函数中初始化该服务
  3. 在 `Close()` 函数中添加关闭逻辑
  4. `main.go` 无需任何改动
