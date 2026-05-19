# Feature: Spec Lint

> [SpecScore.**Studio**](https://specscore.studio): | [Explore](https://specscore.studio/app/p/github.com/specscore/specscore-cli/spec/features/cli/spec/lint?op=explore) | [Edit](https://specscore.studio/app/p/github.com/specscore/specscore-cli/spec/features/cli/spec/lint?op=edit) | [Ask question](https://specscore.studio/app/p/github.com/specscore/specscore-cli/spec/features/cli/spec/lint?op=ask) | [Request change](https://specscore.studio/app/p/github.com/specscore/specscore-cli/spec/features/cli/spec/lint?op=request-change) |
>
> **AI skill:** [GitHub](https://github.com/synchestra-io/ai-plugin-specscore/blob/main/skills/spec/references/lint.md) · [local](../../../../../../ai-plugin-specscore/skills/spec/references/lint.md) — if this command's CLI signature or behavior changes, update the linked skill to keep agents in sync.

**Status:** Stable
**Source Ideas:** index-entries-autofix

## Summary

`specscore spec lint` scans the specification tree and reports violations of structural conventions. Violations are categorized by severity (error, warning, info). `--fix` applies autofixes for rules that support them (adherence footers, view links, idea sync / index / archived-order rules, phantom rows in feature indices, missing rows for orphan child directories).

## Contents

| Directory | Description |
|---|---|
| [plan-rules/](plan-rules/README.md) | Lint rules `P-001`–`P-004` and parser extensions for single-file Plans (`**Mode:**`, `**Status:**`, `**Depends-On:**`, placeholder body token) |

## Synopsis

```
specscore spec lint [--fix] [--severity <error|warning|info>] [--rules <list>] [--ignore <list>] [--format <text|json|yaml>] [--project <path>]
```

## Problem

SpecScore's structural rules are useless if they are not enforced. Humans miss adherence footers, ship OQ sections that say nothing, mis-nest headings, or let indexes drift from reality. A deterministic linter catches every violation mechanically so review effort goes to content, not conventions.

## Behavior

### Rule suite

The default run enables every known rule. The canonical rule list lives in the [lint package](../../../../../pkg/lint) and is not duplicated here.

#### REQ: default-runs-all-rules

Without `--rules` or `--ignore`, `spec lint` MUST execute every registered rule against the project.

#### REQ: rules-whitelist

`--rules a,b,c` MUST restrict execution to the named rules. Any rule name not in the registered set MUST exit `2` (InvalidArgs) with a message naming the unknown rule. The command MUST fail fast — no lint run is performed when any name is invalid.

#### REQ: ignore-blacklist

`--ignore x,y` MUST exclude the named rules from execution. Unknown rule names in `--ignore` follow the same hard-error policy as `--rules`.

#### REQ: mutually-exclusive-filters

`--rules` and `--ignore` MUST NOT be combined. Supplying both MUST exit `2`.

### Features index synchronization

Every directory under `spec/features/` that contains a `README.md` is treated as a feature index for its immediate sub-features. The `index-entries` rule keeps the index in sync with the filesystem in both directions.

#### REQ: index-entries-bidirectional

`index-entries` MUST report a violation when:

- the index contains a Markdown link to a child README (link target ending in `<dirname>/README.md`) but that directory does not exist on disk, OR
- a child directory exists on disk (with its own `README.md`) but is not linked from the parent index.

Both directions apply at every level of the feature tree, including the root `spec/features/README.md`. Hidden directories (starting with `.`) and underscore-prefixed convention directories (e.g. `_args/`) are excluded.

#### REQ: index-entries-fix-deletes-phantom-rows

When `index-entries` reports `Index mentions non-existent directory: <name>` and `spec lint` runs with `--fix`, the fixer MUST remove from the parent README's index table the single row whose link target ends in `<name>/README.md`. Surrounding rows, table delimiters, and the rest of the document MUST be preserved.

#### REQ: index-entries-fix-inserts-orphan-rows

When `index-entries` reports `Child directory not listed in index: <name>` and `spec lint` runs with `--fix`, the fixer MUST append a row that links the missing child. The row shape MUST match what `specscore feature new` already writes via `UpdateFeatureIndex` / `UpdateParentContents`:

- At the root features index (`spec/features/README.md`), a 4-cell row of the form `| \[<name>\]\(<name>/README.md\) | <status> | — | TODO: Add description. |`. `<status>` is parsed from the child's `**Status:**` header via `feature.ParseFeatureStatus`; `Kind` and `Description` use the same hand-maintained placeholder convention `feature new` codifies.
- At a nested feature index, a 2-cell row in the `## Contents` table of the form `| \[<name>\]\(<name>/README.md\) | TODO: Add description. |`. The `## Contents` block is created if absent.

The fixer MUST NOT mutate any cell beyond the inserted row; existing rows are preserved byte-for-byte. The deletion direction (phantom rows) runs first so the insertion phase reads a phantom-free index.

This REQ does NOT violate `fix-is-safe-subset`. Status flows from a structurally-parsed field; Kind and Description use placeholders the project has already codified for `feature new`, so the autofix is byte-identical to user-driven scaffolding. The placeholders are visibly under-filled (`—`, `TODO: ...`), inviting the author to populate them rather than masking missing intent.

### Severity filtering

Each rule has a built-in severity (`error`, `warning`, `info`). `--severity` sets the minimum severity reported.

#### REQ: severity-default

Default `--severity` MUST be `error`. Warnings and info-level findings are suppressed unless the caller explicitly widens the filter.

#### REQ: severity-values

`--severity` MUST accept `error`, `warning`, or `info`. Any other value MUST exit `2`. The ordering is strict: `info ⊂ warning ⊂ error` (wider filter includes narrower).

### Autofix

Rules that support autofix declare so in their registration. `--fix` applies only those fixes; rules without autofix still report violations unchanged.

#### REQ: fix-is-safe-subset

`--fix` MUST only mutate what the rule declares safe to mutate. Mutations that require semantic interpretation of document intent beyond structural conventions MUST NOT be autofixed. Structural rewrites of a recognized trailing adherence-footer block are safe and allowed.

#### REQ: adherence-footer-fix-rewrites-trailing-footer

When the `adherence-footer` rule runs with `--fix`:

- if the required URL is missing and no adherence-footer block exists at end-of-file, the canonical footer block MUST be appended;
- if an adherence-footer block exists at end-of-file but carries the wrong `https://specscore.md/*-specification` URL, the fixer MUST rewrite that existing block to the canonical URL for the document type;
- the fixer MUST leave exactly one adherence-footer block at end-of-file.

#### REQ: fix-is-idempotent

Running `spec lint --fix` twice in a row MUST yield no changes on the second run. The second run exits `0` with no output beyond the standard violation report.

### Exit codes

`spec lint` signals violations through the exit code so CI can gate on it.

#### REQ: exit-1-on-violations

If any violation at or above the effective `--severity` is found, the command MUST exit `1`. Exit `0` is only returned when zero such violations exist.

### Output formats

Default output is text. `--format json` and `--format yaml` produce structured output suitable for tool consumption.

### Repo-config gate

`spec lint` operates on a SpecScore-managed project, which is anchored by a [`specscore.yaml`](../../../repo-config/README.md) file at the project root. Unlike `feature` / `idea` / `task` commands, lint does NOT fall back to a bare `spec/features/` directory — a config file is the single source of truth for project identity, viewer settings, and module layout, all of which the linter relies on.

#### REQ: specscore-yaml-required

`spec lint` MUST refuse to run when no `specscore.yaml` is found in the working directory or any of its ancestors. The command MUST exit `3` (NotFound) with a message that (1) names `specscore.yaml` as mandatory and (2) tells the caller to run `specscore init` to create it. The message MUST NOT mention the legacy `spec/features/` fallback used by other commands.

## Parameters

None. All inputs are flags.

## Exit codes

| Code | Condition |
|---|---|
| `0` | No violations at or above `--severity` |
| `1` | One or more violations at or above `--severity` |
| `2` | Invalid `--rules` / `--ignore` name, invalid `--severity`/`--format` value, conflicting filters |
| `3` | `specscore.yaml` not found in any ancestor directory |
| `10` | Unexpected I/O failure while scanning |

## Interaction with Other Features

| Feature | Interaction |
|---|---|
| [adherence-footer](../../../adherence-footer/README.md) | Defines the footer contract; `spec lint` enforces it. Autofix inserts missing footers and rewrites incorrect trailing footer URLs. |
| [idea](../../../idea/README.md) | Declares idea-specific rules; `spec lint` runs them alongside shared rules. |
| [feature](../../../feature/README.md) | Declares the required-sections rules for feature READMEs. |
| [CLI](../../README.md) | Inherits exit-code contract and project autodetection. `spec lint`'s exit-1-on-violations convention is part of the shared contract. |

## Acceptance Criteria

### AC: clean-tree-exits-0

**Requirements:** cli/spec/lint#req:exit-1-on-violations

On a specification tree with zero error-severity violations, `specscore spec lint` exits `0` with no violation lines printed.

### AC: violations-exit-1

**Requirements:** cli/spec/lint#req:exit-1-on-violations

A tree containing at least one error-severity violation exits `1`. The violation list is printed to stdout (text format) or is the payload of the structured output.

### AC: unknown-rule-name-exits-2

**Requirements:** cli/spec/lint#req:rules-whitelist

`specscore spec lint --rules not-a-rule` exits `2` with a message naming the unknown rule. No lint run is performed.

### AC: fix-idempotent

**Requirements:** cli/spec/lint#req:fix-is-idempotent

Running `spec lint --fix` twice consecutively on the same tree yields no file changes on the second run and exits `0`.

### AC: adherence-footer-fix-replaces-trailing-wrong-url

**Requirements:** cli/spec/lint#req:adherence-footer-fix-rewrites-trailing-footer

If a document ends with an adherence-footer block that uses the wrong `specscore.md/*-specification` URL, running `spec lint --fix` rewrites that block to the canonical URL and does not append a second footer block.

### AC: index-entries-fix-removes-phantom-row

**Requirements:** cli/spec/lint#req:index-entries-fix-deletes-phantom-rows

Given a parent README whose index table contains a Markdown link whose target is `ghost/README.md` while no `ghost/` directory exists on disk, running `specscore spec lint --fix` removes that single row from the index table and leaves every other row intact. A second consecutive `spec lint --fix` produces no further changes (per `fix-is-idempotent`).

### AC: index-entries-fix-inserts-orphan-row

**Requirements:** cli/spec/lint#req:index-entries-fix-inserts-orphan-rows

Given a root features index that lists `auth` while a `billing/` directory with `**Status:** Stable` also exists on disk but is unlinked, running `specscore spec lint --fix` appends the row `| \[billing\]\(billing/README.md\) | Stable | — | TODO: Add description. |` to the index table, preserves the existing `auth` row byte-for-byte, and emits no further changes on a second consecutive pass. The nested case behaves the same way with a 2-cell `| \[<name>\]\(<name>/README.md\) | TODO: Add description. |` row in the `## Contents` table of the parent feature.

### AC: index-entries-flags-orphan-child

**Requirements:** cli/spec/lint#req:index-entries-bidirectional

Given a feature tree where `spec/features/orphan/README.md` exists on disk but `spec/features/README.md` does not link to `orphan/`, running `specscore spec lint` exits `1` with an `index-entries` violation on `features/README.md` whose message names the unlisted child directory.

### AC: missing-specscore-yaml-exits-3

**Requirements:** cli/spec/lint#req:specscore-yaml-required

Running `specscore spec lint` in a directory tree that has no `specscore.yaml` in any ancestor exits `3`. The error message names `specscore.yaml` as mandatory and instructs the caller to run `specscore init` to create it. The presence of a bare `spec/features/` directory does NOT satisfy the gate.
## Outstanding Questions

- Should `spec lint` accept a path argument (`spec lint spec/features/cli/`) to lint a subtree, for faster feedback during development? Today the full tree is always scanned.
- Should `--fix` have a paired `--dry-run` that prints the intended edits without applying them, so authors can preview fixes before accepting?

---
*This document follows the https://specscore.md/feature-specification*
