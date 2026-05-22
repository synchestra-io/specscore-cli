---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Expected vs Actual

Expected: one I-008 violation reporting non-canonical section order.
Actual: validated by the test.

## Description

Fixture exercises I-008: the three required H2 sections are present but
appear in non-canonical order (Expected vs Actual first, then
Description, then Steps to Reproduce).

## Steps to Reproduce

Run `specscore spec lint` against this tree.
