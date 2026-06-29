# Architecture

Read when:
- You need to add a new feature or endpoint.
- You need to move code between `api`, `worker`, `cli` and `application`.
- You need to understand where business logic belongs.

## Current style

`postflow` uses a **modular monolith** with clear boundaries:

- `cmd/`: entrypoints (`server`, `worker`, `cli`).
- `internal/api`: HTTP/MCP adapters (parsing, HTTP status, redirects, render).
- `internal/worker`: runtime adapter for background execution.
- `internal/cli`: CLI adapter.
- `internal/application`: use cases and orchestration.
- `internal/db`, `internal/postflow`, `internal/genai`, `internal/secure`: infrastructure.
- `internal/domain`: entities and status enums.

## AI generation (`internal/genai` + `internal/application/generation`)

In-app text and image generation. `internal/genai` is an infrastructure package
mirroring `internal/postflow`: small `TextProvider`/`ImageProvider` interfaces with
concrete clients (Anthropic via the official Go SDK, OpenAI via HTTP) plus mock
implementations used by `POSTFLOW_DRIVER=mock` and tests. Provider selection is
dynamic — the API key is supplied per call from encrypted settings — so the package
exposes `NewTextProvider`/`NewImageProvider` factories instead of a registry.

`internal/application/generation` holds the use cases: provider config and brand
profiles persisted in the `settings` table (API keys encrypted with `internal/secure`,
following the SMTP config pattern) and `GenerateText`/`GenerateImage`, which merge the
selected brand profile and platform rules into the prompt before delegating to a
provider. A brand profile may reference an uploaded media item as a visual style
reference; the adapter supplies a `MediaReader` so `GenerateImage` can load the bytes
and pass them to the provider for image-to-image generation (OpenAI `/images/edits`).

**Intentional parity exception:** generation is exposed only through the Web UI
(`/?view=generate` plus the `/generate/*` and `/settings/generation/*` endpoints). MCP
and CLI clients already generate content through their own LLM, so generation tools are
not added to those surfaces.

## Durable content plans (`internal/application/contentplans`)

Content plans are first-class editorial aggregates, independent from campaigns. Each
plan owns brand/account cadence blocks, shared time-slot ideas, per-account variants,
and a durable generation job. The application service validates the 90-day/500-variant
limits and connected accounts before persistence. The worker claims jobs with a SQLite
lease, generates shared ideas before platform variants, persists each result
independently, and safely resumes incomplete work after restart.

Review and scheduling remain separate. Plan variants can be edited or regenerated;
materialization delegates to the existing posts application services so provider
validation, calendar conflicts, duplicate-content checks, campaigns, and idempotency
remain consistent. HTTP, MCP, CLI, and Web UI are adapters over this shared behavior.

## Auto-approve safety gate (`internal/application/safetygate`)

The safety gate is the unattended-operation layer over the editorial loop. It
promotes `needs_review` posts that require approval to `approved` when they pass
deterministic `SafetyRule`s (domain entity), so generated content can flow
through to scheduling without a human gate. Posts that fail a `block` rule stay
in `needs_review` with a `BlockedReason` (the editorial status enum is not
extended), so they remain in human review and are never auto-scheduled.

`Evaluate` is a pure application function that applies all enabled,
platform-applicable rules and returns a verdict plus a stable
`AutoApprovedReason` audit string. `ApproveEligible` is the sweep use case: it
lists eligible posts from the `campaign_posts` join (where editorial metadata
lives), evaluates each, and persists per-post mutations independently so a
mid-batch failure does not roll back already-committed posts. The worker runs an
async sweep on a configurable cadence with a DB-backed lease
(`POSTFLOW_SAFETY_SWEEP_INTERVAL`, default 30s). Rules and the sweep are exposed
with surface parity on HTTP, MCP, and CLI (`internal/parity` enforces it).

## Layer rules

1. Adapters (`api`, `worker`, `cli`) call `application`.
2. `application` depends on interfaces (`ports`) and `domain`, not on adapters.
3. Infrastructure packages (`db`, provider SDK clients) are wired at the edges.
4. Error mapping to HTTP/CLI messages stays in adapters.

## Use cases already extracted

- `internal/application/posts/create.go`
- `internal/application/posts/mutations.go`
- `internal/application/media/service.go`
- `internal/application/dlq/service.go`
- `internal/application/publishcycle/runner.go`
- shared ports: `internal/application/ports/ports.go`

## Practical checklist for new work

1. Add/update use case in `internal/application/*`.
2. Add unit tests for the use case first.
3. Keep adapter handlers thin (request parsing + response mapping only).
4. Run full gate: `gofmt`, `go test ./...`, `go test -race ./...`, `golangci-lint`.
