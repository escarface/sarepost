# Codex Memory

## Summary

- Date: 2026-06-22
- Project: `postflow`
- Branch: `main`
- Working tree: clean
- Focus: durable project context, memory graph, and current product surface

## Current State

- PostFlow is a Go + SQLite modular monolith with four main user-facing surfaces: Web UI, HTTP API, MCP, and CLI.
- Business logic is expected to live in `internal/application`, with thin adapters in `internal/api`, `internal/cli`, and `internal/worker`.
- Surface parity is a core product rule: API, MCP, and CLI should expose the same capabilities unless an exception is explicitly documented.
- The main intentional parity exception is in-app AI generation, which is Web-UI-only. Durable content plans, campaigns, scheduling, media, approval, and DLQ flows are shared across HTTP, MCP, and CLI.
- Recent repo work on `main` includes post creation reuse via `source_post_id` and Instagram caption validation coverage.

## Decisions

- Treat `docs/architecture.md` and `AGENTS.md` as the authoritative guidance for layering, parity, migration safety, and required validation.
- Use this file for operational repo state and short rehydration, while durable historical/project memory lives in the Obsidian PostFlow memory graph.
- Keep project memory connected through the PostFlow Obsidian index instead of storing isolated session notes.

## Important References

- [AGENTS.md](/Users/asierluengo/Development/postflow/AGENTS.md:1)
- [README.md](/Users/asierluengo/Development/postflow/README.md:1)
- [docs/architecture.md](/Users/asierluengo/Development/postflow/docs/architecture.md:1)
- [docs/specs/mcp.md](/Users/asierluengo/Development/postflow/docs/specs/mcp.md:1)
- [cmd/postflow-server/main.go](/Users/asierluengo/Development/postflow/cmd/postflow-server/main.go:1)
- [cmd/postflow/main.go](/Users/asierluengo/Development/postflow/cmd/postflow/main.go:1)
- [internal/application](/Users/asierluengo/Development/postflow/internal/application)
- [internal/parity](/Users/asierluengo/Development/postflow/internal/parity)

## Verification

- `git status --short` -> clean
- `git branch --show-current` -> `main`
- `git log --oneline -5` reviewed for recent direction
- Documentation reviewed: `README.md`, `docs/architecture.md`, existing project memory notes

## Risks And Watchouts

- Do not add business logic directly into API handlers, CLI commands, or worker loops.
- Do not introduce surface drift between API, MCP, and CLI unless the exception is intentional and documented.
- SQLite migrations must remain additive and safe; no destructive reset patterns.
- Real publishing workflows must target production through the supported PostFlow CLI/MCP flow, never ad hoc DB writes.

## Next-Thread Startup Prompt

Load `docs/codex-memory.md` and the Obsidian note `Saredigital/Memories/projects/postflow/2026-06/2026-06-22-project-foundation-and-memory-graph.md`, then inspect `AGENTS.md`, `README.md`, and `docs/architecture.md` before changing application-layer behavior or cross-surface features.
