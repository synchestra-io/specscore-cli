package lint

import (
	"os"

	"github.com/specscore/specscore-cli/pkg/plan"
)

// test_seams_coverage.go provides package-level var seams needed
// to reach 100% statement coverage when tests run as root (where
// os.Chmod is ineffective for restricting access). Production code
// calls these vars; tests swap them via t.Cleanup to inject errors.

var (
	// osReadFileSidekickSeed wraps os.ReadFile in sidekick_seed.go check.
	osReadFileSidekickSeed = os.ReadFile

	// osReadDirSidekickSeed wraps os.ReadDir in sidekick_seed.go check.
	osReadDirSidekickSeed = os.ReadDir

	// osReadDirPlanRules wraps os.ReadDir in plan_rules.go check.
	osReadDirPlanRules = os.ReadDir

	// planParseFn wraps plan.Parse in plan_rules.go check.
	planParseFn = plan.Parse

	// osOpenDogfood wraps os.Open in dogfood_version.go check.
	osOpenDogfood = os.Open

	// osOpenPlanROI wraps os.Open in plan_roi.go scanROIMetadata.
	osOpenPlanROI = os.Open

	// osOpenHasSection wraps os.Open in plan_hierarchy.go hasSection.
	osOpenHasSection = os.Open

	// osReadDirHasChildPlanDirs wraps os.ReadDir in plan_hierarchy.go hasChildPlanDirs.
	osReadDirHasChildPlanDirs = os.ReadDir

	// osWriteFileIssueRules wraps os.WriteFile in issue_rules.go fix.
	osWriteFileIssueRules = os.WriteFile

	// osReadFileIssueI015 wraps os.ReadFile in issue_rules.go lintI015.
	osReadFileIssueI015 = os.ReadFile

	// osWriteFileAdherenceFix wraps os.WriteFile for adherence_footer.go fix().
	osWriteFileAdherenceFix = os.WriteFile

	// osReadFilePlanReadme wraps os.ReadFile in walkPlanReadmes.
	osReadFilePlanReadme = os.ReadFile

	// osReadFileTaskReadme wraps os.ReadFile in walkTaskReadmes.
	osReadFileTaskReadme = os.ReadFile

	// osReadFileScenariosIdx wraps os.ReadFile in walkScenariosIndexes.
	osReadFileScenariosIdx = os.ReadFile

	// osReadFileScenarioFile wraps os.ReadFile in walkScenarioFiles.
	osReadFileScenarioFile = os.ReadFile

	// osReadFileMatchingFiles wraps os.ReadFile in walkMatchingFiles.
	osReadFileMatchingFiles = os.ReadFile

	// osReadFileFeatureReadme wraps os.ReadFile in walkFeatureReadmes.
	osReadFileFeatureReadme = os.ReadFile
)
