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
│       ├── models/      # Data models (User, Group)
│       ├── repository/  # Data access layer (UserRepository, GroupRepository)
│       ├── telegram.go  # Bot core service
│       ├── handlers.go  # Command handlers
│       └── middleware.go # Permission middlewares
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
- Configuration fields: `TelegramToken`, `BotOwnerIDs`, `MongoURI`, `MongoDBName`
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
- Implements Repository pattern with layered architecture for clean separation of concerns
- Supports long polling mode (bot runs in goroutine, blocking until context cancellation)

**Architecture Components:**

- **models/** - Data models with business logic
  - `User` struct with role-based permissions (Owner/Admin/User)
  - `Group` struct with settings and statistics
  - Permission check methods (`IsOwner()`, `IsAdmin()`, `CanManageUsers()`)

- **repository/** - Data access layer (CRUD operations)
  - `UserRepository`: CreateOrUpdate, GetByTelegramID, GrantAdmin, RevokeAdmin, ListAdmins
  - `GroupRepository`: CreateOrUpdate, MarkBotLeft, ListActiveGroups, UpdateSettings
  - All repositories provide `EnsureIndexes()` for automatic index creation

- **telegram.go** - Core bot service
  - `New(cfg, db)` creates bot instance, registers handlers, initializes indexes
  - `InitFromConfig(appCfg, db)` convenience function
  - `Start(ctx)` runs bot in blocking mode (call in goroutine)
  - `initOwners(ctx)` auto-creates owner users from `BOT_OWNER_IDS` config

- **handlers.go** - Command handlers
  - All handlers follow `bot.HandlerFunc` signature: `func(ctx, *bot.Bot, *models.Update)`
  - Auto-update user info and last_active_at on every command
  - Commands: /start, /ping, /grant, /revoke, /admins, /userinfo

- **middleware.go** - Permission control
  - `RequireOwner(next)` wraps handlers requiring owner permission
  - `RequireAdmin(next)` wraps handlers requiring admin+ permission
  - Middlewares send error messages and log unauthorized attempts

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

**Supported Commands:**

| Command | Permission | Description |
|---------|------------|-------------|
| `/start` | All users | Welcome message, auto-register user |
| `/ping` | All users | Test bot connectivity |
| `/grant <user_id>` | Owner only | Grant admin permission to user |
| `/revoke <user_id>` | Owner only | Revoke admin permission from user |
| `/admins` | Admin+ | List all administrators |
| `/userinfo <user_id>` | Admin+ | View detailed user information |

**Usage Conventions:**

- Handler functions must match `bot.HandlerFunc` signature from go-telegram/bot
- Use middlewares for permission checks (never inline permission logic)
- All repository methods return descriptive errors wrapped with `fmt.Errorf`
- Database operations use upsert pattern (`$set` + `$setOnInsert`) to handle create/update atomically
- Bot token and owner IDs must be configured via environment variables

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
- Update user `last_active_at` automatically in handlers via `userRepo.CreateOrUpdate()` or `UpdateLastActive()`
- Repository methods use MongoDB upsert pattern to atomically handle create/update operations
- Database indexes are ensured automatically during bot initialization via `EnsureIndexes()`
- Owner users are auto-created from `BOT_OWNER_IDS` config during bot startup
- Bot runs in a goroutine with context cancellation for graceful shutdown
