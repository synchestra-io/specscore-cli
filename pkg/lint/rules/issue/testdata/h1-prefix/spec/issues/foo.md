---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Bug: Menu crashes

## Description

Fixture exercises I-007: the H1 must match `^# Issue: .+$`. Here it
starts with `# Bug:` instead — so only I-007 should fire.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-007 violation naming the required H1 pattern.
Actual: validated by the test.
