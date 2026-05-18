# Feature: Feature Approve

> [View in SpecStudio](https://specstudio.synchestra.io/project/features?id=specscore-cli@synchestra-io@github.com&path=spec%2Ffeatures%2Fcli%2Ffeature%2Fapprove) — graph, discussions, approvals
>
> **AI skill:** _planned_ — a `skills/feature/references/approve.md` reference in [`ai-plugin-specscore`](https://github.com/synchestra-io/ai-plugin-specscore) will follow shipping this verb; the skill MUST include a Synchestra-presence pre-flight per the [lifecycle-transitions](../../lifecycle-transitions/README.md) contract.

**Status:** Approved
**Source Ideas:** lifecycle-verbs-for-idea-and-feature

## Summary

`specscore feature approve <feature_id>` transitions a Feature artifact from either `Draft` or `Under Review` to `Approved`. It implements the [lifecycle-transitions](../../lifecycle-transitions/README.md) shared contract — atomicity, rollback, output format, exit-code mapping, slug-positional argument, no coordination, and the architectural positioning vs Synchestra are all inherited and not restated here.

## Synopsis

```
specscore feature approve <feature_id> [--project <path>]
```

## Problem

Today transitioning a Feature from `Draft` (or `Under Review`) to `Approved` is a hand-edit of the `**Status:**` line in `spec/features/<feature_id>/README.md` followed by `specscore spec lint --fix` to sync the features-index row. There is no state-machine validation (a hand-edit can drop `Stable` features to `Draft` with no warning), no atomicity guarantee (rewriting the README and forgetting to sync the index produces stale state), and no machine-readable contract for tooling. This verb closes the gap for the Feature kind's primary forward transition.

## Behavior

This verb inherits every cross-cutting rule from [lifecycle-transitions](../../lifecycle-transitions/README.md). The REQs below are the verb-specific declarations the shared contract requires each verb to make: legal-source set, target status, kind-specific identifier resolution, and the kind-specific index-sync rule.

### Legal transition

#### REQ: legal-source-set

The verb's legal-source set is `{Draft, Under Review}`. The target status is `Approved`. Per [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness), any other source status — including `Approved` (already at target), `Implementing`, `Stable`, `Deprecated`, or any unrecognized value — MUST exit `4` (InvalidTransition). The stderr message MUST name both the current status and the legal-source set `{Draft, Under Review}`.

### Kind-specific identifier resolution

#### REQ: feature-id-resolves-to-spec-features

The `<feature_id>` positional argument MUST resolve to a directory at `spec/features/<feature_id>/` containing a `README.md` file within the project root. The identifier may include slashes for nested features (e.g., `cli/idea/approve`), matching the convention used by `feature info <feature_id>` and `feature deps <feature_id>`. A missing directory or missing `README.md` MUST exit `3` (NotFound) with a message naming the expected path. There is no archived-equivalent location for Features today, so the Meta's exclusion of archived artifacts is vacuous for this verb.

### Index sync target

#### REQ: features-index-sync

The post-mutation `specscore spec lint --fix` invocation (per [lifecycle-transitions#req:index-sync-on-success](../../lifecycle-transitions/README.md#req-index-sync-on-success)) MUST rely on a `feature-index-row-sync` lint rule to rewrite the row for `<feature_id>` in `spec/features/README.md` from its prior status to `Approved`. Today only `idea-index-row-sync` exists in `pkg/lint/`; implementing this verb requires landing an analogous `feature-index-row-sync` rule. The verb itself does not edit the index file directly; the lint rule handles it. Implementation of the verb and the lint rule MUST land together — shipping the verb without the rule would leave the index drifting on every transition.

## Parameters

| Name | Required | Description |
|---|---|---|
| `feature_id` | Yes | Feature identifier — identifies `spec/features/<feature_id>/README.md`. Slashes are allowed for nested features. |

## Flags

| Flag | Required | Description |
|---|---|---|
| `--project` | No | Project root. Autodetected per [CLI#req:project-autodetect](../../README.md#req-project-autodetect) when omitted. |

## Exit codes

Inherits the [lifecycle-transitions exit-code mapping](../../lifecycle-transitions/README.md#shared-exit-code-mapping). For this verb specifically:

| Code | Condition |
|---|---|
| `0` | Feature transitioned from `Draft` or `Under Review` to `Approved` and the features-index row synced. |
| `2` | Missing or malformed `<feature_id>` argument. |
| `3` | No Feature directory or `README.md` at `spec/features/<feature_id>/`. |
| `4` | Source status was not `Draft` or `Under Review` (illegal transition, including re-running on `Approved`). |
| `10` | I/O failure or `spec lint --fix` failed after a successful file rewrite (rollback applied). |

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [lifecycle-transitions](../../lifecycle-transitions/README.md) | Defines every cross-cutting REQ this verb satisfies: atomicity, rollback, output format, slug-positional, no-coordination, exit-code mapping. |
| [feature (CLI group)](../README.md) | Parent group. The Contents table is amended to include this sub-feature. |
| [spec lint](../../spec/lint/README.md) | Invoked internally by the shared contract. The `feature-index-row-sync` lint rule (to be added to `pkg/lint/`) is the specific rule that syncs this verb's effect. |
| [feature](../../../feature/README.md) | Source of truth for the Feature document structure and the `**Status:**` line this verb rewrites. Source of the legal status enumeration (`Draft`, `Under Review`, `Approved`, `Implementing`, `Stable`, `Deprecated`). |
| [cli/idea/approve](../../idea/approve/README.md) | Sibling verb for the Idea kind. Both consume the same shared contract; the differences are the legal-source set (Idea has only `{Draft}`) and the kind-specific identifier/path. |
| Source Idea: [lifecycle-verbs-for-idea-and-feature](../../../../ideas/lifecycle-verbs-for-idea-and-feature.md) | The Recommended Direction specifies `feature approve` accepts `Draft → Approved` and `Under Review → Approved` as legal transitions. This verb implements that decision. |

## Acceptance Criteria

### AC: draft-to-approved-happy-path

**Requirements:** [cli/feature/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:status-line-rewrite](../../lifecycle-transitions/README.md#req-status-line-rewrite), [lifecycle-transitions#req:index-sync-on-success](../../lifecycle-transitions/README.md#req-index-sync-on-success), [lifecycle-transitions#req:success-output-format](../../lifecycle-transitions/README.md#req-success-output-format)

Given a project with `spec/features/auth/README.md` containing `**Status:** Draft`, running `specscore feature approve auth` exits `0`, writes exactly `auth: Draft → Approved\n` to stdout, leaves stderr empty, rewrites the `**Status:**` line in `spec/features/auth/README.md` to `Approved` (with all other lines byte-identical), and updates the corresponding row in `spec/features/README.md` to show status `Approved`.

### AC: under-review-to-approved-happy-path

**Requirements:** [cli/feature/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:status-line-rewrite](../../lifecycle-transitions/README.md#req-status-line-rewrite), [lifecycle-transitions#req:success-output-format](../../lifecycle-transitions/README.md#req-success-output-format)

Given `spec/features/auth/README.md` containing `**Status:** Under Review`, running `specscore feature approve auth` exits `0`, writes exactly `auth: Under Review → Approved\n` to stdout, rewrites the Status line, and syncs the features-index row. The two legal source states are both accepted by the same invocation; no flag selects between them.

### AC: already-approved-rejected

**Requirements:** [cli/feature/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness), [lifecycle-transitions#req:not-idempotent](../../lifecycle-transitions/README.md#req-not-idempotent)

Given the same Feature is in `**Status:** Approved`, running `specscore feature approve auth` exits `4`, writes a stderr message naming `Approved` as the current status and `{Draft, Under Review}` as the legal source set, leaves stdout empty, and makes no change to `spec/features/auth/README.md` or `spec/features/README.md`.

### AC: invalid-source-state-rejected

**Requirements:** [cli/feature/approve#req:legal-source-set](#req-legal-source-set), [lifecycle-transitions#req:state-machine-strictness](../../lifecycle-transitions/README.md#req-state-machine-strictness)

Given a Feature in `**Status:** Stable` (or any value outside `{Draft, Under Review}`), running `specscore feature approve` on it exits `4` with a stderr message naming the current status and listing the legal source set. The file is unchanged.

### AC: nested-feature-id-resolves

**Requirements:** [cli/feature/approve#req:feature-id-resolves-to-spec-features](#req-feature-id-resolves-to-spec-features)

Given a nested feature at `spec/features/cli/idea/approve/README.md` containing `**Status:** Draft`, running `specscore feature approve cli/idea/approve` exits `0`. Slashes in `<feature_id>` are accepted and resolve as a path within `spec/features/`, matching the convention used by `feature info` and `feature deps`.

### AC: missing-feature-id-rejected

**Requirements:** [lifecycle-transitions#req:slug-positional](../../lifecycle-transitions/README.md#req-slug-positional)

Running `specscore feature approve` with no positional argument exits `2`, writes a stderr message stating that `<feature_id>` is required, and makes no filesystem change.

### AC: feature-not-found

**Requirements:** [cli/feature/approve#req:feature-id-resolves-to-spec-features](#req-feature-id-resolves-to-spec-features), [lifecycle-transitions#req:slug-resolves-to-existing-artifact](../../lifecycle-transitions/README.md#req-slug-resolves-to-existing-artifact)

Running `specscore feature approve nonexistent` in a project where `spec/features/nonexistent/README.md` does not exist exits `3`, writes a stderr message naming the expected path, and makes no filesystem change.

### AC: lint-failure-rolls-back

**Requirements:** [lifecycle-transitions#req:rollback-on-lint-failure](../../lifecycle-transitions/README.md#req-rollback-on-lint-failure)

Given `spec/features/auth/README.md` in `**Status:** Draft` and a corrupted `spec/features/README.md` that causes `spec lint --fix` to fail with an error-severity violation, running `specscore feature approve auth` exits `10`, restores `spec/features/auth/README.md` to its pre-invocation `**Status:** Draft` content, writes a stderr message naming the lint violation, and leaves the corrupted `README.md` untouched.

## Outstanding Questions

- The `feature-index-row-sync` lint rule does not exist in `pkg/lint/` today (only `idea-index-row-sync` exists). Should the rule be added as part of implementing this verb (lean: yes), or via a separate prior task? Either way, the verb cannot ship without the rule.
- Should `feature approve` accept a `--require-review` flag that constrains the legal-source set to `{Under Review}` only (i.e., reject direct `Draft → Approved` transitions in projects that enforce a review step)? Deferred; today both sources are accepted unconditionally per the source Idea.
- When the Feature status enumeration grows (e.g., adding `Archived` as a terminal beyond `Deprecated`), is the legal-source set for `feature approve` revisited? Today the rule is `{Draft, Under Review} → Approved`; future statuses might justify additions.
- Should the verb emit a warning when transitioning from `Draft` directly to `Approved` (i.e., skipping `Under Review`)? A "you skipped the review step" advisory could help teams that nominally require review but don't enforce it. Deferred; current position is silent acceptance.

---
*This document follows the https://specscore.md/feature-specification*
