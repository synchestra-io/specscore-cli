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
		"issues/in.md":                  minimalIssue("in"),
		"random-dir/out.md":             minimalIssue("out"),
		"features/foo/issues/scoped.md": minimalIssue("scoped"),
		"ideas/seed.md":                 "# Just an idea\n", // no frontmatter type=issue
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

// ---------------------------------------------------------------------------
// Parse: error path (unreadable file).
// ---------------------------------------------------------------------------

func TestParse_NonexistentFile(t *testing.T) {
	_, err := Parse(filepath.Join(t.TempDir(), "does-not-exist.md"))
	if err == nil {
		t.Fatal("Parse accepted nonexistent file")
	}
}

// ---------------------------------------------------------------------------
// parseBytes: invalid YAML in frontmatter.
// ---------------------------------------------------------------------------

func TestParseBytes_InvalidYAML(t *testing.T) {
	content := "---\n: [invalid yaml\n---\nBody.\n"
	iss := parseBytes("bad.md", []byte(content))
	if !iss.HasFrontmatter {
		t.Error("HasFrontmatter should be true even with invalid YAML")
	}
	// Frontmatter keys should be empty because YAML parse failed.
	if len(iss.FrontmatterKeyOrder) != 0 {
		t.Errorf("FrontmatterKeyOrder should be empty, got %v", iss.FrontmatterKeyOrder)
	}
	if iss.Type != "" {
		t.Errorf("Type should be empty for invalid YAML, got %q", iss.Type)
	}
}

// ---------------------------------------------------------------------------
// extractRawNode: edge cases.
// ---------------------------------------------------------------------------

func TestExtractRawNode_EmptyFrontmatter(t *testing.T) {
	got := extractRawNode("", "bugs")
	if got != nil {
		t.Error("extractRawNode should return nil for empty frontmatter")
	}
	got = extractRawNode("   \n  ", "bugs")
	if got != nil {
		t.Error("extractRawNode should return nil for whitespace-only frontmatter")
	}
}

func TestExtractRawNode_InvalidYAML(t *testing.T) {
	got := extractRawNode(": [broken", "bugs")
	if got != nil {
		t.Error("extractRawNode should return nil for invalid YAML")
	}
}

func TestExtractRawNode_NonMappingYAML(t *testing.T) {
	// A YAML scalar document, not a mapping.
	got := extractRawNode("just a scalar", "bugs")
	if got != nil {
		t.Error("extractRawNode should return nil for non-mapping YAML")
	}
}

func TestExtractRawNode_KeyNotFound(t *testing.T) {
	got := extractRawNode("type: issue\nslug: foo", "bugs")
	if got != nil {
		t.Error("extractRawNode should return nil when key is absent")
	}
}

func TestExtractRawNode_KeyFound(t *testing.T) {
	got := extractRawNode("type: issue\nbugs:\n  - BUG-1", "bugs")
	if got == nil {
		t.Fatal("extractRawNode should return non-nil for existing key")
	}
}

func TestExtractRawNode_EmptyDocument(t *testing.T) {
	// A YAML document with no content nodes (e.g., just a comment).
	got := extractRawNode("# just a comment", "bugs")
	// The YAML unmarshaler may produce zero Content nodes for a pure comment.
	// In that case extractRawNode should return nil gracefully.
	if got != nil {
		t.Error("extractRawNode should return nil for empty YAML document")
	}
}

// ---------------------------------------------------------------------------
// splitFrontmatter: edge cases.
// ---------------------------------------------------------------------------

func TestSplitFrontmatter_EmptyContent(t *testing.T) {
	_, _, ok := splitFrontmatter("")
	if ok {
		t.Error("splitFrontmatter should return false for empty content")
	}
}

func TestSplitFrontmatter_NoClosingDelimiter(t *testing.T) {
	_, _, ok := splitFrontmatter("---\ntype: issue\nno closing delimiter\n")
	if ok {
		t.Error("splitFrontmatter should return false when no closing --- found")
	}
}

func TestSplitFrontmatter_CRLFDelimiters(t *testing.T) {
	front, body, ok := splitFrontmatter("---\r\ntype: issue\r\n---\r\nBody here.\r\n")
	if !ok {
		t.Fatal("splitFrontmatter should handle CRLF delimiters")
	}
	// The split uses \n as separator, so \r remains attached to middle lines.
	// The delimiter lines get matched via TrimRight(\r), but inner content
	// retains the \r in the join.
	if front != "type: issue\r" {
		t.Errorf("front = %q; want %q", front, "type: issue\r")
	}
	if body != "Body here.\r\n" {
		t.Errorf("body = %q; want %q", body, "Body here.\r\n")
	}
}

// ---------------------------------------------------------------------------
// parseFrontmatterKeys: edge cases.
// ---------------------------------------------------------------------------

func TestParseFrontmatterKeys_EmptyFrontmatter(t *testing.T) {
	keys, order, err := parseFrontmatterKeys("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("keys should be empty, got %v", keys)
	}
	if len(order) != 0 {
		t.Errorf("order should be empty, got %v", order)
	}
}

func TestParseFrontmatterKeys_WhitespaceOnly(t *testing.T) {
	keys, order, err := parseFrontmatterKeys("   \n  \t  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("keys should be empty, got %v", keys)
	}
	if len(order) != 0 {
		t.Errorf("order should be empty, got %v", order)
	}
}

func TestParseFrontmatterKeys_InvalidYAML(t *testing.T) {
	_, _, err := parseFrontmatterKeys(": [broken yaml")
	if err == nil {
		t.Fatal("parseFrontmatterKeys should return error for invalid YAML")
	}
}

func TestParseFrontmatterKeys_NonMapping(t *testing.T) {
	_, _, err := parseFrontmatterKeys("- list\n- items")
	if err == nil {
		t.Fatal("parseFrontmatterKeys should return error for non-mapping YAML")
	}
}

func TestParseFrontmatterKeys_EmptyDocumentNode(t *testing.T) {
	// A YAML document with just a comment; unmarshaler may produce zero Content.
	keys, order, err := parseFrontmatterKeys("# just a comment\n")
	// Depending on the YAML parser, this could produce an empty doc node.
	// The function should handle it gracefully.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("keys should be empty, got %v", keys)
	}
	if len(order) != 0 {
		t.Errorf("order should be empty, got %v", order)
	}
}

// ---------------------------------------------------------------------------
// DiscoverAll: edge cases.
// ---------------------------------------------------------------------------

func TestDiscoverAll_NonexistentRoot(t *testing.T) {
	got, err := DiscoverAll(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("DiscoverAll should not error for nonexistent root: %v", err)
	}
	if got != nil {
		t.Errorf("DiscoverAll should return nil for nonexistent root, got %v", got)
	}
}

func TestDiscoverAll_RootIsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-dir.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverAll(path)
	if err != nil {
		t.Fatalf("DiscoverAll should not error when root is a file: %v", err)
	}
	if got != nil {
		t.Errorf("DiscoverAll should return nil when root is a file, got %v", got)
	}
}

func TestDiscoverAll_SkipsHiddenDirs(t *testing.T) {
	specRoot := t.TempDir()
	// Hidden dir containing an issue file.
	hiddenDir := filepath.Join(specRoot, ".hidden", "issues")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte(minimalIssue("secret")), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverAll should skip hidden dirs, found %d files: %+v", len(got), got)
	}
}

func TestDiscoverAll_SkipsREADME(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A README.md with type: issue should be skipped.
	if err := os.WriteFile(filepath.Join(issuesDir, "README.md"), []byte(minimalIssue("readme")), 0o644); err != nil {
		t.Fatal(err)
	}
	// A real issue file should be found.
	if err := os.WriteFile(filepath.Join(issuesDir, "real.md"), []byte(minimalIssue("real")), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d; want 1 (README.md should be skipped); got %+v", len(got), got)
	}
	if got[0].Slug != "real" {
		t.Errorf("Slug = %q; want %q", got[0].Slug, "real")
	}
}

func TestDiscoverAll_SkipsNonMdFiles(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "notes.txt"), []byte("not markdown"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverAll should skip non-.md files, found %d", len(got))
	}
}

func TestDiscoverAll_SkipsNonIssueType(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file with type: feature (not issue) should be skipped.
	content := "---\ntype: feature\nslug: not-issue\n---\n# Not an issue\n"
	if err := os.WriteFile(filepath.Join(issuesDir, "not-issue.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverAll should skip files with type != issue, found %d", len(got))
	}
}

func TestDiscoverAll_EmptyRoot(t *testing.T) {
	specRoot := t.TempDir()
	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverAll should return empty for empty root, found %d", len(got))
	}
}

func TestDiscoverAll_UnreadableFile(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file that we'll make unreadable.
	unreadable := filepath.Join(issuesDir, "unreadable.md")
	if err := os.WriteFile(unreadable, []byte(minimalIssue("unreadable")), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make it unreadable.
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o644) })

	// Should not error at the DiscoverAll level - unreadable files are ignored.
	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll should not propagate unreadable file errors: %v", err)
	}
	// The unreadable file should be skipped (perr != nil path).
	if len(got) != 0 {
		t.Errorf("DiscoverAll should skip unreadable files, found %d", len(got))
	}
}

func TestDiscoverAll_UnreadableSubdir(t *testing.T) {
	specRoot := t.TempDir()
	// Create a subdirectory and make it unreadable to trigger walk errors.
	badDir := filepath.Join(specRoot, "issues", "badperm")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Put a file in it before locking.
	if err := os.WriteFile(filepath.Join(badDir, "test.md"), []byte(minimalIssue("test")), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the directory unreadable.
	if err := os.Chmod(badDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(badDir, 0o755) })

	// DiscoverAll should return an error from the walk when it can't read the dir.
	_, err := DiscoverAll(specRoot)
	if err == nil {
		t.Fatal("DiscoverAll should propagate walk errors from unreadable directories")
	}
}

// ---------------------------------------------------------------------------
// classifyPath: edge cases.
// ---------------------------------------------------------------------------

func TestClassifyPath_ExtraSegments(t *testing.T) {
	// Too many segments for pattern 1.
	match, slug := classifyPath("issues/sub/deep.md")
	if match {
		t.Error("issues/sub/deep.md should not match")
	}
	if slug != "" {
		t.Errorf("slug should be empty, got %q", slug)
	}
}

func TestClassifyPath_WrongPrefix(t *testing.T) {
	match, _ := classifyPath("other/foo.md")
	if match {
		t.Error("other/foo.md should not match any pattern")
	}
}

func TestClassifyPath_SingleSegment(t *testing.T) {
	match, _ := classifyPath("foo.md")
	if match {
		t.Error("foo.md should not match any pattern")
	}
}

func TestClassifyPath_FiveSegments(t *testing.T) {
	match, _ := classifyPath("features/a/issues/b/c.md")
	if match {
		t.Error("five-segment path should not match")
	}
}

func TestClassifyPath_FeaturePatternWrongDirs(t *testing.T) {
	// 4 segments but wrong directory names.
	match, _ := classifyPath("other/foo/issues/bar.md")
	if match {
		t.Error("4 segments with wrong first dir should not match")
	}
	match, _ = classifyPath("features/foo/other/bar.md")
	if match {
		t.Error("4 segments with wrong third dir should not match")
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
