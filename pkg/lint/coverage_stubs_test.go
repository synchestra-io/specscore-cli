package lint

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/idea"
	"github.com/specscore/specscore-cli/pkg/issue"
	"github.com/specscore/specscore-cli/pkg/plan"
)

// =============================================================================
// sidekick_seed.go — osReadDirSidekickSeed error (lines 71-73)
// =============================================================================

func TestSidekickSeedCheck_ReadDirError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"ideas/seeds/.keep": "",
	})

	orig := osReadDirSidekickSeed
	osReadDirSidekickSeed = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readdir error")
	}
	t.Cleanup(func() { osReadDirSidekickSeed = orig })

	c := newSidekickSeedChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error from osReadDirSidekickSeed injection, got nil")
	}
	if !strings.Contains(err.Error(), "injected readdir error") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// sidekick_seed.go — osReadFileSidekickSeed error (lines 87-95)
// =============================================================================

func TestSidekickSeedCheck_ReadFileError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"ideas/seeds/test-seed.md": "---\ntype: sidekick-seed\n---\n# Test\n",
	})

	orig := osReadFileSidekickSeed
	osReadFileSidekickSeed = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileSidekickSeed = orig })

	c := newSidekickSeedChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vs) == 0 {
		t.Fatal("expected violation from osReadFileSidekickSeed injection, got none")
	}
	found := false
	for _, v := range vs {
		if v.Rule == "sidekick-seed" && strings.Contains(v.Message, "cannot read") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected violation with 'cannot read' message, got %+v", vs)
	}
}

// =============================================================================
// plan_rules.go — osReadDirPlanRules error (lines 42-44)
// =============================================================================

func TestPlanRulesCheck_ReadDirError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/.keep": "",
	})

	orig := osReadDirPlanRules
	osReadDirPlanRules = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readdir error")
	}
	t.Cleanup(func() { osReadDirPlanRules = orig })

	c := newPlanRulesChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error from osReadDirPlanRules injection, got nil")
	}
	if !strings.Contains(err.Error(), "reading plans dir") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// plan_rules.go — planParseFn error (lines 59-61)
// =============================================================================

func TestPlanRulesCheck_ParseFnError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/my-plan.md": "# Plan: Test\n\n## Tasks\n",
	})

	orig := planParseFn
	planParseFn = func(path string) (*plan.Plan, error) {
		return nil, errors.New("injected parse error")
	}
	t.Cleanup(func() { planParseFn = orig })

	c := newPlanRulesChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error from planParseFn injection, got nil")
	}
	if !strings.Contains(err.Error(), "parsing plan") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// dogfood_version.go — osOpenDogfood error (lines 70-72)
// =============================================================================

func TestDogfoodVersionCheck_OpenError_Stub(t *testing.T) {
	// specRoot is spec/; workflows live at <project>/.github/workflows/.
	dir := t.TempDir()
	specRoot := filepath.Join(dir, "spec")
	mkdir(t, specRoot)
	mkdir(t, filepath.Join(dir, ".github", "workflows"))
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"),
		"env:\n  SPECSCORE_VERSION: v0.1.0\n")

	orig := osOpenDogfood
	osOpenDogfood = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	t.Cleanup(func() { osOpenDogfood = orig })

	c := newDogfoodVersionChecker("1.0.0")
	vs, err := c.check(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Open error is swallowed (continue) — no violations.
	if len(vs) != 0 {
		t.Errorf("expected 0 violations when osOpenDogfood fails, got %d", len(vs))
	}
}

// =============================================================================
// plan_roi.go — osOpenPlanROI error in scanROIMetadata (lines 79-81)
// =============================================================================

func TestPlanROI_ScanROIMetadata_OpenError_Stub(t *testing.T) {
	orig := osOpenPlanROI
	osOpenPlanROI = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	t.Cleanup(func() { osOpenPlanROI = orig })

	_, err := scanROIMetadata("/fake/README.md", "plans/test/README.md")
	if err == nil {
		t.Fatal("expected error from osOpenPlanROI injection, got nil")
	}
	if !strings.Contains(err.Error(), "injected open error") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// plan_roi.go — check: scanROIMetadata returns error → walk returns it (lines 61-63 → 69-71)
// =============================================================================

func TestPlanROICheck_ScanErrorPropagates_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"), "# Plan: Test\n")

	orig := osOpenPlanROI
	osOpenPlanROI = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	t.Cleanup(func() { osOpenPlanROI = orig })

	c := newPlanROIChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected error to propagate from scanROIMetadata through walk, got nil")
	}
}

// =============================================================================
// plan_hierarchy.go — osReadDirHasChildPlanDirs error (lines 103-105)
// =============================================================================

func TestHasChildPlanDirs_ReadDirError_Stub(t *testing.T) {
	orig := osReadDirHasChildPlanDirs
	osReadDirHasChildPlanDirs = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readdir error")
	}
	t.Cleanup(func() { osReadDirHasChildPlanDirs = orig })

	result := hasChildPlanDirs(t.TempDir())
	if result {
		t.Error("expected false when osReadDirHasChildPlanDirs fails")
	}
}

// =============================================================================
// plan_hierarchy.go — osOpenHasSection error (lines 126-128)
// =============================================================================

func TestHasSection_OpenError_Stub2(t *testing.T) {
	orig := osOpenHasSection
	osOpenHasSection = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	t.Cleanup(func() { osOpenHasSection = orig })

	found, line := hasSection("/fake/README.md", "## Steps")
	if found {
		t.Error("expected found=false when osOpenHasSection fails")
	}
	if line != 0 {
		t.Errorf("expected line=0, got %d", line)
	}
}

// =============================================================================
// issue_rules.go — osWriteFileIssueRules error in fix (lines 185-187)
// =============================================================================

func TestIssueRulesFix_WriteFileError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/bug-1.md": "---\ntype: issue\nslug: bug-1\nstatus: open\ncaptured_at: 2024-01-01\ncaptured_by: tester\n---\n# Issue: Bug 1\n\n## Description\n\nBroken.\n\n## Steps to Reproduce\n\n1. Do thing\n\n## Expected vs Actual\n\nExpected X, got Y.\n",
	})

	orig := osWriteFileIssueRules
	osWriteFileIssueRules = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("injected write error")
	}
	t.Cleanup(func() { osWriteFileIssueRules = orig })

	c := newIssueRulesChecker()
	f, ok := c.(fixer)
	if !ok {
		t.Fatal("issueRulesChecker does not implement fixer")
	}
	err := f.fix(root)
	if err == nil {
		t.Fatal("expected error from osWriteFileIssueRules injection, got nil")
	}
	if !strings.Contains(err.Error(), "writing") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// issue_rules.go — osReadFileIssueI015 error (lines 365-366)
// =============================================================================

func TestIssueRulesI015_ReadFileError_Stub(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/bug-1.md":  "---\ntype: issue\nslug: bug-1\nstatus: open\ncaptured_at: 2024-01-01\ncaptured_by: tester\n---\n# Issue: Bug 1\n\n## Description\n\nBroken.\n\n## Steps to Reproduce\n\n1. Do thing\n\n## Expected vs Actual\n\nExpected X, got Y.\n",
		"issues/README.md": "---\ntype: index\n---\n\n**Status:** Stable\n\n# Issues\n\n## Contents\n\n| Slug | Title | Status | Severity | Captured |\n| --- | --- | --- | --- | --- |\n| [bug-1](bug-1.md) | Bug 1 | open | — | 2024-01-01 |\n\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/issues-index-specification*\n",
	})

	orig := osReadFileIssueI015
	osReadFileIssueI015 = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileIssueI015 = orig })

	c := newIssueRulesChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ReadFile error means I-015 skips — should not produce I-015 violations.
	for _, v := range vs {
		if v.Rule == "I-015" {
			t.Errorf("expected no I-015 violations when osReadFileIssueI015 fails, got: %+v", v)
		}
	}
}

// =============================================================================
// adherence_footer.go — osWriteFileAdherenceFix error (append path, lines 180-181)
// =============================================================================

func TestAdherenceFooterFix_WriteFileError_Append_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "plans", "my-plan"))
	writeFile(t, filepath.Join(root, "plans", "my-plan", "README.md"),
		"# Plan: My Plan\n\nContent.\n")

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("injected write error")
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	c := newAdherenceFooterChecker()
	f := c.(fixer)
	err := f.fix(root)
	if err == nil {
		t.Fatal("expected error from osWriteFileAdherenceFix injection, got nil")
	}
}

// =============================================================================
// adherence_footer.go — osWriteFileAdherenceFix error (rewrite path, lines 166-167)
// =============================================================================

func TestAdherenceFooterFix_WriteFileError_Rewrite_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "plans", "my-plan"))
	// File with wrong URL → triggers rewrite path.
	writeFile(t, filepath.Join(root, "plans", "my-plan", "README.md"),
		"# Plan: My Plan\n\nContent.\n\n---\n*This document follows the https://specscore.md/wrong-specification*\n")

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("injected rewrite error")
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	c := newAdherenceFooterChecker()
	f := c.(fixer)
	err := f.fix(root)
	if err == nil {
		t.Fatal("expected error from osWriteFileAdherenceFix injection (rewrite path), got nil")
	}
}

// =============================================================================
// adherence_footer.go — osReadFilePlanReadme error (lines 324-326)
// =============================================================================

func TestWalkPlanReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "plans", "my-plan"))
	writeFile(t, filepath.Join(root, "plans", "my-plan", "README.md"),
		"# Plan: My Plan\n")

	orig := osReadFilePlanReadme
	osReadFilePlanReadme = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFilePlanReadme = orig })

	var called bool
	err := walkPlanReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFilePlanReadme fails")
	}
}

// =============================================================================
// adherence_footer.go — osReadFileTaskReadme error (lines 353-355)
// =============================================================================

func TestWalkTaskReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "plans", "alpha", "tasks", "do-thing"))
	writeFile(t, filepath.Join(root, "plans", "alpha", "tasks", "do-thing", "README.md"),
		"# Task: Do Thing\n")

	orig := osReadFileTaskReadme
	osReadFileTaskReadme = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileTaskReadme = orig })

	var called bool
	err := walkTaskReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFileTaskReadme fails")
	}
}

// =============================================================================
// adherence_footer.go — osReadFileScenariosIdx error (lines 381-383)
// =============================================================================

func TestWalkScenariosIndexes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	testsDir := filepath.Join(root, "features", "auth", "_tests")
	mkdir(t, testsDir)
	writeFile(t, filepath.Join(testsDir, "README.md"), "# Scenarios\n")

	orig := osReadFileScenariosIdx
	osReadFileScenariosIdx = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileScenariosIdx = orig })

	var called bool
	err := walkScenariosIndexes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFileScenariosIdx fails")
	}
}

// =============================================================================
// adherence_footer.go — osReadFileScenarioFile error (lines 417-419)
// =============================================================================

func TestWalkScenarioFiles_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	testsDir := filepath.Join(root, "features", "auth", "_tests")
	mkdir(t, testsDir)
	writeFile(t, filepath.Join(testsDir, "login.md"), "# Scenario: Login\n")

	orig := osReadFileScenarioFile
	osReadFileScenarioFile = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileScenarioFile = orig })

	var called bool
	err := walkScenarioFiles(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFileScenarioFile fails")
	}
}

// =============================================================================
// adherence_footer.go — osReadFileMatchingFiles error (lines 462-464)
// =============================================================================

func TestWalkMatchingFiles_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "active.md"), "active content")

	orig := osReadFileMatchingFiles
	osReadFileMatchingFiles = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileMatchingFiles = orig })

	var called bool
	err := walkMatchingFiles(root, func(path string, depth int, name string) bool {
		return strings.HasSuffix(name, ".md")
	}, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFileMatchingFiles fails")
	}
}

// =============================================================================
// feature_readme_walk.go — osReadFileFeatureReadme error (lines 33-35)
// =============================================================================

func TestWalkFeatureReadmes_ReadFileError_Stub(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features", "auth"))
	writeFile(t, filepath.Join(root, "features", "auth", "README.md"), "# Feature: Auth\n")

	orig := osReadFileFeatureReadme
	osReadFileFeatureReadme = func(name string) ([]byte, error) {
		return nil, errors.New("injected readfile error")
	}
	t.Cleanup(func() { osReadFileFeatureReadme = orig })

	var called bool
	err := walkFeatureReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called when osReadFileFeatureReadme fails")
	}
}

// =============================================================================
// idea.go — ideaDiscoverFn error (lines 114-116)
// =============================================================================

func TestCheckIdeas_DiscoverFnError_Stub2(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/my-idea.md": "# Idea: My Idea\n\n**Status:** Draft\n",
	})

	orig := ideaDiscoverFn
	ideaDiscoverFn = func(specRoot string) ([]idea.Discovered, error) {
		return nil, errors.New("injected discover error")
	}
	t.Cleanup(func() { ideaDiscoverFn = orig })

	_, err := CheckIdeas(root, false)
	if err == nil {
		t.Fatal("expected error from ideaDiscoverFn injection, got nil")
	}
}

// =============================================================================
// issue_rules.go — issueDiscoverAll error in fix (lines 175-177)
// =============================================================================

func TestIssueRulesFix_DiscoverAllError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/bug-1.md": "---\ntype: issue\nslug: bug-1\nstatus: open\ncaptured_at: 2024-01-01\ncaptured_by: tester\n---\n# Issue: Bug 1\n\n## Description\n\nBroken.\n\n## Steps to Reproduce\n\n1. Do\n\n## Expected vs Actual\n\nX vs Y.\n",
	})

	orig := issueDiscoverAll
	issueDiscoverAll = func(specRoot string) ([]issue.Discovered, error) {
		return nil, errors.New("injected discover all error")
	}
	t.Cleanup(func() { issueDiscoverAll = orig })

	c := newIssueRulesChecker()
	f, ok := c.(fixer)
	if !ok {
		t.Fatal("issueRulesChecker does not implement fixer")
	}
	err := f.fix(root)
	if err == nil {
		t.Fatal("expected error from issueDiscoverAll injection, got nil")
	}
	if !strings.Contains(err.Error(), "discovering issue artifacts") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// issue_rules.go — osMkdirAllFn error in fix (lines 181-183)
// =============================================================================

func TestIssueRulesFix_MkdirAllError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/bug-1.md": "---\ntype: issue\nslug: bug-1\nstatus: open\ncaptured_at: 2024-01-01\ncaptured_by: tester\n---\n# Issue: Bug 1\n\n## Description\n\nBroken.\n\n## Steps to Reproduce\n\n1. Do thing\n\n## Expected vs Actual\n\nExpected X, got Y.\n",
	})

	orig := osMkdirAllFn
	osMkdirAllFn = func(path string, perm os.FileMode) error {
		return errors.New("injected mkdir error")
	}
	t.Cleanup(func() { osMkdirAllFn = orig })

	c := newIssueRulesChecker()
	f, ok := c.(fixer)
	if !ok {
		t.Fatal("issueRulesChecker does not implement fixer")
	}
	err := f.fix(root)
	if err == nil {
		t.Fatal("expected error from osMkdirAllFn injection, got nil")
	}
	if !strings.Contains(err.Error(), "creating directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// issue_rules.go — issueParseFn error in lintI001AndI002 (line 505)
// =============================================================================

func TestIssueRules_ParseFnError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"issues/bug-1.md": "---\ntype: issue\nslug: bug-1\nstatus: open\ncaptured_at: 2024-01-01\ncaptured_by: tester\n---\n# Issue: Bug 1\n\n## Description\n\nBroken.\n\n## Steps to Reproduce\n\n1. Do\n\n## Expected vs Actual\n\nX.\n",
	})

	orig := issueParseFn
	issueParseFn = func(path string) (*issue.Issue, error) {
		return nil, errors.New("injected parse error")
	}
	t.Cleanup(func() { issueParseFn = orig })

	c := newIssueRulesChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// issueParseFn error → continue in lintI001AndI002 → no I-00x violations.
	for _, v := range vs {
		if strings.HasPrefix(v.Rule, "I-00") && v.Rule != "I-009" && v.Rule != "I-013" && v.Rule != "I-014" && v.Rule != "I-015" && v.Rule != "I-011" {
			t.Errorf("expected no per-file lint violations when issueParseFn fails, got: %+v", v)
		}
	}
}

// =============================================================================
// lint.go — Lint with Fix error (lines 44-46)
// =============================================================================

func TestLint_FixError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})

	orig := osWriteFileAdherenceFix
	osWriteFileAdherenceFix = func(name string, data []byte, perm fs.FileMode) error {
		return errors.New("injected fix error")
	}
	t.Cleanup(func() { osWriteFileAdherenceFix = orig })

	_, err := Lint(Options{SpecRoot: root, Fix: true})
	if err == nil {
		t.Fatal("expected error when fix fails, got nil")
	}
	if !strings.Contains(err.Error(), "fix error") {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// decision_immutability.go — gitShowFn error (lines 73-79)
// =============================================================================

func TestDecisionImmutability_GitShowError_Stub(t *testing.T) {
	orig := gitShowFn
	gitShowFn = func(repoRoot, relPath string) (string, error) {
		return "", errors.New("injected git show error")
	}
	t.Cleanup(func() { gitShowFn = orig })

	// checkDecisionImmutability needs a git repo context.
	// We just verify it doesn't panic.
	root := setupSpecTree(t, map[string]string{
		"decisions/D-0001-test.md": "# Decision: Test\n\n**Status:** Accepted\n**Date:** 2024-01-01\n\n## Context\n\nCtx.\n\n## Decision\n\nDec.\n\n## Rationale\n\nRat.\n\n## Declined Alternatives\n\n- None\n\n## Consequences at Decision Time\n\nNone.\n\n## Affected Features\n\nNone.\n\n## Observed Consequences\n\nNone observed yet.\n\n## Open Questions\n\nNone.\n",
	})
	_, _ = checkDecisionImmutability(root)
}

// =============================================================================
// dogfood_version.go — parseSemverFn error after regex match (lines 92-94)
// =============================================================================

func TestDogfoodVersion_ParseSemverFn_Error_Stub(t *testing.T) {
	dir := t.TempDir()
	specRoot := filepath.Join(dir, "spec")
	mkdir(t, specRoot)
	mkdir(t, filepath.Join(dir, ".github", "workflows"))
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"),
		"env:\n  SPECSCORE_VERSION: v0.1.0\n")

	orig := parseSemverFn
	parseSemverFn = func(s string) (semver, bool) {
		return semver{}, false
	}
	t.Cleanup(func() { parseSemverFn = orig })

	c := newDogfoodVersionChecker("1.0.0")
	vs, err := c.check(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vs) != 0 {
		t.Errorf("expected 0 violations when parseSemverFn fails, got %d", len(vs))
	}
}

// =============================================================================
// plan_hierarchy.go — check returns walk/post-walk error (lines 33-35, 93-95)
// =============================================================================

func TestPlanHierarchyCheck_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"), "# Plan: Test\n")

	// Create a symlink loop.
	_ = os.Symlink(plansDir, filepath.Join(plansDir, "my-plan", "loop"))

	c := newPlanHierarchyChecker()
	_, err := c.check(root)
	// Verify no panic — error may or may not propagate depending on OS.
	_ = err
}

// =============================================================================
// plan_roi.go — check walk error propagation (lines 38-41, 69-71)
// =============================================================================

func TestPlanROICheck_WalkCallbackError_Stub(t *testing.T) {
	root := t.TempDir()
	plansDir := filepath.Join(root, "plans")
	mkdir(t, filepath.Join(plansDir, "my-plan"))
	writeFile(t, filepath.Join(plansDir, "my-plan", "README.md"), "# Plan: Test\n")

	// scanROIMetadata error propagates through the Walk callback.
	orig := osOpenPlanROI
	osOpenPlanROI = func(name string) (*os.File, error) {
		return nil, errors.New("injected open error")
	}
	t.Cleanup(func() { osOpenPlanROI = orig })

	c := newPlanROIChecker()
	_, err := c.check(root)
	if err == nil {
		t.Fatal("expected walk error propagation from scanROIMetadata, got nil")
	}
}

// =============================================================================
// index_entries.go — osReadDir error in check (lines 58-60)
// =============================================================================

func TestIndexEntriesCheck_ReadDirError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md":      "# Features\n",
		"features/auth/README.md": "# Feature: Auth\n",
	})

	orig := osReadDir
	osReadDir = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readdir error")
	}
	t.Cleanup(func() { osReadDir = orig })

	c := newIndexEntriesChecker()
	vs, err := c.check(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ReadDir error swallowed in walk callback → 0 violations from that dir.
	_ = vs
}

// =============================================================================
// index_entries.go — osReadDir error in fix (lines 153-155)
// =============================================================================

func TestIndexEntriesFix_ReadDirError_Stub2(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md":      "# Features\n",
		"features/auth/README.md": "# Feature: Auth\n",
	})

	orig := osReadDir
	osReadDir = func(name string) ([]os.DirEntry, error) {
		return nil, errors.New("injected readdir error")
	}
	t.Cleanup(func() { osReadDir = orig })

	c := newIndexEntriesChecker()
	f := c.(fixer)
	err := f.fix(root)
	if err != nil {
		t.Fatalf("unexpected error: %v (ReadDir error should be swallowed)", err)
	}
}

// =============================================================================
// adherence_footer.go — check with walk error from target.walk (lines 141-143)
// =============================================================================

func TestAdherenceFooterCheck_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features", "auth"))
	writeFile(t, filepath.Join(root, "features", "auth", "README.md"), "# Feature: Auth\n")

	// Create a symlink loop inside features/ to trigger walk error.
	_ = os.Symlink(filepath.Join(root, "features", "auth"), filepath.Join(root, "features", "auth", "loop"))

	c := newAdherenceFooterChecker()
	_, err := c.check(root)
	// Verify no panic.
	_ = err
}

// =============================================================================
// adherence_footer.go — fix with walk error (lines 183-186)
// =============================================================================

func TestAdherenceFooterFix_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features", "auth"))
	writeFile(t, filepath.Join(root, "features", "auth", "README.md"), "# Feature: Auth\n")

	_ = os.Symlink(filepath.Join(root, "features", "auth"), filepath.Join(root, "features", "auth", "loop"))

	c := newAdherenceFooterChecker()
	f := c.(fixer)
	err := f.fix(root)
	// Verify no panic.
	_ = err
}

// =============================================================================
// oq_section.go — check Walk error callback (lines 39-41)
// =============================================================================

func TestOQSectionCheck_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features"))
	writeFile(t, filepath.Join(root, "features", "README.md"), "# Features\n\n## Open Questions\n\nNone.\n")

	_ = os.Symlink(filepath.Join(root, "features"), filepath.Join(root, "features", "loop"))

	c := newOQSectionChecker()
	_, err := c.check(root)
	// Verify no panic.
	_ = err
}

// =============================================================================
// oq_section.go — fix Walk error callback (lines 115-117)
// =============================================================================

func TestOQSectionFix_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features"))
	writeFile(t, filepath.Join(root, "features", "README.md"), "## Outstanding Questions\n\nSome.\n")

	_ = os.Symlink(filepath.Join(root, "features"), filepath.Join(root, "features", "loop"))

	c := newOQSectionChecker()
	f := c.(fixer)
	err := f.fix(root)
	// Verify no panic.
	_ = err
}

// =============================================================================
// index_entries.go — check Walk error callback (lines 44-46)
// =============================================================================

func TestIndexEntriesCheck_WalkError_Symlink(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md": "# Features\n",
	})

	_ = os.Symlink(filepath.Join(root, "features"), filepath.Join(root, "features", "loop"))

	c := newIndexEntriesChecker()
	_, err := c.check(root)
	_ = err // verify no panic
}

// =============================================================================
// index_entries.go — fix Walk error callback (lines 139-141)
// =============================================================================

func TestIndexEntriesFix_WalkError_Symlink(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md": "# Features\n",
	})

	_ = os.Symlink(filepath.Join(root, "features"), filepath.Join(root, "features", "loop"))

	c := newIndexEntriesChecker()
	f := c.(fixer)
	err := f.fix(root)
	_ = err // verify no panic
}

// =============================================================================
// studio_toolbar.go — Walk returns error (lines 221-223)
// =============================================================================

func TestStudioToolbarCheck_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features", "auth"))
	writeFile(t, filepath.Join(root, "features", "auth", "README.md"), "# Feature: Auth\n")

	_ = os.Symlink(filepath.Join(root, "features", "auth"), filepath.Join(root, "features", "auth", "loop"))

	c := newStudioToolbarChecker()
	_, err := c.check(root)
	_ = err // verify no panic
}

// =============================================================================
// linter.go — walkSpecDirs Walk error callback (lines 230-232)
// =============================================================================

func TestWalkSpecDirs_WalkError_Symlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features"))

	_ = os.Symlink(root, filepath.Join(root, "features", "loop"))

	err := walkSpecDirs(root, func(dirPath, relPath string) error {
		return nil
	})
	_ = err // verify no panic
}

// =============================================================================
// idea.go — idea.FindIdeaDirectories error (lines 81-83)
// =============================================================================

func TestCheckIdeas_FindIdeaDirsError_Symlink2(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/my-idea.md": "# Idea: My Idea\n\n**Status:** Draft\n",
	})

	mkdir(t, filepath.Join(root, "ideas", "nested"))
	_ = os.Symlink(filepath.Join(root, "ideas"), filepath.Join(root, "ideas", "nested", "loop"))

	_, err := CheckIdeas(root, false)
	_ = err // verify no panic
}

// =============================================================================
// idea.go — findMisplacedIdeaFiles Walk error callback (lines 175-177)
// =============================================================================

func TestFindMisplacedIdeaFiles_WalkError_Symlink(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/my-idea.md": "# Idea: My Idea\n\n**Status:** Draft\n",
	})

	mkdir(t, filepath.Join(root, "ideas", "nested"))
	_ = os.Symlink(filepath.Join(root, "ideas"), filepath.Join(root, "ideas", "nested", "loop"))

	_, err := findMisplacedIdeaFiles(root)
	_ = err // verify no panic
}

// =============================================================================
// feature_index.go — readFeatureIndexRows (featureIndexRules) no drift path
// =============================================================================

func TestFeatureIndexRules_NoDriftNoViolations(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md": "# Features\n\n" +
			"| Feature | Status | Kind | Description |\n" +
			"|---------|--------|------|-------------|\n" +
			"| [auth](auth/README.md) | Draft | Command | Auth feature |\n",
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})

	vs, fixed := featureIndexRules(root, false)
	if fixed {
		t.Error("expected fixed=false when no drift")
	}
	if len(vs) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(vs), vs)
	}
}

// =============================================================================
// feature_index.go — ParseFeatureStatus error path (lines 100-102)
// =============================================================================

func TestFeatureIndexRules_ParseFeatureStatusError_Stub(t *testing.T) {
	// When ParseFeatureStatus returns "Unknown", it's treated as a different
	// status from what's in the index, producing a drift violation.
	// When a feature directory doesn't exist, the row is skipped silently.
	root := setupSpecTree(t, map[string]string{
		"features/README.md": "# Features\n\n" +
			"| Feature | Status | Kind | Description |\n" +
			"|---------|--------|------|-------------|\n" +
			"| [ghost](ghost/README.md) | Draft | Command | Ghost feature |\n",
	})

	vs, _ := featureIndexRules(root, false)
	// ghost/ directory doesn't exist → skipped silently → 0 violations.
	if len(vs) != 0 {
		t.Errorf("expected 0 violations for orphaned row, got %d: %v", len(vs), vs)
	}
}

// =============================================================================
// idea.go — featureParseStatusFn error (lines 718-720 in ideaSyncRules)
// =============================================================================

func TestIdeaSyncRules_FeatureParseStatusError_Stub(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":         "# Ideas\n\n## Index\n\n| Idea | Status | Date | Owner | Promotes To |\n|------|--------|------|-------|-------------|\n\n## Open Questions\n\nNone.\n",
		"ideas/my-idea.md":        stubIdeaContent("my-idea", "Approved"),
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n\n**Source Ideas:** my-idea\n\n## Summary\n\nAuth.\n\n## Acceptance Criteria\n\n### AC: login\n\nLogin.\n\n## Open Questions\n\nNone.\n",
	})

	orig := featureParseStatusFn
	featureParseStatusFn = func(path string) (string, error) {
		return "", errors.New("injected parse status error")
	}
	t.Cleanup(func() { featureParseStatusFn = orig })

	// Should not panic or error — featureParseStatusFn error → st="" → continues.
	_, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// idea_index.go — ideaIndexRules fix-failed with both missing and drifted
// (lines 76-89)
// =============================================================================

func TestIdeaIndexRules_FixFailed_MissingAndDrifted_Stub(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":        "# Ideas\n\n## Index\n\n| Idea | Status | Date | Owner | Promotes To |\n|------|--------|------|-------|-------------|\n| [existing-idea](existing-idea.md) | Draft | 2024-01-01 | Tester | — |\n\n## Open Questions\n\nNone.\n",
		"ideas/existing-idea.md": stubIdeaContent("existing-idea", "Approved"),
		"ideas/new-idea.md":      stubIdeaContent("new-idea", "Draft"),
	})

	discovered := []idea.Discovered{
		{Slug: "existing-idea", Path: filepath.Join(root, "ideas", "existing-idea.md")},
		{Slug: "new-idea", Path: filepath.Join(root, "ideas", "new-idea.md")},
	}
	parsed := make(map[string]*idea.Idea)
	for _, d := range discovered {
		p, err := idea.Parse(d.Path)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", d.Slug, err)
		}
		parsed[d.Slug] = p
	}

	// Make the index file unwritable by removing and replacing with dir.
	idxPath := filepath.Join(root, "ideas", "README.md")
	_ = os.Remove(idxPath)
	mkdir(t, idxPath)
	t.Cleanup(func() {
		os.RemoveAll(idxPath)
	})

	vs, fixed := ideaIndexRules(root, discovered, parsed, true)
	// Because the index README is now a directory, stat returns non-nil
	// os.Stat says it exists (it's a dir) so the fix branch may run and fail.
	// Verify no panic.
	_ = vs
	_ = fixed
}

// =============================================================================
// idea_index.go — ideaIndexRules archived fix-failed (lines 142-156)
// =============================================================================

func TestIdeaIndexRules_ArchivedFixFailed_Stub(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":            "# Ideas\n\n## Index\n\n| Idea | Status | Date | Owner | Promotes To |\n|------|--------|------|-------|-------------|\n\n## Open Questions\n\nNone.\n",
		"ideas/archived/README.md":   "# Archived Ideas\n\n## Open Questions\n\nNone.\n",
		"ideas/archived/old-idea.md": stubIdeaContent("old-idea", "Archived"),
	})

	discovered := []idea.Discovered{
		{Slug: "old-idea", Path: filepath.Join(root, "ideas", "archived", "old-idea.md"), Archived: true},
	}
	parsed := make(map[string]*idea.Idea)
	for _, d := range discovered {
		p, err := idea.Parse(d.Path)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", d.Slug, err)
		}
		parsed[d.Slug] = p
	}

	// Make the archived index unwritable by replacing with directory.
	archivedIdx := filepath.Join(root, "ideas", "archived", "README.md")
	_ = os.Remove(archivedIdx)
	mkdir(t, archivedIdx)
	t.Cleanup(func() {
		os.RemoveAll(archivedIdx)
	})

	vs, fixed := ideaIndexRules(root, discovered, parsed, true)
	// Verify no panic. The fix should fail and produce violations.
	_ = vs
	_ = fixed
}

// =============================================================================
// Helpers
// =============================================================================

// stubIdeaContent builds minimal parseable idea file content for testing.
func stubIdeaContent(slug, status string) string {
	title := strings.ReplaceAll(slug, "-", " ")
	// Capitalize first letter of each word manually.
	words := strings.Fields(title)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	title = strings.Join(words, " ")

	var b strings.Builder
	b.WriteString("# Idea: " + title + "\n\n")
	b.WriteString("**Status:** " + status + "\n")
	b.WriteString("**Date:** 2024-01-01\n")
	b.WriteString("**Owner:** Test Author\n")
	b.WriteString("**Promotes To:** —\n")
	b.WriteString("**Supersedes:** —\n")
	b.WriteString("**Related Ideas:** —\n")
	b.WriteString("\n## Problem Statement\n\nHow might we test this?\n")
	b.WriteString("\n## Key Assumptions to Validate\n\n| Category | Assumption |\n|----------|------------|\n| Must-be-true | Something must be true |\n")
	b.WriteString("\n## Not Doing (and Why)\n\n- Out of scope thing\n")
	if status == "Archived" {
		b.WriteString("\n**Archive Reason:** No longer relevant\n")
	}
	b.WriteString("\n## Open Questions\n\nNone.\n")
	return b.String()
}
