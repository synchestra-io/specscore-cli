package issue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// ---------------------------------------------------------------------------
// discover.go — lines 48-50: Walk callback receives walkErr (stub-based)
// ---------------------------------------------------------------------------

func TestDiscoverAll_WalkCallbackError_Stub(t *testing.T) {
	specRoot := t.TempDir()
	if err := os.MkdirAll(specRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	orig := filepathWalkFn
	filepathWalkFn = func(root string, fn filepath.WalkFunc) error {
		return fn("/fake/path", nil, errors.New("injected walk callback error"))
	}
	t.Cleanup(func() { filepathWalkFn = orig })

	_, err := DiscoverAll(specRoot)
	if err == nil {
		t.Fatal("expected error from walk callback error")
	}
	if err.Error() != "injected walk callback error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// discover.go — lines 67-69: Parse error → skip (return nil)
// This is already tested indirectly via TestDiscoverAll_UnreadableFile, but
// that test fails as root. Exercise via a non-.md file that happens to be
// named .md but contains invalid content.
// ---------------------------------------------------------------------------

func TestDiscoverAll_ParseError_Skip(t *testing.T) {
	specRoot := t.TempDir()
	issuesDir := filepath.Join(specRoot, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write an .md file that is not a valid issue (no frontmatter)
	if err := os.WriteFile(filepath.Join(issuesDir, "bad.md"), []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverAll(specRoot)
	if err != nil {
		t.Fatal(err)
	}
	// bad.md has no `type: issue` frontmatter, so it should be skipped
	if len(got) != 0 {
		t.Errorf("expected 0 discovered (non-issue), got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// discover.go — lines 88-90: Walk returns non-nil error (stub-based)
// ---------------------------------------------------------------------------

func TestDiscoverAll_WalkReturnsError_Stub(t *testing.T) {
	specRoot := t.TempDir()
	if err := os.MkdirAll(specRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	orig := filepathWalkFn
	filepathWalkFn = func(root string, fn filepath.WalkFunc) error {
		return errors.New("injected walk error")
	}
	t.Cleanup(func() { filepathWalkFn = orig })

	_, err := DiscoverAll(specRoot)
	if err == nil {
		t.Fatal("expected error from injected Walk failure")
	}
}

// ---------------------------------------------------------------------------
// transitions.go — lines 119-121: DiscoverAll returns error (stub-based)
// ---------------------------------------------------------------------------

func TestChangeStatus_DiscoverAllError_Stub(t *testing.T) {
	root := setupIssueProject(t, "discover-err", "open", "high")

	orig := discoverAllFn
	discoverAllFn = func(specRoot string) ([]Discovered, error) {
		return nil, fmt.Errorf("injected discover error")
	}
	t.Cleanup(func() { discoverAllFn = orig })

	_, err := ChangeStatus(ChangeStatusOptions{
		SpecRoot: root,
		Slug:     "discover-err",
		To:       "investigating",
	})
	if err == nil {
		t.Fatal("expected error from injected DiscoverAll failure")
	}
	var ecErr *exitcode.Error
	if !errors.As(err, &ecErr) {
		t.Fatalf("error type = %T; want *exitcode.Error", err)
	}
	if ecErr.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code = %d; want %d (Unexpected)", ecErr.ExitCode(), exitcode.Unexpected)
	}
}
