package projectdef

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRaw writes a literal specscore.yaml in dir; used to exercise the
// schema-header validator and viewer-null detection without going through
// WriteSpecConfig (which always emits a valid header).
func writeRaw(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, SpecConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTripWithProjectAndViewer(t *testing.T) {
	dir := t.TempDir()
	cfg := SpecConfig{
		Project: &ProjectConfig{
			Title:        "Test Project",
			Repositories: []string{"https://github.com/test/code.git"},
		},
		Viewer: &ViewerConfig{Name: "AcmeDocs", URL: "https://docs.acme.example/"},
	}
	if err := WriteSpecConfig(dir, cfg); err != nil {
		t.Fatalf("WriteSpecConfig: %v", err)
	}
	got, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if got.Project == nil || got.Project.Title != "Test Project" {
		t.Errorf("Project.Title round-trip failed: %+v", got.Project)
	}
	if got.Viewer == nil || got.Viewer.Name != "AcmeDocs" || got.Viewer.URL != "https://docs.acme.example/" {
		t.Errorf("Viewer round-trip failed: %+v", got.Viewer)
	}
}

func TestSchemaHeaderRequired(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, "project:\n  title: Bad\n")
	_, err := ReadSpecConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "schema header") {
		t.Fatalf("expected schema-header error, got %v", err)
	}
}

func TestSchemaHeaderOnLine1Only(t *testing.T) {
	dir := t.TempDir()
	// Header on line 2 (preceded by blank line) is invalid.
	body := "\n" + SchemaHeader + "\nproject:\n  title: T\n"
	writeRaw(t, dir, body)
	_, err := ReadSpecConfig(dir)
	if err == nil {
		t.Fatal("expected schema-header error for line-2 placement")
	}
}

func TestEmptyConfigWithHeaderValid(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, SchemaHeader+"\n")
	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("expected empty header-only config to be valid, got %v", err)
	}
	if name, url, suppressed := cfg.EffectiveViewer(); suppressed || name != DefaultViewerName || url != DefaultViewerURL {
		t.Errorf("expected SpecStudio defaults; got name=%q url=%q suppressed=%v", name, url, suppressed)
	}
	if cfg.EffectiveSpecsDirName() != DefaultSpecsDirName || cfg.EffectiveDocsDirName() != DefaultDocsDirName {
		t.Errorf("dir-name defaults wrong: specs=%q docs=%q", cfg.EffectiveSpecsDirName(), cfg.EffectiveDocsDirName())
	}
	mods := cfg.EffectiveModules()
	if len(mods) != 1 || mods[0].EffectivePath() != "." || mods[0].EffectiveName() != DefaultModuleName {
		t.Errorf("default module wrong: %+v", mods)
	}
}

func TestViewerExplicitNullSuppresses(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, SchemaHeader+"\nviewer: null\n")
	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if !cfg.IsViewerSuppressed() {
		t.Fatal("expected viewer: null to be detected as suppressed")
	}
	name, url, suppressed := cfg.EffectiveViewer()
	if !suppressed || name != "" || url != "" {
		t.Errorf("EffectiveViewer should report suppressed; got name=%q url=%q suppressed=%v", name, url, suppressed)
	}
}

func TestViewerTildeAndEmptyAreNull(t *testing.T) {
	for _, body := range []string{
		SchemaHeader + "\nviewer: ~\n",
		SchemaHeader + "\nviewer:\n",
	} {
		dir := t.TempDir()
		writeRaw(t, dir, body)
		cfg, err := ReadSpecConfig(dir)
		if err != nil {
			t.Fatalf("ReadSpecConfig(%q): %v", body, err)
		}
		if !cfg.IsViewerSuppressed() {
			t.Errorf("body %q: expected viewer suppressed", body)
		}
	}
}

func TestViewerPartialMappingFailsValidation(t *testing.T) {
	cfg := SpecConfig{Viewer: &ViewerConfig{Name: "AcmeDocs"}} // url missing
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for viewer with name but no url")
	}
	cfg = SpecConfig{Viewer: &ViewerConfig{URL: "https://x/"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for viewer with url but no name")
	}
}

func TestModulesDefaultBehavior(t *testing.T) {
	cfg := SpecConfig{}
	mods := cfg.EffectiveModules()
	if len(mods) != 1 {
		t.Fatalf("expected 1 implicit module, got %d", len(mods))
	}
	if mods[0].EffectivePath() != "." || mods[0].EffectiveName() != DefaultModuleName {
		t.Errorf("implicit module wrong: path=%q name=%q", mods[0].EffectivePath(), mods[0].EffectiveName())
	}
}

func TestModuleNameDeducedFromPath(t *testing.T) {
	m := ModuleConfig{Path: "services/backend"}
	if got := m.EffectiveName(); got != "backend" {
		t.Errorf("expected backend, got %q", got)
	}
	m = ModuleConfig{Name: "Highlevel", Path: "services/backend"}
	if got := m.EffectiveName(); got != "Highlevel" {
		t.Errorf("explicit name should win, got %q", got)
	}
}

func TestModulePathsDuplicateRejected(t *testing.T) {
	cfg := SpecConfig{Modules: []ModuleConfig{
		{Name: "A", Path: "services"},
		{Name: "B", Path: "services"},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate path error")
	}
}

func TestModulePathsNoPathDuplicateRejected(t *testing.T) {
	cfg := SpecConfig{Modules: []ModuleConfig{{Name: "A"}, {Name: "B"}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate path error for two implicit-root modules")
	}
}

func TestModulePathsNestedRejected(t *testing.T) {
	cfg := SpecConfig{Modules: []ModuleConfig{
		{Name: "Backend", Path: "backend"},
		{Name: "BackendAPI", Path: "backend/api"},
	}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected nested-path error")
	}
}

func TestModuleRootCoexistsWithExplicitPaths(t *testing.T) {
	cfg := SpecConfig{Modules: []ModuleConfig{
		{Name: "Highlevel"},
		{Name: "Backend", Path: "backend"},
		{Path: "frontend"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Errorf("implicit-root module should coexist; got %v", err)
	}
}

func TestUnknownFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	body := SchemaHeader + `
project:
  title: T
  custom_field: project-extra
state_repo: https://github.com/org/state
modules:
  - name: M
    path: m
    custom_module_field: module-extra
`
	writeRaw(t, dir, body)
	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if v, ok := cfg.Extras["state_repo"]; !ok || v != "https://github.com/org/state" {
		t.Errorf("root extra not preserved: %v", cfg.Extras)
	}
	if cfg.Project == nil || cfg.Project.Extras["custom_field"] != "project-extra" {
		t.Errorf("project extra not preserved: %+v", cfg.Project)
	}
	if len(cfg.Modules) != 1 || cfg.Modules[0].Extras["custom_module_field"] != "module-extra" {
		t.Errorf("module extra not preserved: %+v", cfg.Modules)
	}
}

func TestSpecConfigFileWritten(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSpecConfig(dir, SpecConfig{Project: &ProjectConfig{Title: "t"}}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, SpecConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.HasPrefix(string(data), SchemaHeader+"\n") {
		t.Errorf("written file does not begin with schema header; got %q", string(data))
	}
}
