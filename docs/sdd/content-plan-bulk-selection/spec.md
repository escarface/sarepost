# Spec: Content Plan bulk selection

## Intent

Reduce the manual effort required to approve a generated Content Plan by allowing the review workspace to select or clear all publishable variants in one action.

## Scope

### In

- Add controls to select all ready/approved variants in the Content Plan review workspace.
- Add a control to clear the current variant selection in one action.
- Keep the existing per-variant selection and scheduling behavior unchanged.

### Out

- No changes to the scheduling API, persistence model, or provider publishing behavior.
- No automatic approval or scheduling without an explicit user action.
- Failed, pending, generating, and already scheduled variants are not included in the bulk publishable selection.

## Requirements

### R1: Select publishable variants in bulk

The system MUST provide a review-workspace control that checks every rendered variant whose status is `ready` or `approved`.

#### Scenario: Select all publishable variants

Given a Content Plan with ready, approved, failed, and scheduled variants
When the user activates “Select all”
Then the ready and approved variant checkboxes are checked
And failed and scheduled variants are not selected for scheduling

### R2: Clear variant selection in bulk

The system MUST provide a review-workspace control that unchecks all variant checkboxes.

#### Scenario: Clear all selected variants

Given one or more selected Content Plan variant checkboxes
When the user activates “Clear selection”
Then all variant checkboxes are unchecked

## Acceptance Criteria

- [ ] The review workspace renders “Select all” and “Clear selection” controls.
- [ ] “Select all” targets only `ready` and `approved` variants, matching the scheduling action's allowed statuses.
- [ ] “Clear selection” unchecks every rendered variant checkbox.
- [ ] Existing individual selection, regeneration, and scheduling flows remain intact.

## Verification Strategy

- Unit: Existing Content Plan service tests remain unchanged because the API contract is unchanged.
- Integration: `internal/api` UI render test asserts the bulk-selection controls and selection logic are present.
- E2E/manual: Inspect the generated review workspace and exercise both controls against mixed variant statuses when a browser session is available.
- Build/type-check: `go test ./internal/api/...`.

## Risks And Tradeoffs

| Risk | Mitigation |
|---|---|
| Selecting failed or non-publishable variants could make the scheduling action misleading. | Bulk selection uses the same `ready`/`approved` status set accepted by scheduling. |
| Re-rendering a plan could lose checkbox state. | Preserve the existing render behavior; bulk controls act on the current rendered list and scheduling remains explicit. |
