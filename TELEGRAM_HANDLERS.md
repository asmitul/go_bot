# Telegram Update Handlers 完整清单

本文档记录了需要在项目中需要实现的所有 Telegram Update Handler 的详细信息。

## 概览

项目共需要实现 **16 个 Update Handler** ：


---

## 1. 基础命令以及管理员命令

### 1.1 `/start` - 用户注册与欢迎

- **文件位置**: ``
- **权限**: 所有用户
- **触发**: `/start` 命令（精确匹配）
- **主要功能**:
  - 自动注册或更新用户信息（UserService.RegisterOrUpdateUser）
  - 发送欢迎消息及可用命令列表
- **Service**: UserService
- **数据库**: 写入 users 集合

### 1.2 `/ping` - 连接测试

- **文件位置**: ``
- **权限**: 所有用户
- **触发**: `/ping` 命令（精确匹配）
- **主要功能**:
  - 更新用户活跃时间
  - 返回 "🏓 Pong!" 响应
- **Service**: UserService
- **数据库**: 更新 users.last_active_at

---

### 1.3 `/add_admin` - 授予管理员

- **文件位置**: ``
- **权限**: Owner（通过 RequireOwner 中间件）
- **触发**: 在群组中回复某人消息 `/add_admin`
- **主要功能**:
  - 授予指定用户管理员权限
  - 自动验证操作者权限、目标用户存在性、是否已是管理员
- **Service**: UserService.AddAdminPermission
- **数据库**: 更新 users.role = "admin"

### 1.4 `/delete_admin` - 撤销管理员

- **文件位置**: ``
- **权限**: Owner
- **触发**: 在群组中回复某人消息 `/delete_admin`
- **主要功能**:
  - 撤销管理员权限，降级为普通用户
  - 防止撤销 Owner 权限
- **Service**: UserService.DeleteAdminPermission
- **数据库**: 更新 users.role = "user"

### 1.5 `/admins` - 管理员列表

- **文件位置**: ``
- **权限**: Owner
- **触发**: `/admins` 精确匹配
- **主要功能**:
  - 列出所有管理员及 Owner
  - 显示角色、用户名、Telegram ID
- **Service**: UserService.ListAllAdmins
- **数据库**: 查询 users 集合（role = admin/owner）

### 1.6 `/userinfo` - 用户详情

- **文件位置**: ``
- **权限**: Admin+
- **触发**: 在群组中回复某人消息 `/userinfo`
- **主要功能**:
  - 查询用户详细信息（角色、Premium 状态、创建时间、最后活跃）
  - 显示格式化的用户档案
- **Service**: UserService.GetUserInfo
- **数据库**: 查询 users 集合

### 1.7 `/leave` - 让机器人自动离开群组

- **文件位置**: ``
- **权限**: Admin+
- **触发**: `/leave` 精确匹配
- **主要功能**:
  - 发送离别信息
  - 离开群组
- **Service**: GroupService
- **数据库**: 删除群组

### 1.8 `/configs` - 设置群组各种设置

- **文件位置**: ``
- **权限**: Admin+
- **触发**: `/configs` 精确匹配
- **主要功能**:
  - 
  - 
- **Service**: GroupService
- **数据库**: 

---

## 2. 核心交互功能

### 2.1 MyChatMember - Bot 状态变化

- **文件位置**: ``
- **权限**: 无
- **触发**: update.MyChatMember != nil
- **主要功能**:
  - Bot 被添加到群组时：创建/更新群组记录，设置 bot_status=active，发送一个消息
  - Bot 被踢出/离开时：标记 bot_status=kicked/left，发送一个消息
- **Service**: GroupService
- **数据库**: 写入/更新 groups 集合

### 2.2 MediaMessage - 媒体消息

- **文件位置**: ``
- **权限**: 无
- **触发**: Message 包含 Photo/Video/Document/Voice/Audio/Sticker/Animation
- **主要功能**:
  - 
  - 
- **Service**: MessageService.HandleMediaMessage
- **数据库**: 


### 2.3 ChannelPost - 频道消息

- **文件位置**: ``
- **权限**: Admin+
- **触发**: update.ChannelPost != nil
- **主要功能**:
  - 记录频道消息（文本或媒体）
  - 
- **Service**: ChannelService
- **数据库**: 

### 2.4 EditedChannelPost - 编辑的频道消息

- **文件位置**: ``
- **权限**: Admin+
- **触发**: update.EditedChannelPost != nil
- **主要功能**:
  - 
  - 
- **Service**: ChannelService.RecordEdit
- **数据库**: 

---

## 3. 

### 3.1 NewChatMembers - 新成员加入系统消息

- **文件位置**: ``
- **权限**: 无
- **触发**: Message.NewChatMembers != nil
- **主要功能**:
  - 
  - 
- **Service**: UserService
- **数据库**: 

### 3.2 LeftChatMember - 成员离开系统消息

- **文件位置**: ``
- **权限**: 无
- **触发**: Message.LeftChatMember != nil
- **主要功能**:
  - 
  - 
- **Service**: UserService
- **数据库**: 

---


## 4. 通用消息处理 (1个)

### 4.1 TextMessage - 普通文本消息

- **文件位置**: ``
- **权限**: 无
- **触发**: 非命令、非媒体、非系统消息的普通文本
- **主要功能**:
  - 
  - 
- **Service**: MessageService
- **数据库**: 

**过滤规则:**
- 排除 `/` 开头的命令消息
- 排除 NewChatMembers/LeftChatMember 系统消息
- 仅处理纯文本消息

### 4.2 EditedMessage - 消息编辑事件

- **文件位置**: ``
- **权限**: 无
- **触发**: update.EditedMessage != nil
- **主要功能**:
  - 
  - 
- **Service**: MessageService.RecordEdit
- **数据库**: 
---

## Handler 执行流程

所有 Handler 都遵循统一的执行模式：

```
Update 接收
    ↓
Worker Pool (asyncHandler 包装)
    ↓
权限检查中间件 (RequireOwner/RequireAdmin)
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
3. **Panic 恢复**: Worker Pool 自动捕获并记录 handler 中的 panic
4. **队列管理**: 当队列满时，新任务会被丢弃并记录警告日志
5. **优雅关闭**: Bot 关闭时，Worker Pool 等待所有运行中的任务完成

---

## 架构设计

### 分层架构

```
Handler Layer ()

Service Layer (service/)
    ↓
Repository Layer (repository/)
    ↓
MongoDB
```

**职责分离:**
- **Handler**: 解析命令参数、提取 Update 数据、调用 Service、发送响应
- **Service**: 业务验证、权限检查、业务规则、错误处理
- **Repository**: 纯数据库 CRUD 操作，不包含业务逻辑

### 权限控制

**角色层级:**
```
Owner (最高权限)
  ↓
Admin (中级权限)
  ↓
User (普通用户)
```

**中间件实现:**
- `RequireOwner(next)`: 仅允许 Owner 访问
- `RequireAdmin(next)`: 允许 Admin 及以上访问

### 消息发送助手

```go
sendMessage(ctx, chatID, text)           // 普通消息
sendErrorMessage(ctx, chatID, message)   // 错误消息 (❌ 前缀)
sendSuccessMessage(ctx, chatID, message) // 成功消息 (✅ 前缀)
```

### 数据库设计

**集合列表:**
- `users` - 用户信息
- `groups` - 群组信息
- `messages` - 消息记录

**Upsert 模式:**
- 使用 `$set` 更新已存在字段
- 使用 `$setOnInsert` 仅在插入时设置字段
- 避免重复插入错误
- 支持原子操作

---

## 扩展指南

### 添加新的 Handler

1. **创建 Handler 函数** (遵循 `bot.HandlerFunc` 签名):
```go
func (b *Bot) handleNewFeature(ctx context.Context, botInstance *bot.Bot, update *botModels.Update) {
    // 实现逻辑
}
```

2. **注册 Handler** (在 `registerHandlers()` 中):
```go
b.bot.RegisterHandler(bot.HandlerTypeMessageText, "/newcmd", bot.MatchTypeExact,
    b.asyncHandler(b.handleNewFeature))
```

3. **添加权限控制** (如需要):
```go
b.asyncHandler(b.RequireAdmin(b.handleNewFeature))
```

4. **实现 Service 方法** (如需要业务逻辑)

5. **更新本文档**

### 最佳实践

1. **Handler 职责**:
   - 仅负责参数解析和响应发送
   - 业务逻辑委托给 Service 层
   - 不直接调用 Repository

2. **错误处理**:
   - 使用 Service 层返回的用户友好错误消息
   - 通过 `sendErrorMessage` 统一发送错误
   - 记录结构化日志

3. **日志规范**:
   - 成功操作使用 `Info` 级别
   - 失败操作使用 `Error` 级别
   - 包含关键上下文（chat_id, user_id, message_id）

4. **数据库操作**:
   - 优先使用 Upsert 模式
   - 在 Service 层处理事务逻辑
   - Repository 只负责数据访问
