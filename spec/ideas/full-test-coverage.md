# Idea: Full Test Coverage — Path to 100%

**Status:** Draft
**Date:** 2026-05-25
**Owner:** alexander.trakhimenok
**Promotes To:** —
**Supersedes:** —
**Related Ideas:** —

## Problem Statement

How might we achieve and maintain 100% test coverage across the specscore-cli module without resorting to impractical testing patterns?

## Context

As of 2026-05-25, the specscore-cli module reached **96.5%** overall test coverage after a concentrated test-writing sprint that introduced `var`-stub indirections for OS functions (`osCreateTemp`, `ioCopy`, `osChmod`, `osRename`, `randRead`, `runtimeGOOS`), refactored telemetry `init()` functions into callable `var` literals (`setupErrorsChannel`, `setupUsageChannel`), and added an `osExit` stub in `root.go`. This approach — package-level `var` stubs defaulting to the real stdlib function, replaced in tests via `t.Cleanup` — proved highly effective: `internal/telemetry` reached 100%, `pkg/lifecycle` reached 96%, and `internal/cli` reached 95%. The remaining ~3.5% clusters into two categories: (1) `internal/cli` at 95% — the `telemetry_wiring.go` functions that integrate with `fang.Execute` and `cobra.Command` lifecycle (not pure functions, require full CLI execution paths), plus scattered `if err != nil` branches throughout `idea_relocate.go`, `init.go`, and `feature.go` where the error originates from a method call on a concrete type (e.g., `yaml.Encoder.Encode`, `cobra.Command.Help`) that cannot be stubbed without interface extraction; (2) `pkg/idearelocate` at 93.1% — git rollback paths in `ExecutePreCommitPhase` and filesystem error branches in `ApplyMutation`/`discoverSiblings` that require multi-repo git state manipulation or OS-level fault injection on `filepath.Walk`/`os.Lstat` calls.

## Recommended Direction

The `var`-stub and `init()` refactoring approaches have been proven effective (see commits `dfd4254` for `osExit`, `bad217b` for lifecycle/telemetry stubs). To close the remaining ~3.5% to 100%, adopt a two-pronged approach:

**(1) Interface extraction for `internal/cli` method-call error paths.** The remaining uncovered `if err != nil` branches in cli originate from method calls on concrete types — `yaml.Encoder.Encode(out)`, `cmd.Help()`, `lint.Lint(opts)` returning function-level errors (not violations). These cannot be stubbed with `var` indirections because the error source is a method on a struct, not a package-level function. Extract thin interfaces where needed:
```go
type linter interface { Lint(opts lint.Options) ([]lint.Violation, error) }
var lintFn linter = realLinter{}
```
This adds interface overhead but makes every remaining `if err != nil` in the CLI testable. Estimated: ~10 interfaces across `spec.go`, `idea.go`, `feature.go`, `init.go`.

**(2) Multi-repo git fixture for `pkg/idearelocate` rollback paths.** `ExecutePreCommitPhase` (66.7%) requires two git repos with staged changes that partially commit, triggering the rollback logic. Create a test helper that sets up source + dest repos with `git init`, stages changes, and simulates commit failure by making `.git/objects` read-only. This is the most complex fixture in the codebase but covers ~15 statements in a single test.

## Alternatives Considered

- **More `var` stubs without interface extraction.** We already proved this pattern works (telemetry hit 100% with it). But the remaining cli paths involve method calls (`encoder.Encode`, `cmd.Help`) that can't be stubbed via package-level vars — the error source is a method on a struct instance, not a free function. Would require wrapping every encoder/command in a `var` which is worse than the interface approach.
- **`//coverage:ignore` directives (Go 1.24+).** Mark the ~50 remaining statements as intentionally uncovered. Honest about what's not tested, but removes them from the denominator rather than actually testing them. Appropriate for the ~5 genuinely unreachable paths (`filepath.Rel` failure, `init()` panic guard) but not for the 45 that are merely hard to test.
- **Accept 96.5% as the practical ceiling.** The remaining uncovered code is defensive error handling for I/O failures that have never fired in production. The ROI of the interface extraction needed to test them is debatable. However, the `var`-stub approach has already proven that most "untestable" code was actually testable with minimal refactoring — the same may be true for the remaining 3.5%.

## MVP Scope

Deliver a single PR that achieves 98%+ overall test coverage by applying the interface-extraction approach (item 1) to `internal/cli` — specifically `spec.go` (lint call), `idea.go` (lint post-mutation hook), and `feature.go` (yaml/json encoder errors). The `pkg/idearelocate` git-rollback fixture (item 2) is a stretch goal. Timeboxed: one working session after approval. Expected LOC: ~300 lines of production refactoring + ~200 lines of test code.

## Not Doing (and Why)

- Full `afero` filesystem abstraction — the `var`-stub approach proved sufficient for all I/O error paths; `afero` would add a dependency and require rewriting every function signature for marginal gain
- Covering `cmd/specscore/main.go` — it's a 3-line wrapper calling `cli.Run()`; Go's coverage tool doesn't attribute subprocess execution to the parent profile. A `main_test.go` with `go build` + `exec.Command` integration tests exists but doesn't contribute to the coverage number
- Achieving exactly 100.0% — approximately 5 statements are genuinely unreachable: `filepath.Rel` failure between two absolute paths on the same volume, `init()` panic guards that protect against impossible matrix states, and `scanner.Err()` after successful iteration (requires mid-read kernel I/O failure). These should use `//coverage:ignore` when Go supports it

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | The interface-extraction approach for cli encoder/lint calls doesn't break the existing 200+ cli tests | Run `go test ./internal/cli/ -count=1` after each interface introduction; zero regressions |
| Should-be-true | The `pkg/idearelocate` git-rollback fixture is reliable across CI environments (macOS + Linux) | The test uses `git init` + `chmod` in temp dirs; verify on both platforms |
| Might-be-true | The remaining ~5 genuinely unreachable statements can be marked with `//coverage:ignore` in a future Go release | Track golang/go#51430 for coverage exclusion directive support |

## SpecScore Integration

- **New Features this would create:** none (this is a quality/infrastructure improvement)
- **Existing Features affected:** none
- **Dependencies:** none

## Open Questions

- Should we adopt a CI coverage gate at 96% (current proven level) to prevent regressions? The gate command would be: `go test ./... -coverprofile=cover.out && go tool cover -func=cover.out | grep total | awk '{if ($NF+0 < 96) exit 1}'`
- Is the `var`-stub pattern safe under `-race`? (Yes — each test restores via `t.Cleanup` and tests run sequentially within a package by default. Parallel sub-tests touching the same `var` would need `sync.Mutex`.)
- Should the ~5 genuinely unreachable statements be deleted rather than marked? E.g., the `init()` panic guard exists for defense-in-depth but will never fire unless someone introduces a self-loop in the transition matrix — which `validateMatrix` already prevents.

---
*This document follows the https://specscore.md/idea-specification*
