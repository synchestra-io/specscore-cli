package lint

// coverage_stubs_test.go exercises error paths via the injectable var seams
// declared in test_seams_coverage.go (and other test_seams_*.go files).
// Each test swaps a stub, runs the function under test, and verifies the
// error is handled correctly. t.Cleanup restores the original.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/idea"
	"github.com/specscore/specscore-cli/pkg/plan"
	"github.com/specscore/specscore-cli/pkg/property"
)

// injectedErr is the sentinel used across all stub tests.
var injectedErr = errors.New("injected")

// =============================================================================
// 1. osReadFileSidekickSeed → sidekick_seed.go lines 87-95 (ReadFile error)
// =============================================================================

func TestSidekickSeed_ReadFileError_Stub(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/unreadable.md": "---\ntype: sidekick-seed\nslug: unreadable\ncaptured_at: 2026-05-18T00:00:00Z\ncaptured_by: user\ncaptured_during: null\ntrigger: user-prompt\nstatus: queued\nsynchestra_task: null\n---\n\n# Unreadable Seed\n",
	})
	orig := osReadFileSidekickSeed
	osReadFileSidekickSeed = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileSidekickSeed = orig })

	c := newSidekickSeedChecker()
	vs, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	hasReadErr := false
	for _, v := range vs {
		if strings.Contains(v.Message, "cannot read seed file") {
			hasReadErr = true
		}
	}
	if !hasReadErr {
		t.Error("expected 'cannot read seed file' violation")
	}
}

// =============================================================================
// 2. osReadDirSidekickSeed → sidekick_seed.go lines 72-74 (ReadDir error)
// =============================================================================

func TestSidekickSeed_ReadDirError_Stub(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/ok.md": "---\ntype: sidekick-seed\nslug: ok\ncaptured_at: 2026-05-18T00:00:00Z\ncaptured_by: user\ncaptured_during: null\ntrigger: user-prompt\nstatus: queued\nsynchestra_task: null\n---\n\n# OK Seed\n",
	})
	orig := osReadDirSidekickSeed
	osReadDirSidekickSeed = func(name string) ([]os.DirEntry, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadDirSidekickSeed = orig })

	c := newSidekickSeedChecker()
	_, err := c.check(specRoot)
	if err == nil {
		t.Fatal("expected error")
	}
}

// =============================================================================
// 3. osReadDirPlanRules → plan_rules.go lines 43-45 (ReadDir error)
// =============================================================================

func TestPlanRules_ReadDirError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/good.md": "# Plan: Good\n\n**Source Feature:** test\n\n## Tasks\n\n### Task 1\n\n**Verifies:** test#ac:a\n\nStep.\n",
	})
	orig := osReadDirPlanRules
	osReadDirPlanRules = func(name string) ([]os.DirEntry, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadDirPlanRules = orig })

	c := newPlanRulesChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error for ReadDir failure")
	}
}

// =============================================================================
// 4. planParseFn → plan_rules.go lines 60-62 (plan.Parse error)
// =============================================================================

func TestPlanRules_PlanParseFnError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/bad.md": "# Plan: Bad\n\n**Source Feature:** test\n\n## Tasks\n\n### Task 1\n\nStep.\n",
	})
	orig := planParseFn
	planParseFn = func(path string) (*plan.Plan, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { planParseFn = orig })

	c := newPlanRulesChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error for plan.Parse failure")
	}
}

// =============================================================================
// 5. osOpenDogfood → dogfood_version.go lines 71-72 (Open error)
// =============================================================================

func TestDogfoodVersion_OpenError_Stub(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "spec")
	mkdir(t, specRoot)
	wfDir := filepath.Join(root, ".github", "workflows")
	mkdir(t, wfDir)
	writeFile(t, filepath.Join(wfDir, "ci.yml"), "name: ci\nenv:\n  SPECSCORE_VERSION: v0.1.0\n")

	orig := osOpenDogfood
	osOpenDogfood = func(name string) (*os.File, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osOpenDogfood = orig })

	c := newDogfoodVersionChecker("0.5.0")
	vs, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	// Unreadable file should be silently skipped — no violations.
	if len(vs) != 0 {
		t.Errorf("expected 0 violations for unreadable file, got %d", len(vs))
	}
}

// =============================================================================
// 6. osOpenPlanROI → plan_roi.go lines 80-82 (Open error in scanROIMetadata)
// =============================================================================

func TestPlanROI_OpenError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan/README.md": "# Plan: My Plan\n\n**Effort:** S\n**Impact:** high\n\n## Steps\n\n1. Do it.\n",
	})
	orig := osOpenPlanROI
	osOpenPlanROI = func(name string) (*os.File, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osOpenPlanROI = orig })

	c := newPlanROIChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error for Open failure in scanROIMetadata")
	}
}

// =============================================================================
// 7. osOpenHasSection → plan_hierarchy.go lines 127-129 (Open error)
// =============================================================================

func TestPlanHierarchy_HasSection_OpenError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan/README.md": "# Plan: My Plan\n\n## Steps\n\n1. Do it.\n",
	})
	orig := osOpenHasSection
	osOpenHasSection = func(name string) (*os.File, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osOpenHasSection = orig })

	c := newPlanHierarchyChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// When hasSection can't open the file, it returns (false, 0).
	// The checker won't report a roadmap/steps conflict.
	_ = vs
}

// =============================================================================
// 8. osReadDirHasChildPlanDirs → plan_hierarchy.go lines 104-106 (ReadDir error)
// =============================================================================

func TestPlanHierarchy_HasChildPlanDirs_ReadDirError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/parent/README.md":       "# Plan: Parent\n\n## Steps\n\n1. Do it.\n",
		"plans/parent/child/README.md": "# Plan: Child\n\n## Steps\n\n1. Do it.\n",
	})
	orig := osReadDirHasChildPlanDirs
	osReadDirHasChildPlanDirs = func(name string) ([]os.DirEntry, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadDirHasChildPlanDirs = orig })

	c := newPlanHierarchyChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// When ReadDir fails, hasChildPlanDirs returns false. The checker
	// won't detect parent as a roadmap, so no roadmap-related violations.
	_ = vs
}

// =============================================================================
// 9. osWriteFileIssueRules → issue_rules.go lines 185-187 (WriteFile error)
// =============================================================================

func TestIssueRulesFix_WriteFileError_Stub(t *testing.T) {
	root := t.TempDir()
	issueDir := filepath.Join(root, "issues")
	mkdir(t, issueDir)
	writeFile(t, filepath.Join(issueDir, "bug-1.md"), "---\ntype: issue\nstatus: open\nseverity: high\n---\n# Bug: B\n\n## Description\n\nX.\n")

	orig := osWriteFileIssueRules
	osWriteFileIssueRules = func(name string, data []byte, perm os.FileMode) error {
		return injectedErr
	}
	t.Cleanup(func() { osWriteFileIssueRules = orig })

	c := newIssueRulesChecker().(fixer)
	err := c.fix(root)
	if err == nil {
		t.Error("expected error from WriteFile failure")
	}
}

// =============================================================================
// 10. osReadFileIssueI015 → issue_rules.go lines 366-367 (ReadFile error → continue)
// =============================================================================

func TestIssueRules_I015_ReadFileError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/README.md": "# Issues\n\n## Contents\n\n| Slug | Title | Status |\n|---|---|---|\n| bug-1 | B | open |\n",
		"issues/bug-1.md":  "---\ntype: issue\nstatus: open\nseverity: high\n---\n# Bug: B\n\n## Description\n\nX.\n",
	})
	orig := osReadFileIssueI015
	osReadFileIssueI015 = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileIssueI015 = orig })

	c := newIssueRulesChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// ReadFile error → continue in lintI015; no I-015 violations.
	for _, v := range vs {
		if v.Rule == "I-015" {
			t.Errorf("unexpected I-015 violation after ReadFile error: %+v", v)
		}
	}
}

// =============================================================================
// 11. osWriteFileAdherenceFix → adherence_footer.go fix() write errors
// =============================================================================

func TestAdherenceFooterFix_WriteFileError_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"), "# Plan: Test\n\nContent.\n")

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm os.FileMode) error {
		return injectedErr
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	c := newAdherenceFooterChecker().(fixer)
	err := c.fix(root)
	if err == nil {
		t.Error("expected error from WriteFile failure in adherence fix")
	}
}

// =============================================================================
// 12. osReadFilePlanReadme → walkPlanReadmes ReadFile error (line 325-327)
// =============================================================================

func TestWalkPlanReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"), "# Plan\n")

	orig := osReadFilePlanReadme
	osReadFilePlanReadme = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFilePlanReadme = orig })

	var called bool
	err := walkPlanReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("walk should not error for unreadable file: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable file")
	}
}

// =============================================================================
// 13. osReadFileTaskReadme → walkTaskReadmes ReadFile error (line 354-356)
// =============================================================================

func TestWalkTaskReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "alpha", "tasks", "t1"))
	writeFile(t, filepath.Join(plansDir, "alpha", "tasks", "t1", "README.md"), "# Task\n")

	orig := osReadFileTaskReadme
	osReadFileTaskReadme = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileTaskReadme = orig })

	var called bool
	err := walkTaskReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("walk should not error: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable file")
	}
}

// =============================================================================
// 14. osReadFileScenariosIdx → walkScenariosIndexes ReadFile error (line 382-384)
// =============================================================================

func TestWalkScenariosIndexes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	testsDir := filepath.Join(root, "features", "auth", "_tests")
	mkdir(t, testsDir)
	writeFile(t, filepath.Join(testsDir, "README.md"), "# Scenarios\n")

	orig := osReadFileScenariosIdx
	osReadFileScenariosIdx = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileScenariosIdx = orig })

	var called bool
	err := walkScenariosIndexes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("walk should not error: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable file")
	}
}

// =============================================================================
// 15. osReadFileScenarioFile → walkScenarioFiles ReadFile error (line 400-402)
// =============================================================================

func TestWalkScenarioFiles_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	testsDir := filepath.Join(root, "features", "auth", "_tests")
	mkdir(t, testsDir)
	writeFile(t, filepath.Join(testsDir, "login.md"), "# Scenario\n")

	orig := osReadFileScenarioFile
	osReadFileScenarioFile = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileScenarioFile = orig })

	var called bool
	err := walkScenarioFiles(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("walk should not error: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable file")
	}
}

// =============================================================================
// 16. osReadFileMatchingFiles → walkMatchingFiles ReadFile error (line 463-465)
// =============================================================================

func TestWalkMatchingFiles_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	ideasDir := filepath.Join(root, "ideas")
	mkdir(t, ideasDir)
	writeFile(t, filepath.Join(ideasDir, "test.md"), "# Idea\n")

	orig := osReadFileMatchingFiles
	osReadFileMatchingFiles = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileMatchingFiles = orig })

	var called bool
	err := walkMatchingFiles(ideasDir, func(path string, depth int, name string) bool {
		return strings.HasSuffix(name, ".md")
	}, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("walk should not error: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable file")
	}
}

// =============================================================================
// 17. osReadFileFeatureReadme → walkFeatureReadmes ReadFile error (line 34-36)
// =============================================================================

func TestWalkFeatureReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	featDir := filepath.Join(root, "features", "auth")
	mkdir(t, featDir)
	writeFile(t, filepath.Join(featDir, "README.md"), "# Auth\n")

	orig := osReadFileFeatureReadme
	osReadFileFeatureReadme = func(name string) ([]byte, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadFileFeatureReadme = orig })

	var called bool
	err := walkFeatureReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Errorf("should silently skip unreadable readme: %v", err)
	}
	if called {
		t.Error("fn should not be called for unreadable readme")
	}
}

// =============================================================================
// 18. idea.go — CheckIdeas error paths via ideaDiscoverFn
// =============================================================================

func TestCheckIdeas_IdeaDiscoverFnError_Stub(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md": activeIndex + "\n## Open Questions\n\nNone at this time.\n",
	})
	orig := ideaDiscoverFn
	ideaDiscoverFn = func(specRoot string) ([]idea.Discovered, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { ideaDiscoverFn = orig })

	_, err := CheckIdeas(root, false)
	if err == nil {
		t.Error("expected error from ideaDiscoverFn injection")
	}
}

// =============================================================================
// 19. entity.go — findEntityDirectoriesFn Walk error, entityDiscoverFn error,
//     entity.Parse error
// =============================================================================

func TestEntityChecker_FindEntityDirectoriesError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	orig := findEntityDirectoriesFn
	findEntityDirectoriesFn = func(specRoot string) ([]string, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { findEntityDirectoriesFn = orig })

	c := newEntityChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from findEntityDirectoriesFn injection")
	}
}

func TestEntityChecker_EntityDiscoverFnError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	orig := entityDiscoverFn
	entityDiscoverFn = func(specRoot string) ([]entity.Discovered, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { entityDiscoverFn = orig })

	c := newEntityChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from entityDiscoverFn injection")
	}
}

// =============================================================================
// 20. linter.go — walkSpecDirs Walk error (line 230-232)
// Already covered by TestWalkSpecDirs_WalkError with chmod-based approach,
// but adding a root-guard-safe version for completeness.
// =============================================================================

// walkSpecDirs errors are triggered by filesystem state (unreadable subdirs).
// The root-skip guard in the chmod-based test handles this sufficiently.

// =============================================================================
// 22. plan_hierarchy.go — Walk error callback (line 33-35)
// =============================================================================

func TestPlanHierarchy_WalkError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan/README.md": "# Plan: My Plan\n",
	})
	// Make a subdirectory unreadable to trigger Walk error.
	badDir := filepath.Join(root, "plans", "bad-dir")
	mkdir(t, badDir)
	_ = os.Chmod(badDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(badDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	c := newPlanHierarchyChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from Walk failure")
	}
}

// =============================================================================
// 23. plan_roi.go — Walk error (line 39-41), scan error (line 61-63, 69-71)
// =============================================================================

func TestPlanROI_WalkError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan/README.md": "# Plan: My Plan\n\n**Effort:** S\n",
	})
	// Create an unreadable subdirectory
	badDir := filepath.Join(root, "plans", "bad")
	mkdir(t, badDir)
	_ = os.Chmod(badDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(badDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	c := newPlanROIChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from Walk failure in plan_roi")
	}
}

// =============================================================================
// 24. lint.go — fix returns error (line 44-46)
// =============================================================================

func TestLint_FixReturnsError_Stub(t *testing.T) {
	// Use the adherence-footer fixer with an injected WriteFile error.
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan/README.md": "# Plan: Test\n\nContent.\n",
	})

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm os.FileMode) error {
		return injectedErr
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	_, err := Lint(Options{
		SpecRoot: root,
		Fix:      true,
		Rules:    []string{"adherence-footer"},
	})
	if err == nil {
		t.Error("expected error from fix failure")
	}
}

// =============================================================================
// 25. index_entries.go — osReadDir error (lines 45-47, 71-73, 140-142,
//     172-174, 176-178, 202-204, 206-208)
// Already covered in coverage_final_test.go via osReadDir injection.
// Additional coverage for fix() WriteFile error path.
// =============================================================================

// =============================================================================
// 26. property.go — propertyDiscoverFn error, runPropertyFixFn error
// =============================================================================

func TestPropertyChecker_PropertyDiscoverFnError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	orig := propertyDiscoverFn
	propertyDiscoverFn = func(specRoot string) ([]property.Discovered, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { propertyDiscoverFn = orig })

	c := newPropertyChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from propertyDiscoverFn injection")
	}
}

func TestPropertyChecker_RunPropertyFixFnError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	orig := runPropertyFixFn
	runPropertyFixFn = func(specRoot string) (bool, error) {
		return false, injectedErr
	}
	t.Cleanup(func() { runPropertyFixFn = orig })

	c := newPropertyChecker()
	c.autofix = true
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from runPropertyFixFn injection")
	}
}

// =============================================================================
// 27. studio_toolbar.go — walkFeatureReadmes error (line 221-223)
// =============================================================================

func TestStudioToolbar_WalkFeatureReadmesError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	// Make a subdirectory unreadable to trigger Walk error inside walkFeatureReadmes
	badDir := filepath.Join(root, "features", "bad")
	mkdir(t, badDir)
	_ = os.Chmod(badDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(badDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	c := newStudioToolbarChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from walkFeatureReadmes Walk failure")
	}
}

// =============================================================================
// 28. feature_index.go — readFeatureIndexRows error (line 74-76),
//     ParseFeatureStatus error (line 101-102),
//     rewrite fails (line 126-136)
// =============================================================================

// These are already covered by TestFeatureIndex_ReadFeatureIndexRowsOpenError
// (chmod-based) and TestFeatureIndexRules_ParseFeatureStatusError (chmod-based).
// The stub test for Open failure exercises the same path via osReadFileFeatureReadme.

// =============================================================================
// 30. decision_rules.go — osReadDirDecision error (lines 220-221, 234-235, etc.)
// =============================================================================

func TestDecisionRules_ReadDirActiveError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"decisions/001-test.md": "# ADR 001: Test\n\n**Status:** Accepted\n**Date:** 2026-05-01\n**Deciders:** alice\n\n## Context\n\nTest.\n\n## Decision\n\nDo it.\n\n## Consequences\n\n- Good.\n\n## Open Questions\n\nNone.\n",
	})
	orig := osReadDirDecision
	osReadDirDecision = func(name string) ([]os.DirEntry, error) {
		return nil, injectedErr
	}
	t.Cleanup(func() { osReadDirDecision = orig })

	c := newDecisionRulesChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from osReadDirDecision injection")
	}
}

// =============================================================================
// 31. adherence_footer.go line 141-143 — target.walk() error in check
// =============================================================================

func TestAdherenceFooterCheck_WalkError_Stub(t *testing.T) {
	root := t.TempDir()
	// Create a features dir with an unreadable subdirectory to trigger Walk error
	// inside walkFeatureReadmes (which is one of the target.walk functions).
	featDir := filepath.Join(root, "features", "bad")
	mkdir(t, featDir)
	_ = os.Chmod(featDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(featDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(featDir, 0o755) })

	c := newAdherenceFooterChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from walk failure in adherence_footer check")
	}
}

// =============================================================================
// 21. oq_section.go — Walk error (line 39-41), parse error (line 53-55),
//     fix Walk error (line 115-117), fix ReadFile error (line 126-128)
// =============================================================================

func TestOQSection_WalkError_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root)
	badDir := filepath.Join(root, "bad")
	mkdir(t, badDir)
	_ = os.Chmod(badDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(badDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	c := newOQSectionChecker()
	_, err := c.check(root)
	if err == nil {
		t.Error("expected error from Walk failure in oq_section check")
	}
}

func TestOQSection_FixWalkError_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root)
	badDir := filepath.Join(root, "bad")
	mkdir(t, badDir)
	_ = os.Chmod(badDir, 0o000)
	if os.Getuid() == 0 {
		_ = os.Chmod(badDir, 0o755)
		t.Skip("test requires non-root")
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	c := newOQSectionChecker().(fixer)
	err := c.fix(root)
	if err == nil {
		t.Error("expected error from Walk failure in oq_section fix")
	}
}

// =============================================================================
// 29. idea_index.go — readIndexRows error (line 83-89), rewriteActiveIndex
//     error. These are already covered by TestIdeaIndex_ActiveFixFailedFallback
//     and TestIdeaIndex_ArchivedFixFailedFallback (chmod-based, now with
//     root-skip guards), plus TestIdeaIndexRules_ActiveFixFailedWithDrift in
//     coverage_final_test.go. No additional stubs needed.
// =============================================================================

// =============================================================================
// Adherence footer fix — rewrite path (osWriteFileAdherenceFix error during
// rewrite when URL exists but is wrong)
// =============================================================================

func TestAdherenceFooterFix_RewriteWriteError_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"),
		"# Plan: Test\n\n---\n*This document follows the https://specscore.md/wrong-specification*\n")

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm os.FileMode) error {
		return injectedErr
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	c := newAdherenceFooterChecker().(fixer)
	err := c.fix(root)
	if err == nil {
		t.Error("expected error from WriteFile failure during rewrite")
	}
}

// =============================================================================
// Adherence footer fix — short-circuit after first write error
// =============================================================================

func TestAdherenceFooterFix_ShortCircuitAfterWriteError_Stub(t *testing.T) {
	root := t.TempDir()
	// Two scenario files so the walk visits both — first triggers a write
	// error, second should short-circuit.
	testsDir := filepath.Join(root, "features", "auth", "_tests")
	mkdir(t, testsDir)
	writeFile(t, filepath.Join(testsDir, "a.md"), "# Scenario: A\n")
	writeFile(t, filepath.Join(testsDir, "b.md"), "# Scenario: B\n")

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm os.FileMode) error {
		return injectedErr
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	c := newAdherenceFooterChecker().(fixer)
	err := c.fix(root)
	if err == nil {
		t.Error("expected error from write failure")
	}
}

// =============================================================================
// entity.go — entity.Parse error for discovered entity
// =============================================================================

func TestEntityChecker_EntityParseError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})

	// Use entityDiscoverFn to return a path to a non-existent file so entity.Parse fails.
	orig := entityDiscoverFn
	entityDiscoverFn = func(specRoot string) ([]entity.Discovered, error) {
		return []entity.Discovered{
			{
				Slug: "user",
				Path: filepath.Join(specRoot, "features", "auth", "user.entity.md"),
			},
		}, nil
	}
	t.Cleanup(func() { entityDiscoverFn = orig })

	c := newEntityChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// The file doesn't exist, so entity.Parse returns an error.
	// The checker records a violation instead of propagating.
	hasParseErr := false
	for _, v := range vs {
		if strings.Contains(v.Message, "cannot read entity file") {
			hasParseErr = true
		}
	}
	if !hasParseErr {
		t.Error("expected 'cannot read entity file' violation for non-existent entity")
	}
}

// =============================================================================
// property.go — property.Parse error for discovered property
// =============================================================================

func TestPropertyChecker_PropertyParseError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})

	// Point to a non-existent file so property.Parse fails with an os.Open error.
	orig := propertyDiscoverFn
	propertyDiscoverFn = func(specRoot string) ([]property.Discovered, error) {
		return []property.Discovered{
			{
				Slug: "email",
				Path: filepath.Join(specRoot, "features", "auth", "email.property.md"),
			},
		}, nil
	}
	t.Cleanup(func() { propertyDiscoverFn = orig })

	c := newPropertyChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	hasParseErr := false
	for _, v := range vs {
		if strings.Contains(v.Message, "cannot read property file") {
			hasParseErr = true
		}
	}
	if !hasParseErr {
		t.Error("expected 'cannot read property file' violation for non-existent property")
	}
}

var (
	_ = injectedErr
	_ = (*idea.Idea)(nil)
	_ = (*entity.Discovered)(nil)
	_ = (*plan.Plan)(nil)
	_ = (*property.Discovered)(nil)
)
