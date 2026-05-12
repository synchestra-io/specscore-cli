package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
	"github.com/synchestra-io/specscore-cli/pkg/projectdef"
)

// runSpecLint runs `specscore spec lint` against the given working
// directory and returns stdout, stderr, and the exit error.
func runSpecLintCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := specCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"lint"}, args...))
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// writeValidSpecscoreYAML drops a minimal lint-valid specscore.yaml at
// dir so that findRepoConfigRoot finds it. The schema-header comment on
// line 1 is mandatory per repo-config#req:schema-header-comment.
func writeValidSpecscoreYAML(t *testing.T, dir string) {
	t.Helper()
	if err := projectdef.WriteSpecConfig(dir, projectdef.SpecConfig{}); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
}

// AC: missing-specscore-yaml-exits-3 — bare spec/features/ tree without
// specscore.yaml MUST exit 3 with init-pointing message.
func TestSpecLint_MissingSpecscoreYAML_ExitsNotFound(t *testing.T) {
	root := t.TempDir()
	// Create the legacy spec/features/ fallback to prove it does NOT
	// satisfy the lint gate.
	if err := os.MkdirAll(filepath.Join(root, "spec", "features"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, _, err := runSpecLintCmd(t, "--project", root)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := exitCodeOf(err); got != exitcode.NotFound {
		t.Fatalf("exit code = %d, want %d (NotFound); err = %v", got, exitcode.NotFound, err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "specscore.yaml") {
		t.Errorf("error message should name specscore.yaml; got: %q", msg)
	}
	if !strings.Contains(msg, "specscore init") {
		t.Errorf("error message should instruct caller to run `specscore init`; got: %q", msg)
	}
}

// Walking-up behavior — a specscore.yaml in an ancestor of the start
// directory MUST satisfy the gate.
func TestSpecLint_FindsSpecscoreYAMLInAncestor(t *testing.T) {
	root := t.TempDir()
	writeValidSpecscoreYAML(t, root)
	// Build an empty (but valid) spec tree so lint has zero violations.
	if err := os.MkdirAll(filepath.Join(root, "spec", "features"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	specReadme := "# Specifications\n\nTest tree.\n\n## Outstanding Questions\n\nNone at this time.\n"
	if err := os.WriteFile(filepath.Join(root, "spec", "README.md"), []byte(specReadme), 0o644); err != nil {
		t.Fatalf("write spec/README.md: %v", err)
	}
	featReadme := "# Features\n\n## Index\n\n| Feature | Status | Description |\n|---------|--------|-------------|\n\n## Outstanding Questions\n\nNone at this time.\n"
	if err := os.WriteFile(filepath.Join(root, "spec", "features", "README.md"), []byte(featReadme), 0o644); err != nil {
		t.Fatalf("write features/README.md: %v", err)
	}

	// Run from a nested subdirectory — the walk-up must locate
	// specscore.yaml at root.
	nested := filepath.Join(root, "spec", "features")
	_, _, err := runSpecLintCmd(t, "--project", nested)
	// We don't assert success of every rule (empty tree may still have
	// rule complaints); we assert we do NOT get the NotFound gate error.
	if err != nil {
		if got := exitCodeOf(err); got == exitcode.NotFound {
			t.Fatalf("unexpected NotFound from lint when specscore.yaml exists at %s: %v", root, err)
		}
	}
}
