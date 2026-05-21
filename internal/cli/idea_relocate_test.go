package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
)

// stageRelocateRepo creates a SpecScore-managed repo dir at
// <parent>/<name> with project.org=project.repo=<repoSlug>. Returns
// the repo's absolute path. The shared org=repo convention matches the
// single-org SpecScore workspaces these tests model; Task 4's cross-
// repo link rewrite reads project.org from the target repo, so setting
// it here lets every test fixture build well-formed GitHub URLs without
// per-test boilerplate.
func stageRelocateRepo(t *testing.T, parent, name, repoSlug string) string {
	t.Helper()
	root := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Join(root, "spec", "ideas", "seeds"), 0o755); err != nil {
		t.Fatalf("mkdir spec tree: %v", err)
	}
	yaml := "# SpecScore Repo Config Schema: https://specscore.md/repo-config\n" +
		"project:\n" +
		"  title: " + name + "\n" +
		"  org: " + repoSlug + "\n" +
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

// initGitRepoForTest runs git init + commit-all in root. After this
// call the working tree is clean from git's perspective.
func initGitRepoForTest(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), root, err, out)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", "-A")
	run("commit", "--no-gpg-sign", "-m", "initial")
}

// AC: preflight-source-dirty
func TestIdeaRelocateCLI_PreflightSourceDirty(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	writeIdeaFile(t, source, "foo")
	initGitRepoForTest(t, source)
	initGitRepoForTest(t, target)

	// Modify foo.md after the initial commit — unstaged dirty.
	if err := os.WriteFile(
		filepath.Join(source, "spec", "ideas", "foo.md"),
		[]byte("# Idea: foo\n\nedited after commit\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if got := exitCodeFromErr(t, err); got != exitcode.DirtyTree {
		t.Errorf("exit code: got %d want %d (DirtyTree)", got, exitcode.DirtyTree)
	}
	if !strings.Contains(err.Error(), "spec/ideas/foo.md") {
		t.Errorf("error should name the dirty source path; got: %v", err)
	}
}

// AC: preflight-sibling-with-references-dirty
func TestIdeaRelocateCLI_PreflightSiblingWithReferencesDirty(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	sib := stageRelocateRepo(t, parent, "sib", "sib")
	writeIdeaFile(t, source, "foo")

	// Sibling has a Feature that references the slug via markdown link.
	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "x"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refFile := filepath.Join(sib, "spec", "features", "x", "README.md")
	if err := os.WriteFile(refFile,
		[]byte("# Feature: X\n\nSee [the Idea](../../../src/spec/ideas/foo.md).\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	initGitRepoForTest(t, source)
	initGitRepoForTest(t, target)
	initGitRepoForTest(t, sib)

	// Now dirty the sibling reference file.
	if err := os.WriteFile(refFile,
		[]byte("# Feature: X\n\nSee [the Idea](../../../src/spec/ideas/foo.md). modified\n"), 0o644); err != nil {
		t.Fatalf("modify ref: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if got := exitCodeFromErr(t, err); got != exitcode.DirtyTree {
		t.Errorf("exit code: got %d want %d (DirtyTree)", got, exitcode.DirtyTree)
	}
	// Error message should name the sibling repo path AND the affected file.
	msg := err.Error()
	if !strings.Contains(msg, sib) {
		t.Errorf("error should name sibling repo %q; got: %s", sib, msg)
	}
	if !strings.Contains(msg, filepath.Join("spec", "features", "x", "README.md")) {
		t.Errorf("error should name sibling ref path; got: %s", msg)
	}
}

// Sanity: when everything is git-clean, preflight passes and the verb
// reaches Task 1's resolution-summary output. After Task 3 the verb
// also performs the file mutation, so the source artifact is gone
// after this call.
func TestIdeaRelocateCLI_PreflightCleanAllowsThrough(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	writeIdeaFile(t, source, "foo")
	initGitRepoForTest(t, source)
	initGitRepoForTest(t, target)

	stdout, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "resolved:") {
		t.Errorf("expected resolution-summary output, got: %s", stdout)
	}
}

// AC: destination-collision
func TestIdeaRelocateCLI_DestinationCollision(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	writeIdeaFile(t, source, "foo")
	// Pre-existing file at the destination → collision.
	writeIdeaFile(t, target, "foo")

	srcPath := filepath.Join(source, "spec", "ideas", "foo.md")
	tgtPath := filepath.Join(target, "spec", "ideas", "foo.md")
	srcBefore, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read src: %v", err)
	}
	tgtBefore, err := os.ReadFile(tgtPath)
	if err != nil {
		t.Fatalf("read tgt: %v", err)
	}

	_, _, err = runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if got := exitCodeFromErr(t, err); got != exitcode.Conflict {
		t.Errorf("exit code: got %d want %d (Conflict)", got, exitcode.Conflict)
	}
	srcAfter, _ := os.ReadFile(srcPath)
	tgtAfter, _ := os.ReadFile(tgtPath)
	if string(srcBefore) != string(srcAfter) {
		t.Errorf("source artifact must be unchanged on collision; before=%q after=%q", srcBefore, srcAfter)
	}
	if string(tgtBefore) != string(tgtAfter) {
		t.Errorf("target artifact must be unchanged on collision; before=%q after=%q", tgtBefore, tgtAfter)
	}
}

// AC: in-file-rewrite-org-rename
func TestIdeaRelocateCLI_InFileRewriteOrgRename(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	body := "# Idea: foo\n\nSee synchestra-io/specscore for the worked example, and synchestra-io/specscore-cli for the verb.\n"
	srcPath := filepath.Join(source, "spec", "ideas", "foo.md")
	if err := os.WriteFile(srcPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tgtPath := filepath.Join(target, "spec", "ideas", "foo.md")
	got, err := os.ReadFile(tgtPath)
	if err != nil {
		t.Fatalf("read tgt: %v", err)
	}
	if !strings.Contains(string(got), "specscore/specscore") {
		t.Errorf("expected synchestra-io/specscore → specscore/specscore; got:\n%s", got)
	}
	if !strings.Contains(string(got), "specscore/specscore-cli") {
		t.Errorf("expected synchestra-io/specscore-cli → specscore/specscore-cli; got:\n%s", got)
	}
	if strings.Contains(string(got), "synchestra-io/") {
		t.Errorf("expected all synchestra-io/ replaced; got:\n%s", got)
	}
	// Source artifact deleted post-copy.
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Errorf("source artifact must be removed after successful relocate; stat err: %v", err)
	}
}

// AC: in-file-rewrite-this-repo
func TestIdeaRelocateCLI_InFileRewriteThisRepo(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "src", "src")
	target := stageRelocateRepo(t, parent, "tgt", "tgt")
	body := "# Idea: foo\n\n" +
		"Existing artifacts in this repo are migrated by a one-shot script.\n\n" +
		"```\n" +
		"git -C this-repo log\n" +
		"this repo intact inside fence\n" +
		"```\n"
	srcPath := filepath.Join(source, "spec", "ideas", "foo.md")
	if err := os.WriteFile(srcPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=tgt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tgtPath := filepath.Join(target, "spec", "ideas", "foo.md")
	got, err := os.ReadFile(tgtPath)
	if err != nil {
		t.Fatalf("read tgt: %v", err)
	}
	if !strings.Contains(string(got), "Existing artifacts in tgt are migrated") {
		t.Errorf("expected 'this repo' rewritten to 'tgt' in body prose; got:\n%s", got)
	}
	if !strings.Contains(string(got), "git -C this-repo log") {
		t.Errorf("expected fenced 'this-repo' content preserved; got:\n%s", got)
	}
	if !strings.Contains(string(got), "this repo intact inside fence") {
		t.Errorf("expected fenced 'this repo' phrase preserved; got:\n%s", got)
	}
}

// AC: cross-repo-link-cleanup-markdown-link
func TestIdeaRelocateCLI_CrossRepoLinkCleanupMarkdownLink(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "specstudio-skills", "specstudio-skills")
	stageRelocateRepo(t, parent, "specscore", "specscore")
	sib := stageRelocateRepo(t, parent, "specscore-cli", "specscore-cli")
	writeIdeaFile(t, source, "foo")

	// Sibling repo has a Feature linking to the source artifact.
	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "x"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(sib, "spec", "features", "x", "README.md")
	if err := os.WriteFile(refPath,
		[]byte("# Feature: X\n\nSee [the Idea](../../../specstudio-skills/spec/ideas/foo.md) for context.\n"),
		0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=specscore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read ref after: %v", err)
	}
	wantURL := "https://github.com/specscore/specscore/blob/main/spec/ideas/foo.md"
	if !strings.Contains(string(got), wantURL) {
		t.Errorf("expected sibling link rewritten to %q; got:\n%s", wantURL, got)
	}
	if !strings.Contains(string(got), "[the Idea]") {
		t.Errorf("expected display text 'the Idea' preserved; got:\n%s", got)
	}
	if strings.Contains(string(got), "../../../specstudio-skills/spec/ideas/foo.md") {
		t.Errorf("expected old relative link replaced; got:\n%s", got)
	}
}

// AC: cross-repo-link-cleanup-preserves-slug-metadata
func TestIdeaRelocateCLI_CrossRepoLinkCleanupPreservesSlugMetadata(t *testing.T) {
	parent := t.TempDir()
	source := stageRelocateRepo(t, parent, "specstudio-skills", "specstudio-skills")
	stageRelocateRepo(t, parent, "specscore", "specscore")
	sib := stageRelocateRepo(t, parent, "specscore-cli", "specscore-cli")
	writeIdeaFile(t, source, "foo")

	// Sibling Feature has a slug-only metadata reference. After
	// relocate, that line must be byte-for-byte unchanged.
	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "y"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(sib, "spec", "features", "y", "README.md")
	original := "# Feature: Y\n\n**Source Ideas:** foo\n\nBody text.\n"
	if err := os.WriteFile(refPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	_, _, err := runIdeaRelocateCLI(t, source, "foo", "--to-repo=specscore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read ref after: %v", err)
	}
	if string(got) != original {
		t.Errorf("expected slug-only metadata line preserved;\nwant:\n%s\ngot:\n%s", original, got)
	}
}
