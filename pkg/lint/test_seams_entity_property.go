package lint

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/property"
)

// test_seams_entity_property.go provides package-level var seams for the
// entity-* and property-* checkers. Production code calls these vars;
// tests replace them via t.Cleanup to drive defensive error branches
// (e.g., os.WriteFile failure during the autofix phase) that cannot be
// reliably triggered through filesystem state alone.
//
// Each seam is documented at its declaration with the production call
// site(s) that consume it.
var (
	// osWriteFileEntity is invoked from (*entityChecker).check() phase-2
	// when persisting the autofix rewrites.
	osWriteFileEntity = os.WriteFile

	// osWriteFileProperty is invoked from rewritePropertyFile during the
	// property autofix flow.
	osWriteFileProperty = os.WriteFile

	// osReadFileEntity is invoked from applyManagedRewrites,
	// applyIDEqualsSlugFix, and the title-rewrite stage.
	osReadFileEntity = os.ReadFile

	// osReadFileProperty is invoked from rewritePropertyFile.
	osReadFileProperty = os.ReadFile

	// yamlMarshalEntity wraps yaml.Marshal for applyIDEqualsSlugFix.
	yamlMarshalEntity = yaml.Marshal

	// yamlMarshalProperty wraps yaml.Marshal for rewritePropertyFrontmatterID.
	yamlMarshalProperty = yaml.Marshal

	// filepathAbsLint wraps filepath.Abs for paths that lint code must
	// canonicalize. Production code uses it where a fallback would
	// silently mask the canonicalization error.
	filepathAbsLint = filepath.Abs

	// filepathRelLint wraps filepath.Rel for paths that lint code must
	// relativize and falls back to the absolute when the relativization
	// fails (paths share no common ancestor). Tests inject failures.
	filepathRelLint = filepath.Rel

	// entityDiscoverFn wraps entity.Discover for tests that need to drive
	// the early-return branches in the entity checker.
	entityDiscoverFn = entity.Discover

	// propertyDiscoverFn wraps property.Discover for the same purpose.
	propertyDiscoverFn = property.Discover

	// findEntityDirectoriesFn wraps the local findEntityDirectories helper.
	findEntityDirectoriesFn = findEntityDirectories

	// findMisplacedPropertyFilesFn wraps findMisplacedPropertyFiles.
	findMisplacedPropertyFilesFn = findMisplacedPropertyFiles

	// findPropertyDirectoriesFn wraps findPropertyDirectories.
	findPropertyDirectoriesFn = findPropertyDirectories

	// runPropertyFixFn wraps runPropertyFix to test the autofix-error
	// early return in checkProperties.
	runPropertyFixFn = runPropertyFix

	// yamlUnmarshalProperty wraps yaml.Unmarshal for the property's
	// id-rewriter. Tests swap it to produce a DocumentNode with empty
	// Content — a state real yaml input never produces.
	yamlUnmarshalProperty = yaml.Unmarshal

	// rewriteEntityTitleFn wraps rewriteEntityTitle for the autofix
	// title loop's no-change defensive branch (line 238-239 in
	// entity.go's check function). The real rewriter always reports
	// change=true when the outer guard signalled change-needed; the
	// seam exists to drive the otherwise-unreachable defensive guard.
	rewriteEntityTitleFn = rewriteEntityTitle
)
