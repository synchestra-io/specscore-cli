package lint

import (
	"os"
	"path/filepath"
	"testing"
)

// AC: type-registered (rule-family half — every I-NNN ID is wired to a
// checker that the linter knows about).
func TestIssueRules_AllFifteenIDsRegistered(t *testing.T) {
	l := newLinter(Options{SpecRoot: t.TempDir()})
	for _, id := range issueRuleIDs {
		if _, ok := l.ruleSet[id]; !ok {
			t.Errorf("rule %q is not registered with the linter", id)
		}
	}
	// Sanity: every ID is also in allRuleNames (so ValidateRuleNames accepts it).
	for _, id := range issueRuleIDs {
		if !allRuleNames[id] {
			t.Errorf("rule %q missing from allRuleNames", id)
		}
	}
}

// AC: default-suite-includes-i-rules. A spec tree with one valid issue
// must lint with zero violations when no flags are passed (all 15 I-
// rules run; stubs emit nothing; I-009 sees a pattern-matching path).
func TestIssueRules_DefaultSuiteIncludesIRules(t *testing.T) {
	specRoot := copyTestdataSpec(t, "rules/issue/testdata/default-suite/spec")
	vs, err := Lint(Options{SpecRoot: specRoot})
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	for _, v := range vs {
		if v.Rule[:2] == "I-" {
			t.Errorf("unexpected I-rule violation on default-suite fixture: %+v", v)
		}
	}
}

// AC: rules-filter-by-id. When --rules I-009 is set, only I-009
// violations surface (other I-rules whose stubs would emit nothing
// anyway are filtered too, but the key invariant is: a non-I-009
// violation that would otherwise fire is suppressed).
//
// In this scaffold only I-009 has logic, so we exercise the filter by
// confirming (a) ValidateRuleNames accepts "I-009" alone, and (b) when
// the fixture trips I-009, the violation surfaces under --rules=I-009
// and is the only one.
func TestIssueRules_FilterByID_AcceptsI009(t *testing.T) {
	if err := ValidateRuleNames([]string{"I-009"}); err != nil {
		t.Fatalf("ValidateRuleNames(I-009): %v", err)
	}
}

func TestIssueRules_FilterByID_OnlyI009Emits(t *testing.T) {
	specRoot := copyTestdataSpec(t, "rules/issue/testdata/dual-location/spec")
	vs, err := Lint(Options{SpecRoot: specRoot, Rules: []string{"I-009"}})
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if len(vs) == 0 {
		t.Fatalf("expected at least one I-009 violation; got none")
	}
	for _, v := range vs {
		if v.Rule != "I-009" {
			t.Errorf("unexpected rule under --rules=I-009 filter: %+v", v)
		}
	}
}

// AC: dual-location-violation. A file at spec/random-dir/foo.md
// declaring `type: issue` triggers I-009.
func TestIssueRules_I009_DualLocationViolation(t *testing.T) {
	specRoot := copyTestdataSpec(t, "rules/issue/testdata/dual-location/spec")
	vs, err := Lint(Options{SpecRoot: specRoot})
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	var i009 *Violation
	for i := range vs {
		if vs[i].Rule == "I-009" {
			i009 = &vs[i]
			break
		}
	}
	if i009 == nil {
		t.Fatalf("expected an I-009 violation; got %+v", vs)
	}
	if i009.File != "random-dir/foo.md" {
		t.Errorf("violation file = %q; want %q", i009.File, "random-dir/foo.md")
	}
	if i009.Severity != "error" {
		t.Errorf("violation severity = %q; want %q", i009.Severity, "error")
	}
}

// copyTestdataSpec copies the spec/ subtree of a testdata fixture into
// a temporary spec root. The fixture directory passed must contain a
// `spec/` subtree (rules/issue/testdata/<name>/spec/...). The function
// returns the path to the temp `spec/` directory.
func copyTestdataSpec(t *testing.T, fixtureSpecPath string) string {
	t.Helper()
	srcRoot := filepath.Join(fixtureSpecPath)
	dstRoot := filepath.Join(t.TempDir(), "spec")
	if err := os.MkdirAll(dstRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	err := filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcRoot, path)
		if relErr != nil {
			return relErr
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy testdata: %v", err)
	}
	return dstRoot
}
