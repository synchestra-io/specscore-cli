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

As of 2026-05-25, the specscore-cli module sits at ~96% overall test coverage after a concentrated test-writing sprint. The remaining ~4% clusters into three categories of code that resist standard unit testing: (1) OS-level error branches (disk full, permission denied on freshly-created temp files, `crypto/rand.Read` failures) that require fault injection or syscall-level mocking; (2) `os.Exit`-calling entry points (`root.go:Run`, `root.go:Fatal`, `telemetry_wiring.go:executeWithPanicRecovery`) that kill the test process; (3) compile-time-guarded initialization code (`telemetry/errors.go:init`, `telemetry/usage.go:init`) that branches on build-tag constants like `sentryDSN` and `posthogWriteKey`. Additionally, ~15 trivial `severity()` and `name()` one-liner methods in `pkg/lint` are never called directly in tests because the lint framework dispatches them internally — they ARE exercised at runtime but Go's coverage tool doesn't attribute the call.

## Recommended Direction

Adopt a three-pronged approach: (1) **Subprocess testing for os.Exit code.** Use the `exec.Command(os.Args[0], "-test.run=TestXxx")` pattern (standard in the Go stdlib itself — see `log_test.go`) to spawn the test binary as a subprocess, verify it exits with the expected code and stderr output, without killing the parent test process. This covers `Run`, `Fatal`, `executeWithPanicRecovery`. (2) **Interface extraction for I/O-dependent code.** Extract a thin `fileSystem` interface (or use `afero`) for the ~10 functions that do `os.WriteFile`/`os.Chmod`/`os.MkdirAll` and then have error-returning branches. Inject a mock FS in tests that returns errors on demand. This covers `atomicWriteFile`, `writeFileAtomic`, `ApplyMutation` error paths, and the `Discover` unreadable-dir branches. The production code uses the real OS; tests inject a faulty implementation. (3) **Build-tag-aware testing for telemetry init.** Create a `//go:build testcoverage` tagged test file that sets `sentryDSN` and `posthogWriteKey` to test values before `init()` runs, verifying both branches. Or, refactor `init()` into a called function `setupChannel(dsn string)` that's directly testable.

For the lint `severity()` methods: the simplest fix is a single table-driven test in `pkg/lint/severity_test.go` that instantiates each checker struct and asserts its `severity()` return value. This is ~30 lines and immediately covers ~15 functions.

## Alternatives Considered

- **Exclude untestable code via `//go:build !test` or coverage exclusion comments.** Artificially inflates the percentage by removing code from the denominator rather than actually testing it. Creates a false sense of security — the excluded code is exactly the code most likely to harbor bugs (error handling, process lifecycle). Would require Go 1.24+ `//coverage:ignore` directives which are still experimental.
- **100% coverage via integration tests only (no unit test refactoring).** Build the full binary, run it against real filesystems with injected failures (docker `--device`, `LD_PRELOAD`, etc.). Covers everything but is slow (10-60s per scenario), flaky on CI, OS-specific, and hostile to developer iteration. Good as a supplement, not as the primary strategy.
- **Accept ~96% as the practical ceiling and skip the remaining 4%.** The honest option. The uncovered code is defensive error handling that has never fired in production. The ROI of the refactoring needed to test it is debatable. However, this leaves known dead-code risk: if `atomicWriteFile` error handling is wrong, we won't know until data is lost.

## MVP Scope

Deliver a single PR that achieves 99%+ test coverage on `internal/cli` and `internal/telemetry` combined, using only the subprocess-testing pattern (item 1) and the lint severity test (item 3). No interface extraction (item 2) in the MVP — that's a larger refactor. Timeboxed: one working session after approval. Expected LOC: ~200 lines of test code.

## Not Doing (and Why)

- Full `afero`/interface extraction for filesystem mocking — high disruption for marginal gain; deferred to a follow-up if the subprocess approach proves insufficient
- Windows-specific branch coverage for `userStateDir` — requires CI Windows runners we don't have; accepted as permanently uncovered on macOS/Linux
- Covering `cmd/specscore/main.go` — it's a 3-line wrapper calling `cli.Run()`; testing it means testing os.Exit which is covered by the subprocess pattern on `Run` itself

## Key Assumptions to Validate

| Tier | Assumption | How to validate |
|------|------------|-----------------|
| Must-be-true | The subprocess testing pattern (`exec.Command(os.Args[0])`) works with Go's coverage tool and attributes coverage to the parent profile | Write one test for `Fatal`, run with `-coverprofile`, verify the `Fatal` function shows >0% |
| Should-be-true | Refactoring `init()` into a called function doesn't break the telemetry initialization order | Add a test that calls the extracted function directly; verify `RegisterChannel` succeeds |
| Might-be-true | The lint `severity()` methods can be tested without importing all checker dependencies | Instantiate each checker struct directly in a test file within the same package |

## SpecScore Integration

- **New Features this would create:** none (this is a quality/infrastructure improvement)
- **Existing Features affected:** none
- **Dependencies:** none

## Open Questions

- Should we adopt a CI coverage gate (e.g., `go test -coverprofile=... && go tool cover -func=... | grep total | awk '{if ($NF+0 < 96) exit 1}'`) to prevent regressions?
- Is the subprocess testing pattern compatible with `-race` detection? (It should be — the child process runs independently — but worth verifying.)
- Should we measure branch coverage (via `-covermode=atomic`) instead of statement coverage for the final 100% target?

---
*This document follows the https://specscore.md/idea-specification*
