package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// rewrite.go — lines 100-102: writeFileAtomic fails during Rewrite
// (already partially covered by TestWriteFileAtomic_ReadOnlyDir, but that
// test fails as root. Use injected osCreateTemp failure instead.)
// ---------------------------------------------------------------------------

func TestRewrite_WriteFileAtomicError_Stub(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.md")
	content := "# Title\n\n**Status:** Draft\n\nBody.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := osCreateTemp
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return nil, fmt.Errorf("injected createtemp error")
	}
	t.Cleanup(func() { osCreateTemp = orig })

	_, err := Rewrite(path, "Approved")
	if err == nil {
		t.Fatal("expected error from writeFileAtomic failure during Rewrite")
	}
	if !strings.Contains(err.Error(), "injected createtemp error") {
		t.Errorf("unexpected error: %v", err)
	}

	// Original content should be unchanged (the file was read but the write failed).
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "Draft") {
		t.Error("original file should still contain Draft")
	}
}

// ---------------------------------------------------------------------------
// writeFileAtomic — ReadOnlyDir (stub-based, works as root)
// ---------------------------------------------------------------------------

func TestWriteFileAtomic_ReadOnlyDir_Stub(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.md")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := osCreateTemp
	osCreateTemp = func(dir, pattern string) (*os.File, error) {
		return nil, fmt.Errorf("injected createtemp error")
	}
	t.Cleanup(func() { osCreateTemp = orig })

	err := writeFileAtomic(path, []byte("new"))
	if err == nil {
		t.Fatal("expected error when createtemp is stubbed to fail")
	}
}
