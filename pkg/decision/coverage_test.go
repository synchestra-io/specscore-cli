package decision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// scaffold.go:36 — titleCaseFromSlug "" (empty-part) branch
// =============================================================================
//
// The slug splitter visits empty parts only when the slug contains a
// leading/trailing/double-hyphen — ValidateSlug rejects those before
// the title helper runs. The helper itself is exported only via Scaffold,
// so we call it directly to exercise the empty-part skip on line 39.
// The slug must use the validator's rules; we use the unexported helper.

func TestTitleCaseFromSlug_HandlesEmptyParts(t *testing.T) {
	// A slug formed via strings.Split with embedded empties exercises the
	// `if p == "" { continue }` branch on line 39-41.
	got := titleCaseFromSlug("foo--bar")
	want := "Foo  Bar"
	if got != want {
		t.Errorf("titleCaseFromSlug(%q) = %q, want %q", "foo--bar", got, want)
	}
}

// =============================================================================
// scaffold.go:49 — NextNumber filename that doesn't match the NNNN regex
// =============================================================================
//
// The active and archived dirs may contain files whose names don't match
// `^(\d{4})-`. The continue at line 65-66 is reached then.

func TestNextNumber_SkipsNonMatchingFilenames(t *testing.T) {
	root := t.TempDir()
	decisionsDir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-matching filenames must be skipped silently.
	for _, name := range []string{"README.md", "draft.md", "abc-123-foo.md"} {
		if err := os.WriteFile(filepath.Join(decisionsDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// And one matching file so we can verify highest+1 logic still works.
	if err := os.WriteFile(filepath.Join(decisionsDir, "0007-real.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := NextNumber(root)
	if err != nil {
		t.Fatal(err)
	}
	if n != 8 {
		t.Errorf("NextNumber = %d, want 8", n)
	}
}

func TestNextNumber_SkipsSubdirectoryEntries(t *testing.T) {
	// A non-archived subdirectory under decisions/ must be skipped
	// (line 61-63: if e.IsDir() { continue }).
	root := t.TempDir()
	decisionsDir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(filepath.Join(decisionsDir, "0001-something"), 0o755); err != nil {
		t.Fatal(err)
	}
	n, err := NextNumber(root)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("NextNumber = %d, want 1 (dirs ignored)", n)
	}
}

// =============================================================================
// scaffold.go:80 — AllNumbers (0% coverage)
// =============================================================================

func TestAllNumbers_EmptyTree(t *testing.T) {
	root := t.TempDir()
	if got := AllNumbers(root); got != nil {
		t.Errorf("AllNumbers on empty root = %v, want nil", got)
	}
}

func TestAllNumbers_ActiveOnly(t *testing.T) {
	root := t.TempDir()
	decisionsDir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(decisionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "0003-c.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "0001-a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := AllNumbers(root)
	want := []int{1, 3}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("AllNumbers = %v, want %v", got, want)
	}
}

func TestAllNumbers_IncludesArchived(t *testing.T) {
	root := t.TempDir()
	decisionsDir := filepath.Join(root, "decisions")
	archivedDir := filepath.Join(decisionsDir, "archived")
	if err := os.MkdirAll(archivedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "0002-active.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archivedDir, "0001-old.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := AllNumbers(root)
	want := []int{1, 2}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("AllNumbers = %v, want %v", got, want)
	}
}

func TestAllNumbers_SkipsNonMatchingFilenamesAndSubdirs(t *testing.T) {
	root := t.TempDir()
	decisionsDir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(filepath.Join(decisionsDir, "0001-folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "stale.md", "0002-real.md"} {
		if err := os.WriteFile(filepath.Join(decisionsDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := AllNumbers(root)
	want := []int{2}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("AllNumbers = %v, want %v (dirs and non-NNNN files skipped)", got, want)
	}
}

// =============================================================================
// scaffold.go:108 — Scaffold default-date branch (line 124-126)
// =============================================================================
//
// When opts.Date is empty, Scaffold injects time.Now().UTC().Format("2006-01-02").
// All existing tests pass an explicit Date, so this branch is uncovered.

func TestScaffold_DefaultDateInjected(t *testing.T) {
	body, err := Scaffold(ScaffoldOptions{
		Slug:  "auto-date",
		Owner: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	// The date line must be present and look like YYYY-MM-DD (no asserting
	// the actual value — that depends on system clock).
	if !strings.Contains(s, "**Date:** ") {
		t.Errorf("missing **Date:** line; got:\n%s", s)
	}
	// The injected default must not be empty or a dash.
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "**Date:** ") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "**Date:** "))
			if val == "" || val == "—" {
				t.Errorf("expected real default date, got %q", val)
			}
			// Sanity: shape is YYYY-MM-DD (10 chars, two hyphens).
			if len(val) != 10 || strings.Count(val, "-") != 2 {
				t.Errorf("default date %q does not match YYYY-MM-DD", val)
			}
		}
	}
}
