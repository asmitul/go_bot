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

| 名称          | 说明                                     | 默认值    |
| ----------- | -------------------------------------- | ------ |
| `LOG_LEVEL` | 日志级别（支持：`debug`、`info`、`warn`、`error`） | `info` |

> 💡 设置 `LOG_LEVEL=debug` 可在开发调试时查看更详细的日志输出。

---

## 🪵 4. 日志模块

本项目日志模块位于 `internal/logger/` 目录，使用 [**logrus**](https://github.com/sirupsen/logrus) 作为日志记录库。
其支持结构化日志输出、日志级别控制、文件输出等特性，适用于开发与生产环境。


## 🗄️ 5. 数据库模块

本项目数据库模块位于 `internal/mongo/` 目录，使用 [**MongoDB 官方 Go 驱动**](https://github.com/mongodb/mongo-go-driver) 实现。

- **连接配置**：通过环境变量 `MONGO_URI` 配置数据库连接字符串（如：`mongodb+srv://<user>:<password>@cluster0.mongodb.net/<dbname>?retryWrites=true&w=majority`）
