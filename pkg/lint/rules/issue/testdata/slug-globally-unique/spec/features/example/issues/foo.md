---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Feature-scoped twin of `spec/issues/foo.md` — lint-valid in isolation,
collides with the root-level fixture to trigger I-011.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: an I-011 violation naming both colliding paths and the slug `foo`.
