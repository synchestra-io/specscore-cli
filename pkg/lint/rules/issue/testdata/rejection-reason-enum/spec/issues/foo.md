---
type: issue
slug: foo
status: rejected
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
severity: low
rejection_reason: not-real-enough
---

# Issue: Foo

## Description

Fixture exercises I-006 (rejection_reason enum). `not-real-enough` is not
one of the six valid values. Because `severity` is set to a valid non-
`unset` value (`low`), I-005 stays silent — only I-006 fires.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-006 violation listing the six valid `rejection_reason`
values. I-005 emits nothing on this fixture. Actual: validated by the test.
