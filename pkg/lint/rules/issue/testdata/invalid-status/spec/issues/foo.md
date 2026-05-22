---
type: issue
slug: foo
status: triaged
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Fixture exercises I-002: `triaged` is not one of the four valid status values.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-002 violation listing the four valid status values. Actual: validated by the test.
