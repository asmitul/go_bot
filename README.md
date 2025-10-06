## 如何使用项目

### 1. 下载项目
直接从 GitHub 克隆或下载项目到本地。

### 2. 配置 GitHub Action 的 Secrets
在项目的 GitHub 仓库中设置以下 Secrets

- `TELEGRAM_TOKEN` – 你的 Telegram 机器人令牌
- `MONGO_URI` – MongoDB 数据库连接字符串
- `BOT_OWNER_IDS` – 机器人管理员 ID 列表，使用","隔开
- `VPS_HOST` – 远程服务器地址
- `VPS_USER` – 远程服务器用户名
- `VPS_PORT` – SSH 端口（默认 22）
- `SSH_KEY` – 用于连接 VPS 的私钥
