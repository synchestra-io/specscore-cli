---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Fixture exercises I-011 global slug uniqueness — this issue is lint-valid
in isolation but collides with `spec/features/example/issues/foo.md`.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: an I-011 violation naming both colliding paths and the slug `foo`.
