---
name: sare-task-executor
description: "Trigger: execute tasks, continue tasks, mark tasks, multi-agent tasks. Implement and verify SDD task files."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load when implementing from `docs/sdd/<change>/tasks.md`, continuing incomplete tasks, or coordinating multiple agents against a shared task file.

## Hard Rules

- Read `spec.md` before executing any task.
- Claim only tasks marked `[ ]` or explicitly assigned to this agent.
- Change a task to `[~]` before editing files.
- Mark `[x]` only after running the task's verification and recording evidence.
- Use `[!]` with a short reason when blocked.
- Do not overwrite another agent's completed evidence.

## Decision Gates

| Condition | Action |
|---|---|
| Task lacks spec mapping | Stop and update tasks before coding. |
| Task lacks verification | Add verification step before coding. |
| Independent tasks exist | Execute one task at a time or delegate separate tasks. |
| Shared file conflict risk | Serialize tasks or assign one agent owner. |
| Verification fails | Keep task incomplete and record failure. |

## Execution Steps

1. Read `spec.md`, `tasks.md`, and optional `design.md`.
2. Select the next pending task and mark it `[~]` with agent name.
3. Implement only the task scope.
4. Run the task verification command.
5. If passing, mark `[x]` and paste concise evidence.
6. If failing or blocked, mark `[!]` or restore `[ ]` with reason.
7. Run broader verification when the touched surface is shared.

## Output Contract

Return task IDs changed, files touched, verification evidence, remaining tasks, and blockers.

## References

None.
