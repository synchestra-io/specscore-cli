---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Fixture issue under spec/issues/ alongside a README.md whose Contents
table is missing the `Severity` column so rule I-015 fires.

## Steps to Reproduce

Run `specscore spec lint`.

## Expected vs Actual

Expected: I-015 violation. Actual: I-015 violation.
