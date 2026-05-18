package entity

import (
	"path/filepath"
	"strings"
	"testing"
)

// testdataPath returns the absolute path to a fixture under _testdata/.
func testdataPath(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("_testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		slug    string
		wantErr bool
	}{
		{"user", false},
		{"line-item", false},
		{"iso-currency-code", false},
		{"a", false},
		{"a1-b2", false},
		{"", true},
		{"User", true},
		{"user_id", true},
		{"-user", true},
		{"user-", true},
		{"user--id", true},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			err := ValidateSlug(tc.slug)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSlug(%q) err=%v, wantErr=%v", tc.slug, err, tc.wantErr)
			}
		})
	}
}

func TestParse_ValidMinimal(t *testing.T) {
	doc, err := Parse(testdataPath(t, "valid-minimal.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil Doc")
		return
	}
	if doc.Slug != "valid-minimal" {
		t.Errorf("Slug = %q, want valid-minimal", doc.Slug)
	}
	if !doc.HasTitle {
		t.Error("HasTitle = false, want true")
	}
	if doc.TitleName != "ValidMinimal" {
		t.Errorf("TitleName = %q, want ValidMinimal", doc.TitleName)
	}
	if doc.Title != "# Entity: ValidMinimal" {
		t.Errorf("Title = %q, want '# Entity: ValidMinimal'", doc.Title)
	}
	if doc.Frontmatter == nil {
		t.Fatal("Frontmatter should not be nil for a well-formed file")
	}
	if doc.Frontmatter.Kind != "entity" {
		t.Errorf("Frontmatter.Kind = %q, want entity", doc.Frontmatter.Kind)
	}
	if doc.Frontmatter.ID != "valid-minimal" {
		t.Errorf("Frontmatter.ID = %q, want valid-minimal", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Singular != "ValidMinimal" {
		t.Errorf("Frontmatter.Singular = %q, want ValidMinimal", doc.Frontmatter.Singular)
	}
	if doc.Frontmatter.Plural != "ValidMinimals" {
		t.Errorf("Frontmatter.Plural = %q, want ValidMinimals", doc.Frontmatter.Plural)
	}
	if len(doc.Properties) != 0 {
		t.Errorf("Properties = %d items, want 0", len(doc.Properties))
	}
	if doc.FmRaw == nil {
		t.Error("FmRaw should be non-nil so the --fix rewriter can round-trip the frontmatter")
	}
	if len(doc.RawLines) == 0 {
		t.Error("RawLines should be non-empty")
	}
	if _, ok := doc.SectionByTitle["Description"]; !ok {
		t.Error("expected ## Description in SectionByTitle")
	}
	if _, ok := doc.SectionByTitle["Properties"]; !ok {
		t.Error("expected ## Properties in SectionByTitle")
	}
	if _, ok := doc.SectionByTitle["Referenced by"]; !ok {
		t.Error("expected ## Referenced by in SectionByTitle")
	}
}

func TestParse_ValidWithInherits(t *testing.T) {
	doc, err := Parse(testdataPath(t, "valid-with-inherits.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Frontmatter == nil {
		t.Fatal("expected non-nil Frontmatter")
	}
	if doc.Frontmatter.Inherits != "./valid-minimal.entity.md" {
		t.Errorf("Inherits = %q, want ./valid-minimal.entity.md", doc.Frontmatter.Inherits)
	}
	if len(doc.Properties) != 1 {
		t.Fatalf("Properties = %d, want 1", len(doc.Properties))
	}
	if doc.Properties[0].Name != "child_field" {
		t.Errorf("Properties[0].Name = %q, want child_field", doc.Properties[0].Name)
	}
	if doc.Properties[0].DataType != "string" {
		t.Errorf("Properties[0].DataType = %q, want string", doc.Properties[0].DataType)
	}
}

func TestParse_ValidWithRefProperty(t *testing.T) {
	doc, err := Parse(testdataPath(t, "valid-with-ref-property.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Properties) != 2 {
		t.Fatalf("Properties = %d, want 2", len(doc.Properties))
	}
	if doc.Properties[0].Name != "id" || doc.Properties[0].DataType != "string" {
		t.Errorf("Properties[0] = %+v, want {Name:id, DataType:string}", doc.Properties[0])
	}
	if doc.Properties[1].Name != "email" {
		t.Errorf("Properties[1].Name = %q, want email", doc.Properties[1].Name)
	}
	if doc.Properties[1].Ref != "./email.property.md" {
		t.Errorf("Properties[1].Ref = %q, want ./email.property.md", doc.Properties[1].Ref)
	}
	if doc.Properties[1].DataType != "" {
		t.Errorf("Properties[1].DataType = %q, want empty for ref-form items", doc.Properties[1].DataType)
	}
}

func TestParse_MissingFrontmatter(t *testing.T) {
	doc, err := Parse(testdataPath(t, "missing-frontmatter.entity.md"))
	// Parse is resilient — returns a partial Doc even when frontmatter
	// is absent. Lint surfaces the absence.
	if err != nil {
		t.Fatalf("Parse should be resilient, got err: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil Doc even when frontmatter is missing")
		return
	}
	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter should be nil when absent, got %+v", doc.Frontmatter)
	}
	if doc.Slug != "missing-frontmatter" {
		t.Errorf("Slug = %q, want missing-frontmatter (derived from filename)", doc.Slug)
	}
	if !doc.HasTitle {
		t.Error("Title is present even though frontmatter is missing; HasTitle should be true")
	}
}

func TestParse_FrontmatterMissingRequiredFields(t *testing.T) {
	doc, err := Parse(testdataPath(t, "frontmatter-missing-required-fields.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.Frontmatter == nil {
		t.Fatal("expected non-nil Frontmatter — YAML parsed cleanly, only the id field is missing")
	}
	if doc.Frontmatter.ID != "" {
		t.Errorf("Frontmatter.ID = %q, want empty (the fixture omits id)", doc.Frontmatter.ID)
	}
	if doc.Frontmatter.Kind != "entity" {
		t.Errorf("Frontmatter.Kind = %q, want entity", doc.Frontmatter.Kind)
	}
}

func TestParse_DuplicatePropertyName(t *testing.T) {
	// Parse must not deduplicate — every property item from the
	// frontmatter list MUST appear in doc.Properties so lint can
	// inspect them.
	doc, err := Parse(testdataPath(t, "duplicate-property-name.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Properties) != 2 {
		t.Fatalf("Properties = %d, want 2 (parser does not dedupe)", len(doc.Properties))
	}
	if doc.Properties[0].Name != "email" || doc.Properties[1].Name != "email" {
		t.Errorf("expected two items named 'email', got %+v", doc.Properties)
	}
}

func TestParse_IDMismatchSlug(t *testing.T) {
	doc, err := Parse(testdataPath(t, "id-mismatch-slug.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Parse surfaces both values — lint compares.
	if doc.Slug != "id-mismatch-slug" {
		t.Errorf("Slug = %q, want id-mismatch-slug", doc.Slug)
	}
	if doc.Frontmatter == nil || doc.Frontmatter.ID != "not-the-slug" {
		t.Errorf("Frontmatter.ID = %q, want not-the-slug", doc.Frontmatter.ID)
	}
}

func TestParse_TitleMismatchSingular(t *testing.T) {
	doc, err := Parse(testdataPath(t, "title-mismatch-singular.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if doc.TitleName != "User" {
		t.Errorf("TitleName = %q, want User", doc.TitleName)
	}
	if doc.Frontmatter == nil || doc.Frontmatter.Singular != "Person" {
		t.Errorf("Frontmatter.Singular = %q, want Person", doc.Frontmatter.Singular)
	}
}

func TestParse_MalformedYAML(t *testing.T) {
	doc, err := Parse(testdataPath(t, "malformed-yaml.entity.md"))
	if err != nil {
		t.Fatalf("Parse should be resilient on malformed YAML, got err: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil Doc even when YAML is malformed")
		return
	}
	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter should be nil when YAML fails to parse, got %+v", doc.Frontmatter)
	}
	// Slug must still be derivable from the filename.
	if doc.Slug != "malformed-yaml" {
		t.Errorf("Slug = %q, want malformed-yaml", doc.Slug)
	}
	// Body sections must still be discovered.
	if _, ok := doc.SectionByTitle["Properties"]; !ok {
		t.Error("expected ## Properties to be parsed even when frontmatter is malformed")
	}
}

func TestParse_FrontmatterNotFirstBlock(t *testing.T) {
	doc, err := Parse(testdataPath(t, "frontmatter-not-first-block.entity.md"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// When the leading content is not the frontmatter, Parse MUST NOT
	// silently pick up the later --- block: doing so would mask the
	// REQ:frontmatter-required violation. Frontmatter should be nil.
	if doc.Frontmatter != nil {
		t.Errorf("Frontmatter should be nil when not at the leading position, got %+v", doc.Frontmatter)
	}
	// The title is still parsed.
	if doc.TitleName != "Wrong Order" {
		t.Errorf("TitleName = %q, want 'Wrong Order'", doc.TitleName)
	}
}

func TestParse_RawLinesPreserved(t *testing.T) {
	doc, err := Parse(testdataPath(t, "valid-minimal.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.RawLines) < 10 {
		t.Fatalf("RawLines too short (%d) — byte-for-byte body preservation is required for managed-section rewriting", len(doc.RawLines))
	}
	// The "# Entity: ValidMinimal" line should appear verbatim.
	found := false
	for _, ln := range doc.RawLines {
		if strings.TrimSpace(ln) == "# Entity: ValidMinimal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("RawLines should contain the title line verbatim")
	}
}

func TestParse_FileNotFound(t *testing.T) {
	_, err := Parse(filepath.Join(t.TempDir(), "does-not-exist.entity.md"))
	if err == nil {
		t.Fatal("expected error when reading a nonexistent file")
	}
}

func TestParse_FmRawRoundTripsForIDRewrite(t *testing.T) {
	// The id-equals-slug autofix needs FmRaw to be a yaml.Node so it can
	// rewrite the id field without losing comments or key order. Verify
	// FmRaw is populated for the malformed-id fixture (frontmatter
	// parses cleanly even though id is wrong).
	doc, err := Parse(testdataPath(t, "id-mismatch-slug.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.FmRaw == nil {
		t.Fatal("FmRaw should be non-nil for a YAML-parseable frontmatter")
	}
	// A mapping node has at least kind == yaml.MappingNode.
	if doc.FmRaw.Kind == 0 {
		t.Errorf("FmRaw.Kind = 0 — expected populated yaml.Node")
	}
}
