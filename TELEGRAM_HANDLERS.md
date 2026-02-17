# Telegram Update Handlers 完整清单

本文档记录了项目中所有已实现的 Telegram Update Handler 的详细信息。

## 概览

项目当前注册了 **30 个 Update Handler**：
- 20 个命令处理器（Command Handlers）
- 3 个回调处理器（Callback Handlers）
- 7 个事件处理器（Event Handlers）

---

## 群组分级

- **普通群 (BasicGroup)**：默认级别，仅启用基础功能
- **商户群 (MerchantGroup)**：在群内绑定商户号（`绑定 123456`）后自动升级；解绑后回落为普通群
- **上游群 (UpstreamGroup)**：在群内绑定一个或多个接口（名称 + ID + 费率，例如 `绑定接口 支付宝8888 123 7%`）后自动升级；解绑后同样回落为普通群
- 商户号与接口 ID 互斥，绑定/解绑操作均要求 Admin+，所有变更会记录日志


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
  - 当前菜单项均源自 `internal/telegram/config_definitions.go`，包括：
    - `🧮 计算器功能`（开关，默认开启）
    - `💰 USDT价格查询`（开关，默认开启）
    - `📊 USDT浮动费率`（选择 `0.00`/`0.08`/`0.09` 等，默认 `0.12`）
    - `📢 接收频道转发`（开关，默认开启）
    - `💳 收支记账`（开关，默认关闭）
    - `🏦 四方支付查询`（开关，默认开启）
    - `🔍 四方自动查单`（开关，默认开启；需先开启四方支付查询）
  - 菜单内容会根据群等级自动裁剪：普通群只看到通用开关，商户群独占四方相关选项，上游群预留专属配置
  - 按钮文本统一为 `图标 + 名称 + 状态`（✅/❌ 或选项图标）
  - 底部提供 `🔄 刷新` 与 `❌ 关闭` 快捷按钮
- **Service**: ConfigMenuService, GroupService
- **数据库**: 查询 `groups` 集合获取当前设置

---

### 1.9 `余额` - 查询四方支付账户余额

- **文件位置**: `internal/telegram/features/sifang/feature.go:92`
- **权限**: 商户群成员（需启用四方支付功能）
- **触发**: 文本消息 `余额`（精确匹配）
- **前置条件**:
  - 群组已绑定商户号（见商户号管理功能）
  - `/configs` 菜单中启用了「四方支付查询」开关
  - 部署环境配置了 `SIFANG_BASE_URL`、签名密钥等变量
- **主要功能**:
  - 调用四方支付 `/balance` 接口
  - 支持追加日期后缀（如 `余额10-30`）查询对应历史余额
  - 仅返回目标日期的余额金额，保持消息简洁
  - 在启用「🔍 四方自动查单」开关时，自动扫描群内文字消息（包含机器人消息）中的订单号并异步回复查单结果
  - 订单号提取规则：字母数字组合、长度 10-60、且至少包含 1 位数字
- **Service**: SifangService (`internal/payment/service`)
- **数据库**: 无

### 1.10 `账单` - 查询四方支付日汇总

- **文件位置**: `internal/telegram/features/sifang/feature.go:122`
- **权限**: 商户群成员（需启用四方支付功能）
- **触发**: 文本消息 `账单`、`账单10月26` 等（缺省日期时取当天）
- **前置条件**: 同 `余额` 命令，需要绑定商户号并启用「四方支付查询」开关
- **主要功能**:
  - 解析命令中的日期文本，默认当天，若指定日期晚于当前日期则回退一年
  - 日期解析与统计时间窗口统一使用北京时间（Asia/Shanghai）
  - 调用四方支付 `/summarybyday` 接口获取按日汇总数据
  - 格式化订单笔数、总金额、商户实收、代理收益等信息后返回
  - 同步返回目标日期的提款明细（含总计与逐笔列表）与余额（仅金额）
  - 当日无数据时提示“暂无账单数据”
- **自动推送**:
  - `internal/telegram/daily_summary_scheduler.go` 中的调度器会在每天北京时间 00:00:05 触发，将昨日账单推送给所有已绑定商户号且启用了「四方支付查询」功能的活跃群组
  - 通过环境变量 `DAILY_BILL_PUSH_ENABLED=false` 可关闭该功能
- **Service**: SifangService (`internal/payment/service`)
- **数据库**: 无

### 1.11 `通道账单` - 查询四方支付日通道汇总

- **文件位置**: `internal/telegram/features/sifang/feature.go:178`
- **权限**: 商户群成员（需启用四方支付功能）
- **触发**: 文本消息 `通道账单`、`通道账单10月26`
- **前置条件**: 同 `账单` 命令
- **主要功能**:
  - 解析日期（默认当天，按北京时间）
  - 调用四方支付 `/summarybydaychannel` 接口，获取按通道拆分的跑量、成交（商户实收+代理收益）、笔数
  - 返回按通道分组的明细，并附带提款明细与余额，与 `账单` 命令保持一致
  - 若接口无数据则提示“暂无通道账单数据”
- **Service**: SifangService (`internal/payment/service`)
- **数据库**: 无

### 1.12 `下发` - 发起四方支付提款

- **文件位置**: `internal/telegram/features/sifang/feature.go:555`
- **权限**: 商户群 + Admin+（通过 `UserService.CheckAdminPermission` 动态校验）
- **触发**: `下发 <金额 或 表达式> [可选6位谷歌验证码]`（如 `下发 1000`、`下发 500*3 123456`）
- **前置条件**: 群组启用了「四方支付查询」并已绑定商户号，部署环境配置四方支付 API 与签名参数
- **主要功能**:
  - 解析金额文本，支持四则运算与千分位，校验结果为正数，可附带空格分隔的 6 位谷歌验证码
  - 在内存中创建 60 秒有效的待确认请求，返回包含 `✅确认/❌取消` 的 InlineKeyboard
  - 限定只有触发命令的管理员可以操作回调；取消时清理待确认状态并提示“已取消下发…”
  - 确认后调用 `paymentService.SendMoney` 发起下发，依据 API 回包格式化成功提示或展示错误原因

### 1.12.1 `模拟下单` / `模拟创建订单` - 模拟创建四方订单

- **文件位置**: `internal/telegram/features/sifang/feature.go:614`
- **权限**: 商户群 + Admin+（通过 `UserService.CheckAdminPermission` 动态校验）
- **触发**:
  - `模拟下单 <金额> [通道代码] [订单号]`
  - `模拟创建订单 <金额> [通道代码] [订单号]`
- **前置条件**: 同 `下发` 命令（需启用四方支付、已绑定商户号、服务端已配置签名参数）
- **主要功能**:
  - 解析金额，支持四则运算表达式（与 `下发` 金额解析一致）
  - 可选透传 `channel_code` 与 `merchant_order_no`（第三个参数）
  - 调用 `paymentService.CreateOrder` 对接四方 `/createorder` 接口
  - 成功后返回商户号、订单号、金额、通道、支付链接与状态等关键信息
  - 失败时展示四方接口返回错误
- **注意事项**:
  - 该命令会真实写入订单，请在测试环境或小额场景使用

### 1.13 `费率` - 查询四方支付通道状态

- **文件位置**: `internal/telegram/features/sifang/feature.go:586`
- **权限**: 商户群成员（需启用四方支付功能）
- **触发**: 文本消息 `费率`
- **前置条件**: 同 `账单` 命令
- **主要功能**:
  - 调用四方支付 `/channelstatus` 接口，读取通道启用状态、费率与限额信息
  - 仅展示系统与商户皆启用的通道（过滤未开通通道）
  - 过滤通道代码以 `test` 结尾的测试通道，保持 API 返回顺序
  - 将费率统一转为百分比字符串并以 `<pre>` 保持表格对齐
- **Service**: SifangService (`internal/payment/service`)
- **数据库**: 无

### 1.14 `提款明细` - 查询四方支付提现列表

- **文件位置**: `internal/telegram/features/sifang/feature.go:258`
- **权限**: 商户群成员（需启用四方支付功能）
- **触发**: 文本消息 `提款明细`、`提款明细10月26`
- **前置条件**: 同 `账单` 命令
- **主要功能**:
  - 解析日期（默认当天，按北京时间）
  - 调用四方支付 `/withdrawlist` 接口，默认查询当天前 20 条提现记录
  - 格式化展示提现单号、订单号、金额、手续费、状态、时间以及通道信息；无记录时提示“暂无提款记录”
- **Service**: SifangService (`internal/payment/service`)
- **数据库**: 无

### 1.15 `查询记账` - 拉取账单

- **文件位置**: `internal/telegram/handlers.go:744`
- **权限**: 所有群成员
- **触发**: 文本消息 `查询记账`（精确匹配）
- **主要功能**:
  - 确保当前群组存在并启用收支记账功能（GroupService.GetOrCreateGroup）
  - 通过 AccountingService 查询当日收支明细并格式化输出
- **Service**: GroupService, AccountingService
- **数据库**: 读取 `groups.settings.accounting_enabled`、`accounting_records`

### 1.16 `删除记账记录` - 打开删除菜单

- **文件位置**: `internal/telegram/handlers.go:780`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: 文本消息 `删除记账记录`
- **主要功能**:
  - 校验群组已启用记账功能
  - 构建最近两天的记账记录列表并以 InlineKeyboard 展示
  - 每个按钮携带 `acc_del:<record_id>` 回调数据
- **Service**: GroupService, AccountingService
- **数据库**: 读取 `accounting_records`

### 1.17 `清零记账` - 清空账本

- **文件位置**: `internal/telegram/handlers.go:932`
- **权限**: Admin+（通过 `RequireAdmin` 中间件）
- **触发**: 文本消息 `清零记账`
- **主要功能**:
  - 校验群组已启用记账功能
  - 调用 AccountingService.ClearAllRecords 删除该群全部记账记录
  - 返回成功提示并显示删除数量
- **Service**: GroupService, AccountingService
- **数据库**: 删除 `accounting_records`

### 1.18 `撤回` - 管理员引用撤回机器人消息

- **文件位置**: `internal/telegram/handlers.go:515`
- **权限**: Admin+（动态校验 `CheckAdminPermission`）
- **触发**: 管理员回复本机器人发送的消息并输入 `撤回`
- **主要功能**:
  - 校验触发者是否拥有管理员权限
  - 确认被引用消息的发送者是当前 Bot（`bot.ID()`）
  - 删除被引用的机器人消息
  - 删除触发命令的管理员消息（失败时仅记日志，不影响主流程）
- **Service**: UserService（权限校验）
- **数据库**: 无

### 1.19 `/validate` - 群组数据体检（Owner）

- **文件位置**: `internal/telegram/handlers.go:304`
- **权限**: Owner only（通过 `RequireOwner` 中间件）
- **触发**: `/validate` 命令（精确匹配，建议在 Owner 私聊中使用）
- **主要功能**:
  - 调用 `GroupService.ValidateGroups`（`internal/telegram/service/group_validation.go`）遍历 `groups` 集合
  - 统计总群组数量与存在异常的群组数量
  - 列出最多 10 个有问题的群组，展示 `tier`、`bot_status` 与逐条问题描述（如缺少 `tier` 字段、配置冲突等）
  - 若异常群组超过 10 个，会提示剩余数量，方便登录数据库继续排查
- **Service**: GroupService
- **数据库**: 全量读取 `groups` 集合用于校验

### 1.20 `/repair` - 自动修复群组配置（Owner）

- **文件位置**: `internal/telegram/handlers.go:359`
- **权限**: Owner only
- **触发**: `/repair` 命令（精确匹配）
- **主要功能**:
  - 调用 `GroupService.RepairGroups`（`internal/telegram/service/group_repair.go`）扫描全部群组
  - 自动补写缺失或错误的 `tier`，将 `tier` 与 `GroupSettings` 当前状态保持一致
  - 若群组关闭了「🏦 四方支付查询」却仍开启「🔍 四方自动查单」，会自动关闭查单开关
  - 汇总修复数量（成功写入 / 跳过 / tier 修复 / 自动查单关闭）并返回
- **Service**: GroupService
- **数据库**: 读取并按需更新 `groups` 集合

---

## 2. 配置回调处理器（Callback Handler）

### 2.1 ConfigCallback - 配置菜单回调

- **文件位置**: `internal/telegram/handlers_config.go:77`
- **权限**: Admin+（handler 内部检查 `user.IsAdmin()`）
- **触发**: `update.CallbackQuery != nil && strings.HasPrefix(data, "config:")`
- **回调数据格式**（`config:<type>:<id>` 或专用指令）：
  - `config:toggle:calculator_enabled` / `config:toggle:accounting_enabled`
  - `config:select:crypto_float_rate`
  - `config:refresh`、`config:close`
  - 输入型/动作型保留扩展：`config:input:<id>` / `config:action:<id>`
- **主要功能**:
  - 处理用户点击 InlineKeyboard 按钮的回调
  - 验证用户权限（只有管理员可操作）
  - 调用 ConfigMenuService.HandleCallback 处理业务逻辑
  - 根据操作结果更新菜单（EditMessageText）
  - 显示操作反馈（AnswerCallbackQuery）
  - 特殊操作：关闭菜单时删除消息
- **Service**: ConfigMenuService, UserService, GroupService
- **数据库**: 更新 `groups.settings`

### 2.2 ForwardRecallCallback - 频道转发撤回

- **文件位置**: `internal/telegram/handlers.go:725`（入口），实际处理在 `internal/telegram/forward/handlers.go`
- **权限**: Admin+（通过 ForwardService 内部校验）
- **触发**: `recall:<task_id>`、`recall_confirm:<task_id>`、`recall_cancel`
- **主要功能**:
  - 入口 handler 将回调转交给 ForwardService
  - `recall:` 展示二次确认按钮，`recall_confirm:` 执行撤回并展示结果，`recall_cancel` 还原按钮
- **Service**: ForwardService
- **数据库**: 更新/删除 `forward_records`

### 2.3 AccountingDeleteCallback - 删除记账记录

- **文件位置**: `internal/telegram/handlers.go:933`
- **权限**: Admin+（间接依赖前置命令）
- **触发**: `acc_del:<record_id>`
- **主要功能**:
  - 调用 AccountingService.DeleteRecord 删除对应记录
  - 使用 AnswerCallbackQuery 返回结果
  - 删除成功后自动发送最新账单
- **Service**: AccountingService
- **数据库**: 删除 `accounting_records`

---

## 3. 事件处理器（Event Handlers）

### 3.1 MyChatMember - Bot 状态变化

- **文件位置**: `internal/telegram/handlers.go:363`
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

- **文件位置**: `internal/telegram/handlers.go:567`
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
  - 若开启「🏦 四方支付查询」与「🔍 四方自动查单」，从 caption/文件名提取订单号并交由 Sifang Feature 异步查单（包含机器人消息）
- **Service**: MessageService
- **数据库**: 写入 `messages` 集合（包含 media_file_id, media_file_size, media_mime_type）

### 3.3 ChannelPost - 频道消息

- **文件位置**: `internal/telegram/handlers.go:650`
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

- **文件位置**: `internal/telegram/handlers.go:692`
- **权限**: 无（自动处理）
- **触发**: `update.EditedChannelPost != nil && update.EditedChannelPost.Text != ""`
- **主要功能**:
  - 更新频道消息的编辑记录
  - 提取编辑时间（EditDate）
  - 调用 MessageService.HandleEditedMessage 更新消息
- **Service**: MessageService
- **数据库**: 更新 `messages` 集合（`is_edited=true`, `edited_at=时间戳`）

### 3.5 LeftChatMember - 成员离开

- **文件位置**: `internal/telegram/handlers.go:707`
- **权限**: 无（自动触发）
- **触发**: `update.Message.LeftChatMember != nil`
- **主要功能**:
  - 记录成员离开日志（chat_id, user_id, username）
  - 当前仅记录事件，不发送离别消息
  - 预留扩展点：可添加离别消息、统计更新、事件记录等
- **Service**: 无（仅日志记录）
- **数据库**: 无

### 3.6 TextMessage - 普通文本消息

- **文件位置**: `internal/telegram/handlers.go:417`
- **权限**: 无（自动记录所有文本消息）
- **触发**: 非命令、非媒体、非系统消息的普通文本
- **过滤规则**:
  - 排除以 `/` 开头的命令消息
  - 排除 NewChatMembers/LeftChatMember 系统消息
  - 排除媒体消息（Photo/Video/Document/Voice/Audio/Sticker/Animation）
- **主要功能**（按优先级顺序）:
  1. **管理员撤回命令**：`tryHandleRecallCommand` 拦截管理员回复的 `撤回`
     - 仅允许 Admin+ 操作
     - 只处理由本 Bot 发送的消息，并尝试删除触发命令
  2. **配置输入处理**：检查用户是否处于配置菜单的输入模式
     - 如果是，调用 ConfigMenuService.ProcessUserInput 处理输入
     - 显示成功/失败消息后直接返回，不记录为普通消息
  3. **功能插件处理** (Feature Manager)：
     - 调用 FeatureManager.Process() 按优先级执行所有已启用的功能插件
     - 已实现的功能插件：
      - **计算器**（优先级 20）：检测数学表达式并返回计算结果
      - **商户号管理**（优先级 15）：解析“绑定 123456”/“解绑”等命令
      - **接口管理**（优先级 16）：解析“绑定接口 [接口名称] [接口ID] [费率]”/“解绑接口 [接口ID]”等命令，可为上游群维护带名称和费率的接口列表，仅在普通/上游群启用
      - **上游账单查询**（优先级 18）：匹配「上游账单[ 接口ID ][ 日期 ]」，调用 `/summarybydaypzid` 为绑定的接口 ID 拉取按日汇总，仅在上游群启用
        - 命令格式：`上游账单 [接口ID或名称] [可选日期]`，日期留空默认当天，北京时间
      - **四方支付查询**（优先级 25）：显式指令（如 `余额`、`下发`、`模拟下单`）与自动订单查单
      - **USDT 价格查询**（优先级 30）：解析 OKX 指令（如 `z3 100`）
     - 功能可声明允许的群等级，Feature Manager 会自动依据群级别选择性启用
     - 如果任何功能返回 `handled=true`，停止后续处理，不记录为普通消息
     - 功能插件可通过 `/configs` 菜单在群组中启用/禁用
  4. **记录普通消息**：
     - 提取消息文本、reply_to_message_id、发送时间
     - 调用 MessageService.HandleTextMessage 记录消息
     - 自动更新群组统计（total_messages, last_message_at）
- **Service**: ConfigMenuService → FeatureManager → MessageService
- **数据库**: 写入 `messages` 集合，更新 `groups.stats`
- **处理流程**:
  ```
  TextMessage
      ↓
  ConfigMenuInput 检查 → 如果是输入模式 → 处理并返回
      ↓
  Feature Manager → 按优先级执行功能插件 → 如果 handled=true → 返回
      ↓
  记录普通消息到数据库
  ```

### 3.7 EditedMessage - 消息编辑事件

- **文件位置**: `internal/telegram/handlers.go:635`
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
	Feature Manager (仅 TextMessage handler - 可选)
	    ├── Calculator Feature (检测数学表达式)
	    ├── Merchant Feature (商户号管理)
	    ├── Upstream Feature (接口 ID 管理)
	    ├── Sifang Feature (四方支付查询)
	    └── ... 其他功能插件
    ↓
Service 层业务逻辑
    ↓
Repository 层数据访问
    ↓
MongoDB 数据库
    ↓
统一响应 (sendMessage/sendErrorMessage/sendSuccessMessage)
```

**说明**：
- Feature Manager 仅在 TextMessage handler 中使用
- 按优先级顺序执行功能插件（优先级低的数字先执行）
- 如果任何功能返回 `handled=true`，停止后续流程
- 功能插件可通过配置系统在群组中启用/禁用
- 若群类型不在 Feature 声明的 `AllowedGroupTiers` 内，Feature Manager 会直接返回提示并阻断后续插件执行
- 功能可实现 `AllowedGroupTiers()` 接口指定可用的群等级；不符合等级的群会自动跳过该功能

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
Feature Plugin Layer (features/) [仅 TextMessage handler]
    ├── Feature Manager
    ├── Calculator Feature
    ├── Merchant Feature
    ├── Upstream Feature
    ├── Sifang Feature
    └── ... 更多功能插件
    ↓
Service Layer (service/)
    ↓
Repository Layer (repository/)
    ↓
MongoDB
```

**职责分离:**
- **Handler**: 解析命令参数、提取 Update 数据、调用 Service、发送响应
- **Feature Plugin**: 处理基于消息的功能（计算器、支付查询等），独立可插拔
  - 每个功能实现 Feature 接口（Name, Enabled, Match, Process, Priority）
  - Feature Manager 按优先级顺序执行所有已启用且匹配的功能
  - 功能可通过群组配置动态启用/禁用
  - 可选实现 `AllowedGroupTiers()` 以限制功能只在指定群等级响应
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

### 添加新的 Feature Plugin

Feature Plugin 系统允许你添加基于消息的功能（如计算器、支付查询等），无需修改 handler 代码。

#### 1. 创建 Feature 包

在 `internal/telegram/features/` 下创建新功能目录：
```bash
mkdir -p internal/telegram/features/<feature_name>
```

#### 2. 实现 Feature 接口

创建 `feature.go` 并实现 Feature 接口，例如一个简单的关键字回复功能：

```go
// internal/telegram/features/example/feature.go
package example

import (
    "context"
    "strings"

    "go_bot/internal/logger"
    "go_bot/internal/telegram/models"
    botModels "github.com/go-telegram/bot/models"
)

type Feature struct{}

func New() *Feature {
    return &Feature{}
}

func (f *Feature) Name() string {
    return "example"
}

func (f *Feature) Enabled(ctx context.Context, group *models.Group) bool {
    return true // 或根据群组配置开关控制
}

func (f *Feature) Match(ctx context.Context, msg *botModels.Message) bool {
    return strings.HasPrefix(msg.Text, "例子 ")
}

func (f *Feature) Process(ctx context.Context, msg *botModels.Message) (string, bool, error) {
    logger.L().Infof("Example feature triggered: chat_id=%d", msg.Chat.ID)
    return "示例功能已触发", true, nil
}

func (f *Feature) Priority() int {
    return 40
}
```

根据需要替换 `Match` 和 `Process` 中的逻辑。

#### 3. 注册 Feature

在 `internal/telegram/telegram.go` 的 `registerFeatures()` 中注册新功能：

```go
func (b *Bot) registerFeatures() {
    b.featureManager.Register(calculator.New())
    b.featureManager.Register(example.New())

    logger.L().Infof("Registered %d features: %v", len(b.featureManager.ListFeatures()), b.featureManager.ListFeatures())
}
```

并在文件顶部添加 import：
```go
import (
    "go_bot/internal/telegram/features/example"
)
```

#### 4. 添加配置字段（可选）

若功能需要可配置开关，可在 `models/group.go` 等位置添加布尔字段，并在 `config_definitions.go` 注册对应的配置项，逻辑与现有功能保持一致。

#### 5. 添加测试（推荐）

为 Feature 编写单元测试，覆盖 `Match` 和 `Process` 的关键路径，保持测试风格与现有功能一致。

#### 6. 删除 Feature

移除注册调用并清理多余的配置或依赖即可：
```go
// b.featureManager.Register(example.New())
```

#### Feature 优先级指南

- **1-20**: 高优先级（商户号/接口 ID 管理、数学计算等需要优先消费的命令）
- **21-50**: 中优先级（价格查询等扩展功能）
- **51-100**: 低优先级（AI 对话、关键词回复等可选功能）

优先级低的数字先执行，避免低优先级功能抢占高优先级功能的消息。

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
| 1 | `/start` | 命令 | All | `handlers.go:123` |
| 2 | `/ping` | 命令 | All | `handlers.go:152` |
| 3 | `/grant` | 命令 | Owner | `handlers.go:166` |
| 4 | `/revoke` | 命令 | Owner | `handlers.go:197` |
| 5 | `/admins` | 命令 | Admin+ | `handlers.go:228` |
| 6 | `/userinfo` | 命令 | Admin+ | `handlers.go:265` |
| 7 | `/leave` | 命令 | Admin+ | `handlers.go:332` |
| 8 | `/configs` | 命令 | Admin+ | `handlers_config.go:18` |
| 9 | `查询记账` | 命令 | All | `handlers.go:805` |
| 10 | `删除记账记录` | 命令 | Admin+ | `handlers.go:842` |
| 11 | `清零记账` | 命令 | Admin+ | `handlers.go:981` |
| 12 | `撤回` | 命令 | Admin+ | `handlers.go:438` / `handlers.go:515` |
| 13 | ConfigCallback | 回调 | Admin+ | `handlers_config.go:77` |
| 14 | ForwardRecallCallback | 回调 | Admin+ | `handlers.go:725` / `forward/handlers.go` |
| 15 | AccountingDeleteCallback | 回调 | Admin+ | `handlers.go:933` |
| 16 | MyChatMember | 事件 | 无 | `handlers.go:363` |
| 17 | TextMessage | 事件 | 无 | `handlers.go:417` |
| 18 | MediaMessage | 事件 | 无 | `handlers.go:567` |
| 19 | ChannelPost | 事件 | 无 | `handlers.go:650` |
| 20 | EditedChannelPost | 事件 | 无 | `handlers.go:692` |
| 21 | LeftChatMember | 事件 | 无 | `handlers.go:707` |
| 22 | EditedMessage | 事件 | 无 | `handlers.go:635` |

**总计**: 22 个 Handler（12 命令 + 3 回调 + 7 事件）
