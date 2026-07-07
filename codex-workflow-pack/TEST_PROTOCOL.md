# Test Protocol

## Goal

Validate that the workflow pack is useful in real development without adding unnecessary ceremony or token cost.

## Test Levels

### Level 1: Dry Run

Use the pack as written, but do not install it globally.

Run three simulated requests in a disposable repo or small project:

1. Simple explanation: "Explain how this module works."
2. Bug fix: "Fix this failing behavior."
3. New feature slice: "Add a small user-facing feature."

Pass criteria:

- `sare-workflow-lite` chooses the expected mode.
- SDD/TDD skills are loaded only when needed.
- The final answer includes verification evidence or explicitly states what was not verified.

### Level 2: Project Pilot

Install `AGENTS.md` and the skills in one active project only.

Run two real tasks:

1. One bug fix with `sare-tdd-lite`.
2. One feature with `sare-sdd-lite`.

Pass criteria:

- The workflow catches at least one missing assumption, test case, or verification gap.
- The artifacts are short enough to maintain.
- The agent does not ask excessive questions.
- The final changes remain reviewable.

### Level 3: Greenfield Pilot

Use `sare-new-app` to create a small app from scratch.

Pass criteria:

- The first screen is usable, not a placeholder.
- The app has a clear first vertical slice.
- Local run command works.
- At least one verification path exists on day one.

## Evaluation Matrix

| Dimension | Good Signal | Bad Signal |
|---|---|---|
| Token cost | Only relevant skills load. | Every task becomes full SDD. |
| Correctness | Claims include command evidence. | Claims are based on assumptions. |
| Flow fit | Agent keeps moving with one-question max. | Agent blocks on minor uncertainty. |
| TDD fit | Tests prove behavior where useful. | TDD is forced onto config or styling. |
| SDD fit | Specs clarify product behavior. | Artifacts repeat obvious implementation details. |
| Maintainability | Commits are focused and conventional. | One large mixed change. |

## Recommended First Test Prompt

Use this in a small existing repo:

```text
Apply our workflow pack to this task: find one small bug or missing validation in this project, propose the smallest safe fix, implement it with TDD-lite if feasible, and verify it. Keep artifacts lightweight.
```

## Review Questions After Testing

- Did the router choose the right mode?
- Did TDD-lite improve confidence or slow the task down?
- Did SDD-lite clarify scope or create noise?
- Was the final verification evidence enough to trust the result?
- Which rule felt too strict?
- Which missing rule would have prevented a mistake?

