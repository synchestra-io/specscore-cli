package projectdef

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// EffectiveSpecsDirName / EffectiveDocsDirName — custom values
// ---------------------------------------------------------------------------

func TestEffectiveSpecsDirName_Custom(t *testing.T) {
	cfg := SpecConfig{SpecsDirName: "custom-specs"}
	if got := cfg.EffectiveSpecsDirName(); got != "custom-specs" {
		t.Errorf("got %q, want %q", got, "custom-specs")
	}
}

func TestEffectiveDocsDirName_Custom(t *testing.T) {
	cfg := SpecConfig{DocsDirName: "my-docs"}
	if got := cfg.EffectiveDocsDirName(); got != "my-docs" {
		t.Errorf("got %q, want %q", got, "my-docs")
	}
}

// ---------------------------------------------------------------------------
// EffectiveStudio — non-nil Studio block
// ---------------------------------------------------------------------------

func TestEffectiveStudio_ExplicitValues(t *testing.T) {
	cfg := SpecConfig{Studio: &StudioConfig{Name: "MyStudio", URL: "https://my.studio/"}}
	name, url, suppressed := cfg.EffectiveStudio()
	if suppressed {
		t.Error("should not be suppressed")
	}
	if name != "MyStudio" || url != "https://my.studio/" {
		t.Errorf("got name=%q url=%q", name, url)
	}
}

// ---------------------------------------------------------------------------
// EffectiveModules — non-empty modules
// ---------------------------------------------------------------------------

func TestEffectiveModules_NonEmpty(t *testing.T) {
	cfg := SpecConfig{Modules: []ModuleConfig{{Name: "X", Path: "x"}}}
	mods := cfg.EffectiveModules()
	if len(mods) != 1 || mods[0].Name != "X" {
		t.Errorf("got %+v", mods)
	}
}

// ---------------------------------------------------------------------------
// ValidateSchemaHeader — data with no newline
// ---------------------------------------------------------------------------

func TestValidateSchemaHeader_NoNewline(t *testing.T) {
	// File with just the header and no trailing newline
	data := []byte(SchemaHeader)
	if err := ValidateSchemaHeader(data); err != nil {
		t.Errorf("expected valid header without newline; got %v", err)
	}
}

func TestValidateSchemaHeader_CRLFLineEnding(t *testing.T) {
	data := []byte(SchemaHeader + "\r\n")
	if err := ValidateSchemaHeader(data); err != nil {
		t.Errorf("expected valid header with CRLF; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReadSpecConfig — file not found
// ---------------------------------------------------------------------------

func TestReadSpecConfig_FileNotFound(t *testing.T) {
	_, err := ReadSpecConfig(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading spec config") {
		t.Errorf("expected wrapped read error; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReadSpecConfig — invalid YAML
// ---------------------------------------------------------------------------

func TestReadSpecConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	body := SchemaHeader + "\n\n[invalid: yaml: {{\n"
	writeRaw(t, dir, body)
	_, err := ReadSpecConfig(dir)
	if err == nil {
		t.Fatal("expected parsing error")
	}
	if !strings.Contains(err.Error(), "parsing spec config") {
		t.Errorf("expected parsing error; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// WriteSpecConfig — non-writable directory
// ---------------------------------------------------------------------------

func TestWriteSpecConfig_NonWritableDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := WriteSpecConfig(dir, SpecConfig{})
	if err == nil {
		t.Fatal("expected write error for read-only directory")
	}
	if !strings.Contains(err.Error(), "writing spec config") {
		t.Errorf("expected wrapped write error; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// isAncestorPath — additional cases
// ---------------------------------------------------------------------------

func TestIsAncestorPath_SamePath(t *testing.T) {
	if isAncestorPath("a/b", "a/b") {
		t.Error("same path should not be ancestor")
	}
}

func TestIsAncestorPath_NotAncestor(t *testing.T) {
	if isAncestorPath("abc", "abcdef/child") {
		t.Error("abc is not an ancestor of abcdef/child")
	}
}

// ---------------------------------------------------------------------------
// detectStudioExplicitNull — non-document YAML
// ---------------------------------------------------------------------------

func TestDetectStudioExplicitNull_InvalidYAML(t *testing.T) {
	if detectStudioExplicitNull([]byte("[invalid:yaml")) {
		t.Error("invalid YAML should return false")
	}
}

func TestDetectStudioExplicitNull_NonMappingRoot(t *testing.T) {
	// A YAML sequence as root
	if detectStudioExplicitNull([]byte("- item\n")) {
		t.Error("sequence root should return false")
	}
}

func TestDetectStudioExplicitNull_StudioIsMapping(t *testing.T) {
	// studio: as a mapping (not null)
	data := []byte("studio:\n  name: X\n  url: https://x.example/\n")
	if detectStudioExplicitNull(data) {
		t.Error("studio mapping should not be detected as null")
	}
}

// ---------------------------------------------------------------------------
// detectViewerKey — non-document YAML and edge cases
// ---------------------------------------------------------------------------

func TestDetectViewerKey_InvalidYAML(t *testing.T) {
	if detectViewerKey([]byte("[invalid:yaml")) {
		t.Error("invalid YAML should return false")
	}
}

func TestDetectViewerKey_NonMappingRoot(t *testing.T) {
	if detectViewerKey([]byte("- item\n")) {
		t.Error("sequence root should return false")
	}
}

func TestDetectViewerKey_NoViewerKey(t *testing.T) {
	if detectViewerKey([]byte("project:\n  title: T\n")) {
		t.Error("no viewer key should return false")
	}
}

// ---------------------------------------------------------------------------
// detectStudioExplicitNull with various null forms
// ---------------------------------------------------------------------------

func TestDetectStudioExplicitNull_AllNullForms(t *testing.T) {
	nullForms := []string{
		"studio: null\n",
		"studio: Null\n",
		"studio: NULL\n",
		"studio: ~\n",
		"studio:\n",
	}
	for _, form := range nullForms {
		if !detectStudioExplicitNull([]byte(form)) {
			t.Errorf("expected true for %q", form)
		}
	}
}

// ---------------------------------------------------------------------------
// ReadSpecConfig — studio omitted (not null)
// ---------------------------------------------------------------------------

func TestReadSpecConfig_StudioOmittedNotNull(t *testing.T) {
	dir := t.TempDir()
	body := SchemaHeader + "\nproject:\n  title: T\n"
	writeRaw(t, dir, body)
	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if cfg.IsStudioSuppressed() {
		t.Error("studio omitted should not be suppressed")
	}
}

// ---------------------------------------------------------------------------
// WriteSpecConfig — marshal error (unlikely but covers the branch)
// ---------------------------------------------------------------------------

// TestWriteSpecConfig_MarshalSuccess exercises WriteSpecConfig with a
// custom struct that exercises the marshal path comprehensively.
func TestWriteSpecConfig_WithModules(t *testing.T) {
	dir := t.TempDir()
	cfg := SpecConfig{
		SpecsDirName: "s",
		DocsDirName:  "d",
		Modules: []ModuleConfig{
			{Name: "M", Path: "m"},
		},
	}
	if err := WriteSpecConfig(dir, cfg); err != nil {
		t.Fatalf("WriteSpecConfig: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, SpecConfigFile))
	if !strings.Contains(string(data), "specs_dir_name: s") {
		t.Error("specs_dir_name not in output")
	}
}
