# Tasks: Workflow Smoke Test

## Status Legend

- `[ ]` Pending
- `[~]` In progress
- `[x]` Complete and verified
- `[!]` Blocked

## Tasks

- [x] T1: Verify workflow skill files are present
  - Spec: R1 / Scenario `Skill files are present`
  - Agent: Codex
  - Files: `.codex/skills`
  - Work: List installed workflow skill files.
  - Verification: `find .codex/skills -maxdepth 2 -name SKILL.md | sort`
  - Evidence: Passed. Found six workflow skills: `sare-workflow-lite`, `sare-sdd-lite`, `sare-task-executor`, `sare-tdd-lite`, `sare-verification-gate`, and `sare-new-app`.

- [x] T2: Verify AGENTS references spec/task SDD policy
  - Spec: R2 / Scenario `AGENTS defines SDD artifacts`
  - Agent: Codex
  - Files: `AGENTS.md`, `.codex/skills`
  - Work: Search for the project-visible SDD artifact policy.
  - Verification: `rg -n "docs/sdd/<change>|sare-task-executor|spec.md|tasks.md" AGENTS.md .codex/skills`
  - Evidence: Passed. Matches found in `AGENTS.md`, `sare-sdd-lite`, `sare-task-executor`, and `sare-workflow-lite`.

## Verification Summary

| Task | Status | Evidence |
|---|---|---|
| T1 | Complete | Workflow skill files found. |
| T2 | Complete | SDD artifact policy found. |
