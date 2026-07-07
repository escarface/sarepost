---
name: sare-sdd-lite
description: "Trigger: feature, architecture, API, data model, workflow, SDD. Plan and implement with lightweight specs."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load for new features, architecture decisions, public APIs, data model changes, cross-file workflows, or any user-invoked SDD flow.

## Hard Rules

- Write down observable behavior before implementation.
- Keep artifacts lightweight unless risk requires full OpenSpec.
- Acceptance criteria must be testable.
- Design must name tradeoffs when more than one valid approach exists.
- Tasks must be concrete enough to verify.
- Do not implement until intent, criteria, and design are clear.

## Decision Gates

| Risk | Artifact Level |
|---|---|
| Single component, low risk | Inline brief: intent, criteria, tasks. |
| Cross-file feature | `docs/sdd/<change>/proposal.md`, `design.md`, `tasks.md`. |
| Public API, persistence, auth, payments, security, migrations | Full SDD/OpenSpec-style artifacts. |
| Ambiguous product behavior | Ask one focused question before planning. |

## Execution Steps

1. Explore current code and constraints.
2. Define intent in one paragraph.
3. Define acceptance criteria as observable bullets.
4. Write design notes: approach, touched areas, tradeoffs, rollback.
5. Split work into reviewable tasks.
6. Implement task by task, preferring TDD-lite for behavioral slices.
7. Verify each acceptance criterion with tests, build, type-check, browser checks, or explicit manual evidence.
8. Update artifacts if implementation changes the design.

## Output Contract

Return intent, accepted scope, changed files, verification evidence, and any criteria not fully verified.

## References

None.

