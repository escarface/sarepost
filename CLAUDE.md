# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

PostFlow (module `github.com/escarface/sarepost`) is a self-hosted social publishing
service (Web UI, HTTP API, MCP endpoint, CLI) for scheduling and publishing posts to
X, LinkedIn, Facebook, and Instagram. Go + SQLite, single binary, LLM-first: every
capability must be consistently exposed across API, MCP, and CLI surfaces.

## Commands

```bash
# Run the server locally (preferred: hot reload via air, from repo root)
air

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/application/posts/...

# Run a single test
go test ./internal/application/posts/... -run TestCreatePost_ValidatesThread

# Race detection
go test -race ./...

# Format changed files
gofmt -w <changed-go-files>

# Coverage gate (per-package thresholds, see below)
./scripts/check-coverage.sh

# Vulnerability check
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Coverage thresholds enforced by CI/`check-coverage.sh`: total 50%, `internal/worker`
80%, `internal/api` 60%, `internal/cli` 40%, `internal/db` 55%, `internal/postflow` 50%.

CI also enforces `gofmt`, `go mod tidy` consistency, `golangci-lint`, build/test/race.

## Architecture

Modular monolith with a strict application layer (see `docs/architecture.md` and
`docs/adr/0001-modular-monolith-application-layer.md`):

- `cmd/postflow-server`, `cmd/postflow`: entrypoints for the server and CLI.
- `internal/api`: HTTP + MCP adapters — request parsing, transport mapping, HTML
  rendering (templates/assets under `internal/api/templates`, `internal/api/assets`).
- `internal/cli`: CLI adapter.
- `internal/worker`: background execution runtime (the publish cycle).
- `internal/application`: business use cases/orchestration, organized by domain:
  - `posts/` (create, mutations, schedule listing, thread validation)
  - `media/`, `dlq/`, `notifications/`, `publishcycle/`
  - `ports/`: interfaces for infra dependencies (e.g. `ProviderRegistry`,
    `CredentialsStore`, `PublishFailureNotifier`)
- `internal/db`: SQLite persistence and versioned migrations
  (`internal/db/db_migrations.go` + `db_migrations_test.go`).
- `internal/postflow`: provider SDK clients (X, LinkedIn, Meta) and OAuth/auth helpers.
- `internal/secure`: AES-GCM encryption for stored secrets/tokens.
- `internal/domain`: entities and status enums.
- `internal/parity`: cross-surface contract/parity tests (API/CLI/MCP/threads/platform rules).
- `internal/capabilities`, `internal/config`, `internal/observability`, `internal/textfmt`:
  supporting infrastructure.

### Layering rules

1. Adapters (`api`, `cli`, `worker`) call into `application` use cases only.
2. `application` depends on `ports` + `domain`, never on transport adapters.
3. Error-to-HTTP/CLI mapping stays in adapters, not in `application`.
4. Business logic does not live in handlers, CLI commands, or worker loops.

### Surface parity (critical)

PostFlow is LLM-first; every capability must work the same way via API, MCP, and CLI:

1. Implement behavior once in `internal/application`.
2. Expose it consistently through API/MCP/CLI where applicable.
3. Update parity tests in `internal/parity`.
4. Keep error semantics consistent across surfaces.

Do not ship a feature on only one surface unless explicitly intentional and documented.

### Database and migrations

- SQLite is used in production; data safety is mandatory — no destructive resets.
- Schema changes go through versioned migrations in `internal/db/db_migrations.go`
  (tracked via `schema_migrations`), with matching tests in `db_migrations_test.go`.
- Migrations must be idempotent/safe. Startup applies pending migrations
  non-destructively and creates a backup snapshot first if any are pending.

### File size

Prefer Go source files `< 500 LOC`. Split large files by feature boundary
(parser/handler/service/helpers) before adding complexity.

### Documentation

If behavior/API/contracts change, update in the same change set: `README.md`,
`docs/specs/openapi.yaml`, and architecture docs/ADRs if design rules change.
Documentation language is English.

## Test strategy

Prefer black-box, behavior-level tests over implementation-coupled ones:

- Prioritize: end-to-end adapter tests (`internal/api`, `internal/cli`,
  `internal/worker`), integration tests against a real DB, and `internal/parity` tests.
- Avoid brittle call-count tests or tests coupled to private internals.
- When fixing a bug, add a regression test for the failure path.

## Environment / config

Key env vars (see `.env.example` / README for full list):

- `POSTFLOW_MASTER_KEY` (base64, 32-byte) — required, encrypts stored secrets.
- `API_TOKEN` — bearer token for API/MCP/CLI auth (server reads `API_TOKEN`).
- `PUBLIC_BASE_URL` — public URL for OAuth callbacks and media URLs (server-side only).
- `POSTFLOW_DRIVER` — `mock` (default, no real API calls) or `live`.
- `OWNER_EMAIL` / `OWNER_PASSWORD_HASH` — web UI login (generate hash via
  `go run ./scripts/hash-password.go '<password>'`).
- CLI-only: `POSTFLOW_BASE_URL` (server URL) and `POSTFLOW_API_TOKEN` (auth token) —
  distinct from the server's `API_TOKEN`/`PUBLIC_BASE_URL`.

## Production publishing operations

When asked to create, upload, schedule, or inspect **real** social publications, use
the production instance, not local/mock:

```bash
export POSTFLOW_BASE_URL="https://sarepost.casacasala.online"
export POSTFLOW_API_TOKEN="<configured production token>"
go run ./cmd/postflow --json accounts list
```

Rules:

- Use the PostFlow CLI or MCP tools for real publications — never ad hoc DB writes
  or direct HTTP calls for production scheduling.
- Do not use `http://localhost:8080` for real publishing unless explicitly requested;
  the local `postflow.db` may have no connected accounts.
- Before publishing: run `accounts list` against production and pick the account by
  both platform and display name (e.g. for Sare Digital, don't pick the wrong page).
  Then run `schedule list` for the target time window to check for conflicts.
- Use RFC3339 timestamps with explicit timezone offsets.
- For image-based posts, use the `imagegen` skill, save assets under
  `output/imagegen/`, upload them, and verify the scheduled post reports
  `has_media: true`.

Standard CLI flow:

```bash
go run ./cmd/postflow --json accounts list
go run ./cmd/postflow --json schedule list \
  --from 2026-06-09T00:00:00+02:00 --to 2026-06-09T23:59:59+02:00
go run ./cmd/postflow --json media upload --file /absolute/path/to/image.png --kind image
go run ./cmd/postflow --json posts create \
  --account-id acc_xxx --media-id med_xxx --text "Publication copy" \
  --scheduled-at 2026-06-09T17:00:00+02:00 --idempotency-key descriptive-unique-key
go run ./cmd/postflow --json schedule list \
  --from 2026-06-09T16:59:00+02:00 --to 2026-06-09T17:01:00+02:00
```
