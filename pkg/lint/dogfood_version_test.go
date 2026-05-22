package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupProjectWithWorkflow stages a temp project root with a spec/ dir
// (so SpecRoot resolves) plus a .github/workflows/<name> file whose body
// is the given content. Returns the spec/ subdir, suitable to pass as
// SpecRoot to the checker.
func setupProjectWithWorkflow(t *testing.T, workflowName, workflowBody string) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, workflowName), []byte(workflowBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return specDir
}

func TestDogfoodVersion_StalePinWarns(t *testing.T) {
	body := "env:\n  SPECSCORE_VERSION: v0.2.0  # bump intentionally via PR\n"
	specRoot := setupProjectWithWorkflow(t, "dogfood.yml", body)

	c := newDogfoodVersionChecker("0.3.0")
	v, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(v), v)
	}
	if v[0].Rule != "dogfood-version-bump" {
		t.Errorf("rule: got %s", v[0].Rule)
	}
	if v[0].Severity != "warning" {
		t.Errorf("severity: got %s", v[0].Severity)
	}
	if !strings.Contains(v[0].Message, "v0.2.0") || !strings.Contains(v[0].Message, "v0.3.0") {
		t.Errorf("message should name both versions: %s", v[0].Message)
	}
	if !strings.HasSuffix(v[0].File, "dogfood.yml") {
		t.Errorf("file should be the workflow file: %s", v[0].File)
	}
	if v[0].Line != 2 {
		t.Errorf("line should point at the pin line (2), got %d", v[0].Line)
	}
}

func TestDogfoodVersion_EqualVersionsSilent(t *testing.T) {
	body := "env:\n  SPECSCORE_VERSION: v0.3.0\n"
	specRoot := setupProjectWithWorkflow(t, "dogfood.yml", body)

	c := newDogfoodVersionChecker("0.3.0")
	v, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations (pinned == binary), got %d: %v", len(v), v)
	}
}

func TestDogfoodVersion_NewerPinSilent(t *testing.T) {
	// Forward-pinning (e.g., CI ships a release ahead of the contributor's
	// local CLI) is the user's deliberate choice — no warning.
	body := "env:\n  SPECSCORE_VERSION: v1.0.0\n"
	specRoot := setupProjectWithWorkflow(t, "dogfood.yml", body)

	c := newDogfoodVersionChecker("0.3.0")
	v, err := c.check(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations (pinned > binary), got %d: %v", len(v), v)
	}
}

func TestDogfoodVersion_DevBinaryDisablesRule(t *testing.T) {
	// `dev` (or any non-semver) is an explicit override — local builds.
	// The rule has nothing meaningful to compare against and must stay silent.
	body := "env:\n  SPECSCORE_VERSION: v0.0.1\n"
	specRoot := setupProjectWithWorkflow(t, "dogfood.yml", body)

	for _, cliVer := range []string{"dev", "", "1.2", "garbage"} {
		t.Run(cliVer, func(t *testing.T) {
			c := newDogfoodVersionChecker(cliVer)
			v, err := c.check(specRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(v) != 0 {
				t.Errorf("dev/empty CLI version must disable rule, got %d violations: %v", len(v), v)
			}
		})
	}
}

func TestDogfoodVersion_UnparseablePinSkipped(t *testing.T) {
	// `latest`, `main`, and template expressions are intentional overrides
	// and must be skipped silently, not warned on.
	for _, pinned := range []string{"latest", "main", "${{ inputs.version }}", "v1.2"} {
		t.Run(pinned, func(t *testing.T) {
			body := "env:\n  SPECSCORE_VERSION: " + pinned + "\n"
			specRoot := setupProjectWithWorkflow(t, "dogfood.yml", body)
			c := newDogfoodVersionChecker("0.3.0")
			v, err := c.check(specRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(v) != 0 {
				t.Errorf("non-semver pin must be skipped silently, got %d: %v", len(v), v)
			}
		})
	}
}

func TestDogfoodVersion_NoWorkflowsDirectorySilent(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No .github/workflows — should silently no-op.
	c := newDogfoodVersionChecker("0.3.0")
	v, err := c.check(specDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 0 {
		t.Errorf("no workflows dir → no violations, got %d: %v", len(v), v)
	}
}

func TestDogfoodVersion_MultipleWorkflowsAllScanned(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "dogfood.yml"),
		[]byte("env:\n  SPECSCORE_VERSION: v0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yaml"),
		[]byte("env:\n  SPECSCORE_VERSION: v0.2.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := newDogfoodVersionChecker("0.5.0")
	v, err := c.check(specDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 2 {
		t.Fatalf("expected 2 violations (one per workflow), got %d: %v", len(v), v)
	}
}

func TestDogfoodVersion_PinFormatVariants(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantHit bool
	}{
		{"no v prefix", "env:\n  SPECSCORE_VERSION: 0.2.0\n", true},
		{"quoted", "env:\n  SPECSCORE_VERSION: \"v0.2.0\"\n", true},
		{"with comment", "env:\n  SPECSCORE_VERSION: v0.2.0  # bump intentionally via PR\n", true},
		{"with patch suffix", "env:\n  SPECSCORE_VERSION: v0.2.0-rc.1\n", true},
		{"no SPECSCORE_VERSION", "env:\n  OTHER: v0.2.0\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specRoot := setupProjectWithWorkflow(t, "dogfood.yml", tc.body)
			c := newDogfoodVersionChecker("0.3.0")
			v, err := c.check(specRoot)
			if err != nil {
				t.Fatal(err)
			}
			hit := len(v) > 0
			if hit != tc.wantHit {
				t.Errorf("case %q: hit=%v, want=%v (violations=%v)", tc.name, hit, tc.wantHit, v)
			}
		})
	}
}

func TestParseSemver(t *testing.T) {
	cases := map[string]struct {
		ok bool
		sv semver
	}{
		"0.2.0":      {true, semver{0, 2, 0}},
		"v0.2.0":     {true, semver{0, 2, 0}},
		"v1.2.3-rc1": {true, semver{1, 2, 3}},
		"1.2.3+meta": {true, semver{1, 2, 3}},
		" v0.2.0 ":   {true, semver{0, 2, 0}},
		"":           {false, semver{}},
		"dev":        {false, semver{}},
		"v1.2":       {false, semver{}},
		"v1.2.3.4":   {false, semver{}},
		"v-1.2.3":    {false, semver{}},
	}
	for input, want := range cases {
		got, ok := parseSemver(input)
		if ok != want.ok || got != want.sv {
			t.Errorf("parseSemver(%q) = (%+v, %v), want (%+v, %v)", input, got, ok, want.sv, want.ok)
		}
	}
}
