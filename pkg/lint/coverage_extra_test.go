package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// sidekick_seed.go — uncovered branches
// =============================================================================

func TestSidekickSeed_InvalidYAML(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/bad.md": "---\n: invalid yaml\n  no good\n---\n\n# Bad seed\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Error("expected violations for invalid YAML")
	}
}

func TestSidekickSeed_NonMappingYAML(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/list.md": "---\n- item1\n- item2\n---\n\n# List seed\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Error("expected violations for non-mapping YAML")
	}
}

func TestSidekickSeed_EmptyFrontmatter(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/empty-fm.md": "---\n---\n\n# Empty Frontmatter\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Error("expected violations for empty frontmatter (missing required keys)")
	}
}

func TestSidekickSeed_MissingFrontmatter_Extra(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/no-fm.md": "# No frontmatter\n\nJust text.\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) == 0 {
		t.Error("expected violations for missing frontmatter")
	}
}

func TestSidekickSeed_NoH1InBody(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/no-h1.md": validSeedBody("no-h1", "", "user-prompt") + "\nNo heading, just text.\n",
	})
	// Rewrite to have a body without H1
	path := filepath.Join(specRoot, "ideas", "seeds", "no-h1.md")
	content := "---\ntype: sidekick-seed\nslug: no-h1\ncaptured_at: 2026-05-18T00:00:00Z\ncaptured_by: user\ncaptured_during: null\ntrigger: user-prompt\nstatus: queued\nsynchestra_task: null\n---\n\nJust text, no H1 heading.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	hasBodyViolation := false
	for _, v := range violations {
		if strings.Contains(v.Message, "H1") || strings.Contains(v.Message, "heading") {
			hasBodyViolation = true
		}
	}
	if !hasBodyViolation {
		t.Error("expected violation for missing H1 in body")
	}
}

func TestSidekickSeed_UnknownFrontmatterKey_Extra(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/unknown.md": "---\ntype: sidekick-seed\nslug: unknown\ncaptured_at: 2026-05-18T00:00:00Z\ncaptured_by: user\ncaptured_during: null\ntrigger: user-prompt\nstatus: queued\nsynchestra_task: null\nextra_key: bad\n---\n\n# Unknown Key Seed\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	hasUnknown := false
	for _, v := range violations {
		if strings.Contains(v.Message, "unknown") || strings.Contains(v.Message, "extra_key") {
			hasUnknown = true
		}
	}
	if !hasUnknown {
		t.Error("expected violation for unknown frontmatter key")
	}
}

func TestSidekickSeed_WrongSlug(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		// Slug in frontmatter doesn't match filename (actual-name.md vs slug: different-name)
		"ideas/seeds/actual-name.md": "---\ntype: sidekick-seed\nslug: different-name\ncaptured_at: 2026-05-18T00:00:00Z\ncaptured_by: user\ncaptured_during: null\ntrigger: user-prompt\nstatus: queued\nsynchestra_task: null\n---\n\n# Different Name\n",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	// There should be at least one violation related to slug
	if len(violations) == 0 {
		t.Error("expected at least one violation for wrong slug seed")
	}
}

func TestSidekickSeed_DirectoryInSeeds(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/valid.md": validSeedBody("valid", "Valid Seed", "user-prompt"),
	})
	// Create a directory inside seeds (should be skipped)
	mkdir(t, filepath.Join(specRoot, "ideas", "seeds", "subdir"))

	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	// Should only process .md files, not directories
	for _, v := range violations {
		if strings.Contains(v.File, "subdir") {
			t.Errorf("directory should not be processed: %v", v)
		}
	}
}

func TestSidekickSeed_NonMdFileSkipped(t *testing.T) {
	specRoot := writeSpec(t, map[string]string{
		"ideas/seeds/notes.txt": "just notes",
	})
	c := newSidekickSeedChecker()
	violations, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations for non-.md file, got %d", len(violations))
	}
}

func TestBodyFirstLineIsH1_EmptyBody(t *testing.T) {
	if bodyFirstLineIsH1("") {
		t.Error("empty body should not have H1")
	}
	if bodyFirstLineIsH1("\n\n") {
		t.Error("blank-only body should not have H1")
	}
	if !bodyFirstLineIsH1("# Title\n") {
		t.Error("H1 line should be detected")
	}
	if bodyFirstLineIsH1("Not H1\n# Title\n") {
		t.Error("first non-blank line is not H1")
	}
}

// =============================================================================
// plan_rules.go — additional plan lint rule coverage
// =============================================================================

func TestPlanRules_NoPlanFiles(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/README.md": "# Plans\n\nNo plans yet.\n",
	})
	c := newPlanRulesChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations for no plan files, got %d", len(v))
	}
}

func TestPlanRules_NoPlansDir(t *testing.T) {
	root := t.TempDir()
	c := newPlanRulesChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations for missing plans dir, got %d", len(v))
	}
}

// =============================================================================
// plan_hierarchy.go — additional branch coverage
// =============================================================================

func TestPlanHierarchy_SinglePlanNoChildren(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/simple/README.md": "# Simple Plan\n\n## Steps\n\n- Step 1\n",
	})
	c := newPlanHierarchyChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations for simple plan, got %d: %v", len(v), v)
	}
}

func TestPlanHierarchy_PlanWithChildPlansSection(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/parent/README.md":      "# Parent Plan\n\n## Child Plans\n\n- child\n",
		"plans/parent/child/README.md": "# Child Plan\n\n## Steps\n\n- Step 1\n",
	})
	c := newPlanHierarchyChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// Should have 0 violations for valid 2-level hierarchy
	if len(v) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(v), v)
	}
}

// =============================================================================
// plan_roi.go — additional coverage
// =============================================================================

func TestPlanROI_BothInvalid(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"plans/bad-roi/README.md": "# Bad ROI Plan\n\n**Effort:** tiny\n**Impact:** huge\n\n## Steps\n\n- Step 1\n",
	})
	c := newPlanROIChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 2 {
		t.Errorf("expected 2 violations (bad Effort + bad Impact), got %d: %v", len(v), v)
	}
}

func TestPlanROI_ValidSizes(t *testing.T) {
	for _, effort := range []string{"S", "M", "L", "XL"} {
		root := setupSpecTree(t, map[string]string{
			"plans/valid/README.md": "# Valid Plan\n\n**Effort:** " + effort + "\n**Impact:** high\n\n## Steps\n\n- Step 1\n",
		})
		c := newPlanROIChecker()
		v, err := c.check(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != 0 {
			t.Errorf("expected 0 violations for Effort=%s, got %d: %v", effort, len(v), v)
		}
	}
}

// =============================================================================
// oq_section.go — edge case: OQ heading at EOF
// =============================================================================

func TestOQSection_HeadingAtEOF(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/cli/README.md": "# CLI\n\n## Open Questions",
	})
	c := newOQSectionChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// OQ section exists but is empty (no content after it)
	hasEmpty := false
	for _, vi := range v {
		if vi.Rule == "oq-not-empty" {
			hasEmpty = true
		}
	}
	if !hasEmpty {
		t.Error("expected oq-not-empty violation when OQ heading is at EOF")
	}
}

func TestOQSection_ContentAfterOQ(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/cli/README.md": "# CLI\n\n## Open Questions\n\n- Question here\n",
	})
	c := newOQSectionChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations for populated OQ section, got %d: %v", len(v), v)
	}
}

// =============================================================================
// oq_section.go — fix path with non-existent specRoot
// =============================================================================

func TestOQSection_FixNonExistentRoot(t *testing.T) {
	c := newOQSectionChecker().(fixer)
	err := c.fix("/nonexistent/path")
	if err != nil {
		t.Errorf("fix should silently skip nonexistent path, got: %v", err)
	}
}

func TestOQSection_CheckNonExistentRoot(t *testing.T) {
	c := newOQSectionChecker()
	v, err := c.check("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations for nonexistent root, got %d", len(v))
	}
}

// =============================================================================
// idea.go — CheckIdeas additional coverage
// =============================================================================

func TestCheckIdeas_NoIdeasDir(t *testing.T) {
	root := t.TempDir()
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Errorf("expected nil for missing ideas dir, got %v", v)
	}
}

func TestCheckIdeas_WithFix(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md": activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/good-idea.md": validIdeaBody("Good Idea", "Draft", nil) + "\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, true)
	if err != nil {
		t.Fatal(err)
	}
	_ = v // fix mode may or may not produce violations depending on state
}

// =============================================================================
// idea.go — findMisplacedIdeaFiles
// =============================================================================

func TestFindMisplacedIdeaFiles_DeepNesting(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/archived/deep/nested.md": "# Idea: Deep Nested\n",
	})

	vs, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasMisplaced := false
	for _, v := range vs {
		if v.Rule == "idea-location" {
			hasMisplaced = true
		}
	}
	if !hasMisplaced {
		t.Error("expected idea-location violation for deeply nested file")
	}
}

// =============================================================================
// index_entries.go — additional branches
// =============================================================================

func TestIndexEntries_ChildWithNoTable(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/cli/README.md":      "# CLI\n\nNo table here.\n",
		"features/cli/task/README.md": "# Task\n",
	})
	c := newIndexEntriesChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// Should flag orphan child even when there's no table
	orphanFound := false
	for _, vi := range v {
		if strings.Contains(vi.Message, "not listed in index") {
			orphanFound = true
		}
	}
	if !orphanFound {
		t.Error("expected 'not listed in index' violation")
	}
}

func TestIndexEntries_RootFeaturesMissing(t *testing.T) {
	root := setupSpecTree(t, map[string]string{
		"features/README.md":     "# Features\n\n| Feature | Status | Kind | Description |\n|---|---|---|---|\n",
		"features/auth/README.md": "# Feature: Auth\n\n**Status:** Draft\n",
	})
	c := newIndexEntriesChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	// auth is not listed in the root features index
	orphanFound := false
	for _, vi := range v {
		if strings.Contains(vi.Message, "auth") && strings.Contains(vi.Message, "not listed") {
			orphanFound = true
		}
	}
	if !orphanFound {
		t.Error("expected violation for orphan auth feature")
	}
}

// =============================================================================
// adherence_footer.go — fix with write error simulation
// =============================================================================

func TestAdherenceFooterFix_IdeasIndexAppended(t *testing.T) {
	root := t.TempDir()
	ideasDir := filepath.Join(root, "ideas")
	mkdir(t, ideasDir)
	writeFile(t, filepath.Join(ideasDir, "README.md"), "# Ideas Index\n\nSome ideas.\n")

	c := newAdherenceFooterChecker().(fixer)
	if err := c.fix(root); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(ideasDir, "README.md"))
	if !strings.Contains(string(got), "https://specscore.md/ideas-index-specification") {
		t.Errorf("expected ideas-index-specification URL:\n%s", got)
	}
}

func TestAdherenceFooterFix_FeaturesIndexAppended(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features"))
	writeFile(t, filepath.Join(root, "features", "README.md"), "# Features\n\n| Feature | Status |\n|---|---|\n")

	c := newAdherenceFooterChecker().(fixer)
	if err := c.fix(root); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(filepath.Join(root, "features", "README.md"))
	if !strings.Contains(string(got), "https://specscore.md/features-index-specification") {
		t.Errorf("expected features-index-specification URL:\n%s", got)
	}
}

// =============================================================================
// readme_exists.go — seedsDir skip
// =============================================================================

func TestReadmeExists_SkipsSeedsDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "# Root\n")
	mkdir(t, filepath.Join(root, "ideas", "seeds"))
	// seeds dir without README — should NOT trigger violation

	c := newReadmeExistsChecker()
	v, err := c.check(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, vi := range v {
		if strings.Contains(vi.File, "seeds") {
			t.Errorf("seeds dir should be skipped: %v", vi)
		}
	}
}

// =============================================================================
// feature_readme_walk.go — walkFeatureReadmes
// =============================================================================

func TestWalkFeatureReadmes_NoFeaturesDir(t *testing.T) {
	root := t.TempDir()
	var called bool
	err := walkFeatureReadmes(root, func(path string, content []byte) {
		called = true
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("should not call fn when features dir doesn't exist")
	}
}

func TestWalkFeatureReadmes_SkipsNonReadme(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "features", "auth"))
	writeFile(t, filepath.Join(root, "features", "auth", "README.md"), "# Auth\n")
	writeFile(t, filepath.Join(root, "features", "auth", "notes.md"), "# Notes\n")

	var paths []string
	err := walkFeatureReadmes(root, func(path string, content []byte) {
		paths = append(paths, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if strings.Contains(p, "notes.md") {
			t.Error("notes.md should not be visited by walkFeatureReadmes")
		}
	}
}

// =============================================================================
// idea.go — ideaFileRules edge cases
// =============================================================================

func TestIdeaRules_IdeaWithBadSlug(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":     activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/BAD_SLUG.md":   "# Idea: Bad Slug\n\n**Status:** Draft\n\n**Date:** 2026-05-01\n**Owner:** alice\n**Promotes To:** —\n**Supersedes:** —\n**Related Ideas:** —\n\n## Problem Statement\nHow Might We x.\n\n## Context\nx\n\n## Recommended Direction\nx\n\n## Alternatives Considered\nx\n\n## MVP Scope\nx\n\n## Not Doing (and Why)\n- x — y\n\n## Key Assumptions to Validate\n| Tier | Assumption | How to validate |\n|---|---|---|\n| Must-be-true | x | x |\n\n## SpecScore Integration\n- x\n\n## Open Questions\nNone at this time.\n\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasSlugRule := false
	for _, vi := range v {
		if vi.Rule == "idea-slug-format" {
			hasSlugRule = true
		}
	}
	if !hasSlugRule {
		t.Error("expected idea-slug-format violation for BAD_SLUG.md")
	}
}

func TestIdeaRules_IdeaWithWrongTitle(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":   activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/bad-title.md": "# Wrong Title Format\n\n**Status:** Draft\n**Date:** 2026-05-01\n**Owner:** alice\n**Promotes To:** —\n**Supersedes:** —\n**Related Ideas:** —\n\n## Problem Statement\nHow Might We x.\n\n## Context\nx\n\n## Recommended Direction\nx\n\n## Alternatives Considered\nx\n\n## MVP Scope\nx\n\n## Not Doing (and Why)\n- x — y\n\n## Key Assumptions to Validate\n| Tier | Assumption | How to validate |\n|---|---|---|\n| Must-be-true | x | x |\n\n## SpecScore Integration\n- x\n\n## Open Questions\nNone at this time.\n\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasTitleRule := false
	for _, vi := range v {
		if vi.Rule == "idea-title-format" {
			hasTitleRule = true
		}
	}
	if !hasTitleRule {
		t.Error("expected idea-title-format violation for wrong title format")
	}
}

func TestIdeaRules_IdeaWithInvalidStatus(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":      activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/bad-status.md":  "# Idea: Bad Status\n\n**Status:** Invalid\n**Date:** 2026-05-01\n**Owner:** alice\n**Promotes To:** —\n**Supersedes:** —\n**Related Ideas:** —\n\n## Problem Statement\nHow Might We x.\n\n## Context\nx\n\n## Recommended Direction\nx\n\n## Alternatives Considered\nx\n\n## MVP Scope\nx\n\n## Not Doing (and Why)\n- x — y\n\n## Key Assumptions to Validate\n| Tier | Assumption | How to validate |\n|---|---|---|\n| Must-be-true | x | x |\n\n## SpecScore Integration\n- x\n\n## Open Questions\nNone at this time.\n\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasStatusRule := false
	for _, vi := range v {
		if vi.Rule == "idea-status-values" {
			hasStatusRule = true
		}
	}
	if !hasStatusRule {
		t.Error("expected idea-status-values violation for invalid status")
	}
}

func TestIdeaRules_IdeaMissingRequiredSections(t *testing.T) {
	root := writeSpec(t, map[string]string{
		"ideas/README.md":      activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/no-sections.md": "# Idea: No Sections\n\n**Status:** Draft\n**Date:** 2026-05-01\n**Owner:** alice\n**Promotes To:** —\n**Supersedes:** —\n**Related Ideas:** —\n\nJust text, no sections.\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasSectionRule := false
	for _, vi := range v {
		if vi.Rule == "idea-required-sections" {
			hasSectionRule = true
		}
	}
	if !hasSectionRule {
		t.Error("expected idea-required-sections violation")
	}
}

func TestIdeaRules_IdeaWithEmptyNotDoing(t *testing.T) {
	body := validIdeaBody("Empty Not Doing", "Draft", nil)
	body = strings.Replace(body, "- Thing — reason", "<!-- empty -->", 1)
	root := writeSpec(t, map[string]string{
		"ideas/README.md":          activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/empty-not-doing.md": body + "\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasNotDoingRule := false
	for _, vi := range v {
		if vi.Rule == "idea-not-doing-non-empty" {
			hasNotDoingRule = true
		}
	}
	if !hasNotDoingRule {
		t.Error("expected idea-not-doing-non-empty violation")
	}
}

func TestIdeaRules_IdeaMissingHMW(t *testing.T) {
	body := validIdeaBody("No HMW", "Draft", nil)
	body = strings.Replace(body, "How Might We ship faster?", "Just a statement.", 1)
	root := writeSpec(t, map[string]string{
		"ideas/README.md":    activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/no-hmw.md":    body + "\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasHMW := false
	for _, vi := range v {
		if vi.Rule == "idea-hmw-framing" {
			hasHMW = true
		}
	}
	if !hasHMW {
		t.Error("expected idea-hmw-framing violation")
	}
}

func TestIdeaRules_IdeaMissingMustBeTrue(t *testing.T) {
	body := validIdeaBody("No Must Be True", "Draft", nil)
	body = strings.Replace(body, "| Must-be-true | Users want this | Survey |", "| Should-be-true | Users want this | Survey |", 1)
	root := writeSpec(t, map[string]string{
		"ideas/README.md":             activeIndex + "\n## Open Questions\n\nNone at this time.\n\n---\n*This document follows the https://specscore.md/ideas-index-specification*\n",
		"ideas/no-must-be-true.md":    body + "\n---\n*This document follows the https://specscore.md/idea-specification*\n",
	})
	v, err := CheckIdeas(root, false)
	if err != nil {
		t.Fatal(err)
	}
	hasMBT := false
	for _, vi := range v {
		if vi.Rule == "idea-must-be-true-present" {
			hasMBT = true
		}
	}
	if !hasMBT {
		t.Error("expected idea-must-be-true-present violation")
	}
}
