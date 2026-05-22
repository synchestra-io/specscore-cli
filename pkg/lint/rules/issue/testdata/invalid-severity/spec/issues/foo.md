---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
severity: extreme
---

# Issue: Foo

## Description

Fixture exercises I-003: `extreme` is not one of the five valid `severity`
values (`low`, `medium`, `high`, `critical`, `unset`).

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-003 violation listing the five valid `severity` values.
Actual: validated by the test.
