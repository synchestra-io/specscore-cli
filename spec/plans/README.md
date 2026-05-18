# SpecScore CLI Plans

Canonical index of all plans in this repository. Each plan lives in its own directory under `spec/plans/` and is governed by the [plan Feature](https://specscore.md/plan-specification) in the SpecScore meta-spec.

## Contents

| Plan | Status | Features | Effort | Impact | Author | Approved |
|---|---|---|---|---|---|---|
| [lifecycle-verbs-implementation](lifecycle-verbs-implementation/README.md) | approved | lifecycle-transitions, idea/change-status, feature/change-status | M | high | alexander.trakhimenok | 2026-05-18 |

### lifecycle-verbs-implementation

Implements the two `change-status` CLI verbs (one per doc kind), the shared `pkg/lifecycle/` package, and the new `feature-index-row-sync` lint rule, parallelizing the work across subagents. Realizes the [lifecycle-verbs-for-idea-and-feature Idea](../ideas/lifecycle-verbs-for-idea-and-feature.md) and the three feature specifications under `spec/features/cli/`.

## Recently Closed

None at this time.

## Outstanding Questions

None at this time.

---
*This document follows the https://specscore.md/plans-index-specification*
