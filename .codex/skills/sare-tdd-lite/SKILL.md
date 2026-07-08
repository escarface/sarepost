---
name: sare-tdd-lite
description: "Trigger: bugfix, regression, behavior change, business logic, integration. Use pragmatic red-green verification."
license: Apache-2.0
metadata:
  author: "asierluengo"
  version: "1.0"
---

## Activation Contract

Load for bug fixes, regressions, business logic, integrations, behavior changes, and any implementation where tests can prove behavior.

## Hard Rules

- Reproduce or specify the failing behavior before changing production code.
- Prefer a failing automated test first.
- If test-first is impractical, state why and define substitute verification before editing.
- Test behavior, not implementation details.
- Keep the implementation minimal until the test passes.
- Do not claim a regression is fixed without runtime evidence.

## Decision Gates

| Situation | Action |
|---|---|
| Existing test runner and behavior is testable | Write failing test, run it, implement, rerun. |
| Bug cannot be unit-tested cheaply | Add integration/e2e/manual reproduction evidence. |
| UI-only styling | Use screenshot/browser verification instead of unit TDD. |
| Configuration-only change | Verify with build, lint, dry-run, or CLI validation. |
| No test framework exists | Add the smallest project-consistent test setup only if the change risk justifies it. |

## Execution Steps

1. Identify expected behavior and current failure.
2. Add or select the narrowest test/reproduction.
3. Run it and confirm the failure is meaningful.
4. Implement the smallest code change.
5. Rerun the narrow test.
6. Run broader verification when the touched surface is shared.
7. Refactor only after green verification.

## Output Contract

Report RED evidence, GREEN evidence, broader verification, and any skipped test-first rationale.

## References

None.

