package property

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// discover.go — shouldSkipDir empty / branches
// ---------------------------------------------------------------------------

// TestShouldSkipDir_EmptyName covers line 72-74.
func TestShouldSkipDir_EmptyName(t *testing.T) {
	if shouldSkipDir("") {
		t.Error("shouldSkipDir(\"\") = true, want false")
	}
}

func TestShouldSkipDir_VisibleName(t *testing.T) {
	if shouldSkipDir("shared") {
		t.Error("shouldSkipDir(\"shared\") = true, want false")
	}
}

func TestShouldSkipDir_HiddenName(t *testing.T) {
	if !shouldSkipDir(".cache") {
		t.Error("shouldSkipDir(\".cache\") = false, want true")
	}
}

func TestShouldSkipDir_UnderscoreName(t *testing.T) {
	if !shouldSkipDir("_tests") {
		t.Error("shouldSkipDir(\"_tests\") = false, want true")
	}
}

// ---------------------------------------------------------------------------
// discover.go — walkErr propagation + sort stability for same-slug case
// ---------------------------------------------------------------------------

// TestDiscover_WalkError exercises the walk-error propagation path
// (discover.go:38-40 and :55-57). An unreadable subdirectory under
// spec/features/ triggers walkErr != nil.
func TestDiscover_WalkError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	sub := filepath.Join(featuresDir, "shared")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if _, err := Discover(root); err == nil {
		t.Error("expected discover to surface the walk error")
	}
}

// TestDiscover_IgnoresNonPropertyFiles covers discover.go:48-50 — files
// without the .property.md suffix must be ignored during the walk.
func TestDiscover_IgnoresNonPropertyFiles(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# stub\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("features/shared/email.property.md")
	mk("features/shared/README.md")
	mk("features/shared/notes.txt")

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 entry (only .property.md), got %+v", got)
	}
}

// TestDiscover_SameSlugSortedByPath covers the sort.Slice tie-breaker
// (discover.go:60-64). When two files have the same slug, the sort must
// break ties by path. We can't easily reproduce same-slug-different-path
// (filename forces uniqueness within a dir), but we CAN exercise the sort
// path by having different slugs sorted alphabetically.
func TestDiscover_SameSlugTieBreaker(t *testing.T) {
	root := t.TempDir()
	mkP := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# stub\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Place the same slug in two different dirs.
	mkP("features/a/email.property.md")
	mkP("features/b/email.property.md")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	if got[0].Path > got[1].Path {
		t.Errorf("entries not sorted by path tie-break: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// walk.go — Walk error propagation paths
// ---------------------------------------------------------------------------

// TestWalk_DiscoverError exercises the Discover-error propagation
// (walk.go:13-15).
func TestWalk_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	sub := filepath.Join(featuresDir, "shared")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if err := Walk(root, func(d *Doc) error { return nil }); err == nil {
		t.Error("expected Walk to surface the Discover error")
	}
}

// TestWalk_ParseError exercises the Parse-error propagation (walk.go:18-20).
func TestWalk_ParseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	abs := filepath.Join(root, "features", "shared", "email.property.md")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(abs, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(abs, 0o644) })

	if err := Walk(root, func(d *Doc) error { return nil }); err == nil {
		t.Error("expected Walk to surface the Parse error")
	}
}

// ---------------------------------------------------------------------------
// parse.go — Parse os.Open error + scanner error
// ---------------------------------------------------------------------------

// TestParse_MissingFile exercises line 116-118 — os.Open failure.
func TestParse_MissingFile(t *testing.T) {
	_, err := Parse(filepath.Join(t.TempDir(), "no-such.property.md"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestParse_ScannerError_TooLongLine exercises line 132-134 (bufio.Scanner.Err()).
func TestParse_ScannerError_TooLongLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "huge.property.md")
	huge := strings.Repeat("a", 2*1024*1024)
	body := "---\nkind: property\nid: huge\ndata_type: string\nchecks: {}\n" + huge + "\n---\n# Property: huge\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(path); err == nil {
		t.Error("expected scanner-error from oversize line")
	}
}

// ---------------------------------------------------------------------------
// parse.go — parseFrontmatter degenerate cases
// ---------------------------------------------------------------------------

// TestParse_FrontmatterUnclosed covers line 179-181 — opening `---` with
// no closing, parseFrontmatter returns 0.
func TestParse_FrontmatterUnclosed(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "unclosed.property.md")
	body := "---\nkind: property\nid: foo\nNO CLOSE\n\n# Property: foo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter should be nil for unclosed frontmatter, got %+v", doc.Frontmatter)
	}
}

// TestParse_FrontmatterMalformedYAML covers line 185-188.
func TestParse_FrontmatterMalformedYAML(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "malformed.property.md")
	body := "---\nkind: property\n bad: ::: indent\n---\n\n# Property: malformed\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter should be nil for malformed YAML; got %+v", doc.Frontmatter)
	}
}

// ---------------------------------------------------------------------------
// parse.go — decodeFrontmatter degenerate cases
// ---------------------------------------------------------------------------

// TestDecodeFrontmatter_EmptyDocumentNode covers line 203-205.
func TestDecodeFrontmatter_EmptyDocumentNode(t *testing.T) {
	if got := decodeFrontmatter(&yaml.Node{Kind: yaml.DocumentNode}); got != nil {
		t.Errorf("expected nil for empty document node, got %+v", got)
	}
}

// TestDecodeFrontmatter_NonMappingRoot covers line 208-210.
func TestDecodeFrontmatter_NonMappingRoot(t *testing.T) {
	if got := decodeFrontmatter(&yaml.Node{Kind: yaml.ScalarNode, Value: "x"}); got != nil {
		t.Errorf("expected nil for non-mapping root, got %+v", got)
	}
}

// TestDecodeFrontmatter_NonScalarKey covers line 217-218 (key.Kind != Scalar).
// Build a mapping node whose key is itself a mapping → must be skipped.
func TestDecodeFrontmatter_NonScalarKey(t *testing.T) {
	mapping := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode}, // weird non-scalar key
			{Kind: yaml.ScalarNode, Value: "x"},
			{Kind: yaml.ScalarNode, Value: "kind"},
			{Kind: yaml.ScalarNode, Value: "property"},
		},
	}
	got := decodeFrontmatter(mapping)
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if got.Kind != "property" {
		t.Errorf("Kind = %q, want property", got.Kind)
	}
}

// ---------------------------------------------------------------------------
// parse.go — decodeChecks branches
// ---------------------------------------------------------------------------

// TestDecodeChecks_NilNode covers line 250-252.
func TestDecodeChecks_NilNode(t *testing.T) {
	got := decodeChecks(nil)
	if got == nil {
		t.Fatal("expected non-nil empty map, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %+v", got)
	}
}

// TestDecodeChecks_NonMappingNode covers same line.
func TestDecodeChecks_NonMappingNode(t *testing.T) {
	got := decodeChecks(&yaml.Node{Kind: yaml.ScalarNode, Value: "x"})
	if got == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for non-mapping node, got %+v", got)
	}
}

// TestDecodeChecks_DecodeError covers line 253-255 — yamlNodeDecodeFn
// returns an error. Real yaml.Node.Decode never fails for the shapes we
// pass; the seam exists so tests can drive the failure branch.
func TestDecodeChecks_DecodeError(t *testing.T) {
	orig := yamlNodeDecodeFn
	t.Cleanup(func() { yamlNodeDecodeFn = orig })
	yamlNodeDecodeFn = func(n *yaml.Node, out interface{}) error {
		return errCoverage
	}

	got := decodeChecks(&yaml.Node{Kind: yaml.MappingNode})
	if got == nil {
		t.Fatal("expected non-nil empty map even on Decode error")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map after Decode error; got %+v", got)
	}
}

var errCoverage = &coverageErr{}

type coverageErr struct{}

func (e *coverageErr) Error() string { return "coverage sentinel" }

// TestDecodeChecks_ValidMapping exercises the happy path.
func TestDecodeChecks_ValidMapping(t *testing.T) {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "required"},
			{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"},
		},
	}
	got := decodeChecks(node)
	if v, ok := got["required"].(bool); !ok || !v {
		t.Errorf("required = %v, want true (bool)", got["required"])
	}
}
