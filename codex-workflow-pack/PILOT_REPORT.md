# Pilot Report

## Date

2026-07-07

## Scope

Level 1 dry run using a disposable Python project in `work/workflow-pack-pilot`.

## Workflow Tested

- `karpathy-guidelines`
- `sare-workflow-lite`
- `sare-tdd-lite`
- `sare-verification-gate`

## Simulated Task

Fix missing checkout validation: applying a fixed coupon must never produce a negative total.

## Result

### Routing

`sare-workflow-lite` selected bug/behavior-change mode, which correctly loaded `sare-tdd-lite`.

### RED Evidence

Command:

```bash
python3 -m unittest tests.test_checkout.ApplyCouponTests.test_never_returns_negative_total -v
```

Result:

```text
FAILED (failures=1)
AssertionError: -500 != 0
```

### GREEN Evidence

Command:

```bash
python3 -m unittest tests.test_checkout.ApplyCouponTests.test_never_returns_negative_total -v
```

Result:

```text
Ran 1 test in 0.000s
OK
```

### Broader Verification

Command:

```bash
python3 -m unittest discover -s tests -v
```

Result:

```text
Ran 4 tests in 0.000s
OK
```

## Files Changed In Fixture

- `work/workflow-pack-pilot/src/checkout.py`
- `work/workflow-pack-pilot/tests/test_checkout.py`

## Assessment

Pass. The lightweight workflow produced the desired discipline without full SDD ceremony:

- It used a focused workflow decision.
- It added a failing regression test before production code.
- It made the smallest implementation change.
- It required fresh verification evidence before reporting success.

## Follow-Up Tests

- Run `sare-sdd-lite` on a small feature slice.
- Run `sare-new-app` on a small greenfield app.
- Test the same pack inside a real active repository with existing conventions.

