---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
affected_component: nonexistent-feature
---

# Issue: Foo

## Description

Fixture issue whose `affected_component` references a Feature slug that
does not resolve to `spec/features/nonexistent-feature/README.md`.
Rule I-012 must emit a violation naming the unresolved slug.

## Steps to Reproduce

Run `specscore spec lint`.

## Expected vs Actual

Expected: one I-012 violation. Actual: validated by the test.
