package idearelocate

import (
	"os"
	"path/filepath"
)

// stubs.go provides var-based seams for idearelocate error-path testing.
// Production code calls these vars; tests replace them via t.Cleanup.

var (
	// updateCrossRepoLinksFn is the function used by ExecutePreCommitPhase
	// to call UpdateCrossRepoLinks. Tests can swap it to inject failures.
	updateCrossRepoLinksFn = UpdateCrossRepoLinks

	// filepathRelFn wraps filepath.Rel. Swapped in tests to inject errors.
	filepathRelFn = filepath.Rel

	// filepathAbsFn wraps filepath.Abs. Swapped in tests to inject errors.
	filepathAbsFn = filepath.Abs

	// filepathWalkFn wraps filepath.Walk. Swapped in tests to inject errors.
	filepathWalkFn = filepath.Walk

	// osLstatFn wraps os.Lstat. Swapped in tests to inject errors.
	osLstatFn = os.Lstat

	// isPathCleanFn wraps IsPathClean for CheckPreflight error propagation tests.
	isPathCleanFn = IsPathClean

	// findReferencesFn wraps FindReferences for PreflightSubjectsForRelocate
	// error propagation tests.
	findReferencesFn = FindReferences

	// gitRevParseHEADFn retrieves HEAD SHA after a commit. Swapped in tests
	// to simulate post-commit rev-parse failures.
	gitRevParseHEADFn = defaultGitRevParseHEAD

	// isGitRepoFn reports whether repoRoot is inside a git work tree.
	// Swapped in tests to control git-repo detection.
	isGitRepoFn = defaultIsGitRepo
)
