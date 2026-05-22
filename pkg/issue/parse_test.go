package issue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.md")
	content := `---
type: issue
slug: foo
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: alice
---

# Issue: Foo

## Description

Body.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	iss, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if iss.Type != "issue" {
		t.Errorf("Type = %q; want %q", iss.Type, "issue")
	}
	if iss.Slug != "foo" {
		t.Errorf("Slug = %q; want %q", iss.Slug, "foo")
	}
	if !iss.HasFrontmatter {
		t.Errorf("HasFrontmatter = false; want true")
	}
	if iss.Frontmatter["status"] != "open" {
		t.Errorf("Frontmatter[status] = %q; want %q", iss.Frontmatter["status"], "open")
	}
}

func TestParse_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-frontmatter.md")
	if err := os.WriteFile(path, []byte("# Plain markdown\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	iss, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if iss.HasFrontmatter {
		t.Errorf("HasFrontmatter = true; want false")
	}
	if iss.Type != "" {
		t.Errorf("Type = %q; want empty", iss.Type)
	}
}

func TestDiscoverAll_FindsIssueOutsidePatterns(t *testing.T) {
	specRoot := t.TempDir()
	// One in canonical location, one off-pattern.
	files := map[string]string{
		"issues/in.md":         minimalIssue("in"),
		"random-dir/out.md":    minimalIssue("out"),
		"features/foo/issues/scoped.md": minimalIssue("scoped"),
		"ideas/seed.md":        "# Just an idea\n", // no frontmatter type=issue
	}
	for rel, c := range files {
		p := filepath.Join(specRoot, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d; want 3; got %+v", len(got), got)
	}
	byRel := map[string]Discovered{}
	for _, d := range got {
		byRel[d.RelPath] = d
	}
	if d, ok := byRel["issues/in.md"]; !ok || !d.MatchesPattern || d.FeatureSlug != "" {
		t.Errorf("issues/in.md classification wrong: %+v", d)
	}
	if d, ok := byRel["random-dir/out.md"]; !ok || d.MatchesPattern {
		t.Errorf("random-dir/out.md should be off-pattern: %+v", d)
	}
	if d, ok := byRel["features/foo/issues/scoped.md"]; !ok || !d.MatchesPattern || d.FeatureSlug != "foo" {
		t.Errorf("features/foo/issues/scoped.md classification wrong: %+v", d)
	}
}

func minimalIssue(slug string) string {
	return `---
type: issue
slug: ` + slug + `
status: open
captured_at: 2026-05-22T00:00:00Z
captured_by: tester
---

# Issue: ` + slug + `

## Description
Body.
`
}
