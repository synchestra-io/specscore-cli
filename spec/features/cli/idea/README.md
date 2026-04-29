# Feature: Idea (CLI)

> [View in Synchestra Hub](https://hub.synchestra.io/project/features?id=specscore-cli@synchestra-io@github.com&path=spec%2Ffeatures%2Fcli%2Fidea) — graph, discussions, approvals

**Status:** Stable

## Summary

`specscore idea` commands manage SpecScore Idea artifacts — pre-spec, lintable one-pagers stored at `spec/ideas/<slug>.md`. The MVP surface is `idea new`, which scaffolds a fresh Idea with every required section in lint-clean form.

## Problem

Ideas have a strict required-sections contract defined by the [idea](../../idea/README.md) feature. Hand-authoring them means remembering HMW framing, `Not Doing` entries, owner attribution, and status rules. A scaffolder produces a skeleton that is lint-clean on creation, so authors edit content rather than fight structure.

## Contents

| Directory | Description |
|---|---|
| [new/](new/README.md) | Scaffold a new Idea artifact at `spec/ideas/<slug>.md` |

### new

Creates a lint-clean Idea skeleton with every required section, HTML-comment prompts describing what belongs in each, and either flag-supplied or interactively prompted content for the core fields (title, HMW, owner, not-doing entries).

## Behavior

### Scope of this group

Commands under `specscore idea` operate on a single Idea artifact at `spec/ideas/<slug>.md`. They MUST NOT create features — `feature new` is the entry point for features, and this split keeps the two flows discoverable.

#### REQ: ideas-only

No subcommand of `specscore idea` may mutate `spec/features/`. The canonical write target for this group is `spec/ideas/`.

### Lint-clean output

Every artifact scaffolded under this group MUST pass `specscore spec lint` on first creation.

#### REQ: lint-clean-on-create

A file produced by any `specscore idea` mutation command MUST satisfy every lint rule applicable to the Idea document kind. Commands that cannot produce a lint-clean artifact for the given inputs (e.g., a bad slug) MUST exit with a failure code BEFORE writing the file.

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [idea](../../idea/README.md) | Defines the required shape of an Idea. `idea new` produces instances. |
| [feature/new](../feature/new/README.md) | Sibling scaffolder for features. Lives in the `feature` group because features are queried there too. |
| [CLI](../README.md) | Inherits shared exit-code contract, `--project`, `--format`. |
| [`idea/` skill](https://github.com/synchestra-io/ai-plugin-specscore/blob/main/skills/idea/SKILL.md) (ai-plugin-specscore) | Agent-side wrapper for `idea new`. Treats this feature spec as the authoritative contract. |

## Outstanding Questions

- Should `specscore idea` grow read commands (`idea list`, `idea info`) symmetric with `feature list` / `feature info`, or does the index in `spec/ideas/README.md` (maintained by `spec lint --fix`) cover the same ground at lower cost?

---
*This document follows the https://specscore.md/feature-specification*
