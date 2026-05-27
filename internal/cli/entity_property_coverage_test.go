package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/entity"
)

// failingWriter returns errFailingWriter on every Write call. Used to
// drive yaml.Encoder.Encode and json.Encoder.Encode error paths in
// internal/cli entity/property verbs (writeEntityListYAML/JSON,
// writePropertyListYAML/JSON, writePropertyRefsYAML/JSON).
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errFailingWriter }

var errFailingWriter = errors.New("failing-writer-sentinel")

// Compile-time guard: failingWriter must satisfy io.Writer.
var _ io.Writer = failingWriter{}

// ==========================================================================
// internal/cli/entity.go:39 — projectRelativePath error path
// ==========================================================================
//
// filepath.Rel returns an error when one path is absolute and the other
// is relative. In that case projectRelativePath must fall back to the
// absolute path argument.

func TestProjectRelativePath_RelErrorFallsBackToAbs(t *testing.T) {
	// projectRoot is relative, absPath is absolute → filepath.Rel fails.
	got := projectRelativePath("relative-root", "/absolute/path/file")
	if got != "/absolute/path/file" {
		t.Errorf("projectRelativePath fallback = %q, want absolute path", got)
	}
}

func TestProjectRelativePath_HappyPath(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(root, "spec", "features", "user", "user.entity.md")
	got := projectRelativePath(root, abs)
	want := filepath.Join("spec", "features", "user", "user.entity.md")
	if got != want {
		t.Errorf("projectRelativePath = %q, want %q", got, want)
	}
}

// ==========================================================================
// internal/cli/entity.go:73 — runEntityList invalid format
// ==========================================================================

func TestEntityList_InvalidFormat(t *testing.T) {
	_ = setupEntitySpec(t)
	_, _, err := runEntity(t, "list", "--format", "xml")
	if err == nil {
		t.Fatal("expected error for --format=xml")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

// ==========================================================================
// internal/cli/entity.go:164 — runEntityRefs too many args
// ==========================================================================

func TestEntityRefs_TooManyArgs(t *testing.T) {
	_ = setupEntitySpec(t)
	_, _, err := runEntity(t, "refs", "user", "extra")
	if err == nil {
		t.Fatal("expected error for too many positional args")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

// ==========================================================================
// internal/cli/entity.go:164 — runEntityRefs yaml/json with consumers
// ==========================================================================

func TestEntityRefs_OneConsumer_YAML(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	out, _, err := runEntity(t, "refs", "parent", "--format", "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "consumers:") {
		t.Errorf("expected consumers: in yaml output, got %q", out)
	}
	if !strings.Contains(out, "child") {
		t.Errorf("expected child in yaml output, got %q", out)
	}
}

func TestEntityRefs_OneConsumer_JSON(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	out, _, err := runEntity(t, "refs", "parent", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"consumers"`) {
		t.Errorf("expected `consumers` key in json output, got %q", out)
	}
	if !strings.Contains(out, `"child"`) {
		t.Errorf("expected child in json output, got %q", out)
	}
}

// ==========================================================================
// internal/cli/entity.go:221-227 — runEntityRefs resolve error / empty
// ==========================================================================
//
// entity.ResolveInherits can fail when filepath.Abs errs. Inject via the
// pkg/entity seam to drive this branch.

func TestEntityRefs_ResolveInheritsError(t *testing.T) {
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	// We can't reach into pkg/entity seams from this package without
	// importing them. Instead, we exercise the URL-form inherits which
	// makes ResolveInherits return ("", false, nil) — that satisfies the
	// `resolved == ""` branch at line 226-227.
	urlChild := `---
kind: entity
id: child
singular: Child
plural: Children
inherits: https://example.com/parent.entity.md
properties: []
---

# Entity: Child

## Description

X
`
	abs := filepath.Join(root, "spec", "features", "parent", "child.entity.md")
	if err := os.WriteFile(abs, []byte(urlChild), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runEntity(t, "refs", "parent")
	if err != nil {
		t.Fatal(err)
	}
}

// ==========================================================================
// internal/cli/entity.go:164 — runEntityRefs Parse error path
// ==========================================================================
//
// When the entity exists at discovery but Parse fails (file made
// unreadable after Discover), entity refs surfaces an exit-10 error.

func TestEntityRefs_ParseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	other := writeEntity(t, root, "parent", "other", "")
	// Make 'other' unreadable so the loop body's entity.Parse fails.
	if err := os.Chmod(other, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(other, 0o644) })

	_, _, err := runEntity(t, "refs", "parent")
	if err == nil {
		t.Fatal("expected parse error to surface")
	}
}

// ==========================================================================
// internal/cli/entity.go — runEntityRefs Discover error
// ==========================================================================

func TestEntityRefs_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupEntitySpec(t)
	// Pre-create a feature with an entity so it's not the "id not found"
	// path; then break the features dir.
	writeEntity(t, root, "user", "user", "")
	featuresDir := filepath.Join(root, "spec", "features")
	sub := filepath.Join(featuresDir, "user")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, _, err := runEntity(t, "refs", "user")
	if err == nil {
		t.Fatal("expected discover error to surface")
	}
}

// ==========================================================================
// internal/cli/entity.go:291 — runEntityTree Discover error
// ==========================================================================

func TestEntityTree_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")
	sub := filepath.Join(root, "spec", "features", "user")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, _, err := runEntity(t, "tree")
	if err == nil {
		t.Fatal("expected discover error to surface")
	}
}

// ==========================================================================
// runEntityRefs / runEntityList YAML+JSON encode errors via failing writer
// ==========================================================================
//
// The encoder error paths in runEntityRefs (lines 244-246 yaml, 251-253
// json) and runEntityList (118-120 yaml, 127-129 json) are reached when
// the underlying io.Writer fails. We swap cmd.OutOrStdout for a writer
// that always errs.

func runEntityRefsWithFailingWriter(t *testing.T, format string) error {
	t.Helper()
	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	cmd := entityCommand()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(failingWriter{})
	cmd.SetArgs([]string{"refs", "parent", "--format", format})
	return cmd.Execute()
}

func TestEntityRefs_YAML_EncodeError(t *testing.T) {
	if err := runEntityRefsWithFailingWriter(t, "yaml"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestEntityRefs_JSON_EncodeError(t *testing.T) {
	if err := runEntityRefsWithFailingWriter(t, "json"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func runEntityListWithFailingWriter(t *testing.T, format string) error {
	t.Helper()
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	cmd := entityCommand()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(failingWriter{})
	cmd.SetArgs([]string{"list", "--format", format})
	return cmd.Execute()
}

func TestEntityList_YAML_EncodeError(t *testing.T) {
	if err := runEntityListWithFailingWriter(t, "yaml"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestEntityList_JSON_EncodeError(t *testing.T) {
	if err := runEntityListWithFailingWriter(t, "json"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

// ==========================================================================
// internal/cli/property.go:261-262 — runPropertyRefs absResolved != target skip
// ==========================================================================

func TestPropertyRefs_NonMatchingRefSkipped(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writePropertyFile(t, root, "spec/features/shared/phone.property.md", "phone", "string")
	// Entity refs phone but query is email → entry NOT matching.
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"phone": "./phone.property.md"})

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("expected empty (entity refs different property); got %q", out)
	}
}

// ==========================================================================
// internal/cli/property.go:230 — runPropertyRefs entity.Discover error via seam
// ==========================================================================

func TestPropertyRefs_EntityDiscoverError_Seam(t *testing.T) {
	orig := entityDiscoverCLI
	t.Cleanup(func() { entityDiscoverCLI = orig })
	entityDiscoverCLI = func(specRoot string) ([]entity.Discovered, error) {
		return nil, os.ErrInvalid
	}

	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	_, _, err := runProperty(t, "refs", "email")
	if err == nil {
		t.Fatal("expected entity.Discover error to surface")
	}
}

// ==========================================================================
// internal/cli/entity.go:221-224 — runEntityRefs resolveErr branch via seam
// ==========================================================================

func TestEntityRefs_ResolveInheritsError_Seam(t *testing.T) {
	orig := entityResolveInheritsCLI
	t.Cleanup(func() { entityResolveInheritsCLI = orig })
	entityResolveInheritsCLI = func(specRoot, entityPath, inherits string) (string, bool, error) {
		return "", false, os.ErrInvalid
	}

	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")
	out, _, err := runEntity(t, "refs", "parent")
	if err != nil {
		t.Fatal(err)
	}
	// With ResolveInherits errored on every consumer candidate, no
	// consumers are recorded.
	if out != "" {
		t.Errorf("expected empty consumers; got %q", out)
	}
}

// ==========================================================================
// internal/cli/property.go:270-271 — runPropertyRefs `seen[entityID]` break
// ==========================================================================
//
// Two SEPARATE entity files share the same frontmatter `id:` (e.g.
// because both are draft drafts of the same canonical entity). The
// second one to be visited must NOT re-emit the duplicate consumer.

func TestPropertyRefs_DedupAcrossEntityFiles(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	entityWithRef := func(slug string) string {
		return "---\nkind: entity\nid: shared-user\nsingular: U\nplural: Us\nproperties:\n  - name: email\n    ref: ./email.property.md\n---\n# Entity: U\n\n## Description\n\n.\n"
	}
	// Two distinct files, same frontmatter id "shared-user" → dedup.
	a := filepath.Join(root, "spec", "features", "shared", "a.entity.md")
	b := filepath.Join(root, "spec", "features", "shared", "b.entity.md")
	if err := os.WriteFile(a, []byte(entityWithRef("a")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(entityWithRef("b")), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "shared-user") != 1 {
		t.Errorf("expected exactly one occurrence of `shared-user`; got: %q", out)
	}
}

// ==========================================================================
// internal/cli/entity.go & property.go — filepath.Abs fallbacks via seam
// ==========================================================================

func TestEntityRefs_FilepathAbsFallback(t *testing.T) {
	orig := filepathAbsCLI
	t.Cleanup(func() { filepathAbsCLI = orig })
	filepathAbsCLI = func(p string) (string, error) { return "", os.ErrInvalid }

	root := setupEntitySpec(t)
	writeEntity(t, root, "parent", "parent", "")
	writeEntity(t, root, "parent", "child", "./parent.entity.md")

	out, _, err := runEntity(t, "refs", "parent")
	if err != nil {
		t.Fatal(err)
	}
	// Verb still runs to completion using the fallback paths.
	_ = out
}

func TestEntityTree_FilepathAbsFallback(t *testing.T) {
	orig := filepathAbsCLI
	t.Cleanup(func() { filepathAbsCLI = orig })
	filepathAbsCLI = func(p string) (string, error) { return "", os.ErrInvalid }

	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")

	if _, _, err := runEntity(t, "tree"); err != nil {
		t.Fatal(err)
	}
}

func TestPropertyList_FilepathRelFallback(t *testing.T) {
	orig := filepathRelFn
	t.Cleanup(func() { filepathRelFn = orig })
	filepathRelFn = func(base, target string) (string, error) { return "", os.ErrInvalid }

	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "list", "--format", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	// On Rel-error fallback, the rendered path equals the absolute p.Path.
	if !strings.Contains(out, "email.property.md") {
		t.Errorf("expected email.property.md in output: %q", out)
	}
}

func TestPropertyRefs_FilepathAbsFallback(t *testing.T) {
	orig := filepathAbsCLI
	t.Cleanup(func() { filepathAbsCLI = orig })
	filepathAbsCLI = func(p string) (string, error) { return "", os.ErrInvalid }

	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "./email.property.md"})

	_, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatal(err)
	}
}

// ==========================================================================
// internal/cli/property.go:195-197 — runPropertyRefs invalid format
// ==========================================================================

func TestPropertyRefs_InvalidFormat(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	_, _, err := runProperty(t, "refs", "email", "--format", "bogus")
	if err == nil {
		t.Fatal("expected invalid-format error")
	}
	if got := exitCodeOfErr(err); got != 2 {
		t.Errorf("exit code = %d, want 2", got)
	}
}

// ==========================================================================
// runPropertyList / runPropertyRefs encoder errors via failing writer
// ==========================================================================

func runPropertyListWithFailingWriter(t *testing.T, format string) error {
	t.Helper()
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	cmd := propertyCommand()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(failingWriter{})
	cmd.SetArgs([]string{"list", "--format", format})
	return cmd.Execute()
}

func TestPropertyList_YAML_EncodeError(t *testing.T) {
	if err := runPropertyListWithFailingWriter(t, "yaml"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestPropertyList_JSON_EncodeError(t *testing.T) {
	if err := runPropertyListWithFailingWriter(t, "json"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func runPropertyRefsWithFailingWriter(t *testing.T, format string) error {
	t.Helper()
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "./email.property.md"})

	cmd := propertyCommand()
	cmd.SetOut(failingWriter{})
	cmd.SetErr(failingWriter{})
	cmd.SetArgs([]string{"refs", "email", "--format", format})
	return cmd.Execute()
}

func TestPropertyRefs_YAML_EncodeError(t *testing.T) {
	if err := runPropertyRefsWithFailingWriter(t, "yaml"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestPropertyRefs_JSON_EncodeError_ViaCmd(t *testing.T) {
	if err := runPropertyRefsWithFailingWriter(t, "json"); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

// ==========================================================================
// internal/cli/entity.go:183 — runEntityRefs resolveSpecRoot error
// ==========================================================================

func TestEntityRefs_NoProject(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)

	_, _, err := runEntity(t, "refs", "anything")
	if err == nil {
		t.Fatal("expected resolveSpecRoot error to surface")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

// ==========================================================================
// internal/cli/entity.go:302 — runEntityTree resolveSpecRoot error
// ==========================================================================

func TestEntityTree_NoProject(t *testing.T) {
	dir := t.TempDir()
	withCwd(t, dir)

	_, _, err := runEntity(t, "tree")
	if err == nil {
		t.Fatal("expected resolveSpecRoot error to surface")
	}
	if got := exitCodeOfErr(err); got != 3 {
		t.Errorf("exit code = %d, want 3", got)
	}
}

// ==========================================================================
// internal/cli/property.go:230 — runPropertyRefs entity.Discover error
// ==========================================================================
//
// After property.Discover succeeds and we have the target property, the
// next entity.Discover call (line 229) errs. We can't reliably trigger
// this without seam injection — defensible-only.

// ==========================================================================
// internal/cli/entity.go:291 — runEntityTree Parse error
// ==========================================================================

func TestEntityTree_ParseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupEntitySpec(t)
	p := writeEntity(t, root, "user", "user", "")
	if err := os.Chmod(p, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	_, _, err := runEntity(t, "tree")
	if err == nil {
		t.Fatal("expected parse error to surface")
	}
}

// ==========================================================================
// internal/cli/entity.go:80 — runEntityList Discover error
// ==========================================================================

func TestEntityList_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupEntitySpec(t)
	writeEntity(t, root, "user", "user", "")
	sub := filepath.Join(root, "spec", "features", "user")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, _, err := runEntity(t, "list")
	if err == nil {
		t.Fatal("expected discover error to surface")
	}
}

// ==========================================================================
// internal/cli/entity.go:419 — markReachable already-seen branch
// ==========================================================================
//
// markReachable's `if seen[slug]` guard returns early when revisited.
// Triggered by a diamond inheritance graph: two children inherit from a
// common parent, both visit the parent via the parent's own child list.
// The forest is reconstructed during entity-tree rendering, and
// markReachable is invoked once per root. A second pass over the same
// subtree must hit the early-return.

func TestEntityTree_MarkReachableEarlyReturn(t *testing.T) {
	// Build a graph where a node appears once as a root.
	// markReachable is called for every root; the "already-seen" branch
	// fires when a node has multiple ancestor paths back to a root.
	//
	// Simulate: root → child; another orphan node has parent=child by
	// way of a back-and-forth. The simplest way is two independent
	// roots, then a third entity whose inherits points to either.
	root := setupEntitySpec(t)
	writeEntity(t, root, "tree", "alpha", "")
	writeEntity(t, root, "tree", "beta", "")
	// beta inherits alpha → alpha's child set: [beta].
	// Then add an entity gamma that inherits beta.
	writeEntity(t, root, "tree", "gamma", "./beta.entity.md")
	// Use direct markReachable to exercise the seen guard.
	children := map[string][]string{
		"alpha": {"beta"},
		"beta":  {"gamma"},
		"gamma": {"beta"}, // back edge → already-seen path
	}
	seen := map[string]bool{}
	markReachable("alpha", children, seen)
	// Both branches must have been visited; ensure no infinite loop and
	// every node ended up in seen.
	for _, s := range []string{"alpha", "beta", "gamma"} {
		if !seen[s] {
			t.Errorf("expected %s to be marked reachable", s)
		}
	}
}

// ==========================================================================
// internal/cli/property.go — writePropertyRefsJSON 0%
// ==========================================================================

func TestPropertyRefs_JSON_WithConsumers(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "./email.property.md"})

	out, _, err := runProperty(t, "refs", "email", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `"consumers"`) {
		t.Errorf("missing `consumers` key in JSON output: %q", out)
	}
	if !strings.Contains(out, `"user"`) {
		t.Errorf("missing user in JSON output: %q", out)
	}
}

func TestPropertyRefs_JSON_NoConsumers(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")

	out, _, err := runProperty(t, "refs", "email", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, `"consumers": []`) {
		t.Errorf("expected empty consumers array in JSON; got %q", out)
	}
}

// ==========================================================================
// internal/cli/property.go:80 — runPropertyList Discover error
// ==========================================================================

func TestPropertyList_DiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	sub := filepath.Join(root, "spec", "features", "shared")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, _, err := runProperty(t, "list")
	if err == nil {
		t.Fatal("expected discover error to surface")
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs Discover error (property)
// ==========================================================================

func TestPropertyRefs_DiscoverError_Property(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	sub := filepath.Join(root, "spec", "features", "shared")
	if err := os.Chmod(sub, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	_, _, err := runProperty(t, "refs", "email")
	if err == nil {
		t.Fatal("expected discover error to surface")
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs entity.Discover error
// ==========================================================================
//
// Discovery of properties succeeds but the second entity.Discover errs:
// build a scenario where the property file dir has the property file
// readable, but the entity dir is unreadable.

func TestPropertyRefs_EntityDiscoverError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupPropertyProject(t)
	// Keep the property file in a readable dir.
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// Create an entity in a separate dir, then make THAT dir unreadable.
	entityDir := filepath.Join(root, "spec", "features", "broken")
	if err := os.MkdirAll(entityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Touch a placeholder so Walk descends.
	if err := os.WriteFile(filepath.Join(entityDir, ".keep"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(entityDir, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(entityDir, 0o755) })

	_, _, err := runProperty(t, "refs", "email")
	if err == nil {
		t.Fatal("expected error from unreadable entity dir")
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs Parse error (entity)
// ==========================================================================

func TestPropertyRefs_EntityParseError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	entityPath := filepath.Join(root, "spec", "features", "shared", "user.entity.md")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "./email.property.md"})
	if err := os.Chmod(entityPath, 0o000); err != nil {
		t.Skip("chmod failed")
	}
	t.Cleanup(func() { _ = os.Chmod(entityPath, 0o644) })

	_, _, err := runProperty(t, "refs", "email")
	if err == nil {
		t.Fatal("expected entity parse error to surface")
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs entity nil/missing frontmatter
// is skipped. Also covers the dedup `break` on already-seen entityID.
// ==========================================================================

func TestPropertyRefs_DedupOnDuplicateRefs_JSON(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{
			"home_email": "./email.property.md",
			"work_email": "./email.property.md",
		})

	out, _, err := runProperty(t, "refs", "email", "--format", "json")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	count := strings.Count(out, `"user"`)
	if count != 1 {
		t.Errorf("dedup failed: %d occurrences of `user`, want 1; out=%q", count, out)
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs missing-frontmatter skip
// ==========================================================================

func TestPropertyRefs_EntityMissingFrontmatterSkipped(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// Entity with no frontmatter at all.
	emptyEntityPath := filepath.Join(root, "spec", "features", "shared", "ghost.entity.md")
	if err := os.WriteFile(emptyEntityPath, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// No consumers because the only entity has no frontmatter.
	if out != "" {
		t.Errorf("expected empty stdout (entity has no FM), got %q", out)
	}
}

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs URL-ref skipped
// ==========================================================================

func TestPropertyRefs_URLRefSkipped(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// Entity with URL ref — resolve returns isLocal=false, must skip.
	writeEntityFileWithRefs(t, root, "spec/features/shared/user.entity.md", "user",
		map[string]string{"email": "https://example.com/email.property.md"})

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty (URL ref doesn't count), got %q", out)
	}
}

// ==========================================================================
// Encoder error paths covered via a failing io.Writer
// ==========================================================================

func TestWriteEntityListYAML_EncodeError(t *testing.T) {
	items := []entityListItem{{ID: "x", Path: "p"}}
	if err := writeEntityListYAML(failingWriter{}, items); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestWriteEntityListJSON_EncodeError(t *testing.T) {
	items := []entityListItem{{ID: "x", Path: "p"}}
	if err := writeEntityListJSON(failingWriter{}, items); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestWritePropertyListYAML_EncodeError(t *testing.T) {
	if err := writePropertyListYAML(failingWriter{}, []propertyListEntry{{ID: "x", Path: "p"}}); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestWritePropertyListJSON_EncodeError(t *testing.T) {
	if err := writePropertyListJSON(failingWriter{}, []propertyListEntry{{ID: "x", Path: "p"}}); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestWritePropertyRefsYAML_EncodeError(t *testing.T) {
	if err := writePropertyRefsYAML(failingWriter{}, []string{"a"}); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

func TestWritePropertyRefsJSON_EncodeError(t *testing.T) {
	if err := writePropertyRefsJSON(failingWriter{}, []string{"a"}); err == nil {
		t.Error("expected encode error from failing writer")
	}
}

// ==========================================================================
// internal/cli/property.go:80 — runPropertyList YAML/JSON via property list verb
// (writePropertyListYAML and writePropertyListJSON covered above; force the
// encode-error path is impractical without a faulty io.Writer — leave as is.)
// ==========================================================================

// ==========================================================================
// internal/cli/property.go:191 — runPropertyRefs entity frontmatter ID
// fallback to slug (entityID == "" branch at line 267-269).
// ==========================================================================

func TestPropertyRefs_EntityIDFallbackToSlug(t *testing.T) {
	root := setupPropertyProject(t)
	writePropertyFile(t, root, "spec/features/shared/email.property.md", "email", "string")
	// Entity with no `id:` in frontmatter — the runPropertyRefs loop must
	// fall back to using the filename slug.
	entityBody := `---
kind: entity
singular: User
plural: Users
properties:
  - name: email
    ref: ./email.property.md
---

# Entity: User

## Description

stub.
`
	abs := filepath.Join(root, "spec", "features", "shared", "user.entity.md")
	if err := os.WriteFile(abs, []byte(entityBody), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runProperty(t, "refs", "email")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(out, "user") {
		t.Errorf("expected fallback to filename slug, got %q", out)
	}
}
