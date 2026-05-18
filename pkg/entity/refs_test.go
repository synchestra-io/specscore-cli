package entity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRef_RelativeLocalPath(t *testing.T) {
	root := t.TempDir()
	entityPath := filepath.Join(root, "features", "user", "user.entity.md")
	propPath := filepath.Join(root, "features", "shared", "email.property.md")
	if err := os.MkdirAll(filepath.Dir(entityPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(propPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entityPath, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(propPath, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, isLocal, err := ResolveRef(root, entityPath, "../shared/email.property.md")
	if err != nil {
		t.Fatal(err)
	}
	if !isLocal {
		t.Errorf("isLocal = false, want true for an in-tree relative ref")
	}
	gotAbs, _ := filepath.Abs(resolved)
	wantAbs, _ := filepath.Abs(propPath)
	if gotAbs != wantAbs {
		t.Errorf("resolved = %q, want %q", gotAbs, wantAbs)
	}
}

func TestResolveRef_URLForm(t *testing.T) {
	root := t.TempDir()
	entityPath := filepath.Join(root, "features", "user", "user.entity.md")
	resolved, isLocal, err := ResolveRef(root, entityPath, "https://specscore.md/some/property")
	if err != nil {
		t.Fatalf("URL ref MUST NOT be an error (cross-repo @import not yet shipped): %v", err)
	}
	if resolved != "" {
		t.Errorf("resolved = %q, want empty for URL form", resolved)
	}
	if isLocal {
		t.Error("isLocal = true, want false for URL form")
	}
}

func TestResolveRef_OutsideSpecRoot(t *testing.T) {
	root := t.TempDir()
	entityPath := filepath.Join(root, "features", "user", "user.entity.md")
	// A relative ref that escapes the spec root must still resolve to
	// an absolute path, but isLocal must be false.
	_, isLocal, err := ResolveRef(root, entityPath, "../../../outside.property.md")
	if err != nil {
		t.Fatal(err)
	}
	if isLocal {
		t.Error("isLocal = true for an out-of-tree relative ref, want false")
	}
}

func TestResolveInherits_RelativeLocalPath(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "features", "user", "admin-user.entity.md")
	parent := filepath.Join(root, "features", "user", "user.entity.md")
	if err := os.MkdirAll(filepath.Dir(child), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("# stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, isLocal, err := ResolveInherits(root, child, "./user.entity.md")
	if err != nil {
		t.Fatal(err)
	}
	if !isLocal {
		t.Errorf("isLocal = false, want true for in-tree inherits")
	}
	gotAbs, _ := filepath.Abs(resolved)
	wantAbs, _ := filepath.Abs(parent)
	if gotAbs != wantAbs {
		t.Errorf("resolved = %q, want %q", gotAbs, wantAbs)
	}
}

func TestResolveInherits_URLForm(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "features", "user", "admin-user.entity.md")
	resolved, isLocal, err := ResolveInherits(root, child, "https://specscore.md/some/entity")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != "" || isLocal {
		t.Errorf("URL inherits should yield (\"\", false, nil), got (%q, %v)", resolved, isLocal)
	}
}
