# Codex Workflow Pack

This pack is a lightweight alternative to a strict Superpowers workflow. It keeps the useful guarantees: explicit routing, spec-before-code for meaningful features, test-first where it pays off, and evidence before completion claims.

## Files

- `AGENTS.md`: project-level operating contract.
- `skills/sare-workflow-lite`: router skill loaded for coding tasks.
- `skills/sare-sdd-lite`: lightweight SDD for features and architecture.
- `skills/sare-tdd-lite`: pragmatic red-green workflow.
- `skills/sare-verification-gate`: evidence gate before completion claims.
- `skills/sare-new-app`: greenfield app workflow.

## Philosophy

The workflow is intentionally token-aware:

- Always load only `karpathy-guidelines` and `sare-workflow-lite`.
- Load SDD/TDD/new-app/verification only when the task requires it.
- Keep skill bodies short and put detail in project artifacts when needed.
- Escalate from inline notes to full SDD only when risk justifies it.

## Suggested Installation

After review, copy each folder under `skills/` into your Codex skills directory or a project-local skills directory, then place `AGENTS.md` at the project root.

Do not install globally until the wording matches your preferred operating style.

See `ADOPTION.md` for rollout, token-budget, and CLI-access policy.
