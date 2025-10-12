# Telegram Update Handlers 完整清单

本文档记录了项目中所有已实现的 Telegram Update Handler 的详细信息。

## 概览

项目共实现了 **17 个 Update Handler**：
- 8 个命令处理器（Command Handlers）
- 1 个配置回调处理器（Callback Handler）
- 8 个事件处理器（Event Handlers）


---

## 1. 命令处理器（Command Handlers）

### 1.1 `/start` - 用户注册与欢迎

- **文件位置**: `internal/telegram/handlers.go:104`
- **权限**: 所有用户
- **触发**: `/start` 命令（精确匹配 `MatchTypeExact`）
- **主要功能**:
  - 自动注册或更新用户信息（UserService.RegisterOrUpdateUser）
  - 发送欢迎消息及可用命令列表
- **Service**: UserService
- **数据库**: 写入 `users` 集合

### 1.2 `/ping` - 连接测试

- **文件位置**: `internal/telegram/handlers.go:133`
- **权限**: 所有用户
- **触发**: `/ping` 命令（精确匹配 `MatchTypeExact`）
- **主要功能**:
  - 更新用户活跃时间（UserService.UpdateUserActivity）
  - 返回 "🏓 Pong!" 响应
- **Service**: UserService
- **数据库**: 更新 `users.last_active_at`

---

### 1.3 `/grant` - 授予管理员权限

- **文件位置**: `internal/telegram/handlers.go:147`
- **权限**: Owner only（通过 `RequireOwner` 中间件）
- **触发**: `/grant <user_id>` 命令（前缀匹配 `MatchTypePrefix`）
- **参数格式**: `/grant 123456789`
- **主要功能**:
  - 授予指定用户管理员权限
  - 自动验证操作者权限、目标用户存在性、是否已是管理员
- **Service**: UserService.GrantAdminPermission
- **数据库**: 更新 `users.role = "admin"`

### 1.4 `/revoke` - 撤销管理员权限

- **文件位置**: `internal/telegram/handlers.go:178`
- **权限**: Owner only（通过 `RequireOwner` 中间件）
- **触发**: `/revoke <user_id>` 命令（前缀匹配 `MatchTypePrefix`）
- **参数格式**: `/revoke 123456789`
- **主要功能**:
  - 撤销管理员权限，降级为普通用户
  - 防止撤销 Owner 权限
- **Service**: UserService.RevokeAdminPermission
- **数据库**: 更新 `users.role = "user"`

### 1.5 `/admins` - 管理员列表

- **文件位置**: `internal/telegram/handlers.go:209`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: `/admins` 命令（精确匹配 `MatchTypeExact`）
- **主要功能**:
  - 列出所有管理员及 Owner
  - 显示角色（👑 Owner / ⭐ Admin）、用户名、Telegram ID
- **Service**: UserService.ListAllAdmins
- **数据库**: 查询 `users` 集合（role = admin/owner）

### 1.6 `/userinfo` - 用户详情

- **文件位置**: `internal/telegram/handlers.go:246`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: `/userinfo <user_id>` 命令（前缀匹配 `MatchTypePrefix`）
- **参数格式**: `/userinfo 123456789`
- **主要功能**:
  - 查询用户详细信息（角色、Premium 状态、创建时间、最后活跃）
  - 显示格式化的用户档案（包含 💎 Premium 标识）
- **Service**: UserService.GetUserInfo
- **数据库**: 查询 `users` 集合

### 1.7 `/leave` - Bot 离开群组

- **文件位置**: `internal/telegram/handlers.go:310`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: `/leave` 命令（精确匹配 `MatchTypeExact`）
- **主要功能**:
  - 验证只能在群组中使用（group/supergroup）
  - 发送离别消息："👋 再见！我将离开这个群组。"
  - 调用 GroupService.LeaveGroup 删除群组记录
  - 调用 Bot API 离开群组
- **Service**: GroupService
- **数据库**: 删除 `groups` 集合记录

### 1.8 `/configs` - 群组配置菜单

- **文件位置**: `internal/telegram/handlers_config.go:15`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: `/configs` 命令（精确匹配 `MatchTypeExact`）
- **主要功能**:
  - 显示交互式配置菜单（HTML 格式 InlineKeyboard）
  - 提供以下配置选项：
    - 欢迎消息开关（✅/❌）
    - 欢迎文本自定义（✏️ 输入模式）
    - 反垃圾设置
    - 语言设置
  - 菜单说明：✅/❌ = 开关状态，✏️ = 点击后输入，▶️ = 执行操作
- **Service**: ConfigMenuService, GroupService
- **数据库**: 查询 `groups` 集合获取当前设置

---

## 2. 配置回调处理器（Callback Handler）

### 2.1 ConfigCallback - 配置菜单回调

- **文件位置**: `internal/telegram/handlers_config.go:57`
- **权限**: Admin+（handler 内部检查 `user.IsAdmin()`）
- **触发**: `update.CallbackQuery != nil && update.CallbackQuery.Data` 以 `config:` 开头
- **回调数据格式**:
  - `config:toggle_welcome` - 切换欢迎消息开关
  - `config:edit_welcome` - 进入欢迎文本编辑模式
  - `config:close` - 关闭配置菜单
- **主要功能**:
  - 处理用户点击 InlineKeyboard 按钮的回调
  - 验证用户权限（只有管理员可操作）
  - 调用 ConfigMenuService.HandleCallback 处理业务逻辑
  - 根据操作结果更新菜单（EditMessageText）
  - 显示操作反馈（AnswerCallbackQuery）
  - 特殊操作：关闭菜单时删除消息
- **Service**: ConfigMenuService, UserService, GroupService
- **数据库**: 更新 `groups.settings`

---

## 3. 事件处理器（Event Handlers）

### 3.1 MyChatMember - Bot 状态变化

- **文件位置**: `internal/telegram/handlers.go:341`
- **权限**: 无（自动触发）
- **触发**: `update.MyChatMember != nil`（Bot 在群组中的成员状态变化）
- **主要功能**:
  - **Bot 被添加到群组**（`left/banned` → `member/administrator`）：
    - 创建/更新群组记录（设置 `bot_status=active`）
    - 调用 GroupService.HandleBotAddedToGroup
    - 发送欢迎消息："👋 你好！我是 Bot，感谢邀请我加入 {群组名}！"
  - **Bot 被踢出/离开群组**（`member/administrator` → `left/banned`）：
    - 判断原因（kicked 或 left）
    - 调用 GroupService.HandleBotRemovedFromGroup
    - 标记 `bot_status=kicked/left`
- **Service**: GroupService
- **数据库**: 写入/更新 `groups` 集合

### 3.2 MediaMessage - 媒体消息

- **文件位置**: `internal/telegram/handlers.go:448`
- **权限**: 无（自动记录所有媒体消息）
- **触发**: `update.Message` 包含 Photo/Video/Document/Voice/Audio/Sticker/Animation
- **支持的媒体类型**:
  - Photo（照片，取最大尺寸）
  - Video（视频）
  - Document（文件）
  - Voice（语音）
  - Audio（音频）
  - Sticker（贴纸）
  - Animation（GIF 动画）
- **主要功能**:
  - 自动识别媒体类型
  - 提取媒体元数据（file_id, file_size, mime_type）
  - 提取 caption（媒体说明文字）
  - 调用 MessageService.HandleMediaMessage 记录消息
- **Service**: MessageService
- **数据库**: 写入 `messages` 集合（包含 media_file_id, media_file_size, media_mime_type）

### 3.3 ChannelPost - 频道消息

- **文件位置**: `internal/telegram/handlers.go:531`
- **权限**: 无（自动记录所有频道消息）
- **触发**: `update.ChannelPost != nil`
- **主要功能**:
  - 记录频道发布的消息（文本或媒体）
  - 消息类型设置为 `channel_post`
  - 如果是媒体消息，提取 file_id（Photo/Video/Document）
  - 调用 MessageService.RecordChannelPost（user_id=0 表示频道消息）
- **Service**: MessageService
- **数据库**: 写入 `messages` 集合（`user_id=0`, `message_type=channel_post`）

### 3.4 EditedChannelPost - 编辑的频道消息

- **文件位置**: `internal/telegram/handlers.go:566`
- **权限**: 无（自动处理）
- **触发**: `update.EditedChannelPost != nil && update.EditedChannelPost.Text != ""`
- **主要功能**:
  - 更新频道消息的编辑记录
  - 提取编辑时间（EditDate）
  - 调用 MessageService.HandleEditedMessage 更新消息
- **Service**: MessageService
- **数据库**: 更新 `messages` 集合（`is_edited=true`, `edited_at=时间戳`）

### 3.5 NewChatMembers - 新成员加入

- **文件位置**: `internal/telegram/handlers.go:581`
- **权限**: 无（自动触发）
- **触发**: `update.Message.NewChatMembers != nil`
- **主要功能**:
  - 查询群组设置，检查是否启用欢迎消息（`group.settings.welcome_enabled`）
  - 如果启用，向每个新成员发送欢迎消息
  - 支持占位符替换：`{name}` → 名字，`{username}` → @用户名
  - 跳过 Bot 加入（Bot 加入由 MyChatMember 处理）
  - 记录日志：`chat_id` 和 `user_id`
- **Service**: GroupService
- **数据库**: 查询 `groups.settings.welcome_enabled` 和 `welcome_text`

### 3.6 LeftChatMember - 成员离开

- **文件位置**: `internal/telegram/handlers.go:623`
- **权限**: 无（自动触发）
- **触发**: `update.Message.LeftChatMember != nil`
- **主要功能**:
  - 记录成员离开日志（chat_id, user_id, username）
  - 当前仅记录事件，不发送离别消息
  - 预留扩展点：可添加离别消息、统计更新、事件记录等
- **Service**: 无（仅日志记录）
- **数据库**: 无

### 3.7 TextMessage - 普通文本消息

- **文件位置**: `internal/telegram/handlers.go:393`
- **权限**: 无（自动记录所有文本消息）
- **触发**: 非命令、非媒体、非系统消息的普通文本
- **过滤规则**:
  - 排除以 `/` 开头的命令消息
  - 排除 NewChatMembers/LeftChatMember 系统消息
  - 排除媒体消息（Photo/Video/Document/Voice/Audio/Sticker/Animation）
- **主要功能**:
  - **优先处理配置输入**：检查用户是否处于配置菜单的输入模式
    - 如果是，调用 ConfigMenuService.ProcessUserInput 处理输入
    - 显示成功/失败消息后直接返回，不记录为普通消息
  - **记录普通消息**：
    - 提取消息文本、reply_to_message_id、发送时间
    - 调用 MessageService.HandleTextMessage 记录消息
    - 自动更新群组统计（total_messages, last_message_at）
- **Service**: MessageService, ConfigMenuService
- **数据库**: 写入 `messages` 集合，更新 `groups.stats`

### 3.8 EditedMessage - 消息编辑事件

- **文件位置**: `internal/telegram/handlers.go:516`
- **权限**: 无（自动处理）
- **触发**: `update.EditedMessage != nil && update.EditedMessage.Text != ""`
- **主要功能**:
  - 捕获用户编辑消息的事件
  - 提取编辑后的文本和编辑时间（EditDate）
  - 调用 MessageService.HandleEditedMessage 更新消息记录
  - 标记 `is_edited=true`，记录 `edited_at` 时间戳
- **Service**: MessageService
- **数据库**: 更新 `messages` 集合（`is_edited=true`, `edited_at=时间戳`, `text=新文本`）
---

## Handler 注册与执行流程

### Handler 注册方式

所有 Handler 在 `registerHandlers()` 中注册，使用以下方式：

**精确匹配命令**（`MatchTypeExact`）：
```go
b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact,
    b.asyncHandler(b.handleStart))
```

**前缀匹配命令**（`MatchTypePrefix`）：
```go
b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/grant", bot.MatchTypePrefix,
    b.asyncHandler(b.RequireOwner(b.handleGrantAdmin)))
```

**自定义匹配函数**（`RegisterHandlerMatchFunc`）：
```go
b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
    return update.MyChatMember != nil
}, b.asyncHandler(b.handleMyChatMember))
```

### 执行流程

所有 Handler 都遵循统一的执行模式：

```
Update 接收
    ↓
Worker Pool (asyncHandler 包装)
    ↓
权限检查中间件 (RequireOwner/RequireAdmin - 可选)
    ↓
Handler 函数
    ↓
Service 层业务逻辑
    ↓
Repository 层数据访问
    ↓
MongoDB 数据库
    ↓
统一响应 (sendMessage/sendErrorMessage/sendSuccessMessage)
```

### 执行特点

1. **异步执行**: 所有 handler 通过 `asyncHandler()` 包装后提交到 Worker Pool
2. **并发处理**: Worker Pool 管理固定数量的 worker goroutine 并发处理任务
   - 默认配置：10 个 worker，队列大小 100
3. **Panic 恢复**: Worker Pool 自动捕获并记录 handler 中的 panic，发送错误消息给用户
4. **队列管理**: 当队列满时，新任务会被丢弃并记录警告日志
5. **优雅关闭**: Bot 关闭时，Worker Pool 等待所有运行中的任务完成

---

## 架构设计

### 分层架构

```
Handler Layer (handlers.go, handlers_config.go)
    ↓
Service Layer (service/)
    ↓
Repository Layer (repository/)
    ↓
MongoDB
```

**职责分离:**
- **Handler**: 解析命令参数、提取 Update 数据、调用 Service、发送响应
- **Service**: 业务验证、权限检查、业务规则、错误处理、返回用户友好的错误消息
- **Repository**: 纯数据库 CRUD 操作，不包含业务逻辑

### 权限控制

**角色层级:**
```
Owner (最高权限) - 由 BOT_OWNER_IDS 配置
  ↓
Admin (中级权限) - 由 Owner 通过 /grant 授予
  ↓
User (普通用户) - 默认角色
```

**中间件实现:**
- `RequireOwner(next)`: 仅允许 Owner 访问（/grant, /revoke）
- `RequireAdmin(next)`: 允许 Admin 及以上访问（/admins, /userinfo, /leave, /configs）

**权限检查方法** (`models/user.go`)：
- `user.IsOwner()` - 检查是否为 Owner
- `user.IsAdmin()` - 检查是否为 Admin 或 Owner
- `user.CanManageUsers()` - 检查是否可以管理用户（Owner only）

### 消息发送助手

统一的消息发送接口（`helpers.go`）：

```go
sendMessage(ctx, chatID, text)           // 普通消息
sendErrorMessage(ctx, chatID, message)   // 错误消息 (❌ 前缀)
sendSuccessMessage(ctx, chatID, message) // 成功消息 (✅ 前缀)
```

**好处**：
- 统一错误处理（自动记录发送失败日志）
- 统一 UI 表现（错误/成功消息有固定前缀）
- 简化 handler 代码

### 数据库设计

**集合列表:**
- `users` - 用户信息（telegram_id, role, username, last_active_at）
- `groups` - 群组信息（telegram_id, bot_status, settings, stats）
- `messages` - 消息记录（telegram_message_id, chat_id, user_id, message_type, text, media_*）

**核心索引:**
- `users`: `telegram_id` (唯一), `role`, `last_active_at`
- `groups`: `telegram_id` (唯一), `bot_status`
- `messages`: `telegram_message_id + chat_id` (复合唯一), `chat_id + sent_at`, `user_id + sent_at`, `message_type`

**Upsert 模式:**
- 使用 `$set` 更新已存在字段
- 使用 `$setOnInsert` 仅在插入时设置字段（如 created_at）
- 避免重复插入错误
- 支持原子操作（create 和 update 统一处理）

---

## 扩展指南

### 添加新的 Handler

#### 1. 创建 Handler 函数
遵循 `bot.HandlerFunc` 签名：
```go
func (b *Bot) handleNewFeature(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
    if update.Message == nil {
        return // 基本的 nil 检查
    }

    // 解析命令参数
    parts := strings.Fields(update.Message.Text)

    // 调用 Service 层处理业务逻辑
    if err := b.someService.DoSomething(ctx, ...); err != nil {
        b.sendErrorMessage(ctx, update.Message.Chat.ID, err.Error())
        return
    }

    // 发送成功响应
    b.sendSuccessMessage(ctx, update.Message.Chat.ID, "操作成功")
}
```

#### 2. 注册 Handler
在 `registerHandlers()` 中添加注册代码：

**命令 handler**（精确匹配）：
```go
b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/newcmd", bot.MatchTypeExact,
    b.asyncHandler(b.handleNewFeature))
```

**命令 handler**（前缀匹配，带参数）：
```go
b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/newcmd", bot.MatchTypePrefix,
    b.asyncHandler(b.handleNewFeature))
```

**事件 handler**（自定义匹配）：
```go
b.bot.RegisterHandlerMatchFunc(func(update *botModels.Update) bool {
    return update.Message != nil && update.Message.SomeField != nil
}, b.asyncHandler(b.handleNewFeature))
```

#### 3. 添加权限控制（如需要）

**Owner only**：
```go
b.asyncHandler(b.RequireOwner(b.handleNewFeature))
```

**Admin+**：
```go
b.asyncHandler(b.RequireAdmin(b.handleNewFeature))
```

**Handler 内部检查**（用于回调等特殊场景）：
```go
user, err := b.userService.GetUserInfo(ctx, userID)
if err != nil || !user.IsAdmin() {
    b.sendErrorMessage(ctx, chatID, "权限不足")
    return
}
```

#### 4. 实现 Service 方法（如需要业务逻辑）

在 `service/` 目录创建或扩展 service：
```go
func (s *SomeService) DoSomething(ctx context.Context, params ...) error {
    // 1. 业务验证
    if params == invalid {
        return fmt.Errorf("参数无效")
    }

    // 2. 调用 repository
    if err := s.repo.SaveSomething(ctx, ...); err != nil {
        logger.L().Errorf("Failed to save: %v", err)
        return fmt.Errorf("保存失败")
    }

    // 3. 记录日志
    logger.L().Infof("Something saved successfully: id=%d", id)
    return nil
}
```

#### 5. 更新本文档

在对应的 Handler 部分添加新 handler 的详细信息。

---

### 最佳实践

#### Handler 职责
- ✅ 仅负责参数解析和响应发送
- ✅ 业务逻辑委托给 Service 层
- ✅ 使用 `sendMessage` / `sendErrorMessage` / `sendSuccessMessage` 统一发送消息
- ❌ 不直接调用 Repository
- ❌ 不在 handler 中写复杂业务逻辑

#### 错误处理
- ✅ Service 层返回用户友好的中文错误消息
- ✅ 通过 `sendErrorMessage` 统一发送错误
- ✅ 记录结构化日志（包含关键上下文）
- ❌ 不向用户暴露技术细节或敏感信息

#### 日志规范
- 成功操作使用 `logger.L().Infof()`
- 失败操作使用 `logger.L().Errorf()`
- 包含关键上下文：`chat_id=%d, user_id=%d, message_id=%d`
- 示例：`logger.L().Infof("User granted admin: target_id=%d, granted_by=%d", targetID, grantedBy)`

#### 数据库操作
- ✅ 优先使用 Upsert 模式（避免处理重复插入错误）
- ✅ 在 Service 层处理事务逻辑和业务规则
- ✅ Repository 只负责数据访问（CRUD）
- ❌ 不在 Repository 中写业务验证

#### 并发安全
- 所有 handler 都通过 worker pool 异步执行
- 不需要在 handler 中手动处理 panic（worker pool 自动恢复）
- 避免在 handler 中使用全局状态（除非有适当的锁保护）

#### 用户体验
- 使用表情符号增强消息可读性（✅ ❌ 👋 等）
- 命令参数错误时，提供使用示例
- 权限不足时，提供清晰的错误提示
- 长时间操作考虑发送"处理中"提示

---

## Handler 清单总结

| # | Handler | 类型 | 权限 | 文件位置 |
|---|---------|------|------|----------|
| 1 | `/start` | 命令 | All | `handlers.go:104` |
| 2 | `/ping` | 命令 | All | `handlers.go:133` |
| 3 | `/grant` | 命令 | Owner | `handlers.go:147` |
| 4 | `/revoke` | 命令 | Owner | `handlers.go:178` |
| 5 | `/admins` | 命令 | Admin+ | `handlers.go:209` |
| 6 | `/userinfo` | 命令 | Admin+ | `handlers.go:246` |
| 7 | `/leave` | 命令 | Admin+ | `handlers.go:310` |
| 8 | `/configs` | 命令 | Admin+ | `handlers_config.go:15` |
| 9 | ConfigCallback | 回调 | Admin+ | `handlers_config.go:57` |
| 10 | MyChatMember | 事件 | 无 | `handlers.go:341` |
| 11 | MediaMessage | 事件 | 无 | `handlers.go:448` |
| 12 | ChannelPost | 事件 | 无 | `handlers.go:531` |
| 13 | EditedChannelPost | 事件 | 无 | `handlers.go:566` |
| 14 | NewChatMembers | 事件 | 无 | `handlers.go:581` |
| 15 | LeftChatMember | 事件 | 无 | `handlers.go:623` |
| 16 | TextMessage | 事件 | 无 | `handlers.go:393` |
| 17 | EditedMessage | 事件 | 无 | `handlers.go:516` |

**总计**: 17 个 Handler（8 命令 + 1 回调 + 8 事件）
