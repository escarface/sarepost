# Codex Memory

## Summary

- Date: 2026-06-13
- Project: `postflow`
- Branch: `main`
- Working tree: clean
- Focus: web UI generation and scheduling flow

## Current State

- The in-app image generation flow was fixed by raising the server write timeout to 2 minutes so long-running OpenAI image responses do not get cut off before the handler returns.
- The create-publication form now requires `scheduled_at_local` only for the schedule action at the browser level, while `save draft` and `publish now` bypass native validation with `formnovalidate`.
- Full test suite passed after both fixes.

## Decisions

- Keep image generation synchronous for now. The immediate fix was timeout alignment instead of introducing background jobs.
- Enforce scheduling date presence in the UI for the schedule path rather than relying only on the backend redirect error.
- Preserve the existing backend validation and intent handling in `internal/api/posts_handlers.go`.

## Important References

- [cmd/postflow-server/main.go](/Users/asierluengo/Development/postflow/cmd/postflow-server/main.go:119)
- [cmd/postflow-server/main_test.go](/Users/asierluengo/Development/postflow/cmd/postflow-server/main_test.go:12)
- [internal/api/templates/schedule.html](/Users/asierluengo/Development/postflow/internal/api/templates/schedule.html:4069)
- [internal/api/api_create_accessibility_test.go](/Users/asierluengo/Development/postflow/internal/api/api_create_accessibility_test.go:252)
- [internal/api/posts_handlers.go](/Users/asierluengo/Development/postflow/internal/api/posts_handlers.go:92)

## Verification

- `go test ./cmd/postflow-server`
- `go test ./internal/api`
- `go test ./...`

## Risks And Watchouts

- If image generation still fails in production with `502`, check upstream proxy or Coolify timeouts next; the app server timeout mismatch has already been corrected.
- The create form relies on browser validation for the schedule button and backend validation remains the final safety net.

## Next-Thread Startup Prompt

Review `docs/codex-memory.md`, then inspect the latest two commits on `main` related to image generation timeout and create-form scheduling validation before making further UI generation changes.
