# Idea: Audit lint rules for parallel bugs

**Status:** Draft
**Date:** 2026-05-18
**Owner:** alexander.trakhimenok
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we systematically check every registered lint rule for the same classes of bug we just found in index-entries — silent-pass directionality gaps, missing autofix support where mechanically safe, and same-pass idempotency violations — without doing a 30-rule manual code review every time we ship a fix?

## Context

While shipping the `index-entries-autofix` Idea, three distinct classes of bug surfaced in code that had been live and presumed-working for some time:

1. **Silent-pass directionality gap.** `index-entries` was originally one-directional — it flagged phantom links in the index but not orphan child directories on disk. The asymmetry compiled, lint passed, and contributors who added a feature dir without updating the parent index got zero signal.
2. **`FeatureSourceIdeas` only walked top-level feature dirs.** Nested feature READMEs with `**Source Ideas:**` headers were invisible to the lint engine, so every nested-feature promotion silently no-op'd. `lifecycle-verbs-for-idea-and-feature` had been stuck at Approved for that reason despite three nested features referencing it.
3. **Same-pass idempotency violation in `idea-sync-lint-strict`.** The rule rewrote Idea files on disk in step 6 of the chain, but step 7 (`ideaIndexRules`) read a stale in-memory `parsed` snapshot, so `--fix` needed two passes to fully promote an Idea. The `fix-is-idempotent` REQ on cli/spec/lint says pass 2 must be a no-op.

Each was a one-line or one-loop fix once found, but each had been hiding because nobody had looked. The lint package today registers ~30 rules. We have no reason to believe `index-entries` was special — the same bug classes could be lurking anywhere.

Prior art for the audit approach: the `cli/spec/lint` feature already documents the REQs each rule must satisfy (severity, exit code, idempotency, bidirectional checks where applicable). The audit is checking each registered rule against that contract, not inventing new contracts.

## Recommended Direction

**Spreadsheet, not subsystem.** Audit by hand using a fixed checklist; do not build infrastructure to automate the audit. The audit happens once per rule and the time saved by tooling does not pay back the maintenance.

For each rule registered in `pkg/lint/lint.go` `allRuleNames`:

1. **Check direction.** Does the rule have an inherent direction (X must agree with Y)? If yes, does the check fire both X→Y and Y→X? Write the answer.
2. **Autofix mechanical safety.** Is there a class of violations the rule reports that would be unambiguous to fix? If yes, does the rule implement `fixer`? If no fixer, is the abstention deliberate (per `fix-is-safe-subset`) or accidental?
3. **Same-pass idempotency.** Does the rule mutate state (on disk or in-memory) that a later rule in the chain reads? If yes, is the downstream rule's read refreshed after the upstream's write?
4. **Test coverage.** Does the rule have tests for the directions / fix paths / idempotency it claims to support?

The output is a 30-row Markdown table committed to the repo at `spec/ideas/audit-lint-rules-for-parallel-bugs.md` (this file, updated post-audit) with one row per rule, columns: rule name, direction-gap?, fix-gap?, idempotency-risk?, coverage-gap?, follow-up Idea (if any).

Each cell with a "yes" promotes to a new follow-up Idea (or, if the fix is small, lands directly on a branch with this Idea cited in the commit message).

## Alternatives Considered

**Build a meta-linter that lints the linters.** Tempting — code that walks `allRuleNames` and reports rules missing `fix()` implementations, rules whose check function returns no violations on any fixture, etc. Rejected because the heuristics are noisy (a rule legitimately may not need `fix()`) and the maintenance is real. Manual audit with a checklist catches the same gaps with a fraction of the code.

**Wait for bugs to surface organically.** Defensible — each of the three bugs above was found because a real workflow hit it. We could simply react. Rejected because the cost of an undetected lint gap is months of silent rot (orphan features, stuck Ideas, mis-synced indices) that compound; the audit is a few hours and front-loads the discovery.

**Generate stress-test fixtures for every rule.** Property-based testing or fuzz-style coverage. Rejected as a poor fit — lint rules check structural invariants on small fixed-format documents, not search spaces with adversarial inputs. The cost-to-coverage ratio is bad.

## MVP Scope

Hand-walk every rule in `pkg/lint/lint.go` `allRuleNames`, fill in the 4-column checklist for each, write the output table back into this Idea's body, and file one follow-up Idea per "yes" cell. Single deliverable: the populated table. Timeboxed: two focused sessions of ~90 minutes each.

## Not Doing (and Why)

- Rewriting any individual rule preemptively — this Idea covers detection and prioritization only. Each rule that warrants a fix promotes to its own follow-up.
- Building a meta-linter that lints the linters — scope creep; manual audit with a checklist is the MVP.

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | Every registered rule's check function is reachable from a fixture in the lint test suite (i.e., we are auditing live code, not dead code). | Cross-reference `allRuleNames` against test file mentions; any rule with zero references is dead. |
| Should-be-true | The four checklist axes (direction, fix safety, idempotency, coverage) catch every class of bug we found in this session. No fifth class exists that the checklist would miss. | Apply the checklist to the three known bugs retroactively; each must map cleanly to one of the four axes. |
| Might-be-true | The audit will find at least one bug per ~10 rules on average. If we find zero across 30 rules, the checklist is too coarse and needs revision. | Count findings at the halfway point (15 rules); recalibrate if the rate is materially lower than expected. |


## SpecScore Integration

- **New Features this would create:** none directly. Findings may promote into follow-up Ideas that themselves promote to existing or new lint Feature REQs.
- **Existing Features affected:** [`cli/spec/lint`](../features/cli/spec/lint/README.md) — the audit's contract is whatever this Feature documents about rule behavior.
- **Dependencies:** none. The audit reads code that already exists.

## Open Questions

None at this time.

---
*This document follows the https://specscore.md/idea-specification*
