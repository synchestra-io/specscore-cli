---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Should never be loaded because the file lives outside the two
canonical patterns (`spec/issues/*.md` and
`spec/features/*/issues/*.md`). Rule I-009 must flag it.

## Steps to Reproduce

Place an issue file under spec/random-dir/.

## Expected vs Actual

Expected: I-009 violation. Actual: validated by the test.
