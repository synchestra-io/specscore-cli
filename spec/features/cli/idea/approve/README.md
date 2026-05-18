# Feature: Idea Approve

> [View in SpecStudio](https://specstudio.synchestra.io/project/features?id=specscore-cli@synchestra-io@github.com&path=spec%2Ffeatures%2Fcli%2Fidea%2Fapprove) — graph, discussions, approvals
>
> **AI skill:** _planned_ — a `skills/idea/references/approve.md` reference in [`ai-plugin-specscore`](https://github.com/synchestra-io/ai-plugin-specscore) will follow shipping this verb; the skill MUST include a Synchestra-presence pre-flight per the [lifecycle-transitions](../../lifecycle-transitions/README.md) contract.

**Status:** Approved
**Source Ideas:** lifecycle-verbs-for-idea-and-feature

## Summary

`specscore idea approve <slug>` transitions an Idea artifact from `Draft` to `Approved`. It is the first verb implementing the [lifecycle-transitions](../../lifecycle-transitions/README.md) shared contract — atomicity, rollback, output format, exit-code mapping, slug-positional, no coordination, and the architectural positioning vs Synchestra are all inherited from that contract and are not restated here.

## Synopsis

```
specscore idea approve <slug> [--project <path>]
```

## Problem

Today transitioning an Idea from `Draft` to `Approved` is a hand-edit of the `**Status:**` line followed by `specscore spec lint --fix` to sync the ideas-index row. There is no state-machine validation (nothing prevents a hand-edit from `Implementing` back to `Draft`), no atomicity (mutation succeeds but the lint sync is forgotten), and no machine-readable contract for tooling. This verb closes the gap for the single most common Idea transition.

## Behavior

This verb inherits every cross-cutting rule from [lifecycle-transitions](../../lifecycle-transitions/README.md). The REQs below are the verb-specific declarations the shared contract requires each verb to make: legal-source set, target status, and the kind-specific slug-resolution path.

### Legal transition

#### REQ: legal-source-set

The verb's legal-source set is the single value `Draft`. The target status is `Approved`. Per [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness), any other source status — including `Approved` (already at target), `Specified`, `Implementing`, or any unrecognized value — MUST exit `4` (InvalidTransition).

### Kind-specific slug resolution

#### REQ: slug-resolves-to-spec-ideas

The `<slug>` positional argument MUST resolve to a file at `spec/ideas/<slug>.md` within the project root. Archived Ideas at `spec/ideas/archived/<slug>.md` MUST NOT be matched — per [lifecycle-transitions#req:slug-resolves-to-existing-artifact](../../lifecycle-transitions/README.md#req-slug-resolves-to-existing-artifact), archived artifacts are excluded from the canonical lookup, and approving an archived Idea is a category error. A missing file MUST exit `3` (NotFound) with a message naming the expected path.

### Index sync target

#### REQ: ideas-index-sync

The post-mutation `specscore spec lint --fix` invocation (per [lifecycle-transitions#req:index-sync-on-success](../../lifecycle-transitions/README.md#req-index-sync-on-success)) MUST rely on the `idea-index-row-sync` lint rule to rewrite the row for `<slug>` in `spec/ideas/README.md` from `Draft` to `Approved`. The verb itself does not edit the index file directly; the rule handles it.

## Parameters

| Name | Required | Description |
|---|---|---|
| `slug` | Yes | Idea slug — identifies `spec/ideas/<slug>.md`. |

## Flags

| Flag | Required | Description |
|---|---|---|
| `--project` | No | Project root. Autodetected per [CLI#req:project-autodetect](../../README.md#req-project-autodetect) when omitted. |

## Exit codes

Inherits the [lifecycle-transitions exit-code mapping](../../lifecycle-transitions/README.md#shared-exit-code-mapping). For this verb specifically:

| Code | Condition |
|---|---|
| `0` | Idea transitioned from `Draft` to `Approved` and the ideas-index row synced. |
| `2` | Missing or malformed `<slug>` argument. |
| `3` | No Idea file at `spec/ideas/<slug>.md`. |
| `4` | Source status was not `Draft` (illegal transition, including re-running on `Approved`). |
| `10` | I/O failure or `spec lint --fix` failed after a successful file rewrite (rollback applied). |

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [lifecycle-transitions](../../lifecycle-transitions/README.md) | Defines every cross-cutting REQ this verb satisfies: atomicity, rollback, output format, slug-positional, no-coordination, exit-code mapping. |
| [idea (CLI group)](../README.md) | Parent group. The Contents table is amended to include this sub-feature. The group-level `REQ: ideas-only` rule applies: this verb MUST NOT mutate anything under `spec/features/`. |
| [spec lint](../../spec/lint/README.md) | Invoked internally by the shared contract; the `idea-index-row-sync` rule is the specific rule that syncs this verb's effect. |
| [idea](../../../idea/README.md) | Source of truth for the Idea document structure and the `**Status:**` line this verb rewrites. |
| Source Idea: [lifecycle-verbs-for-idea-and-feature](../../../../ideas/lifecycle-verbs-for-idea-and-feature.md) | This verb is the first slice of the approved Idea. |

## Acceptance Criteria

### AC: draft-to-approved-happy-path

**Requirements:** [cli/idea/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:status-line-rewrite](../../lifecycle-transitions/README.md#req-status-line-rewrite), [lifecycle-transitions#req:index-sync-on-success](../../lifecycle-transitions/README.md#req-index-sync-on-success), [lifecycle-transitions#req:success-output-format](../../lifecycle-transitions/README.md#req-success-output-format)

Given a project with `spec/ideas/foo.md` containing `**Status:** Draft`, running `specscore idea approve foo` exits `0`, writes exactly `foo: Draft → Approved\n` to stdout, leaves stderr empty, rewrites the `**Status:**` line in `spec/ideas/foo.md` to `Approved` (with all other lines byte-identical), and updates the corresponding row in `spec/ideas/README.md` to show status `Approved`.

### AC: already-approved-rejected

**Requirements:** [cli/idea/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness), [lifecycle-transitions#req:not-idempotent](../../lifecycle-transitions/README.md#req-not-idempotent)

Given the same Idea is in `**Status:** Approved`, running `specscore idea approve foo` exits `4`, writes a stderr message naming `Approved` as the current status and `Draft` as the legal source, leaves stdout empty, and makes no change to `spec/ideas/foo.md` or `spec/ideas/README.md`.

### AC: invalid-source-state-rejected

**Requirements:** [cli/idea/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness)

Given an Idea in `**Status:** Implementing` (or any value other than `Draft`), running `specscore idea approve` on it exits `4` with a stderr message naming the current status. The file is unchanged.

### AC: missing-slug-rejected

**Requirements:** [lifecycle-transitions#req:slug-positional](../../lifecycle-transitions/README.md#req-slug-positional)

Running `specscore idea approve` with no positional argument exits `2`, writes a stderr message stating that `<slug>` is required, and makes no filesystem change.

### AC: slug-not-found

**Requirements:** [cli/idea/approve#req:slug-resolves-to-spec-ideas](#req-slug-resolves-to-spec-ideas), [lifecycle-transitions#req:slug-resolves-to-existing-artifact](../../lifecycle-transitions/README.md#req-slug-resolves-to-existing-artifact)

Running `specscore idea approve nonexistent` in a project where `spec/ideas/nonexistent.md` does not exist exits `3`, writes a stderr message naming the expected path, and makes no filesystem change. An archived file at `spec/ideas/archived/nonexistent.md` does NOT satisfy the lookup.

### AC: lint-failure-rolls-back

**Requirements:** [lifecycle-transitions#req:rollback-on-lint-failure](../../lifecycle-transitions/README.md#req-rollback-on-lint-failure)

Given `spec/ideas/foo.md` in `**Status:** Draft` and a corrupted `spec/ideas/README.md` that causes `spec lint --fix` to fail with an error-severity violation, running `specscore idea approve foo` exits `10`, restores `spec/ideas/foo.md` to its pre-invocation `**Status:** Draft` content, writes a stderr message naming the lint violation, and leaves the corrupted `README.md` untouched (the corruption is reported, not fixed).

## Outstanding Questions

- When the Idea status enumeration formally grows beyond `Draft`/`Approved` to include `Specified`, `Implementing`, `Archived` as first-class status values (rather than directory location or Synchestra-managed state), is the legal-source set for `idea approve` revisited? Today the rule is `Draft → Approved` only; future statuses might justify e.g. `Specified → Approved` if a Specified Idea reverts. Lock down at the time the new status lands.
- Should the parent `cli/idea/README.md`'s remaining Outstanding Question (whether `idea list` / `idea info` symmetric with `feature list` / `feature info` is needed) be answered before or after lifecycle verbs ship? They're independent surfaces; lean independent, but worth confirming.

---
*This document follows the https://specscore.md/feature-specification*
