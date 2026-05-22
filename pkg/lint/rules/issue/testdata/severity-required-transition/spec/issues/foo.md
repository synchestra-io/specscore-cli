---
type: issue
slug: foo
status: investigating
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Fixture exercises I-005: once `status` transitions out of `open` (here to
`investigating`), `severity` is required and must be one of
`{low, medium, high, critical}` (not absent, not `unset`).

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-005 violation naming severity-required-on-transition.
Actual: validated by the test.
