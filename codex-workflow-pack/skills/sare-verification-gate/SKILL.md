---
name: sare-verification-gate
description: "Trigger: complete, done, commit, push, PR, verify, final answer. Require evidence before success claims."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load before claiming work is complete, committing, pushing, opening a PR, or reporting that tests/builds pass.

## Hard Rules

- No success claim without fresh evidence from this work session.
- Run the command that actually proves the claim.
- Read command output and exit status before reporting.
- Do not treat lint, type-check, build, and tests as interchangeable.
- If verification cannot run, say why and report residual risk.

## Decision Gates

| Claim | Required Evidence |
|---|---|
| Bug fixed | Reproduction or regression test now passes. |
| Feature complete | Acceptance criteria mapped to passing checks. |
| Tests pass | Test command exit 0 and relevant summary. |
| Build passes | Build/type-check command exit 0. |
| UI works | Browser/screenshot interaction evidence. |
| Ready to commit/PR | Diff reviewed plus relevant checks passed. |

## Execution Steps

1. List the claims you are about to make.
2. Map each claim to a command or inspection.
3. Run the narrowest sufficient checks, then broader checks for shared surfaces.
4. Inspect failures instead of summarizing optimistically.
5. Report pass/fail/not-run with exact commands.

## Output Contract

Return verification commands, results, unverified claims, and next action. Keep it concise.

## References

None.

