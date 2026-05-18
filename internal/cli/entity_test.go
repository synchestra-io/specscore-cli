package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// setupEntitySpec stages a minimal spec tree at a fresh t.TempDir and
// chdirs into it so resolveSpecRoot picks it up via the cwd-anchored
// FindSpecRepoRoot heuristic. Returns the repo root.
//
// The tree is intentionally minimal — only `specscore.yaml` (so the
// project anchor is unambiguous) plus an empty `spec/features/` dir.
// Tests that need entity files write them on top.
func setupEntitySpec(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	featDir := filepath.Join(specDir, "features")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// specscore.yaml — the project anchor FindSpecRepoRoot prefers.
	if err := os.WriteFile(filepath.Join(root, "specscore.yaml"), []byte("name: test\n"), 0o644); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
	withCwd(t, root)
	return root
}

// writeEntity writes a minimal valid-shaped entity file at
// spec/features/<dir>/<slug>.entity.md with the given inherits target
// ("" means no inherits). The content is the minimum that pkg/entity's
// Parse will accept and recognize.
func writeEntity(t *testing.T, root, dir, slug, inherits string) string {
	t.Helper()
	abs := filepath.Join(root, "spec", "features", dir)
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var inheritsLine string
	if inherits != "" {
		inheritsLine = "inherits: " + inherits + "\n"
	}
	body := "---\n" +
		"kind: entity\n" +
		"id: " + slug + "\n" +
		"singular: " + strings.ToTitle(slug[:1]) + slug[1:] + "\n" +
		"plural: " + strings.ToTitle(slug[:1]) + slug[1:] + "s\n" +
		inheritsLine +
		"properties: []\n" +
		"---\n\n" +
		"# Entity: " + strings.ToTitle(slug[:1]) + slug[1:] + "\n\n" +
		"## Description\n\nstub.\n"
	path := filepath.Join(abs, slug+".entity.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// runEntity invokes the entity cobra command tree in-process and
// captures stdout, stderr, and the returned error.
func runEntity(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := entityCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// --- entity list ---

func TestEntityList_Empty(t *testing.T) {
	_ = setupEntitySpec(t)
	out, _, err := runEntity(t, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got %q", out)
	}
}

func TestEntityList_TextDefault(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "order", "order", "")
	writeEntity(t, root, "user", "user", "")

	out, _, err := runEntity(t, "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "order\nuser\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestEntityList_YAML(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	out, _, err := runEntity(t, "list", "--format", "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []entityListItem
	if uerr := yaml.Unmarshal([]byte(out), &items); uerr != nil {
		t.Fatalf("output is not valid YAML: %v\nout=%s", uerr, out)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %+v", len(items), items)
	}
	if items[0].ID != "user" {
		t.Errorf("id = %q, want %q", items[0].ID, "user")
	}
	if items[0].Path != filepath.Join("spec", "features", "user", "user.entity.md") {
		t.Errorf("path = %q, want spec/features/user/user.entity.md", items[0].Path)
	}
}

func TestEntityList_JSON(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	out, _, err := runEntity(t, "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var items []entityListItem
	if jerr := json.Unmarshal([]byte(out), &items); jerr != nil {
		t.Fatalf("output is not valid JSON: %v\nout=%s", jerr, out)
	}
	if len(items) != 1 || items[0].ID != "user" {
		t.Errorf("unexpected items: %+v", items)
	}
}

// --- entity refs ---

func TestEntityRefs_NoConsumers(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	out, _, err := runEntity(t, "refs", "user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty stdout, got %q", out)
	}
}

func TestEntityRefs_NoConsumers_YAML(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	out, _, err := runEntity(t, "refs", "user", "--format", "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect "consumers: []" in some YAML form.
	if !strings.Contains(out, "consumers:") || !strings.Contains(out, "[]") {
		t.Errorf("expected consumers: [] in output, got %q", out)
	}
}

func TestEntityRefs_OneConsumer(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	// Sibling layout — child in same dir uses a relative path.
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	out, _, err := runEntity(t, "refs", "parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "child\n" {
		t.Errorf("stdout = %q, want %q", out, "child\n")
	}
}

func TestEntityRefs_TwoConsumersSorted(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "zebra", "./parent.entity.md")
	writeEntity(t, root, "parent", "alpha", "./parent.entity.md")

	out, _, err := runEntity(t, "refs", "parent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "alpha\nzebra\n" {
		t.Errorf("stdout = %q, want alpha then zebra", out)
	}
}

func TestEntityRefs_UnknownID(t *testing.T) {
	_ = setupEntitySpec(t)
	_, _, err := runEntity(t, "refs", "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

func TestEntityRefs_MissingID(t *testing.T) {
	_ = setupEntitySpec(t)
	_, _, err := runEntity(t, "refs")
	if err == nil {
		t.Fatal("expected error for missing positional <id>")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

func TestEntityRefs_InvalidFormat(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	_, _, err := runEntity(t, "refs", "user", "--format", "xml")
	if err == nil {
		t.Fatal("expected error for --format=xml")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

// --- entity tree ---

func TestEntityTree_TwoLevelHierarchy(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	out, _, err := runEntity(t, "tree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "parent\n  child\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestEntityTree_Cycle(t *testing.T) {
	root := setupEntitySpec(t)
	// a inherits b, b inherits a. Both have parents → neither is a
	// natural root. The tree verb must still terminate and surface a
	// (cycle) marker exactly once on the first re-occurrence.
	writeEntity(t, root, "cycle", "a", "./b.entity.md")
	writeEntity(t, root, "cycle", "b", "./a.entity.md")

	out, _, err := runEntity(t, "tree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(cycle)") {
		t.Errorf("expected a (cycle) marker, got %q", out)
	}
	// The cycle must not loop forever — output should be small.
	if len(out) > 200 {
		t.Errorf("output too long (recursion did not stop on cycle): %q", out)
	}
	// Exactly one (cycle) marker — per REQ, the FIRST edge that closes
	// the cycle is marked and recursion stops.
	if got := strings.Count(out, "(cycle)"); got != 1 {
		t.Errorf("got %d (cycle) markers, want 1: %q", got, out)
	}
}

func TestEntityTree_FormatRejected(t *testing.T) {
	_ = setupEntitySpec(t)
	_, _, err := runEntity(t, "tree", "--format", "yaml")
	if err == nil {
		t.Fatal("expected error for --format on tree")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

func TestEntityTree_SiblingsSortedAlphabetically(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "zebra", "./parent.entity.md")
	writeEntity(t, root, "parent", "alpha", "./parent.entity.md")

	out, _, err := runEntity(t, "tree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "parent\n  alpha\n  zebra\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

// --- discovery / project errors ---

func TestEntity_NoProject(t *testing.T) {
	// Stage a dir tree with NO specscore.yaml in any ancestor.
	// t.TempDir() returns a path under /tmp; there is no specscore.yaml
	// anywhere above so resolveSpecRoot must exit 3.
	dir := t.TempDir()
	withCwd(t, dir)

	_, _, err := runEntity(t, "list")
	if err == nil {
		t.Fatal("expected error when no specscore.yaml in any ancestor")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

// Compile-time anchor: ensure the errors import is referenced even when
// the only consumer is the local exitCodeOfErr helper from feature_test.go.
var _ = errors.New
