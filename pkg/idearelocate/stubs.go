package idearelocate

// stubs.go provides var-based seams for idearelocate error-path testing.

// updateCrossRepoLinksFn is the function used by ExecutePreCommitPhase
// to call UpdateCrossRepoLinks. Tests can swap it to inject failures.
var updateCrossRepoLinksFn = UpdateCrossRepoLinks
