package sourceref

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// scan.go — lines 93-95: Walk callback receives non-nil err
// The walk callback in ExpandGlobPattern does `return nil` on err != nil,
// so the error is swallowed. Exercise via stub.
// ---------------------------------------------------------------------------

func TestExpandGlobPattern_WalkCallbackError_Stub(t *testing.T) {
	orig := filepathWalkFn
	filepathWalkFn = func(root string, fn filepath.WalkFunc) error {
		// Simulate walk callback receiving an error — the code returns nil,
		// so the walk continues but files are skipped.
		_ = fn("/fake/error-path", nil, errors.New("injected walk error"))
		// Also call with a valid file to make sure it processes correctly.
		info, _ := os.Stat(".")
		_ = fn(".", info, nil)
		return nil
	}
	t.Cleanup(func() { filepathWalkFn = orig })

	matches, err := ExpandGlobPattern("**/*")
	if err != nil {
		t.Fatalf("ExpandGlobPattern: %v", err)
	}
	// The error path returns nil (swallowed), so no error is expected.
	// Matches may be empty since we only gave it a directory entry.
	_ = matches
}
