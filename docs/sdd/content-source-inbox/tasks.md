# Tasks: Content Source Inbox

## Status Legend

- `[ ]` Pending
- `[~]` In progress
- `[x]` Complete and verified
- `[!]` Blocked

## Task Graph

| Task | Depends on | Parallel group | Files | Can run in parallel |
|---|---|---|---|---|
| T1 | none | P1 | `internal/domain`, `internal/application/contentsources`, `internal/db` | no |
| T2 | T1 | P2 | `internal/api`, `docs/specs/openapi.yaml` | no |
| T3 | T1, T2 | P3 | `internal/api/mcp_*`, `internal/cli`, `docs/specs/mcp.md`, `internal/parity` | no |
| T4 | T1, T2, T3 | P4 | tests/docs touched by all tasks | no |
| T5 | T1, T2 | P5 | `internal/api/templates`, `internal/api/ui_*`, UI tests | no |

## Tasks

- [x] T1: Add domain, application service, and SQLite persistence
  - Spec: R1, R2, R3 / Scenarios `Create a valid source`, `Reject incomplete source`, `Archive source`, `Generate angles`
  - Depends on: none
  - Parallel group: P1
  - Agent: main
  - Thread: unassigned
  - Worktree: unassigned
  - Branch: unassigned
  - Files: `internal/domain`, `internal/application/contentsources`, `internal/db`
  - Review: implemented locally
  - Work: Add model/statuses, migration/store methods, service validation, list/update/archive, and angle generation prompt orchestration.
  - Verification: `go test ./internal/application/contentsources ./internal/db`
  - Evidence: `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./internal/application/contentsources ./internal/db` passed.

- [x] T2: Add HTTP API endpoints and OpenAPI documentation
  - Spec: R1, R2, R3, R4 / Scenario `Surface parity`
  - Depends on: T1
  - Parallel group: P2
  - Agent: main
  - Thread: unassigned
  - Worktree: unassigned
  - Branch: unassigned
  - Files: `internal/api`, `docs/specs/openapi.yaml`
  - Review: implemented locally
  - Work: Add JSON endpoints for create/list/get/update/archive/generate-angles and document schemas.
  - Verification: `go test ./internal/api`
  - Evidence: `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./internal/api` passed.

- [x] T3: Add MCP and CLI operations with parity coverage
  - Spec: R4 / Scenario `Surface parity`
  - Depends on: T1, T2
  - Parallel group: P3
  - Agent: main
  - Thread: unassigned
  - Worktree: unassigned
  - Branch: unassigned
  - Files: `internal/api/mcp_*`, `internal/cli`, `docs/specs/mcp.md`, `internal/parity`
  - Review: implemented locally
  - Work: Add MCP tools and CLI commands that forward the same content source operations; update capability/parity tests when applicable.
  - Verification: `go test ./internal/cli ./internal/parity ./internal/api`
  - Evidence: `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./internal/cli ./internal/parity ./internal/api` passed.

- [x] T4: Run final verification and update task evidence
  - Spec: all requirements
  - Depends on: T1, T2, T3
  - Parallel group: P4
  - Agent: main
  - Thread: unassigned
  - Worktree: unassigned
  - Branch: unassigned
  - Files: `docs/sdd/content-source-inbox/tasks.md`
  - Review: completed
  - Work: Run broad verification, record evidence, and note residual risks.
  - Verification: `go test ./...`
  - Evidence: `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./...` passed. `.cache/` was removed after verification.

- [x] T5: Add Web UI content sources section
  - Spec: R5 / Scenario `Capture and act on sources in the UI`
  - Depends on: T1, T2
  - Parallel group: P5
  - Agent: main
  - Thread: unassigned
  - Worktree: unassigned
  - Branch: unassigned
  - Files: `internal/api/ui_schedule.go`, `internal/api/ui_view_models.go`, `internal/api/templates/schedule.html`, `internal/api/content_sources_ui_test.go`
  - Review: completed
  - Work: Add the `sources` Web UI view, navigation badge, capture form, source list, angle generation action, archive action, and source-to-post composer link.
  - Verification: `go test ./internal/api`; `go test ./...`
  - Evidence: `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./internal/api` passed. `env GOMODCACHE=/Users/asierluengo/Development/postflow/.cache/gomod GOCACHE=/Users/asierluengo/Development/postflow/.cache/gobuild go test ./...` passed.

## Verification Summary

| Task | Status | Evidence |
|---|---|---|
| T1 | Complete | `go test ./internal/application/contentsources ./internal/db` passed with repo-local Go caches. |
| T2 | Complete | `go test ./internal/api` passed with repo-local Go caches. |
| T3 | Complete | `go test ./internal/cli ./internal/parity ./internal/api` passed with repo-local Go caches. |
| T4 | Complete | `go test ./...` passed with repo-local Go caches; generated `.cache/` removed afterward. |
| T5 | Complete | `go test ./internal/api` and `go test ./...` passed with repo-local Go caches. |
