---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Minimal valid-looking issue fixture used by the default-suite test.
Stub rules (I-001..I-008, I-010..I-015) emit nothing; I-009 sees a
pattern-matching path and emits nothing.

## Steps to Reproduce

Run `specscore spec lint` with no flags.

## Expected vs Actual

Expected: 0 violations. Actual: 0 violations.
