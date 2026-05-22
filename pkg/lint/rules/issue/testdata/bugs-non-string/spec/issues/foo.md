---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
bugs: [123, "valid-slug"]
---

# Issue: Foo

## Description

Fixture exercises I-004: every element of `bugs` MUST be a string. Here
the first element is the integer `123`.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-004 violation stating every element of `bugs` must be a
string. Actual: validated by the test.
