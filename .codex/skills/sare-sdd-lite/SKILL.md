---
name: sare-sdd-lite
description: "Trigger: feature, architecture, API, data model, workflow, SDD, specs, tasks. Create lightweight spec/task artifacts."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load for new features, non-trivial bugs, architecture decisions, public APIs, data model changes, cross-file workflows, or any user-invoked SDD flow.

## Hard Rules

- Persist `docs/sdd/<change>/spec.md` before implementation unless the task is truly trivial.
- Generate `docs/sdd/<change>/tasks.md` from the spec before coding.
- Keep artifacts lightweight unless risk requires full OpenSpec.
- Acceptance criteria must be testable.
- Design must name tradeoffs when more than one valid approach exists.
- Every task must map to a requirement or scenario.
- Do not mark tasks complete without verification evidence.

## Decision Gates

| Risk | Artifact Level |
|---|---|
| Trivial one-file edit | Inline brief is allowed. |
| Non-trivial bug or feature | Create `spec.md` and `tasks.md`. |
| Cross-file or architectural change | Add `design.md`. |
| Public API, persistence, auth, payments, security, migrations | Use full SDD/OpenSpec-style artifacts. |
| Ambiguous product behavior | Ask one focused question before planning. |

## Execution Steps

1. Explore current code and constraints.
2. Create `docs/sdd/<change>/spec.md` from `assets/spec-template.md`.
3. Define requirements and scenarios in observable terms.
4. Create `docs/sdd/<change>/tasks.md` from `assets/tasks-template.md`.
5. Map every task to a spec requirement or scenario.
6. Add `design.md` only when tradeoffs or architecture need persistence.
7. Hand implementation to `sare-task-executor`, preferring TDD-lite for behavioral slices.
8. Update spec/tasks if implementation changes accepted behavior.

## Output Contract

Return artifact paths, accepted scope, task count, verification strategy, and any open questions.

## References

- `assets/spec-template.md`
- `assets/tasks-template.md`
