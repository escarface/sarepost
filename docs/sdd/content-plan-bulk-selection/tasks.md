# Tasks: Content Plan bulk selection

## Status Legend

- `[ ]` Pending
- `[~]` In progress
- `[x]` Complete and verified
- `[!]` Blocked

## Task Graph

| Task | Depends on | Parallel group | Files | Can run in parallel |
|---|---|---|---|---|
| T1 | none | P1 | `internal/api/content_plans_ui_test.go` | no |
| T2 | T1 | P2 | `internal/api/templates/schedule.html` | no |
| T3 | T2 | P3 | `internal/api/content_plans_ui_test.go`, `docs/sdd/content-plan-bulk-selection/tasks.md` | no |

## Tasks

- [x] T1: Add a failing UI render regression test
  - Spec: R1, R2 / Scenarios `Select all publishable variants`, `Clear all selected variants`
  - Depends on: none
  - Parallel group: P1
  - Agent: main
  - Thread: current
  - Worktree: current
  - Branch: current
  - Files: `internal/api/content_plans_ui_test.go`
  - Review: complete
  - Work: Assert that the review workspace exposes both bulk-selection controls and the status-aware selection logic.
  - Verification: Run the focused UI test and confirm it fails before the template change.
  - Evidence: RED confirmed with `go test ./internal/api -run TestGenerateViewIncludesContentPlanBuilderAndReviewWorkspace -count=1`: failed because `content-plan-select-all` was absent.

- [x] T2: Implement bulk selection controls
  - Spec: R1, R2 / Scenarios `Select all publishable variants`, `Clear all selected variants`
  - Depends on: T1
  - Parallel group: P2
  - Agent: main
  - Thread: current
  - Worktree: current
  - Branch: current
  - Files: `internal/api/templates/schedule.html`
  - Review: complete
  - Work: Render Select all/Clear selection controls and wire them to the existing variant checkboxes without changing API scheduling semantics.
  - Verification: Focused UI test passes; inspect the rendered JavaScript for the ready/approved filter.
  - Evidence: GREEN confirmed with the focused UI test; template inspection confirms bulk selection filters to `ready`/`approved` and clear selection unchecks every rendered checkbox.

- [x] T3: Run broader verification and record evidence
  - Spec: Acceptance criteria R1-R2
  - Depends on: T2
  - Parallel group: P3
  - Agent: main
  - Thread: current
  - Worktree: current
  - Branch: current
  - Files: `internal/api/content_plans_ui_test.go`, `docs/sdd/content-plan-bulk-selection/tasks.md`
  - Review: complete
  - Work: Run the relevant API test package and review the final diff for scope and formatting.
  - Verification: `go test ./internal/api/...` and `git diff --check`.
  - Evidence: `go test ./internal/api/... -count=1`, `go test ./internal/application/contentplans/... -count=1`, `go test ./...`, and `git diff --check` all passed. Browser interaction was not run because the user will verify the UI manually.

## Verification Summary

| Task | Status | Evidence |
|---|---|---|
| T1 | Complete | Focused test failed before implementation as expected. |
| T2 | Complete | Focused UI render test passes; status-aware selection logic is present. |
| T3 | Complete | Full Go test suite and diff check pass; browser verification remains user-owned. |
