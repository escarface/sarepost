# Sarepost Rebrand — Design Spec

**Date:** 2026-06-08
**Status:** Approved
**Type:** Rebranding (identity migration)

## Goal

Migrate the project identity from `github.com/antoniolg/postflow` to `github.com/escarface/sarepost` while preserving backward compatibility for all public surfaces (API, MCP tool names, CLI, env vars, binary names, internal identifiers).

## Design Principle

**Change what identifies the project externally. Leave internal wiring alone.**

- Module path, repo URL, Docker images, Homebrew tap, docs URLs → **changed**
- Logos and limited branding strings → **changed**
- Env vars (`POSTFLOW_*`), MCP tool names (`postflow_*`), binaries (`postflow-server`, `postflow`), DB filename, cookies, localStorage keys, scripts → **unchanged**

## Scope: What Changes

### 1. Go Module Path

| File | Change |
|---|---|
| `go.mod:1` | `module github.com/antoniolg/postflow` → `module github.com/escarface/sarepost` |
| ~120 `.go` files | All `"github.com/antoniolg/postflow/..."` imports → `"github.com/escarface/sarepost/..."` |
| `Dockerfile:19` | ldflags `-X github.com/antoniolg/postflow/cmd/postflow-server.Version` → new path |
| `.github/workflows/release-cli-homebrew.yml:80` | ldflags `-X github.com/antoniolg/postflow/internal/cli.Version` → new path |

### 2. Git Remote

| File | Change |
|---|---|
| `.git/config` | `https://github.com/antoniolg/postflow.git` → `https://github.com/escarface/sarepost.git` |

### 3. Docker / GHCR

| File | Line | Change |
|---|---|---|
| `docker-compose.yml:6` | `image: antoniolg/postflow:latest` | → `image: ghcr.io/escarface/sarepost:latest` |
| `.github/workflows/release-image.yml:13` | `IMAGE_NAME: ghcr.io/antoniolg/postflow` | → `ghcr.io/escarface/sarepost` |
| `.github/workflows/release-cli-homebrew.yml:20` | `GH_REPO: antoniolg/postflow` | → `saredigital/sarepost` |

### 4. Homebrew Tap

| File | Line | Change |
|---|---|---|
| `.github/workflows/release-cli-homebrew.yml:19` | `HOMEBREW_TAP_REPO: antoniolg/homebrew-tap` | → `saredigital/homebrew-tap` |
| `README.md:206-207` | `brew tap antoniolg/tap && brew install antoniolg/tap/postflow` | → `saredigital/tap` and `saredigital/tap/postflow` |
| `docs/RELEASING.md:79` | `antoniolg/homebrew-tap` | → `saredigital/homebrew-tap` |

### 5. Docs URLs

| File | Change |
|---|---|
| `README.md` | All `github.com/antoniolg/postflow` → `github.com/escarface/sarepost` |
| `docs/coolify-deploy.md` | `ghcr.io/antoniolg/postflow` → `ghcr.io/escarface/sarepost`, repo URLs |
| `docs/RELEASING.md` | Image URLs and tap references |

### 6. Logos

| File | Change |
|---|---|
| `internal/api/assets/icons/postflow-logo-header-transparent-64.png` | Rename to `sarepost-logo-header-transparent-64.png` |
| `internal/api/assets/icons/postflow-logo-4096.png` | Rename to `sarepost-logo-4096.png` |
| `internal/api/templates/login.html:174` | Update `src` reference |
| `internal/api/templates/schedule.html:3974` | Update `src` reference |
| `docs/branding/postflow-icons/icon-4k.png` | Directory/rename out of scope (branding assets, optional) |

### 7. Branding Strings

| File | Line | Change |
|---|---|---|
| `internal/api/mcp.go:25` | `Name: "postflow-mcp"` | → `Name: "sarepost-mcp"` |
| `cmd/postflow-server/main.go:117` | `"postflow listening"` | → `"sarepost listening"` |
| `internal/cli/run.go:601` | `"postflow - CLI for SareDigital HTTP API"` | → `"postflow - CLI for Sarepost HTTP API"` |
| `skills/postflow-cli/` | Directory and skill name | → `skills/sarepost-cli/`, update SKILL.md content |

## Scope: What Does NOT Change

| Item | Rationale |
|---|---|
| `POSTFLOW_DRIVER`, `POSTFLOW_MASTER_KEY`, `POSTFLOW_BASE_URL`, `POSTFLOW_API_TOKEN` env vars | Preserve deployment compatibility |
| `postflow_*` MCP tool names (23 tools) | Preserve LLM client compatibility |
| Binaries `postflow-server`, `postflow` | Preserve scripting and muscle memory |
| Directories `cmd/postflow-server/`, `cmd/postflow/` | Matches binary names |
| DB filename `postflow.db` | Internal, no user impact |
| Cookie `postflow_session` | Internal, no user impact |
| localStorage keys in schedule.html | Internal frontend, no user impact |
| Template mock content (`@postflow_app`, `post_flow`) | Cosmetic mock data, irrelevant |
| Docker volume names (`postflow_data`) | Internal, no user impact |
| Docker service name (`postflow`) | Internal, no user impact |
| Scripts (`backup.sh`, `restore.sh`, `check-coverage.sh`, `a11y-check.sh`) | Internal tooling |
| UI messages (`ui_messages.go`) | Already branded "SareDigital" |

## Files Already Modified (uncommitted)

These 10 files have in-progress rebranding changes (SareDigital branding in UI). They will be included in the same commit set:

- `README.md`
- `docs/specs/openapi.yaml`
- `internal/api/assets/icons/site.webmanifest`
- `internal/api/mcp.go`
- `internal/api/templates/authorize.html`
- `internal/api/templates/login.html`
- `internal/api/templates/schedule.html`
- `internal/api/ui_messages.go`
- `internal/cli/run.go`
- `internal/cli/run_settings.go`

## Implementation Order

1. Module path: `go.mod` + mass find-and-replace all imports
2. Build verification: `go build ./...` and `go test ./...`
3. Docker and CI workflow updates
4. Docs URL updates
5. Logo renames + template reference updates
6. Branding strings (MCP name, log message, CLI desc, skill)
7. Full gate: `go test ./...`, `go test -race ./...`, lint, coverage, vulncheck

## Risk Assessment

- **Module path change**: Verified by `go build ./...`. If any import is missed, the compiler catches it.
- **Docker image name**: Must match what's pushed to GHCR. Tag `latest` after merge.
- **Homebrew tap**: Requires the `saredigital/homebrew-tap` repo to exist before the release workflow runs.
- **MCP server name**: `sarepost-mcp` is purely cosmetic in the server info response. MCP tool names remain `postflow_*` so no client impact.
- **Skill rename**: The skill file is for agent consumption. Renamed directory and content must be consistent.
