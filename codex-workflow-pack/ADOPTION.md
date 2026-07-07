# Adoption Guide

## Recommended Rollout

1. Start with one active project.
2. Put `AGENTS.md` at the project root.
3. Install the five `sare-*` skills where Codex can discover project or personal skills.
4. Run two real tasks through the workflow:
   - one small bug fix;
   - one new feature slice.
5. Adjust wording only after observing friction in real work.

## Default Flow

```text
request
  -> karpathy-guidelines
  -> sare-workflow-lite
  -> optional mode skill
  -> implementation
  -> sare-verification-gate
  -> concise final report
```

## Token Budget Policy

- Always loaded: `karpathy-guidelines`, `sare-workflow-lite`.
- Loaded on demand: SDD, TDD, new-app, verification.
- Inline artifacts are acceptable for low-risk work.
- File-backed SDD is reserved for durable product or architecture decisions.
- Full SDD/OpenSpec is reserved for public APIs, persistence, auth, payments, security, migrations, or multi-subsystem changes.

## CLI Access Policy

Codex can use team CLIs as working tools, but commands fall into three classes:

| Class | Examples | Approval |
|---|---|---|
| Read-only | `git status`, `gh pr view`, `npm test`, `rg` | Allowed when relevant. |
| Local write | formatting, test snapshots, generated code | Allowed inside workspace when scoped. |
| External/destructive | deploys, production DB writes, destructive git, cloud mutations | Ask first. |

## Review Standard

Every meaningful task should end with:

- What changed.
- What verification ran.
- What was not verified.
- Any risk or follow-up that still matters.

## First Iteration Feedback Questions

After using this pack on two tasks, review:

- Did the router choose the right workflow?
- Was TDD-lite strict enough to catch bugs without blocking progress?
- Did SDD-lite clarify scope or create ceremony?
- Were verification reports concise but trustworthy?
- Did any CLI access need clearer boundaries?

