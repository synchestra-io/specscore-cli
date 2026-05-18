package property

import (
	"path/filepath"
	"testing"
)

func TestDiscover_FindsPropertyFiles(t *testing.T) {
	root := filepath.Join("_testdata", "walktree")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	// Expect email and money only; the hidden and underscore dirs must be skipped.
	wantSlugs := []string{"email", "money"}
	if len(got) != len(wantSlugs) {
		t.Fatalf("got %d discovered, want %d: %+v", len(got), len(wantSlugs), got)
	}
	for i, want := range wantSlugs {
		if got[i].Slug != want {
			t.Errorf("got[%d].Slug = %q, want %q", i, got[i].Slug, want)
		}
		if got[i].Path == "" {
			t.Errorf("got[%d].Path is empty", i)
		}
	}
}

func TestDiscover_MissingSpecRoot(t *testing.T) {
	// A non-existent specRoot must yield no error and an empty result.
	got, err := Discover(filepath.Join("_testdata", "does-not-exist"))
	if err != nil {
		t.Fatalf("Discover on missing root: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %+v", got)
	}
}

func TestDiscover_SkipsNonPropertyFiles(t *testing.T) {
	// The fixture _testdata dir at the top contains a mix of .property.md
	// files at the root level. Discover walks spec/features/** so calling it
	// against _testdata directly (which has no spec/features/ subtree)
	// returns nothing.
	got, err := Discover("_testdata")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty (no spec/features/ subtree), got %+v", got)
	}
}

func TestDiscover_SortedBySlug(t *testing.T) {
	root := filepath.Join("_testdata", "walktree")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Slug > got[i].Slug {
			t.Errorf("results not sorted: %q before %q", got[i-1].Slug, got[i].Slug)
		}
	}
}
