package entity

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// walk.go — lines 12-13: Discover fails → Walk returns error (stub-based)
// ---------------------------------------------------------------------------

func TestWalk_DiscoverError_Stub(t *testing.T) {
	origDiscover := discoverFn
	t.Cleanup(func() { discoverFn = origDiscover })
	discoverFn = func(specRoot string) ([]Discovered, error) {
		return nil, errors.New("injected discover error")
	}

	err := Walk(t.TempDir(), func(d *Doc) error { return nil })
	if err == nil {
		t.Fatal("expected Walk to surface injected Discover error")
	}
	if err.Error() != "injected discover error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// walk.go — lines 17-18: Parse fails → Walk returns error (stub-based)
// ---------------------------------------------------------------------------

func TestWalk_ParseError_Stub(t *testing.T) {
	origDiscover := discoverFn
	origParse := parseFn
	t.Cleanup(func() { discoverFn = origDiscover; parseFn = origParse })

	discoverFn = func(specRoot string) ([]Discovered, error) {
		return []Discovered{{Slug: "bad", Path: "/fake/bad.entity.md"}}, nil
	}
	parseFn = func(path string) (*Doc, error) {
		return nil, errors.New("injected parse error")
	}

	err := Walk(t.TempDir(), func(d *Doc) error { return nil })
	if err == nil {
		t.Fatal("expected Walk to surface injected Parse error")
	}
	if err.Error() != "injected parse error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// discover.go — line 39: walkErr non-nil in callback (stub-based)
// ---------------------------------------------------------------------------

func TestDiscover_WalkCallbackError_Stub(t *testing.T) {
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origWalk := filepathWalkFn
	t.Cleanup(func() { filepathWalkFn = origWalk })

	filepathWalkFn = func(root string, fn filepath.WalkFunc) error {
		// Simulate the Walk callback receiving a non-nil error for a path.
		return fn("/fake/path", nil, errors.New("injected walk callback error"))
	}

	_, err := Discover(root)
	if err == nil {
		t.Fatal("expected Discover to return the walk callback error")
	}
	if err.Error() != "injected walk callback error" {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// discover.go — lines 65-66: Walk returns non-nil error (stub-based)
// ---------------------------------------------------------------------------

func TestDiscover_WalkReturnsError_Stub(t *testing.T) {
	root := t.TempDir()
	featuresDir := filepath.Join(root, "features")
	if err := os.MkdirAll(featuresDir, 0o755); err != nil {
		t.Fatal(err)
	}

	origWalk := filepathWalkFn
	t.Cleanup(func() { filepathWalkFn = origWalk })

	filepathWalkFn = func(root string, fn filepath.WalkFunc) error {
		return errors.New("injected walk error")
	}

	_, err := Discover(root)
	if err == nil {
		t.Fatal("expected Discover to return the walk error")
	}
	if err.Error() != "injected walk error" {
		t.Errorf("unexpected error: %v", err)
	}
}
