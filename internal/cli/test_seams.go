package cli

import (
	"path/filepath"

	"github.com/specscore/specscore-cli/pkg/decision"
	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/feature"
	"github.com/specscore/specscore-cli/pkg/idea"
	"github.com/specscore/specscore-cli/pkg/idearelocate"
	"github.com/specscore/specscore-cli/pkg/issue"
	"github.com/specscore/specscore-cli/pkg/sourceref"
)

// Test seams — package-level vars wrapping external functions.
// Production code calls these vars; tests replace them via t.Cleanup.
var (
	decisionScaffoldFn   = decision.Scaffold
	decisionNextNumberFn = decision.NextNumber
	ideaScaffoldFn       = idea.Scaffold
	issueScaffoldFn      = issue.Scaffold
	issueParseFn         = issue.Parse
	featureFindRefsFn    = feature.FindFeatureRefs
	filepathRelFn        = filepath.Rel

	idearelocateDiscoverSiblingsFn   = idearelocate.DiscoverSiblings
	idearelocateExecuteCommitPhaseFn = idearelocate.ExecuteCommitPhase
	idearelocatePreflightSubjectsFn  = idearelocate.PreflightSubjectsForRelocate
	idearelocateCheckPreflightFn     = idearelocate.CheckPreflight

	// filepathAbsCLI wraps filepath.Abs for the entity/property verbs'
	// defensive fallbacks. Tests inject failures via cleanup-restored swap.
	filepathAbsCLI = filepath.Abs

	// entityDiscoverCLI wraps entity.Discover for the property-refs verb's
	// defensive error-return path (entity.Discover fails after property
	// discovery succeeds — unreachable through filesystem state alone).
	entityDiscoverCLI = entity.Discover

	// entityResolveInheritsCLI wraps entity.ResolveInherits for the
	// runEntityRefs verb's resolveErr branch (URL paths short-circuit
	// before this; only seam injection triggers the error).
	entityResolveInheritsCLI = entity.ResolveInherits

	// sourcerefScanFilesFn wraps sourceref.ScanFiles so tests can inject
	// errors for the code deps error path.
	sourcerefScanFilesFn = sourceref.ScanFiles
)
