package cli

import (
	"path/filepath"

	"github.com/specscore/specscore-cli/pkg/feature"
	"github.com/specscore/specscore-cli/pkg/idea"
	"github.com/specscore/specscore-cli/pkg/idearelocate"
	"github.com/specscore/specscore-cli/pkg/issue"
)

// Test seams — package-level vars wrapping external functions.
// Production code calls these vars; tests replace them via t.Cleanup.
var (
	ideaScaffoldFn   = idea.Scaffold
	issueScaffoldFn  = issue.Scaffold
	issueParseFn     = issue.Parse
	featureFindRefsFn = feature.FindFeatureRefs
	filepathRelFn    = filepath.Rel

	idearelocateDiscoverSiblingsFn   = idearelocate.DiscoverSiblings
	idearelocateExecuteCommitPhaseFn = idearelocate.ExecuteCommitPhase
	idearelocatePreflightSubjectsFn  = idearelocate.PreflightSubjectsForRelocate
	idearelocateCheckPreflightFn     = idearelocate.CheckPreflight
)
