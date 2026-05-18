package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runProperty invokes the `property` cobra command tree in-process and
// captures stdout, stderr, and the returned error.
func runProperty(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := propertyCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// setupPropertyProject builds a minimal project rooted at a fresh
// t.TempDir containing `spec/features/<feat>/`. It writes a stub
// `specscore.yaml` so `FindSpecRepoRoot` anchors here. The caller
// layers property/entity files on top before invoking the verbs.
func setupPropertyProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, "spec")
	featDir := filepath.Join(specDir, "features", "shared")
	if err := os.MkdirAll(featDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := "# SpecScore Repo Config Schema: https://specscore.md/repo-config\nversion: 1\n"
	if err := os.WriteFile(filepath.Join(root, "specscore.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
	withCwd(t, root)
	return root
}

func writePropertyFile(t *testing.T, root, relPath, slug, dataType string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nkind: property\nid: " + slug + "\ndata_type: " + dataType + "\nchecks: {}\n---\n# Property: " + slug + "\n\n## Description\n\nstub.\n\n## Checks\n\n_None._\n\n## Referenced by\n\n<!-- specscore:managed-start -->\n- _No references yet._\n<!-- specscore:managed-end -->\n\n---\n*This document follows the https://specscore.md/property-specification*\n"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func writeEntityFileWithRefs(t *testing.T, root, relPath, slug string, refs map[string]string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var props strings.Builder
	for name, ref := range refs {
		props.WriteString("  - name: " + name + "\n    ref: " + ref + "\n")
	}
	body := "---\nkind: entity\nid: " + slug + "\nsingular: " + slug + "\nplural: " + slug + "s\nproperties:\n" + props.String() + "---\n# Entity: " + slug + "\n\n## Description\n\nstub.\n\n## Properties\n\n_None._\n\n---\n*This document follows the https://specscore.md/entity-specification*\n"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func writeEntityFileWithInlineProperty(t *testing.T, root, relPath, slug, propName, dataType string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "---\nkind: entity\nid: " + slug + "\nsingular: " + slug + "\nplural: " + slug + "s\nproperties:\n  - name: " + propName + "\n    data_type: " + dataType + "\n    checks:\n      required: true\n---\n# Entity: " + slug + "\n\n## Description\n\nstub.\n\n## Properties\n\n_None._\n\n---\n*This document follows the https://specscore.md/entity-specification*\n"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

// --- property list ---

func TestPropertyList_Empty(t *testing.T) {
	_ = setupPropertyProject(t)

	out, _, err := runProperty(t, "list")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestPropertyList_TextDefault(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writePropertyFile(t, root, "spec/features/shared/age.property.md", "age", "integer")

	out, _, err := runProperty(t, "list")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := "age\nemail\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestPropertyList_YAML(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "list", "--format", "yaml")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, "id: email") {
		t.Errorf("stdout missing `id: email`: %q", out)
	}
	if !strings.Contains(out, "path: spec/features/shared/email.property.md") {
		t.Errorf("stdout missing relative path: %q", out)
	}
}

func TestPropertyList_JSON(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "list", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `"id": "email"`) {
		t.Errorf("stdout missing `\"id\": \"email\"`: %q", out)
	}
	if !strings.Contains(out, `"path": "spec/features/shared/email.property.md"`) {
		t.Errorf("stdout missing path: %q", out)
	}
}

func TestPropertyList_InvalidFormat(t *testing.T) {
	_ = setupPropertyProject(t)

	_, _, err := runProperty(t, "list", "--format", "bogus")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

// --- property refs ---

func TestPropertyRefs_NoConsumers(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
}

func TestPropertyRefs_NoConsumersYAML(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "refs", "email", "--format", "yaml")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, "consumers: []") {
		t.Errorf("stdout = %q, want `consumers: []`", out)
	}
}

func TestPropertyRefs_OneConsumer(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "./email.property.md"})

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "user\n" {
		t.Errorf("stdout = %q, want %q", out, "user\n")
	}
}

func TestPropertyRefs_DedupSameEntityTwoNames(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// One entity with TWO property entries (home_email + work_email) both
	// referencing the same email.property.md. Must appear ONCE in output.
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{
			"home_email": "./email.property.md",
			"work_email": "./email.property.md",
		})

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "user\n" {
		t.Errorf("stdout = %q, want %q (deduped)", out, "user\n")
	}
}

func TestPropertyRefs_IgnoresInline(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// Entity has INLINE `email` property (data_type + checks, no ref:).
	// Per [REQ: property-refs], inline definitions MUST NOT appear in
	// `property refs` output.
	writeEntityFileWithInlineProperty(t, root, "spec/features/shared/user.entity.md", "user", "email", "string")

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty (inline excluded)", out)
	}
}

func TestPropertyRefs_UnknownID(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	_, _, err := runProperty(t, "refs", "no-such-property")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

func TestPropertyRefs_MissingID(t *testing.T) {
	_ = setupPropertyProject(t)

	_, _, err := runProperty(t, "refs")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	// cobra's ExactArgs(1) returns a plain error (no exit code wrapper).
	// We still need to surface non-zero — cobra's default error string is
	// the marker. Either typed-as-2 OR a generic cobra error is acceptable.
	if got := exitCodeOfErr(err); got != -1 && got != 2 {
		t.Errorf("exit code = %d, want 2 or cobra-default", got)
	}
}

// --- project resolution ---

func TestProperty_NoProject(t *testing.T) {
	// CWD must be a directory with no specscore.yaml or spec/features/
	// in any ancestor. t.TempDir() lives under /tmp/... so a plain tmp
	// dir satisfies this.
	tmp := t.TempDir()
	withCwd(t, tmp)

	_, _, err := runProperty(t, "list")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

func TestProperty_NoProjectRefs(t *testing.T) {
	tmp := t.TempDir()
	withCwd(t, tmp)

	_, _, err := runProperty(t, "refs", "email")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}
