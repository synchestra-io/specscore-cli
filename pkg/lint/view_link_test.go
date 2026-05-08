package lint

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synchestra-io/specscore-cli/pkg/gitremote"
	"github.com/synchestra-io/specscore-cli/pkg/projectdef"
)

const (
	testViewerName = "SpecStudio"
	testViewerURL  = "https://specstudio.synchestra.io"
	testOwner      = "synchestra-io"
	testRepo       = "specscore"
)

// setupViewerProject creates a minimal project root with:
//   - specscore.yaml containing an explicit `viewer:` block
//   - a git repo whose origin points at a GitHub URL
//   - spec/features/ scaffolding
//
// Returns the project root path; callers should use filepath.Join(root, "spec") as specRoot.
func setupViewerProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in test environment")
	}
	root := t.TempDir()

	if err := projectdef.WriteSpecConfig(root, projectdef.SpecConfig{
		Project: &projectdef.ProjectConfig{Title: "Test"},
		Viewer:  &projectdef.ViewerConfig{Name: testViewerName, URL: testViewerURL},
	}); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "git@github.com:"+testOwner+"/"+testRepo+".git")

	if err := os.MkdirAll(filepath.Join(root, "spec", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// setupDefaultViewerProject writes a header-only specscore.yaml — the
// viewer block is omitted, so SpecStudio defaults apply.
func setupDefaultViewerProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in test environment")
	}
	root := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(root, projectdef.SpecConfigFile),
		[]byte(projectdef.SchemaHeader+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "git@github.com:"+testOwner+"/"+testRepo+".git")
	if err := os.MkdirAll(filepath.Join(root, "spec", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// setupSuppressedViewerProject writes a specscore.yaml with `viewer: null`
// — the opt-out form. The view-link rule MUST be a no-op for this repo.
func setupSuppressedViewerProject(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in test environment")
	}
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, projectdef.SpecConfigFile),
		[]byte(projectdef.SchemaHeader+"\nviewer: null\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "git@github.com:"+testOwner+"/"+testRepo+".git")
	if err := os.MkdirAll(filepath.Join(root, "spec", "features"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeViewFeatureReadme(t *testing.T, root, slug, content string) string {
	t.Helper()
	dir := filepath.Join(root, "spec", "features", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func expectedBlockquoteFor(viewerName, viewerURL, slug string) string {
	r := gitremote.Remote{Owner: testOwner, Repo: testRepo, Host: "github.com"}
	u := BuildViewURL(viewerURL, r, "spec/features/"+slug)
	return viewLinkMarker(viewerName) + u + viewLinkSuffix
}

func expectedBlockquote(slug string) string {
	return expectedBlockquoteFor(testViewerName, testViewerURL, slug)
}

// legacySpecStudioBlockquote builds a "Spec Studio" (with space) era
// blockquote, used to seed pre-rename READMEs in migration tests.
func legacySpecStudioBlockquote(slug string) string {
	r := gitremote.Remote{Owner: testOwner, Repo: testRepo, Host: "github.com"}
	u := BuildViewURL(testViewerURL, r, "spec/features/"+slug)
	return "> [View in Spec Studio](" + u + viewLinkSuffix
}

// legacyHubBlockquote builds a Synchestra-Hub-era blockquote pointing at
// the old hub host, used to seed pre-migration READMEs in tests.
func legacyHubBlockquote(slug string) string {
	r := gitremote.Remote{Owner: testOwner, Repo: testRepo, Host: "github.com"}
	u := BuildViewURL("https://hub.synchestra.io", r, "spec/features/"+slug)
	return "> [View in Synchestra Hub](" + u + viewLinkSuffix
}

func runViewCheck(t *testing.T, root string) []Violation {
	t.Helper()
	c := newViewLinkChecker()
	v, err := c.check(filepath.Join(root, "spec"))
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return v
}

func runViewFix(t *testing.T, root string) {
	t.Helper()
	c := newViewLinkChecker().(fixer)
	if err := c.fix(filepath.Join(root, "spec")); err != nil {
		t.Fatalf("fix: %v", err)
	}
}

// When specscore.yaml is missing entirely, the rule is a no-op (the
// linter has no project to validate against).
func TestViewLink_DisabledWhenNoConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec", "features", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(root, "spec", "features", "x", "README.md"),
		[]byte("# Feature: X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := newViewLinkChecker()
	v, err := c.check(filepath.Join(root, "spec"))
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations when config absent, got %+v", v)
	}
}

func TestViewLink_DisabledWhenViewerNull(t *testing.T) {
	root := setupSuppressedViewerProject(t)
	writeViewFeatureReadme(t, root, "auth", "# Feature: Auth\n\n**Status:** Draft\n")
	v := runViewCheck(t, root)
	if len(v) != 0 {
		t.Errorf("expected 0 violations when viewer: null, got %+v", v)
	}
}

func TestViewLink_DefaultsApplyWhenViewerOmitted(t *testing.T) {
	root := setupDefaultViewerProject(t)
	writeViewFeatureReadme(t, root, "auth", "# Feature: Auth\n\n**Status:** Draft\n")
	v := runViewCheck(t, root)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation under defaults, got %d: %+v", len(v), v)
	}
	if !strings.Contains(v[0].Message, "SpecStudio") {
		t.Errorf("expected SpecStudio in message, got %q", v[0].Message)
	}
}

func TestViewLink_MissingReported(t *testing.T) {
	root := setupViewerProject(t)
	writeViewFeatureReadme(t, root, "auth", "# Feature: Auth\n\n**Status:** Draft\n")
	v := runViewCheck(t, root)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(v), v)
	}
	if v[0].Rule != "view-link" || v[0].Severity != "warning" {
		t.Errorf("bad rule/severity: %+v", v[0])
	}
	if v[0].File != filepath.Join("features", "auth", "README.md") {
		t.Errorf("file = %q", v[0].File)
	}
}

func TestViewLink_PresentNoViolation(t *testing.T) {
	root := setupViewerProject(t)
	content := "# Feature: Auth\n\n" + expectedBlockquote("auth") + "\n\n**Status:** Draft\n"
	writeViewFeatureReadme(t, root, "auth", content)
	v := runViewCheck(t, root)
	if len(v) != 0 {
		t.Errorf("expected 0 violations, got %+v", v)
	}
}

func TestViewLink_StaleReported(t *testing.T) {
	root := setupViewerProject(t)
	stale := viewLinkMarker(testViewerName) + "https://specstudio.example.com/wrong" + viewLinkSuffix
	content := "# Feature: Auth\n\n" + stale + "\n"
	writeViewFeatureReadme(t, root, "auth", content)
	v := runViewCheck(t, root)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %+v", v)
	}
	if !strings.Contains(v[0].Message, "out of date") {
		t.Errorf("expected 'out of date' message, got %q", v[0].Message)
	}
}

func TestViewLink_FixInsertsUnderH1(t *testing.T) {
	root := setupViewerProject(t)
	path := writeViewFeatureReadme(t, root, "auth", "# Feature: Auth\n\n**Status:** Draft\n")
	runViewFix(t, root)
	out, _ := os.ReadFile(path)
	expected := expectedBlockquote("auth")
	if !strings.Contains(string(out), expected) {
		t.Errorf("fix did not insert expected blockquote.\nGot:\n%s\nExpected line:\n%s", out, expected)
	}
	if !strings.HasPrefix(string(out), "# Feature: Auth\n") {
		t.Errorf("H1 not preserved at top: %q", string(out))
	}
	before, _ := os.ReadFile(path)
	runViewFix(t, root)
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("fix not idempotent:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestViewLink_FixReplacesStale(t *testing.T) {
	root := setupViewerProject(t)
	stale := viewLinkMarker(testViewerName) + "https://specstudio.example.com/wrong" + viewLinkSuffix
	path := writeViewFeatureReadme(t, root, "auth",
		"# Feature: Auth\n\n"+stale+"\n\n**Status:** Draft\n")
	runViewFix(t, root)
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "specstudio.example.com/wrong") {
		t.Errorf("stale URL not replaced: %s", out)
	}
	if !strings.Contains(string(out), expectedBlockquote("auth")) {
		t.Errorf("expected blockquote missing: %s", out)
	}
}

// Legacy "View in Synchestra Hub" markers must be classified as stale
// and migrated forward to the current viewer name on --fix.
func TestViewLink_LegacyHubMarkerStaleReported(t *testing.T) {
	root := setupViewerProject(t)
	content := "# Feature: Auth\n\n" + legacyHubBlockquote("auth") + "\n"
	writeViewFeatureReadme(t, root, "auth", content)
	v := runViewCheck(t, root)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %+v", v)
	}
	if !strings.Contains(v[0].Message, "out of date") {
		t.Errorf("expected 'out of date' message, got %q", v[0].Message)
	}
}

func TestViewLink_LegacyHubMarkerMigratedByFix(t *testing.T) {
	root := setupViewerProject(t)
	path := writeViewFeatureReadme(t, root, "auth",
		"# Feature: Auth\n\n"+legacyHubBlockquote("auth")+"\n\n**Status:** Draft\n")
	runViewFix(t, root)
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "View in Synchestra Hub") {
		t.Errorf("legacy Hub marker not removed: %s", out)
	}
	if !strings.Contains(string(out), expectedBlockquote("auth")) {
		t.Errorf("expected current blockquote missing: %s", out)
	}
}

// Legacy "View in Spec Studio" (with space) markers must also be
// classified as stale and migrated forward to the current name (e.g.
// "SpecStudio" without space).
func TestViewLink_LegacySpecStudioMarkerMigratedByFix(t *testing.T) {
	root := setupViewerProject(t)
	path := writeViewFeatureReadme(t, root, "auth",
		"# Feature: Auth\n\n"+legacySpecStudioBlockquote("auth")+"\n\n**Status:** Draft\n")
	runViewFix(t, root)
	out, _ := os.ReadFile(path)
	if strings.Contains(string(out), "View in Spec Studio") {
		t.Errorf("legacy Spec Studio marker not removed: %s", out)
	}
	if !strings.Contains(string(out), expectedBlockquote("auth")) {
		t.Errorf("expected current blockquote missing: %s", out)
	}
}

func TestViewLink_NonGitHubRemoteSkips(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	if err := projectdef.WriteSpecConfig(root, projectdef.SpecConfig{
		Project: &projectdef.ProjectConfig{Title: "T"},
		Viewer:  &projectdef.ViewerConfig{Name: testViewerName, URL: testViewerURL},
	}); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("remote", "add", "origin", "https://gitlab.com/o/r.git")
	writeViewFeatureReadme(t, root, "auth", "# Feature: Auth\n")

	v := runViewCheck(t, root)
	if len(v) != 1 {
		t.Fatalf("expected 1 config-level violation, got %+v", v)
	}
	if v[0].File != projectdef.SpecConfigFile {
		t.Errorf("expected violation on config file, got %q", v[0].File)
	}
}

func TestViewLink_UnderscoreDirsIgnored(t *testing.T) {
	root := setupViewerProject(t)
	writeViewFeatureReadme(t, root, "auth",
		"# Feature: Auth\n\n"+expectedBlockquote("auth")+"\n")
	testsDir := filepath.Join(root, "spec", "features", "auth", "_tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "README.md"),
		[]byte("# Tests\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := runViewCheck(t, root)
	if len(v) != 0 {
		t.Errorf("expected 0 violations, got %+v", v)
	}
}

func TestViewLink_RegisteredAsKnownRule(t *testing.T) {
	if !AllRuleNames()["view-link"] {
		t.Error("view-link should be a known rule")
	}
}

func TestBuildViewURL(t *testing.T) {
	r := gitremote.Remote{Owner: "synchestra-io", Repo: "specscore", Host: "github.com"}
	got := BuildViewURL(testViewerURL, r, "spec/features/bots")
	want := testViewerURL + "/project/features?id=specscore@synchestra-io@github.com&path=spec%2Ffeatures%2Fbots"
	if got != want {
		t.Errorf("BuildViewURL =\n  %q\nwant\n  %q", got, want)
	}
	// Trailing slash on host should be normalized (we already strip
	// trailing slash internally).
	gotSlash := BuildViewURL(testViewerURL+"/", r, "spec/features/bots")
	if gotSlash != want {
		t.Errorf("BuildViewURL with trailing slash =\n  %q\nwant\n  %q", gotSlash, want)
	}
}
