# Repository Guidelines

## Project Structure & Module Organization
- `cmd/bot/main.go` boots the service, wiring configuration, logging, and graceful shutdown flow.
- `internal/app` orchestrates dependency startup, delegating to `internal/config`, `internal/logger`, `internal/mongo`, and `internal/telegram`; each subpackage is focused on one concern.
- Feature logic for the Telegram bot lives under `internal/telegram`, with `features/`, `repository/`, and `service/` mirroring the layered design; Go tests currently sit beside features, e.g. `internal/telegram/features/calculator`.
- Docker assets are under `deployments/docker/`, while `data/` is reserved for MongoDB volumes when the local stack is launched.

## Build, Test, and Development Commands
- `make local-up` starts the MongoDB + bot stack using `docker-compose.local.yml` and `.env.local`.
- `make local-down` or `make local-clean` stops containers; the latter also prunes bound data.
- `go run ./cmd/bot` runs the bot against your current environment variables, useful for iterative debugging.
- `go test ./... -cover` executes unit tests across the repo and reports coverage; prefer running before every push.

## Coding Style & Naming Conventions
- Follow standard Go formatting (tabs for indentation, 100-ish character lines). Always run `gofmt` or `go fmt ./...` prior to committing; `goimports` keeps imports sorted.
- Package names stay lower_snake (e.g., `telegram`), exported identifiers use PascalCase, and internal helpers remain unexported unless shared.
- Log with the structured logger in `internal/logger` and propagate `context.Context` rather than global state whenever feasible.

## Testing Guidelines
- Keep tests adjacent to the code they exercise using the `_test.go` suffix, with table-driven cases for new handlers or services.
- Use `go test ./internal/telegram/...` while focusing on bot behavior, and favor deterministic fakes over hitting the real API.
- Maintain or raise the current coverage; highlight risk areas in the PR if a feature cannot be unit-tested.

## Commit & Pull Request Guidelines
- Adopt Conventional Commit prefixes (`feat:`, `fix:`, `chore:`, `docs:`) as seen in the existing history.
- Squash small fixups locally and ensure each commit builds and tests cleanly.
- PRs should include: concise summary, linked issue or context, explicit test command output (`go test ./...` or `make local-up` smoke check), and screenshots/log excerpts when altering bot interactions; documentation touching contribution practices must link back to this guide.

## Environment & Secrets
- Copy `.env.local.example` to `.env.local`, then provide `TELEGRAM_TOKEN`, `BOT_OWNER_IDS`, and Mongo credentials before running `make local-up`.
- GitHub Actions depend on matching secrets (`TELEGRAM_TOKEN`, `MONGO_URI`, `VPS_*`); new deployments should validate that these stay in sync with the service configuration.
