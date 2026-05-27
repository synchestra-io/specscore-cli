package lint

import (
	"os"
	"path/filepath"
)

// test_seams_decision.go provides package-level var seams for the
// decision-* and DI-* checkers. Production code calls these vars;
// tests replace them via t.Cleanup to drive defensive error branches
// (e.g., os.ReadFile / os.WriteFile failures during the index rewrite)
// that cannot be reliably triggered through filesystem state alone.
//
// Each seam is documented at its declaration with the production call
// site(s) that consume it.
var (
	// osReadFileDecisionIndex wraps os.ReadFile for the decisions-index
	// reader path. Production call sites:
	//   - checkActiveDecisionsIndex (decisions_index_rules.go:102)
	//   - checkArchivedDecisionsIndex (decisions_index_rules.go:326)
	//   - rewriteArchivedDecisionsIndex (decisions_index_rules.go:452)
	// Tests swap this to force the os.ReadFile error branch — otherwise
	// unreachable since the caller (checkDecisionsIndex) has already
	// stat-confirmed the file exists.
	osReadFileDecisionIndex = os.ReadFile

	// osWriteFileDecisionIndex wraps os.WriteFile for the decisions-index
	// rewriter paths. Production call sites:
	//   - rewriteDecisionsIndexTable (decisions_index_rules.go:448)
	//   - rewriteArchivedDecisionsIndex (decisions_index_rules.go:486)
	// Tests swap this to force the write-failed branch which surfaces
	// "(fix failed: %v)" violations.
	osWriteFileDecisionIndex = os.WriteFile

	// osReadDirDecision wraps os.ReadDir for the decision discovery and
	// directory checkers. Production call sites:
	//   - discoverDecisionFiles active branch (decision_rules.go:208)
	//   - discoverDecisionFiles archived branch (decision_rules.go:231)
	//   - checkDecisionDirectories active branch (decision_rules.go:302)
	//   - walkDecisionFiles active branch (decision_rules.go:729)
	//   - walkDecisionFiles archived branch (decision_rules.go:748)
	// Tests swap this to force the ReadDir error branches that filesystem
	// state alone cannot reliably reproduce on every CI host (chmod 000
	// is a no-op when tests run as root).
	osReadDirDecision = os.ReadDir

	// osReadFileDecision wraps os.ReadFile for the decision walker.
	// Production call sites:
	//   - walkDecisionFiles active branch (decision_rules.go:738)
	//   - walkDecisionFiles archived branch (decision_rules.go:757)
	// Tests swap this to force individual file-read failures (the
	// production code swallows these errors and continues to the next
	// entry — covering that branch is the goal).
	osReadFileDecision = os.ReadFile

	// filepathRelDecisionImmutability wraps filepath.Rel for the
	// immutability checker's repo-root-relative path computation.
	// Production call site: checkDecisionImmutability (decision_immutability.go:70).
	// Tests swap this to drive the rel-failed `continue` branch — real
	// filepath.Rel only errors on cross-volume / relative-vs-absolute
	// inputs that production code never produces.
	filepathRelDecisionImmutability = filepath.Rel
)
