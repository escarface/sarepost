# Tasks: Instagram image carousel

## Status Legend

- `[ ]` Pending
- `[~]` In progress
- `[x]` Complete and verified
- `[!]` Blocked

## Task Graph

| Task | Depends on | Parallel group | Files | Can run in parallel |
|---|---|---|---|---|
| T1 | none | P1 | `internal/db/*` | no |
| T2 | T1 | P2 | `internal/postflow/*` | no |
| T3 | T2 | P3 | `internal/api/*`, docs | no |
| T4 | T1,T2,T3 | P4 | verification only | no |

## Tasks

- [x] T1: Persist ordered post media
  - Spec: R1 / Scenario `Create an ordered post`
  - Depends on: none
  - Parallel group: P1
  - Agent: ordered_media
  - Files: `internal/db/db.go`, `internal/db/db_migrations.go`, `internal/db/*_test.go`
  - Work: add and migrate the media position, then preserve it in every post create/edit path.
  - Verification: focused database tests prove supplied order survives create and edit.
  - Evidence: `GOMODCACHE=/tmp/gomodcache GOPATH=/tmp/gopath GOTELEMETRY=off go test ./internal/db -count=1` (pass, 2026-07-26); focused create, edit, thread-write, and legacy-migration tests pass.

- [x] T2: Implement image-carousel validation and Meta publishing
  - Spec: R2, R3, R4 / Scenarios `Validate an image carousel`, `Reject a mixed carousel`, `Publish an ordered carousel`
  - Depends on: T1
  - Parallel group: P2
  - Agent: root
  - Files: `internal/postflow/meta.go`, `internal/postflow/meta*_test.go`
  - Work: add the image-only carousel provider flow while retaining single-image and Reel behavior.
  - Verification: focused provider tests, beginning RED, validate all Meta requests and compatibility behavior.
  - Evidence: `GOMODCACHE=/tmp/gomodcache GOPATH=/tmp/gopath GOTELEMETRY=off go test ./internal/postflow -run 'TestMetaValidateDraftRules|TestInstagramPublishImageCarouselPreservesMediaOrder|TestInstagramPublishUsesMediaURLBuilder|TestInstagramPublishVideoUsesMediaURLBuilder' -count=1` (pass, 2026-07-26).

- [x] T3: Cover API validation and document the supported format
  - Spec: R2, R4 / Scenario `Validate an image carousel`
  - Depends on: T2
  - Parallel group: P3
  - Agent: root
  - Files: `internal/api/*_test.go`, `README.md`
  - Work: add API-level validation coverage and update the supported-media documentation.
  - Verification: focused API tests.
  - Evidence: `GOMODCACHE=/tmp/gomodcache GOPATH=/tmp/gopath GOTELEMETRY=off go test ./internal/api -run 'TestValidatePostEndpoint(AcceptsInstagramImageCarousel)?$' -count=1` and `go test ./internal/api -run TestMCPCreatePostAcceptsInstagramImageCarousel -count=1` (pass, 2026-07-26).

- [x] T4: Run regression verification
  - Spec: R1, R2, R3, R4
  - Depends on: T1, T2, T3
  - Parallel group: P4
  - Agent: root
  - Files: verification only
  - Work: run the full Go test suite and review the scoped diff.
  - Verification: `go test ./...` exits 0.
  - Evidence: `GOMODCACHE=/tmp/gomodcache GOPATH=/tmp/gopath GOTELEMETRY=off go test ./... -count=1` (pass, 2026-07-26); `git diff --check` passed and the scoped diff was reviewed.

## Verification Summary

| Task | Status | Evidence |
|---|---|---|
| T1 | Complete | `go test ./internal/db -count=1` passed (with isolated Go caches) |
| T2 | Complete | focused provider tests passed |
| T3 | Complete | focused API test passed |
| T4 | Complete | `go test ./... -count=1` and `git diff --check` passed |
