package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
)

// stageRelocateRepo creates a SpecScore-managed repo dir at
// <parent>/<name> with project.repo=<repoSlug>. Returns the repo's
// absolute path. The spec tree includes spec/ideas/, spec/ideas/seeds/,
// and lint-friendly index READMEs.
func stageRelocateRepo(t *testing.T, parent, name, repoSlug string) string {
	t.Helper()
	root := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Join(root, "spec", "ideas", "seeds"), 0o755); err != nil {
		t.Fatalf("mkdir spec tree: %v", err)
	}
	yaml := "# SpecScore Repo Config Schema: https://specscore.md/repo-config\n" +
		"project:\n" +
		"  title: " + name + "\n" +
		"  repo: " + repoSlug + "\n"
	if err := os.WriteFile(filepath.Join(root, "specscore.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
	return root
}

func writeIdeaFile(t *testing.T, repoRoot, slug string) {
	t.Helper()
	body := "# Idea: " + slug + "\n\nplaceholder\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write idea: %v", err)
	}
}

func writeSeedFile(t *testing.T, repoRoot, slug string) {
	t.Helper()
	body := "# Seed: " + slug + "\n\nplaceholder\n"
	if err := os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "seeds", slug+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
}

func exitCodeFromErr(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	return ec.ExitCode()
}

// runIdeaRelocateCLI invokes `idea relocate` via the cobra command tree.
// Uses a shared root parent dir so sibling-dir scan finds the configured
// siblings.
func runIdeaRelocateCLI(t *testing.T, sourceDir string, args ...string) (string, string, error) {
	t.Helper()
	withCwd(t, sourceDir)
	full := append([]string{"relocate"}, args...)
	return runIdea(t, full...)
}

// -------- AC tests --------

// AC: ambiguous-slug-rejected
func TestIdeaRelocateCLI_AmbiguousSlugRejected(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	stageRelocateRepo(t, parent, "tgt", "tgt")
	writeIdeaFile(t, source, "foo")
	writeSeedFile(t, source, "foo")

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if got := exitCodeFromErr(t, err); got != exitcode.AmbiguousSlug {
		t.Errorf("exit code: got %d want %d (AmbiguousSlug)", got, exitcode.AmbiguousSlug)
	}
}

// AC: slug-not-found
func TestIdeaRelocateCLI_SlugNotFound(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	stageRelocateRepo(t, parent, "tgt", "tgt")

	_, _, err := runIdeaRelocateCLI(t, source, "nonexistent", "--to-repo=tgt")
	if got := exitCodeFromErr(t, err); got != exitcode.NotFound {
		t.Errorf("exit code: got %d want %d (NotFound)", got, exitcode.NotFound)
	}
}

// AC: to-repo-slug-form-resolves-via-sibling-scan
func TestIdeaRelocateCLI_ToRepoSlugFormResolvesViaScan(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "specstudio-skills", "specstudio-skills")
	target := stageRelocateRepo(t, parent, "specscore", "specscore")
	stageRelocateRepo(t, parent, "specscore-cli", "specscore-cli") // bystander
	writeIdeaFile(t, source, "foo")

	stdout, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=specscore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Task 1 scaffold output names both the source path and the resolved target dir.
	if !strings.Contains(stdout, target) {
		t.Errorf("stdout should name the target dir %q; got: %s", target, stdout)
	}
}

// AC: to-repo-path-form-bypasses-scan
func TestIdeaRelocateCLI_ToRepoPathFormBypassesScan(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt-name-differs-from-dirname")
	writeIdeaFile(t, source, "foo")

	// Path form: contains "/". Should be honored verbatim even though
	// the target's project.repo differs from "tgt".
	stdout, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=../tgt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// macOS /var ↔ /private/var: compare via EvalSymlinks.
	wantAbs, _ := filepath.EvalSymlinks(target)
	if !strings.Contains(stdout, wantAbs) && !strings.Contains(stdout, target) {
		t.Errorf("stdout should name target dir (%s or %s); got: %s",
			target, wantAbs, stdout)
	}
}

// AC: to-repo-without-specscore-yaml-rejected
func TestIdeaRelocateCLI_ToRepoWithoutSpecScoreYamlRejected(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	bareDir := filepath.Join(parent, "bare")
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	writeIdeaFile(t, source, "foo")

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=../bare")
	if got := exitCodeFromErr(t, err); got != exitcode.TargetNotSpecScore {
		t.Errorf("exit code: got %d want %d (TargetNotSpecScore)", got, exitcode.TargetNotSpecScore)
	}
}

// AC: to-repo-slug-multiple-matches-rejected
func TestIdeaRelocateCLI_ToRepoSlugMultipleMatchesRejected(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	stageRelocateRepo(t, parent, "specscore-a", "specscore")
	stageRelocateRepo(t, parent, "specscore-b", "specscore")
	writeIdeaFile(t, source, "foo")

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=specscore")
	if got := exitCodeFromErr(t, err); got != exitcode.InvalidArgs {
		t.Errorf("exit code: got %d want %d (InvalidArgs)", got, exitcode.InvalidArgs)
	}
}

// Smoke: --to-repo is required at flag-parse time.
func TestIdeaRelocateCLI_MissingToRepoRejected(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	writeIdeaFile(t, source, "foo")

	_, _, err := runIdeaRelocateCLI(t, source, "foo")
	if err == nil {
		t.Fatal("expected error when --to-repo is missing")
	}
}
