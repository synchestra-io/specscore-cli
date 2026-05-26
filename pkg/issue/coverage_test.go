package issue

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// DiscoverAll — walk error from unreadable subdirectory
// Makes a subdir unreadable so filepath.Walk returns a non-nil error via the
// callback returning walkErr, causing DiscoverAll to return an error.
// ---------------------------------------------------------------------------

func TestDiscoverAll_WalkError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root")
	}

	specRoot := t.TempDir()

	// Create a subdir that will be made unreadable to cause a walk error.
	subDir := filepath.Join(specRoot, "locked-dir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file inside to ensure the dir matters.
	if err := os.WriteFile(filepath.Join(subDir, "issue.md"), []byte("---\ntype: issue\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make the subdir unreadable so the walk fails when trying to enter it.
	if err := os.Chmod(subDir, 0o000); err != nil {
		t.Skip("cannot change permissions")
	}
	t.Cleanup(func() { _ = os.Chmod(subDir, 0o755) })

	_, err := DiscoverAll(specRoot)
	if err == nil {
		t.Error("expected error for unreadable subdir during walk")
	}
}

// ---------------------------------------------------------------------------
// DiscoverAll — filepath.Rel error path (lines 71-73 in discover.go)
// Overrides fpRel to return an error so the nil-return branch is covered.
// ---------------------------------------------------------------------------

func TestDiscoverAll_FpRelError(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(issuesDir, "rel-err.md"), []byte(minimalIssue("rel-err")), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := fpRel
	fpRel = func(basepath, targpath string) (string, error) {
		return "", errors.New("injected Rel error")
	}
	t.Cleanup(func() { fpRel = orig })

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatalf("DiscoverAll should not propagate fpRel errors: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverAll should skip files where fpRel errors, found %d", len(got))
	}
}
