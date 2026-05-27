package idearelocate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// ---------------------------------------------------------------------------
// resolve.go:177 — discoverSiblings os.ReadDir(parent) error
// resolve.go:135 — resolveTargetBySlug propagates discoverSiblings error
// ---------------------------------------------------------------------------

func TestResolveTargetRepo_SlugDiscoverSiblingsReadDirError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}

	grandparent := t.TempDir()
	parent := filepath.Join(grandparent, "workspace")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	specRoot := filepath.Join(parent, "my-repo")
	if err := os.MkdirAll(specRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(parent, 0o000); err != nil {
		t.Skip("cannot change permissions")
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	_, err := ResolveTargetRepo(specRoot, "some-slug")
	if err == nil {
		t.Error("expected error when parent directory is unreadable")
	}
}

// ---------------------------------------------------------------------------
// resolve.go:199 — discoverSiblings EvalSymlinks error (broken symlink)
// ---------------------------------------------------------------------------

func TestDiscoverSiblings_BrokenSymlink(t *testing.T) {
	grandparent := t.TempDir()
	parent := filepath.Join(grandparent, "workspace")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	specRoot := filepath.Join(parent, "my-repo")
	if err := os.MkdirAll(specRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	brokenLink := filepath.Join(parent, "broken-link")
	if err := os.Symlink(filepath.Join(parent, "does-not-exist"), brokenLink); err != nil {
		t.Skip("cannot create symlink")
	}

	siblings, err := discoverSiblings(specRoot)
	if err != nil {
		t.Errorf("expected no error with broken symlink, got: %v", err)
	}
	for _, s := range siblings {
		if s.Path == brokenLink {
			t.Errorf("broken symlink should be skipped, but found in siblings: %v", s)
		}
	}
}

// ---------------------------------------------------------------------------
// preflight.go:84 — FindReferences Walk: os.ReadFile error (unreadable file)
// ---------------------------------------------------------------------------

func TestFindReferences_UnreadableMdFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}

	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "spec", "features")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(specDir, "hidden.md")
	if err := os.WriteFile(filePath, []byte("**Source Ideas:** my-slug\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filePath, 0o000); err != nil {
		t.Skip("cannot change permissions")
	}
	t.Cleanup(func() { _ = os.Chmod(filePath, 0o644) })

	hits, err := FindReferences(tmp, "my-slug")
	if err != nil {
		t.Errorf("expected no error for unreadable file, got: %v", err)
	}
	for _, h := range hits {
		if filepath.Base(h) == "hidden.md" {
			t.Errorf("unreadable file should be skipped, got hit: %s", h)
		}
	}
}

// ===========================================================================
// Seam-based coverage tests for the remaining 11 uncovered blocks.
// Each test swaps one package-level var, calls the target function, and
// restores the original via t.Cleanup.
// ===========================================================================

// ---------------------------------------------------------------------------
// Block 1: commit.go:283 — commitRepo: gitRevParseHEADFn fails
// ---------------------------------------------------------------------------

func TestCommitRepo_RevParseHEADError(t *testing.T) {
	orig := gitRevParseHEADFn
	gitRevParseHEADFn = func(string) (string, error) {
		return "", errors.New("injected rev-parse error")
	}
	t.Cleanup(func() { gitRevParseHEADFn = orig })

	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	_ = os.MkdirAll(filepath.Join(repoRoot, "spec"), 0o755)
	_ = os.WriteFile(filepath.Join(repoRoot, "spec", "x.md"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	// Stage a new file so git commit succeeds.
	_ = os.WriteFile(filepath.Join(repoRoot, "spec", "y.md"), []byte("y"), 0o644)
	_ = stagePaths(repoRoot, []string{"spec/y.md"})

	sha, _, err := commitRepo(repoRoot, "test commit")
	if err == nil {
		t.Fatal("expected error from rev-parse seam")
	}
	if sha != "" {
		t.Errorf("expected empty SHA, got %q", sha)
	}
}

// ---------------------------------------------------------------------------
// commit.go:276 — defaultGitRevParseHEAD: rev-parse fails on non-git dir
// ---------------------------------------------------------------------------

func TestDefaultGitRevParseHEAD_Error(t *testing.T) {
	tmp := t.TempDir() // not a git repo
	_, err := defaultGitRevParseHEAD(tmp)
	if err == nil {
		t.Fatal("expected error from rev-parse on non-git dir")
	}
}

// ---------------------------------------------------------------------------
// Block 2: linkcleanup.go:119 — UpdateCrossRepoLinks: Walk returns error
// ---------------------------------------------------------------------------

func TestUpdateCrossRepoLinks_WalkError(t *testing.T) {
	orig := filepathWalkFn
	filepathWalkFn = func(string, filepath.WalkFunc) error {
		return errors.New("injected Walk error")
	}
	t.Cleanup(func() { filepathWalkFn = orig })

	parent := t.TempDir()
	repo := stageRepo(t, parent, "repo", "repo")
	target := TargetRepo{Path: repo, RepoName: "repo", Org: "org"}

	_, err := UpdateCrossRepoLinks([]TargetRepo{target}, target, "slug", "spec/ideas/slug.md")
	if err == nil {
		t.Fatal("expected error from Walk seam")
	}
}

// ---------------------------------------------------------------------------
// Block 3: mutate.go:37 — ApplyMutation: filepathRelFn fails
// ---------------------------------------------------------------------------

func TestApplyMutation_FilepathRelError(t *testing.T) {
	orig := filepathRelFn
	filepathRelFn = func(string, string) (string, error) {
		return "", errors.New("injected Rel error")
	}
	t.Cleanup(func() { filepathRelFn = orig })

	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")
	writeIdea(t, source, "rel-err", "body")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "rel-err.md"),
		Kind: KindIdea,
	}
	_, err := ApplyMutation(source, artifact, TargetRepo{Path: target, RepoName: "tgt"})
	if err == nil {
		t.Fatal("expected error from Rel seam")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

// ---------------------------------------------------------------------------
// Block 4: preflight.go:30 — IsPathClean: git status fails
// Inject isGitRepoFn to return true for a non-git dir so the subsequent
// git status command exits non-zero.
// ---------------------------------------------------------------------------

func TestIsPathClean_GitStatusOutputError(t *testing.T) {
	orig := isGitRepoFn
	isGitRepoFn = func(string) bool { return true }
	t.Cleanup(func() { isGitRepoFn = orig })

	tmp := t.TempDir() // not a git repo
	_, err := IsPathClean(tmp, "any-file.md")
	if err == nil {
		t.Fatal("expected error from git status on non-git dir")
	}
}

// ---------------------------------------------------------------------------
// Block 5: preflight.go:91 — FindReferences: filepathRelFn fails
// ---------------------------------------------------------------------------

func TestFindReferences_FilepathRelError(t *testing.T) {
	orig := filepathRelFn
	filepathRelFn = func(string, string) (string, error) {
		return "", errors.New("injected Rel error")
	}
	t.Cleanup(func() { filepathRelFn = orig })

	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "spec", "ideas")
	_ = os.MkdirAll(specDir, 0o755)
	_ = os.WriteFile(filepath.Join(specDir, "ref.md"), []byte("**Source Ideas:** test-slug\n"), 0o644)

	hits, err := FindReferences(tmp, "test-slug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The file should be skipped (not in hits) due to Rel error
	if len(hits) != 0 {
		t.Errorf("expected 0 hits when Rel fails, got %v", hits)
	}
}

// ---------------------------------------------------------------------------
// Block 6: preflight.go:97 — FindReferences: Walk returns error
// ---------------------------------------------------------------------------

func TestFindReferences_WalkReturnsError(t *testing.T) {
	orig := filepathWalkFn
	filepathWalkFn = func(string, filepath.WalkFunc) error {
		return errors.New("injected Walk error")
	}
	t.Cleanup(func() { filepathWalkFn = orig })

	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "spec"), 0o755)

	_, err := FindReferences(tmp, "any-slug")
	if err == nil {
		t.Fatal("expected error from Walk seam")
	}
}

// ---------------------------------------------------------------------------
// Block 7: preflight.go:144 — CheckPreflight: isPathCleanFn returns error
// ---------------------------------------------------------------------------

func TestCheckPreflight_IsPathCleanError(t *testing.T) {
	orig := isPathCleanFn
	isPathCleanFn = func(string, string) (bool, error) {
		return false, errors.New("injected IsPathClean error")
	}
	t.Cleanup(func() { isPathCleanFn = orig })

	subjects := []PreflightSubject{
		{RepoRoot: "/any", RelPath: "spec/ideas/foo.md"},
	}
	_, err := CheckPreflight(subjects)
	if err == nil {
		t.Fatal("expected error from IsPathClean seam")
	}
}

// ---------------------------------------------------------------------------
// Block 8: preflight.go:182 — PreflightSubjectsForRelocate: findReferencesFn
//          returns error
// ---------------------------------------------------------------------------

func TestPreflightSubjectsForRelocate_FindReferencesError(t *testing.T) {
	orig := findReferencesFn
	findReferencesFn = func(string, string) ([]string, error) {
		return nil, errors.New("injected FindReferences error")
	}
	t.Cleanup(func() { findReferencesFn = orig })

	_, err := PreflightSubjectsForRelocate(
		"/src", "spec/ideas/foo.md",
		"/tgt", "spec/ideas/foo.md",
		[]TargetRepo{{Path: "/sib", RepoName: "sib"}},
		"foo",
	)
	if err == nil {
		t.Fatal("expected error from FindReferences seam")
	}
}

// ---------------------------------------------------------------------------
// Block 9: resolve.go:167 — discoverSiblings: filepathAbsFn fails
// ---------------------------------------------------------------------------

func TestDiscoverSiblings_FilepathAbsError(t *testing.T) {
	orig := filepathAbsFn
	filepathAbsFn = func(string) (string, error) {
		return "", errors.New("injected Abs error")
	}
	t.Cleanup(func() { filepathAbsFn = orig })

	_, err := discoverSiblings("anything")
	if err == nil {
		t.Fatal("expected error from Abs seam")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

// ---------------------------------------------------------------------------
// Block 10: resolve.go:171 — discoverSiblings: parent == absSource
//           (filesystem-root edge case)
// ---------------------------------------------------------------------------

func TestDiscoverSiblings_FilesystemRootBranch(t *testing.T) {
	orig := filepathAbsFn
	filepathAbsFn = func(string) (string, error) {
		return "/", nil // filepath.Dir("/") == "/" → triggers parent==absSource
	}
	t.Cleanup(func() { filepathAbsFn = orig })

	siblings, err := discoverSiblings("anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// readSelfAsSibling("/") returns nil — no specscore.yaml at /
	if len(siblings) != 0 {
		t.Errorf("expected 0 siblings for fs root, got %d", len(siblings))
	}
}

// ---------------------------------------------------------------------------
// Block 11: resolve.go:194 — discoverSiblings: osLstatFn fails
// ---------------------------------------------------------------------------

func TestDiscoverSiblings_LstatError(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "real", "real")

	orig := osLstatFn
	osLstatFn = func(string) (os.FileInfo, error) {
		return nil, errors.New("injected Lstat error")
	}
	t.Cleanup(func() { osLstatFn = orig })

	specRoot := filepath.Join(parent, "real")
	siblings, err := discoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All entries fail Lstat → no siblings found
	if len(siblings) != 0 {
		t.Errorf("expected 0 siblings when Lstat fails, got %d", len(siblings))
	}
}
