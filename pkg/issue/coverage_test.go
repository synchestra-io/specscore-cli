package issue

import (
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
