---
name: sare-workflow-lite
description: "Trigger: any coding task, workflow decision, feature, bugfix, new app. Route work into the smallest reliable process."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load for every coding or project-setup task after `karpathy-guidelines`. Use it to choose the workflow, not to perform the whole task.

## Hard Rules

- Use the smallest workflow that preserves correctness.
- Investigate before asserting.
- Do not load heavy SDD/TDD references unless the decision table requires them.
- Keep user-facing updates concise and evidence-based.
- Ask at most one blocking question; otherwise make a conservative assumption and proceed.

## Decision Gates

| Situation | Action |
|---|---|
| Simple explanation | Answer after checking relevant context. |
| Small mechanical edit | Inspect, patch, run narrow verification. |
| Bug or behavior change | Load `sare-tdd-lite`. |
| New feature or architectural change | Load `sare-sdd-lite`. |
| Executing existing SDD tasks | Load `sare-task-executor`. |
| Parallel task execution or worker worktrees | Load `sare-worktree-manager`. |
| Greenfield app | Load `sare-new-app`. |
| Before completion, commit, push, or PR | Load `sare-verification-gate`. |
| Large independent workstreams | Consider delegation. |

## Execution Steps

1. Classify the request using the decision gates.
2. State assumptions only when they affect scope or risk.
3. Choose the narrowest useful investigation path: CodeGraph when available for architecture, otherwise `rg` and targeted reads.
4. Execute the selected workflow.
5. Keep changed files scoped to the request.
6. Verify with the most relevant command before reporting completion.

## Output Contract

Report the selected mode, files changed, verification evidence, and remaining risks or gaps. Omit sections that do not apply.

## References

None.
