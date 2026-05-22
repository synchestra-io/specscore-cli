---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
---

# Issue: Foo

## Description

Fixture exercises I-001 missing-required-field: `captured_by` is absent.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-001 violation naming `captured_by`. Actual: validated by the test.
