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

func TestRoundTripWithProjectAndStudio(t *testing.T) {
	dir := t.TempDir()
	cfg := SpecConfig{
		Project: &ProjectConfig{
			Title: "Test Project",
			Repositories: []RepositoryConfig{
				{URL: "https://github.com/test/code.git", Roles: []Role{RoleCode}},
			},
		},
		Studio: &StudioConfig{Name: "AcmeDocs", URL: "https://docs.acme.example/"},
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
	if got.Studio == nil || got.Studio.Name != "AcmeDocs" || got.Studio.URL != "https://docs.acme.example/" {
		t.Errorf("Studio round-trip failed: %+v", got.Studio)
	}
	if len(got.Project.Repositories) != 1 ||
		got.Project.Repositories[0].URL != "https://github.com/test/code.git" ||
		len(got.Project.Repositories[0].Roles) != 1 ||
		got.Project.Repositories[0].Roles[0] != RoleCode {
		t.Errorf("Repositories round-trip failed: %+v", got.Project.Repositories)
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
	if name, url, suppressed := cfg.EffectiveStudio(); suppressed || name != DefaultStudioName || url != DefaultStudioURL {
		t.Errorf("expected SpecScore.Studio defaults; got name=%q url=%q suppressed=%v", name, url, suppressed)
	}
	// sanity-check the literal default values per the studio-toolbar Feature
	if DefaultStudioName != "SpecScore.Studio" || DefaultStudioURL != "https://specscore.studio/" {
		t.Errorf("default studio constants drifted: name=%q url=%q", DefaultStudioName, DefaultStudioURL)
	}
	if cfg.EffectiveSpecsDirName() != DefaultSpecsDirName || cfg.EffectiveDocsDirName() != DefaultDocsDirName {
		t.Errorf("dir-name defaults wrong: specs=%q docs=%q", cfg.EffectiveSpecsDirName(), cfg.EffectiveDocsDirName())
	}
	mods := cfg.EffectiveModules()
	if len(mods) != 1 || mods[0].EffectivePath() != "." || mods[0].EffectiveName() != DefaultModuleName {
		t.Errorf("default module wrong: %+v", mods)
	}
}

func TestStudioExplicitNullSuppresses(t *testing.T) {
	dir := t.TempDir()
	writeRaw(t, dir, SchemaHeader+"\nstudio: null\n")
	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if !cfg.IsStudioSuppressed() {
		t.Fatal("expected studio: null to be detected as suppressed")
	}
	name, url, suppressed := cfg.EffectiveStudio()
	if !suppressed || name != "" || url != "" {
		t.Errorf("EffectiveStudio should report suppressed; got name=%q url=%q suppressed=%v", name, url, suppressed)
	}
}

func TestStudioTildeAndEmptyAreNull(t *testing.T) {
	for _, body := range []string{
		SchemaHeader + "\nstudio: ~\n",
		SchemaHeader + "\nstudio:\n",
	} {
		dir := t.TempDir()
		writeRaw(t, dir, body)
		cfg, err := ReadSpecConfig(dir)
		if err != nil {
			t.Fatalf("ReadSpecConfig(%q): %v", body, err)
		}
		if !cfg.IsStudioSuppressed() {
			t.Errorf("body %q: expected studio suppressed", body)
		}
	}
}

func TestStudioPartialMappingFailsValidation(t *testing.T) {
	// Use a URL that has the proper trailing-slash form so the failure
	// path is purely partial-mapping, not trailing-slash.
	cfg := SpecConfig{Studio: &StudioConfig{Name: "AcmeDocs"}} // url missing
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for studio with name but no url")
	}
	cfg = SpecConfig{Studio: &StudioConfig{URL: "https://x.example/"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for studio with url but no name")
	}
}

// TestViewerBlockRejected covers repo-config#req:viewer-block-rejected — any
// form of the legacy `viewer:` block (mapping, null, tilde, bare) MUST be
// rejected at parse time with a migration message.
func TestViewerBlockRejected(t *testing.T) {
	bodies := []string{
		SchemaHeader + "\nviewer:\n  name: X\n  url: https://x.example/\n",
		SchemaHeader + "\nviewer: null\n",
		SchemaHeader + "\nviewer:\n",
		SchemaHeader + "\nviewer: ~\n",
	}
	for _, body := range bodies {
		dir := t.TempDir()
		writeRaw(t, dir, body)
		_, err := ReadSpecConfig(dir)
		if err == nil {
			t.Errorf("expected viewer: rejection for body %q, got nil", body)
			continue
		}
		if !strings.Contains(err.Error(), "viewer: block is no longer supported") {
			t.Errorf("expected migration error for body %q; got %v", body, err)
		}
	}
}

// TestStudioURLMustHaveTrailingSlash covers
// repo-config#req:studio-url-trailing-slash.
func TestStudioURLMustHaveTrailingSlash(t *testing.T) {
	cfg := SpecConfig{Studio: &StudioConfig{Name: "X", URL: "https://no.slash.example"}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected trailing-slash error")
	}
	if !strings.Contains(err.Error(), "studio.url must end with exactly one '/'") {
		t.Errorf("expected trailing-slash citation; got %v", err)
	}
	// Two trailing slashes also rejected.
	cfg = SpecConfig{Studio: &StudioConfig{Name: "X", URL: "https://x.example//"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for double-trailing-slash")
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

// TestRepositoriesRoundTripMultiRole exercises AC: repositories-role-tagged.
// Mixed shapes do NOT exist — every entry MUST be a role-tagged object.
// The fixture covers: optional title/comment, multi-valued roles, and
// the implicit single-role shape.
func TestRepositoriesRoundTripMultiRole(t *testing.T) {
	dir := t.TempDir()
	body := SchemaHeader + `
project:
  title: T
  repositories:
    - url: https://github.com/acme/api
      roles: [code]
    - url: https://github.com/acme/spec
      title: Spec Repo
      comment: SpecScore-managed spec for the project
      roles: [specification, code]
    - url: https://github.com/acme/state
      roles: [state]
      tracker: jira-123   # unknown field — must round-trip
`
	writeRaw(t, dir, body)

	cfg, err := ReadSpecConfig(dir)
	if err != nil {
		t.Fatalf("ReadSpecConfig: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	repos := cfg.Project.Repositories
	if len(repos) != 3 {
		t.Fatalf("expected 3 repositories, got %d", len(repos))
	}
	if repos[1].Title != "Spec Repo" || repos[1].Comment == "" {
		t.Errorf("title/comment lost on entry[1]: %+v", repos[1])
	}
	if len(repos[1].Roles) != 2 || repos[1].Roles[0] != RoleSpecification || repos[1].Roles[1] != RoleCode {
		t.Errorf("multi-role list mangled on entry[1]: %v", repos[1].Roles)
	}
	// Unknown field round-trips — write back and read again, then assert
	// the on-disk text still contains the unknown field.
	if err := WriteSpecConfig(dir, cfg); err != nil {
		t.Fatalf("WriteSpecConfig: %v", err)
	}
	written, err := os.ReadFile(filepath.Join(dir, SpecConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "tracker: jira-123") {
		t.Errorf("unknown field `tracker` lost on round-trip; written=%q", written)
	}
}

// TestRepositoriesShapeErrors exercises AC: repositories-shape-errors.
// Every malformed shape produces a hard error from Validate() that names
// both the offending entry index and the violated REQ.
func TestRepositoriesShapeErrors(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantREQ  string
		wantText string // substring expected in the error
	}{
		{
			name: "flat-string entry rejected at decode time",
			body: `project:
  repositories:
    - https://example.com/repo
`,
			wantREQ:  "repo-config#req:repositories-entry-shape",
			wantText: "flat-string entries are not accepted",
		},
		{
			name: "missing url",
			body: `project:
  repositories:
    - roles: [code]
`,
			wantREQ:  "repo-config#req:repositories-entry-shape",
			wantText: "missing `url`",
		},
		{
			name: "missing roles field",
			body: `project:
  repositories:
    - url: https://example.com/repo
`,
			wantREQ:  "repo-config#req:repositories-roles-list",
			wantText: "must be a non-empty list",
		},
		{
			name: "empty roles list",
			body: `project:
  repositories:
    - url: https://example.com/repo
      roles: []
`,
			wantREQ:  "repo-config#req:repositories-roles-list",
			wantText: "must be a non-empty list",
		},
		{
			name: "scalar roles instead of list",
			body: `project:
  repositories:
    - url: https://example.com/repo
      roles: code
`,
			// yaml.v3 raises a type-mismatch decode error on scalar→list;
			// surfaced via UnmarshalYAML wrapper.
			wantREQ:  "decoding project.repositories entry",
			wantText: "decoding project.repositories entry",
		},
		{
			name: "unknown role value",
			body: `project:
  repositories:
    - url: https://example.com/repo
      roles: [helm-chart]
`,
			wantREQ:  "repo-config#req:repositories-roles-enum",
			wantText: `unknown role "helm-chart"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeRaw(t, dir, SchemaHeader+"\n"+tc.body)
			cfg, readErr := ReadSpecConfig(dir)
			// Decode-time errors (UnmarshalYAML) surface from ReadSpecConfig;
			// validation errors surface from Validate().
			err := readErr
			if err == nil {
				err = cfg.Validate()
			}
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error text missing %q; got: %v", tc.wantText, err)
			}
			if !strings.Contains(err.Error(), tc.wantREQ) {
				t.Errorf("error missing REQ reference %q; got: %v", tc.wantREQ, err)
			}
		})
	}
}

// TestIsValidRole asserts the closed enum membership.
func TestIsValidRole(t *testing.T) {
	for _, r := range []Role{RoleCode, RoleSpecification, RoleState, RoleDocs, RoleRunner} {
		if !IsValidRole(r) {
			t.Errorf("%q should be a valid role", r)
		}
	}
	for _, r := range []Role{"", "code-repo", "Code", "CODE", "unknown"} {
		if IsValidRole(r) {
			t.Errorf("%q should NOT be a valid role", r)
		}
	}
}
