---
type: issue
slug: bar
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: Foo

## Description

Fixture exercises I-010 slug-mismatch: frontmatter slug is `bar` but the
filename is `foo.md`.

## Steps to Reproduce

Run `specscore spec lint` against this tree.

## Expected vs Actual

Expected: an I-010 violation naming the mismatch. (I-001 also fires under
its slug-vs-filename template — both rules are intentionally redundant.)
