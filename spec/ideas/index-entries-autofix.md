# Idea: index-entries autofix for spec lint --fix

**Status:** Specified
**Date:** 2026-05-18
**Owner:** alexander.trakhimenok
**Promotes To:** cli/spec/lint
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we let `specscore spec lint --fix` repair drift between feature index READMEs and their child directories so authors no longer have to hand-edit index tables when adding or removing features?

## Context

The `index-entries` lint rule (see [`cli/spec/lint`](../features/cli/spec/lint/README.md#req-index-entries-bidirectional)) now flags drift in both directions: a parent index that links a non-existent child, AND a child directory that is not linked from its parent index. The rule is report-only — `specscore spec lint --fix` does nothing for it. Every drift today requires the author to hand-edit the parent README, which is exactly the kind of mechanical edit a linter should own.

The bidirectional check landed alongside a fix for a silent-pass bug where orphan child directories were never flagged. The `## Behavior > Features index synchronization` section of `cli/spec/lint` defines the report-only contract; this Idea is the natural follow-up that closes the loop by making the rule self-healing where it is safe to do so.

Prior art in the same package: `adherence-footer` already implements `--fix` (rewrites trailing footer URLs and appends missing footers), constrained by the `fix-is-safe-subset` REQ that forbids autofixes "that require semantic interpretation of document intent beyond structural conventions." That REQ is the design constraint this Idea must respect.

## Recommended Direction

**Two-phase MVP, asymmetric by safety.**

**Phase 1 — delete-only (mechanically safe).** When `index-entries` reports `Index mentions non-existent directory: <name>`, `--fix` removes the offending row from the parent README's index table. Deleting a row that points at nothing is unambiguous: there is no `<dirname>/README.md` to read, so the row carries no live information. Implementation: locate the line containing the link target `<name>/README.md` inside the index table, drop that one line, preserve table delimiters and surrounding whitespace. This phase ships first as a small `(c *indexEntriesChecker) fix(specRoot string) error` method.

**Phase 2 — insert-with-metadata (requires parser work).** When `index-entries` reports `Child directory not listed in index: <name>`, `--fix` appends a row to the parent index whose columns are populated from the child README's metadata. This phase is **blocked on a canonical feature-metadata parser** that can read `Status` and the feature title from a feature README without re-implementing the structural rules every checker would otherwise duplicate. The "Kind" and "Description" columns are harder still — "Description" has no canonical home in a feature README today. Phase 2 is **not** in this Idea's MVP; it is named here so future contributors see the boundary.

**MVP boundary.** Ship Phase 1 only. Add a new REQ to `cli/spec/lint` — `index-entries-fix-deletes-phantom-rows` — paired with an AC that exercises the delete path. Add `--fix` support and an idempotency test. The `fix-is-idempotent` REQ already governs the contract; the new fixer must satisfy it without a new clause.

## Alternatives Considered

**Append placeholder rows for orphan children.** Phase 2 could ship today by inserting a row like `| [name](name/README.md) | TBD | TBD | TBD |`. Rejected because (1) `fix-is-safe-subset` forbids it — choosing a description without reading the child's intent is exactly the semantic interpretation that REQ excludes — and (2) the resulting "TBD"-laden index trades one form of drift for another, and the linter has no rule that flags `TBD` cells, so the placeholders rot indefinitely.

**Skip the rule entirely from `--fix` and keep it forever report-only.** Defensible — `index-entries` violations are loud and quick to fix by hand. Rejected because the *delete* direction is mechanically trivial and high-value: phantom rows accumulate when contributors rename or remove feature directories without updating the parent index, and the human edit is pure busywork. Closing the easy half of the loop is worth the small implementation cost.

**Ship both directions together once a feature-metadata parser exists.** Tempting — it would land the rule fully autofix-capable in one PR. Rejected because the parser is a non-trivial design problem (where does "Description" live? does it come from the feature README's `## Summary` first paragraph, from frontmatter, from a new convention?) and gating the easy half on the hard half ships nothing.

## MVP Scope

`specscore spec lint --fix` deletes index rows that point at non-existent child directories. The fix is idempotent (running twice yields no second-pass changes). The orphan-child direction (insert) remains report-only and is out of scope for the MVP.

## Not Doing (and Why)

- Inserting fully-populated rows for new children — requires reading Status/Kind/Description from child READMEs, which has no canonical parser yet.
- Rewriting non-table index formats (bullet lists, prose Contents sections) — out of scope until a canonical index format is settled.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Every `[label](dirname/README.md)` index link sits on a single line within a Markdown table (no multi-line continuations, no link-reference style). | Grep every existing `spec/features/**/README.md` in this repo and in the upstream `specscore` repo for index links; confirm 100% sit on a single line. |
| Should-be-true | Deleting a single table row from a parent README never breaks adjacent rules (heading levels, OQ section, adherence footer). | Add a fixture-based test that runs `spec lint` before, applies `--fix`, then runs `spec lint` again and confirms no rule regresses. |
| Might-be-true | A future canonical feature-metadata parser will land that makes Phase 2 (insert with metadata) tractable without re-implementing structural rules per-checker. | Track whether any in-flight Idea proposes such a parser; if none exists in 2 quarters, re-evaluate whether Phase 2 should ever ship or whether report-only is the permanent posture for the insert direction. |


## SpecScore Integration

- **New Features this would create:** none — extends the existing `cli/spec/lint` Feature with one new REQ (`index-entries-fix-deletes-phantom-rows`) and one matching AC.
- **Existing Features affected:** [`cli/spec/lint`](../features/cli/spec/lint/README.md) (adds autofix support to the `index-entries` rule).
- **Dependencies:** none for Phase 1. Phase 2 (out of MVP) would depend on a canonical feature-metadata parser that does not yet exist.

## Outstanding Questions

None at this time.

---
*This document follows the https://specscore.md/idea-specification*
