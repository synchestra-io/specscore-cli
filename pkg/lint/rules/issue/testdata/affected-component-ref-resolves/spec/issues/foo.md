---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
affected_component: example
---

# Issue: Foo

## Description

Fixture issue whose `affected_component` resolves to
`spec/features/example/README.md`. Rule I-012 must stay silent.

## Steps to Reproduce

Run `specscore spec lint`.

## Expected vs Actual

Expected: no I-012 violation. Actual: validated by the test.
