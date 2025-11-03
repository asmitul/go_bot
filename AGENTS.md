# Agent Onboarding Guide

Welcome to the `go_bot` repository. This document captures the house rules that every agent must follow when touching any file in this repo.

## Repository Topography
- `cmd/bot/main.go`: application entrypoint that wires configuration, logging, graceful shutdown, and Telegram bot bootstrap.
- `internal/app`: dependency bootstrap (config, logger, Mongo, Telegram client). Subpackages each own their concern.
- `internal/telegram`: feature logic lives here, split into `features/`, `repository/`, and `service/` to mirror the layered design. Tests sit beside the code they exercise.
- `deployments/docker`: Docker Compose and runtime assets. Mongo data volumes live under `data/` when the local stack is running.

## Local Development Workflow
1. Duplicate `.env.local.example` to `.env.local` and populate `TELEGRAM_TOKEN`, `BOT_OWNER_IDS`, and Mongo credentials before spinning anything up.
2. Use `make local-up` to start the MongoDB + bot stack with `docker-compose.local.yml`. Tear it down with `make local-down`, or use `make local-clean` to also prune bound data.
3. For quick iteration run `go run ./cmd/bot` with the desired environment variables exported.

## Coding Standards
- Always run `gofmt`/`go fmt ./...` (and preferably `goimports`) before committing; keep line length around 100 characters and use tabs for indentation.
- Package names stay lowercase with no underscores. Exported identifiers use PascalCase, helpers remain unexported unless shared.
- Prefer contextual logging via the structured logger in `internal/logger`. Pass `context.Context` explicitly instead of relying on globals.

## Testing Expectations
- Co-locate Go tests (`*_test.go`) beside the code under test. Favor table-driven tests for handlers or services.
- Run `go test ./... -cover` before every commit. When focused on Telegram features, `go test ./internal/telegram/...` is a quick subset.
- Keep coverage steady or increasing; call out any gaps in your PR if something cannot be reasonably tested.

## Git & PR Etiquette
- Use Conventional Commit prefixes (`feat:`, `fix:`, `chore:`, `docs:`). Squash trivial fixups locally so every commit is green.
- PR descriptions must include: a concise summary, context/issue link if applicable, explicit test command output (e.g., `go test ./...` or `make local-up` smoke check), and screenshots/log excerpts when altering bot interactions or UX.
- Documentation changes that affect contributor workflows should point back to this guide.

These instructions apply repository-wide unless a subdirectory overrides them with its own `AGENTS.md`.
