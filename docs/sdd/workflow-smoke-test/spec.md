# Spec: Workflow Smoke Test

## Intent

Verify that the project-level Codex workflow can create persistent SDD artifacts without touching runtime code.

## Scope

### In

- Confirm the project has the expected workflow skills installed.
- Confirm `AGENTS.md` references spec/task-based SDD.

### Out

- Runtime application changes.
- Product behavior changes.
- Commit or PR creation.

## Requirements

### R1: Workflow artifacts are discoverable

The project MUST contain the workflow skills required to route, plan, execute, and verify work.

#### Scenario: Skill files are present

Given the repository has `.codex/skills`
When an agent lists the workflow skill files
Then `sare-workflow-lite`, `sare-sdd-lite`, `sare-task-executor`, `sare-tdd-lite`, `sare-verification-gate`, and `sare-new-app` are present.

### R2: SDD policy is project-visible

The project MUST document that meaningful features and non-trivial bugs use `spec.md` and `tasks.md`.

#### Scenario: AGENTS defines SDD artifacts

Given the repository has `AGENTS.md`
When an agent searches for SDD artifact policy
Then `docs/sdd/<change>/spec.md` and `docs/sdd/<change>/tasks.md` are referenced.

## Acceptance Criteria

- [x] R1 scenario is verified with a filesystem listing.
- [x] R2 scenario is verified with a text search.

## Verification Strategy

- Filesystem: `find .codex/skills -maxdepth 2 -name SKILL.md | sort`
- Text search: `rg -n "docs/sdd/<change>|sare-task-executor|spec.md|tasks.md" AGENTS.md .codex/skills`
- Runtime tests: not required because no runtime code changes.

## Risks And Tradeoffs

| Risk | Mitigation |
|---|---|
| Treating documentation setup as runtime validation | Explicitly limit this smoke test to workflow artifact validation. |
