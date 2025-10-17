# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Telegram bot built with Go 1.25, using MongoDB for data storage and deployed entirely via GitHub Actions to a VPS. The project follows cloud-native development practices with automated CI/CD pipelines.

## Deployment Workflow

All deployments are automated through GitHub Actions. The workflow is:

1. **On Pull Request** → `.github/workflows/ci.yml` runs:
   - Code linting (golangci-lint)
   - Unit tests with race detection and coverage
   - Integration tests (with MongoDB service)
   - Security scans (Gosec, Trivy)
   - Docker build verification

2. **On Push to `main`** → `.github/workflows/cd.yml` runs:
   - Builds multi-stage Docker image
   - Pushes to GitHub Container Registry (ghcr.io)
   - Deploys to VPS via SSH with automatic rollback on failure

## Required GitHub Secrets

Configure these in repository Settings → Secrets and variables → Actions → Secrets:

| Secret | Description |
|--------|-------------|
| `TELEGRAM_TOKEN` | Telegram bot API token |
| `MONGO_URI` | MongoDB connection string |
| `BOT_OWNER_IDS` | Comma-separated list of admin Telegram user IDs |
| `VPS_HOST` | VPS server hostname/IP |
| `VPS_USER` | SSH username for VPS |
| `VPS_PORT` | SSH port (default: 22) |
| `SSH_KEY` | Private SSH key for VPS authentication |

## Optional GitHub Variables

Configure these in Settings → Secrets and variables → Actions → Variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging level: `debug`, `info`, `warn`, `error` | `info` |
| `MONGO_DB_NAME` | MongoDB database name | Repository name |
| `MESSAGE_RETENTION_DAYS` | Message retention period (days) before auto-deletion | `7` |

## Architecture

### Project Structure

```
go_bot/
├── cmd/bot/               # Application entry point
│   └── main.go           # Initializes app and starts services
├── internal/             # Private application packages
│   ├── app/             # Application layer - service initialization & lifecycle
│   ├── config/          # Configuration management from env variables
│   ├── logger/          # Logrus-based logging with env config
│   ├── mongo/           # MongoDB client wrapper
│   └── telegram/        # Telegram bot service
│       ├── models/      # Data models (User, Group, Message)
│       │   ├── user.go
│       │   ├── group.go
│       │   └── message.go
│       ├── repository/  # Data access layer with interfaces
│       │   ├── user.go
│       │   ├── group.go
│       │   ├── message.go
│       │   └── interfaces.go
│       ├── service/     # Business logic layer
│       │   ├── interfaces.go
│       │   ├── user_service.go
│       │   ├── group_service.go
│       │   └── message_service.go
│       ├── telegram.go  # Bot core service
│       ├── handlers.go  # Command handlers
│       ├── middleware.go # Permission middlewares
│       ├── worker_pool.go # Concurrent handler execution
│       └── helpers.go   # Message sending utilities
└── deployments/docker/  # Multi-stage Dockerfile
```

### Logger Package (`internal/logger`)

- Uses [logrus](https://github.com/sirupsen/logrus) for structured logging
- Configurable via `LOG_LEVEL` environment variable
- Access logger with `logger.L()` after calling `logger.Init()`
- Supports levels: debug, info, warn, error
- Defaults to `info` level if not configured

### Config Package (`internal/config`)

- Centralized configuration management from environment variables
- `Load()` function reads and parses all environment variables into `Config` struct
- Validates and parses `BOT_OWNER_IDS` (supports comma-separated list)
- Configuration fields: `TelegramToken`, `BotOwnerIDs`, `MongoURI`, `MongoDBName`, `MessageRetentionDays`
- `MessageRetentionDays` validation: must be >= 1 day (default: 7 days)
- Use `config.Load()` once in `main.go`, then pass to services

### MongoDB Package (`internal/mongo`)

- Wraps MongoDB official Go driver
- `Config` struct validates required fields (URI, Database, Timeout)
- `NewClient(cfg)` creates client with automatic connection validation
- `InitFromConfig(appCfg)` convenience function - accepts `*config.Config` and handles conversion
- `Client.Database()` returns database handle
- `Client.Ping(ctx)` verifies connection health
- Connection timeout defaults to 10 seconds if not specified

### Telegram Package (`internal/telegram`)

- Telegram bot service using [go-telegram/bot](https://github.com/go-telegram/bot) library
- Implements Repository + Service pattern with layered architecture for clean separation of concerns
- Uses worker pool for concurrent handler execution with panic recovery
- Supports long polling mode (bot runs in goroutine, blocking until context cancellation)

**Architecture Components:**

- **models/** - Data models with business logic
  - `User` struct with role-based permissions (Owner/Admin/User)
  - `Group` struct with settings and statistics
  - `Message` struct for recording all telegram messages (text/media/channel posts)
  - Permission check methods (`IsOwner()`, `IsAdmin()`, `CanManageUsers()`)
  - Message type constants (MessageTypeText, MessageTypePhoto, MessageTypeVideo, etc.)

- **repository/** - Data access layer (CRUD operations)
  - Defines repository interfaces in `repository/interfaces.go`
  - `UserRepository`: CreateOrUpdate, GetByTelegramID, UpdateLastActive, GrantAdmin, RevokeAdmin, ListAdmins, GetUserInfo
  - `GroupRepository`: CreateOrUpdate, GetByTelegramID, MarkBotLeft, DeleteGroup, ListActiveGroups, UpdateSettings, UpdateStats
  - `MessageRepository`: CreateMessage, GetByTelegramID, UpdateMessageEdit, ListMessagesByChat, CountMessagesByType
  - All repositories provide `EnsureIndexes()` for automatic index creation
  - Pure data access layer - no business logic or validation

- **service/** - Business logic layer (Service Layer)
  - Encapsulates business validation and permission checks, separating business logic from handlers
  - **Interface definitions** (`service/interfaces.go`):
    - `UserService`: RegisterOrUpdateUser, GrantAdminPermission, RevokeAdminPermission, GetUserInfo, ListAllAdmins, CheckOwnerPermission, CheckAdminPermission, UpdateUserActivity
    - `GroupService`: CreateOrUpdateGroup, GetGroupInfo, MarkBotLeft, ListActiveGroups, UpdateGroupSettings, LeaveGroup, HandleBotAddedToGroup, HandleBotRemovedFromGroup
    - `MessageService`: HandleTextMessage, HandleMediaMessage, HandleEditedMessage, RecordChannelPost, GetChatMessageHistory
    - DTOs: `TelegramUserInfo`, `TextMessageInfo`, `MediaMessageInfo`, `ChannelPostInfo`
  - **UserService implementation** (`service/user_service.go`):
    - `RegisterOrUpdateUser(info)`: Converts DTO to model, calls repository, logs operation
    - `GrantAdminPermission(targetID, grantedBy)`: Validates granter is Owner → checks target exists → verifies not already admin → executes grant → logs
    - `RevokeAdminPermission(targetID, revokedBy)`: Validates revoker is Owner → prevents revoking Owner → checks current state → executes revoke → logs
    - `CheckOwnerPermission(telegramID)`: Queries user and checks Owner role
    - `CheckAdminPermission(telegramID)`: Queries user and checks Admin+ role
    - All methods include comprehensive error handling and structured logging (Info for success, Error for failures)
    - Returns user-friendly Chinese error messages for direct display to users
  - **GroupService implementation** (`service/group_service.go`):
    - Wraps repository operations with error handling and logging
    - `UpdateGroupSettings`: Updates group welcome message, anti-spam settings, language
    - `LeaveGroup`: Validates group exists, deletes group record when bot leaves
    - `HandleBotAddedToGroup`: Creates/updates group record when bot joins, sets status to active
    - `HandleBotRemovedFromGroup`: Marks group as kicked/left based on reason
  - **MessageService implementation** (`service/message_service.go`):
    - `HandleTextMessage`: Records plain text messages, auto-updates group stats (total messages, last message time)
    - `HandleMediaMessage`: Records photo/video/document/voice/audio/sticker/animation messages
    - `HandleEditedMessage`: Updates message edit history with edited_at timestamp
    - `RecordChannelPost`: Records channel posts (user_id=0 for channel messages)
    - `GetChatMessageHistory`: Retrieves paginated message history for a chat
  - **Responsibility separation from repository**:
    - Repository layer: Pure database CRUD, no business validation
    - Service layer: Business validation, permission checks, business rules, error handling
    - Handlers should call service methods, not repository directly

- **telegram.go** - Core bot service
  - `New(cfg, db)` creates bot instance, registers handlers, initializes indexes
  - `InitFromConfig(appCfg, db)` convenience function
  - `Start(ctx)` runs bot in blocking mode (call in goroutine)
  - `initOwners(ctx)` auto-creates owner users from `BOT_OWNER_IDS` config

- **handlers.go** - Command and event handlers
  - All handlers follow `bot.HandlerFunc` signature: `func(ctx, *bot.Bot, *models.Update)`
  - Handlers call service layer for business logic (e.g., `userService.GrantAdminPermission()`, `messageService.HandleTextMessage()`)
  - All handlers registered with `asyncHandler()` wrapper for concurrent execution via worker pool
  - Handler responsibilities: parse command arguments, call service methods, send responses via helpers
  - **Command handlers**: /start, /ping, /grant, /revoke, /admins, /userinfo, /leave
  - **Event handlers**: MyChatMember (bot status change), EditedMessage, ChannelPost, NewChatMembers, LeftChatMember
  - **Message handlers**: TextMessage (plain text), MediaMessage (photo/video/document/voice/audio/sticker/animation)

- **middleware.go** - Permission control
  - `RequireOwner(next)` wraps handlers requiring owner permission
  - `RequireAdmin(next)` wraps handlers requiring admin+ permission
  - Middlewares send error messages and log unauthorized attempts

- **worker_pool.go** - Worker Pool for concurrent handler execution
  - Implements goroutine pool pattern for processing handler tasks concurrently
  - **Core components**:
    - `HandlerTask`: Encapsulates handler execution context (ctx, bot instance, update, handler function)
    - `WorkerPool`: Manages fixed number of worker goroutines and task queue
  - **Configuration parameters**:
    - `workers`: Number of worker goroutines (concurrency level)
    - `queueSize`: Task queue buffer size (max pending tasks)
  - **Key features**:
    - **Panic recovery**: Automatically catches and logs panics in handlers, sends error message to user
    - **Queue management**: Non-blocking Submit() - drops tasks and logs warning when queue is full
    - **Graceful shutdown**: `Shutdown()` closes queue, waits for all running tasks to complete
  - **Usage**: Bot wraps all handlers with `asyncHandler()` which submits tasks to worker pool
  - **Performance**: Improves bot responsiveness by handling multiple updates concurrently

- **helpers.go** - Message sending utilities
  - Provides unified message sending helpers to avoid code duplication and ensure consistency
  - **Functions**:
    - `sendMessage(ctx, chatID, text)`: Base message sender, automatically logs send failures
    - `sendErrorMessage(ctx, chatID, message)`: Sends error message with ❌ prefix
    - `sendSuccessMessage(ctx, chatID, message)`: Sends success message with ✅ prefix
  - **Benefits**: Consistent error handling, unified UI presentation, simplified handler code

**Permission System:**

- **Owner** (highest) - Set via `BOT_OWNER_IDS` env var, can manage admins
- **Admin** - Can view user info, manage groups, list admins
- **User** (default) - Can use basic commands (/start, /ping)

**Database Collections:**

- **users** collection
  - `telegram_id` (int64, unique index) - Telegram user ID
  - `role` (string, index) - owner/admin/user
  - `username`, `first_name`, `last_name` - User profile
  - `granted_by`, `granted_at` - Permission grant tracking
  - `last_active_at` (time, index) - Last activity timestamp

- **groups** collection
  - `telegram_id` (int64, unique index) - Telegram chat ID
  - `type` (string, index) - group/supergroup/channel
  - `bot_status` (string, index) - active/kicked/left
  - `settings` (embedded) - WelcomeEnabled, AntiSpam, Language
  - `stats` (embedded) - TotalMessages, LastMessageAt

- **messages** collection
  - `telegram_message_id + chat_id` (composite unique index) - Message identifier
  - `user_id` (int64, index) - Sender ID (0 for channel posts)
  - `message_type` (string, index) - text/photo/video/document/voice/audio/sticker/animation/channel_post
  - `text`, `caption` - Message content
  - `media_file_id`, `media_file_size`, `media_mime_type` - Media metadata
  - `reply_to_message_id`, `forward_from_chat_id` - Message relationships
  - `is_edited`, `edited_at` - Edit tracking
  - `sent_at` (time, index with TTL) - Message timestamp
  - Indexes: `chat_id + sent_at` (chat history), `user_id + sent_at` (user messages), `message_type` (statistics)
  - **TTL Index**: `sent_at` field with `expireAfterSeconds` = `MESSAGE_RETENTION_DAYS * 86400`
    - Messages automatically deleted by MongoDB after retention period expires
    - Default: 7 days (604800 seconds)
    - MongoDB background task checks every 60 seconds for expired documents
    - Zero maintenance cost - fully automated by database engine

**Supported Commands:**

| Command | Permission | Description |
|---------|------------|-------------|
| `/start` | All users | Welcome message, auto-register user |
| `/ping` | All users | Test bot connectivity |
| `/grant <user_id>` | Owner only | Grant admin permission to user |
| `/revoke <user_id>` | Owner only | Revoke admin permission from user |
| `/admins` | Admin+ | List all administrators |
| `/userinfo <user_id>` | Admin+ | View detailed user information |
| `/leave` | Admin+ | Bot leaves the group and deletes group record |

**Supported Event Handlers:**

| Event | Description |
|-------|-------------|
| MyChatMember | Bot added/removed from group - creates/updates group record, sends welcome/goodbye message |
| NewChatMembers | New member joins - sends welcome message (if enabled in group settings) |
| LeftChatMember | Member leaves - logs event |
| TextMessage | Plain text message - records to database, updates group stats |
| MediaMessage | Photo/Video/Document/Voice/Audio/Sticker/Animation - records with media metadata |
| EditedMessage | Message edited - updates edit history with timestamp |
| ChannelPost | Channel post - records channel message (user_id=0) |
| EditedChannelPost | Channel post edited - updates channel message edit history |

**Usage Conventions:**

- Handler functions must match `bot.HandlerFunc` signature from go-telegram/bot
- Use middlewares for permission checks (never inline permission logic)
- All repository methods return descriptive errors wrapped with `fmt.Errorf`
- Database operations use upsert pattern (`$set` + `$setOnInsert`) to handle create/update atomically
- Bot token and owner IDs must be configured via environment variables

**Service layer conventions:**
- Handlers should call service methods for business logic, never access repository directly
- Service methods must include comprehensive business validation (permission checks, parameter validation, state checks)
- Service methods should return user-friendly Chinese error messages for direct display to users
- All service operations must include structured logging (Info level for success, Error level for failures)
- DTOs (like `TelegramUserInfo`) should be used for data transfer between handlers and services

**Worker pool conventions:**
- All handlers must be registered with `asyncHandler()` wrapper to enable concurrent execution
- Handlers should avoid long-blocking operations to keep queue flowing smoothly
- Worker pool is automatically shut down when bot closes, no manual management needed
- Panic in handlers is automatically recovered by worker pool - handlers don't need explicit panic handling

**Message sending conventions:**
- Use `sendErrorMessage(ctx, chatID, msg)` for all error responses (ensures ❌ prefix and UI consistency)
- Use `sendSuccessMessage(ctx, chatID, msg)` for all success confirmations (ensures ✅ prefix)
- Use `sendMessage(ctx, chatID, text)` for informational messages
- Never call `bot.SendMessage` directly - always use helpers for consistent error handling

### App Package (`internal/app`)

- Application layer for unified service initialization and lifecycle management
- `App` struct holds all service instances (`MongoDB`, `TelegramBot`, future: `RedisClient`, etc.)
- `New(cfg)` initializes all services in order, returns error if any service fails
- `Close(ctx)` gracefully shuts down all services (Telegram bot first, then MongoDB)
- Access services via `app.MongoDB`, `app.TelegramBot`, etc.
- To add new services: add field to `App`, initialize in `New()`, cleanup in `Close()`
- Telegram bot is started in a goroutine in `main.go` using `app.TelegramBot.Start(ctx)`

### Environment Variables

The application is configured entirely through environment variables:

- `MONGO_URI` - MongoDB connection string (required)
- `MONGO_DB_NAME` - Database name (required)
- `LOG_LEVEL` - Logging verbosity (optional, default: "info")
- `TELEGRAM_TOKEN` - Bot token (configured in deployment)
- `BOT_OWNER_IDS` - Admin user IDs (configured in deployment)
- `MESSAGE_RETENTION_DAYS` - Message retention period in days (optional, default: 7, minimum: 1)

## Docker Deployment

The production image is built using a multi-stage Dockerfile at `deployments/docker/Dockerfile`:

- **Stage 1 (builder)**: Compiles Go binary with static linking (`CGO_ENABLED=0`)
- **Stage 2 (runtime)**: Alpine-based minimal image with ca-certificates and tzdata
- Runs as non-root user (`appuser`)
- Binary location: `/app/bot`
- Includes health check via `pidof bot`

Container is deployed with:
- Automatic restart policy (`unless-stopped`)
- JSON log driver with rotation (max 10MB, 3 files)
- Environment variables injected from GitHub Secrets

## Testing

Tests run automatically in CI pipeline:

- **Unit tests**: `go test -v -race -coverprofile=coverage.out -covermode=atomic ./...`
- **Integration tests**: Run with MongoDB service container, tagged with `//go:build integration`
- **Coverage report**: Generated as `coverage.html` (local artifact only)

## Making Changes

When modifying code:

1. Create a feature branch and open a PR
2. CI pipeline validates changes (lint, test, security scan)
3. After PR approval and merge to `main`, CD automatically deploys
4. Deployment includes automatic rollback if container fails health check within 10 seconds

## Key Conventions

- All configuration via environment variables (no config files)
- Use `logger.L()` for all logging (never `fmt.Print*` or `log.Print*`)
- All services managed by `app` layer, initialized once in `main.go` via `app.New(cfg)`
- Access services through `app` instance (e.g., `app.MongoDB.Database()`, `app.TelegramBot`)
- Error handling: validate early, return descriptive errors with `fmt.Errorf` wrapping

**Telegram-specific conventions:**

- Bot handlers must follow `bot.HandlerFunc` signature: `func(context.Context, *bot.Bot, *models.Update)`
- Use middleware wrappers (`RequireOwner`, `RequireAdmin`) for permission control, never inline checks
- Update user `last_active_at` automatically in handlers via `userService.UpdateUserActivity()` (service layer handles repository calls)
- Handlers should call service methods for business operations, never access repository directly (enforces separation of concerns)
- Repository methods use MongoDB upsert pattern to atomically handle create/update operations
- Database indexes are ensured automatically during bot initialization via `EnsureIndexes()`
- Owner users are auto-created from `BOT_OWNER_IDS` config during bot startup
- Bot runs in a goroutine with context cancellation for graceful shutdown
- All handlers execute asynchronously via worker pool for concurrent request handling
