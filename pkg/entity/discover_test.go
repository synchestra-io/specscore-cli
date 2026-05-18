package entity

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// writeEntityFile writes a minimal entity-shaped file to path. Body content
// is not parsed by Discover, so a one-liner suffices.
func writeEntityFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscover_FindsEntityFiles(t *testing.T) {
	root := t.TempDir()
	writeEntityFile(t, filepath.Join(root, "features", "user", "user.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "shared", "money.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "order", "order.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "order", "line-item.entity.md"))

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 entities, got %d: %+v", len(got), got)
	}

	var slugs []string
	for _, d := range got {
		slugs = append(slugs, d.Slug)
		if !filepath.IsAbs(d.Path) {
			t.Errorf("Discovered.Path should be absolute, got %q", d.Path)
		}
	}
	sort.Strings(slugs)
	wantSlugs := []string{"line-item", "money", "order", "user"}
	for i, s := range wantSlugs {
		if slugs[i] != s {
			t.Errorf("slug[%d] = %q, want %q", i, slugs[i], s)
		}
	}
}

func TestDiscover_SkipsHiddenAndUnderscoreDirs(t *testing.T) {
	root := t.TempDir()
	writeEntityFile(t, filepath.Join(root, "features", "user", "user.entity.md"))
	// Hidden directory — must be skipped.
	writeEntityFile(t, filepath.Join(root, "features", ".hidden", "ghost.entity.md"))
	// Underscore-prefixed directory (e.g., _tests) — must be skipped.
	writeEntityFile(t, filepath.Join(root, "features", "user", "_tests", "fixture.entity.md"))
	// Nested hidden dir deeper in the tree.
	writeEntityFile(t, filepath.Join(root, "features", "user", ".cache", "stale.entity.md"))

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entity (only user), got %d: %+v", len(got), got)
	}
	if got[0].Slug != "user" {
		t.Errorf("slug = %q, want user", got[0].Slug)
	}
}

func TestDiscover_ReturnsEmptyWhenNoFeaturesDir(t *testing.T) {
	root := t.TempDir()
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestDiscover_IgnoresNonEntityMarkdown(t *testing.T) {
	root := t.TempDir()
	writeEntityFile(t, filepath.Join(root, "features", "user", "README.md"))
	writeEntityFile(t, filepath.Join(root, "features", "user", "user.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "user", "user.property.md"))

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected only the .entity.md, got %d: %+v", len(got), got)
	}
	if got[0].Slug != "user" {
		t.Errorf("slug = %q, want user", got[0].Slug)
	}
}

func TestDiscover_SortsByPath(t *testing.T) {
	// The exact ordering contract is not specified by the plan, but
	// Discover should be deterministic — equal inputs MUST yield equal
	// outputs across invocations.
	root := t.TempDir()
	writeEntityFile(t, filepath.Join(root, "features", "z", "z.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "a", "a.entity.md"))
	writeEntityFile(t, filepath.Join(root, "features", "m", "m.entity.md"))

	first, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("length mismatch %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("Discover non-deterministic at i=%d: %+v vs %+v", i, first[i], second[i])
		}
	}
}
