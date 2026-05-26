package feature

import (
	"os"
	"path/filepath"
)

// info_stubs.go provides var-based seams for injecting failures in tests.
// Production code calls these vars; tests replace them with closures
// that return canned errors, restoring the originals via t.Cleanup.

var (
	parseFeatureStatusFn    = ParseFeatureStatus
	parseDependenciesFn     = ParseDependencies
	findFeatureRefsFn       = FindFeatureRefs
	discoverChildFeaturesFn = DiscoverChildFeatures
	findLinkedPlansFn       = FindLinkedPlans
	parseSectionsFn         = ParseSections
	filepathAbsFn           = filepath.Abs
	osWriteFile             = os.WriteFile
)
