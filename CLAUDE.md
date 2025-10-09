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
│   └── mongo/           # MongoDB client wrapper
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

### App Package (`internal/app`)

- Application layer for unified service initialization and lifecycle management
- `App` struct holds all service instances (`MongoDB`, future: `TelegramBot`, etc.)
- `New(cfg)` initializes all services in order, returns error if any service fails
- `Close(ctx)` gracefully shuts down all services
- Access services via `app.MongoDB`, `app.TelegramBot`, etc.
- To add new services: add field to `App`, initialize in `New()`, cleanup in `Close()`

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
- Access services through `app` instance (e.g., `app.MongoDB.Database()`)
- Error handling: validate early, return descriptive errors with `fmt.Errorf` wrapping
