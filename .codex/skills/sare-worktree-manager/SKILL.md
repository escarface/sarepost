---
name: sare-worktree-manager
description: "Trigger: manager thread, parallel tasks, worktrees, delegate tasks. Coordinate SDD tasks across isolated worker threads."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load in the manager thread when `tasks.md` has multiple tasks, the user asks for parallel execution, or workers should run in separate worktrees.

## Hard Rules

- The manager owns `spec.md`, `tasks.md`, task assignment, integration, and final verification.
- Workers own only their assigned task scope and worktree.
- Parallelize only tasks with no dependency or shared-file conflict.
- Create one worker thread/worktree per independent task or task group.
- Do not let two workers edit the same file unless the manager explicitly serializes integration.
- Generate worker prompts from `assets/worker-prompt-template.md`.
- Record multi-worker integration in `integration.md`.
- A task is complete only after worker evidence is reviewed and manager verification passes.

## Decision Gates

| Condition | Action |
|---|---|
| Tasks have dependencies | Run dependent tasks sequentially. |
| Tasks touch disjoint files | Run in parallel worktrees. |
| Tasks touch shared files | Assign one owner or split further. |
| Worker reports blocked | Manager clarifies, reslices, or reassigns. |
| Worker passes local checks | Manager reviews diff, then integrates. |

## Execution Steps

1. Read `spec.md`, `tasks.md`, and optional `design.md`.
2. Build or update the `tasks.md` task graph: dependencies, touched files, verification commands.
3. Mark runnable tasks `[~]` with assigned worker, thread, worktree, and branch.
4. Create worker threads in isolated worktrees and provide only scoped context.
5. Require workers to use `sare-task-executor` and return diff summary plus evidence.
6. Review each worker result for spec compliance, conflicts, and code quality.
7. Integrate completed work one unit at a time, updating `tasks.md` and `integration.md`.
8. Run full verification before final status.

## Output Contract

Return task graph, assignments, worker thread/worktree IDs, integration report path, verification evidence, and blockers.

## References

- `assets/worker-prompt-template.md`
- `assets/integration-template.md`
