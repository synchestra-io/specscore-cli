package entity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// refs.go — uncovered branches of resolveRelativeOrURL
// ---------------------------------------------------------------------------

// TestResolveRef_EmptyValue covers the empty-string early return
// (refs.go:35-37). After TrimSpace, an empty value yields ("", false, nil).
func TestResolveRef_EmptyValue(t *testing.T) {
	root := t.TempDir()
	entityPath := filepath.Join(root, "features", "user", "user.entity.md")
	resolved, isLocal, err := ResolveRef(root, entityPath, "")
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if resolved != "" {
		t.Errorf("resolved = %q, want empty", resolved)
	}
	if isLocal {
		t.Error("isLocal = true, want false")
	}
}

// TestResolveRef_WhitespaceOnly covers the same empty-after-TrimSpace path.
func TestResolveRef_WhitespaceOnly(t *testing.T) {
	root := t.TempDir()
	entityPath := filepath.Join(root, "features", "user", "user.entity.md")
	_, isLocal, err := ResolveRef(root, entityPath, "   \t  ")
	if err != nil {
		t.Fatal(err)
	}
	if isLocal {
		t.Error("isLocal = true, want false for whitespace-only")
	}
}

// TestResolveInherits_EmptyValue exercises the same early-return via
// ResolveInherits to keep the exported helpers in lockstep.
func TestResolveInherits_EmptyValue(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "features", "user", "user.entity.md")
	resolved, isLocal, err := ResolveInherits(root, child, "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "" || isLocal {
		t.Errorf("expected (\"\", false), got (%q, %v)", resolved, isLocal)
	}
}

// TestResolveRef_FilepathAbsResolvedErrors covers refs.go:45-47 — the
// first filepathAbs call (on the joined-resolved path) errs.
func TestResolveRef_FilepathAbsResolvedErrors(t *testing.T) {
	orig := filepathAbsFn
	t.Cleanup(func() { filepathAbsFn = orig })
	wantErr := os.ErrInvalid
	filepathAbsFn = func(p string) (string, error) { return "", wantErr }

	_, _, err := ResolveRef("/spec", "/spec/x.entity.md", "./y.property.md")
	if err == nil {
		t.Error("expected propagated error from filepathAbs")
	}
}

// TestResolveRef_FilepathAbsSpecRootErrors covers refs.go:49-51 — the
// second filepathAbs call (on specRoot) errs while the first succeeded.
func TestResolveRef_FilepathAbsSpecRootErrors(t *testing.T) {
	orig := filepathAbsFn
	t.Cleanup(func() { filepathAbsFn = orig })
	calls := 0
	filepathAbsFn = func(p string) (string, error) {
		calls++
		if calls == 1 {
			return p, nil
		}
		return "", os.ErrInvalid
	}

	_, _, err := ResolveRef("/spec", "/spec/x.entity.md", "./y.property.md")
	if err == nil {
		t.Error("expected propagated error from second filepathAbs")
	}
}

// TestResolveRef_FilepathRelErrors covers refs.go:53-55 — filepathRel
// errors. The function returns (absResolved, false, nil) — no error
// propagation; the local-relativity flag is just false.
func TestResolveRef_FilepathRelErrors(t *testing.T) {
	orig := filepathRelFn
	t.Cleanup(func() { filepathRelFn = orig })
	filepathRelFn = func(basepath, targpath string) (string, error) {
		return "", os.ErrInvalid
	}

	resolved, isLocal, err := ResolveRef("/spec", "/spec/x.entity.md", "./y.property.md")
	if err != nil {
		t.Errorf("expected no error (Rel-failure is silently absorbed); got %v", err)
	}
	if isLocal {
		t.Error("expected isLocal=false after Rel error")
	}
	if resolved == "" {
		t.Error("expected non-empty resolved path")
	}
}

// ---------------------------------------------------------------------------
// walk.go — Walk error propagation paths
// ---------------------------------------------------------------------------

// TestWalk_DiscoverError exercises the Discover-error propagation path
// (walk.go:11-13). When Discover fails (parent dir unreadable) Walk MUST
// return the error.
func TestWalk_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(featuresDir, "secret")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	called := false
	err := Walk(root, func(d *Doc) error {
		called = true
		return nil
	})
	if err == nil {
		t.Error("expected Walk to surface the Discover error")
	}
	_ = called
}

// TestWalk_ParseError exercises the Parse-error propagation path
// (walk.go:16-18). When Parse fails (file unreadable) Walk MUST return
// the error.
func TestWalk_ParseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	abs := filepath.Join(root, "features", "user", "user.entity.md")
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

	err := Walk(root, func(d *Doc) error { return nil })
	if err == nil {
		t.Error("expected Walk to surface the Parse error")
	}
}

// ---------------------------------------------------------------------------
// discover.go — branches we missed
// ---------------------------------------------------------------------------

// TestDiscover_FilepathAbsError covers discover.go:55-57 — when
// filepathAbs fails for an entity file, the loop falls back to the
// non-absolute path.
func TestDiscover_FilepathAbsError(t *testing.T) {
	orig := filepathAbsFn
	t.Cleanup(func() { filepathAbsFn = orig })
	filepathAbsFn = func(p string) (string, error) { return "", os.ErrInvalid }

	root := t.TempDir()
	abs := filepath.Join(root, "features", "user", "user.entity.md")
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entity, got %+v", got)
	}
	// Path falls back to the non-absolute walk-supplied value when
	// filepath.Abs fails — slug derivation still works correctly.
	if got[0].Slug != "user" {
		t.Errorf("slug = %q, want user", got[0].Slug)
	}
}

// TestDiscover_WalkError exercises the walk-error propagation path
// (discover.go:38-40 and :64-66). An unreadable subdirectory under
// spec/features triggers walkErr != nil.
func TestDiscover_WalkError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	sub := filepath.Join(featuresDir, "user")
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

// ---------------------------------------------------------------------------
// parse.go — extractLeadingFrontmatter unclosed branch (line 225)
// ---------------------------------------------------------------------------

func TestParse_FrontmatterUnclosed(t *testing.T) {
	// Opening "---" but no closing "---" → Frontmatter must remain nil.
	root := t.TempDir()
	path := filepath.Join(root, "unclosed.entity.md")
	body := "---\nkind: entity\nid: foo\nNO CLOSE\n\n# Entity: Foo\n"
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

// TestParse_LeadingBlankLines covers extractLeadingFrontmatter's
// blank-line skip (parse.go:208-209). A file with blank lines before
// the opening "---" must still recognize the frontmatter.
func TestParse_LeadingBlankLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "leading-blanks.entity.md")
	body := "\n  \n---\nkind: entity\nid: leading-blanks\nsingular: X\nplural: Xs\nproperties: []\n---\n\n# Entity: X\n\n## Description\n\n.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter == nil {
		t.Error("expected non-nil frontmatter when leading blanks precede ---")
	}
	if doc.Frontmatter != nil && doc.Frontmatter.Kind != "entity" {
		t.Errorf("Kind = %q, want entity", doc.Frontmatter.Kind)
	}
}

// ---------------------------------------------------------------------------
// parse.go — extras handling (line 270-273) and description (line 264-265)
// ---------------------------------------------------------------------------

func TestParse_ExtrasAndDescriptionAndInherits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "extras.entity.md")
	body := `---
kind: entity
id: extras
singular: Extras
plural: Extra
description: An entity with description and an extra key.
inherits: ./does-not-exist.entity.md
properties: []
custom_unknown_key: some-value
---

# Entity: Extras

## Description

x

## Properties

stub

## Referenced by

stub
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter == nil {
		t.Fatal("Frontmatter nil")
	}
	if doc.Frontmatter.Description != "An entity with description and an extra key." {
		t.Errorf("Description = %q", doc.Frontmatter.Description)
	}
	if doc.Frontmatter.Inherits != "./does-not-exist.entity.md" {
		t.Errorf("Inherits = %q", doc.Frontmatter.Inherits)
	}
	got, ok := doc.Frontmatter.Extras["custom_unknown_key"]
	if !ok {
		t.Errorf("expected custom_unknown_key in Extras; got %+v", doc.Frontmatter.Extras)
	}
	if s, _ := got.(string); s != "some-value" {
		t.Errorf("extras value = %v, want some-value", got)
	}
}

// ---------------------------------------------------------------------------
// parse.go — parseFrontmatter degenerate cases
// ---------------------------------------------------------------------------

// TestParseFrontmatter_EmptyContent exercises line 238-240 (node.Kind == 0).
func TestParseFrontmatter_EmptyContent(t *testing.T) {
	fm, raw := parseFrontmatter([]string{})
	if fm != nil || raw != nil {
		t.Errorf("expected (nil, nil) for empty content; got (%+v, %+v)", fm, raw)
	}
}

// TestParseFrontmatter_SequenceRoot exercises line 247-249 (non-mapping root).
func TestParseFrontmatter_SequenceRoot(t *testing.T) {
	fm, raw := parseFrontmatter([]string{"- a", "- b"})
	if fm != nil || raw != nil {
		t.Errorf("expected (nil, nil) for non-mapping root; got (%+v, %+v)", fm, raw)
	}
}

// ---------------------------------------------------------------------------
// parse.go — parsePropertiesList degenerate cases
// ---------------------------------------------------------------------------

// TestParsePropertiesList_NilNode exercises line 283-285.
func TestParsePropertiesList_NilNode(t *testing.T) {
	got := parsePropertiesList(nil)
	if got != nil {
		t.Errorf("expected nil for nil node, got %+v", got)
	}
}

// TestParsePropertiesList_ScalarNode exercises the non-sequence early
// return (line 283-285).
func TestParsePropertiesList_ScalarNode(t *testing.T) {
	got := parsePropertiesList(&yaml.Node{Kind: yaml.ScalarNode, Value: "x"})
	if got != nil {
		t.Errorf("expected nil for scalar input, got %+v", got)
	}
}

// TestParsePropertiesList_SkipsNonMappingItem exercises line 288-289.
// Mixed sequence: a scalar item between two mapping items must be silently
// skipped, returning the two mapping items.
func TestParsePropertiesList_SkipsNonMappingItem(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mixed.entity.md")
	body := `---
kind: entity
id: mixed
singular: Mixed
plural: Mixed
properties:
  - name: a
    data_type: string
  - "scalar-item-skipped"
  - name: b
    data_type: integer
    description: B
    checks:
      required: true
---

# Entity: Mixed

## Description

X

## Properties

stub

## Referenced by

stub
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Frontmatter == nil {
		t.Fatal("Frontmatter nil")
	}
	if len(doc.Properties) != 2 {
		t.Errorf("Properties len = %d, want 2 (scalar item must be skipped); got %+v",
			len(doc.Properties), doc.Properties)
	}
	// Property "b" must have its description and checks populated.
	var bItem *PropertyItem
	for i := range doc.Properties {
		if doc.Properties[i].Name == "b" {
			bItem = &doc.Properties[i]
			break
		}
	}
	if bItem == nil {
		t.Fatal("missing property b")
	}
	if bItem.Description != "B" {
		t.Errorf("b.Description = %q", bItem.Description)
	}
	if v, ok := bItem.Checks["required"].(bool); !ok || !v {
		t.Errorf("b.Checks[required] = %v, want true", bItem.Checks["required"])
	}
}

// ---------------------------------------------------------------------------
// parse.go — baseName fallthrough (line 324)
// ---------------------------------------------------------------------------

// TestBaseName_NoSeparator covers the no-separator path (line 318-324).
func TestBaseName_NoSeparator(t *testing.T) {
	got := baseName("just-a-name.entity.md")
	if got != "just-a-name.entity.md" {
		t.Errorf("baseName = %q, want unchanged", got)
	}
}

// ---------------------------------------------------------------------------
// parse.go:109 — scanner error path
// ---------------------------------------------------------------------------
//
// Trigger bufio.Scanner.Err() by writing a line longer than the buffer's
// max size (1 MiB). The parser sets MaxScanTokenSize indirectly via
// scanner.Buffer; an oversize line yields bufio.ErrTooLong.
func TestParse_ScannerError_TooLongLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "huge.entity.md")
	// 2 MiB line forces bufio.ErrTooLong.
	huge := strings.Repeat("a", 2*1024*1024)
	if err := os.WriteFile(path, []byte("---\nkind: entity\nid: huge\n"+huge+"\n---\n# Entity: Huge\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Parse(path)
	if err == nil {
		t.Errorf("expected scanner-error from oversize line, got nil")
	}
}

// ---------------------------------------------------------------------------
// parse.go:132 — frontmatterHasKey integration via Parse seems covered
// elsewhere; here ensure parseFrontmatter handles the "description" key
// alone (covered by extras test above).
// ---------------------------------------------------------------------------
