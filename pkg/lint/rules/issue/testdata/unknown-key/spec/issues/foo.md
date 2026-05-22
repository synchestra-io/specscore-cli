---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
priority: high
---

# Issue: Foo

## Description

Fixture exercises I-001 unknown-field: `priority` is not a known issue
frontmatter key.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: one I-001 violation naming `priority` under the unknown-field
template. Actual: validated by the test.
