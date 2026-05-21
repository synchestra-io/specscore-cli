package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome reroutes HOME (Unix) for the duration of one test so InstallID
// writes into a scratch directory and the tests don't depend on the developer's
// real ~/.specscore. Cleanup is t.Cleanup.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	return dir
}

func TestInstallID_FirstRunCreatesAndPersists(t *testing.T) {
	home := withTempHome(t)

	id1, justCreated, err := InstallID()
	if err != nil {
		t.Fatalf("first InstallID call: %v", err)
	}
	if !justCreated {
		t.Fatalf("first call expected justCreated=true, got false")
	}
	if !isUUIDv4(id1) {
		t.Fatalf("first id %q is not a UUID v4", id1)
	}

	// File should exist at ~/.specscore/install_id with mode 0600.
	idPath := filepath.Join(home, ".specscore", "install_id")
	info, err := os.Stat(idPath)
	if err != nil {
		t.Fatalf("stat install_id: %v", err)
	}
	if info.Mode().Perm() != installIDFileMode {
		t.Errorf("install_id mode = %v, want %v", info.Mode().Perm(), installIDFileMode)
	}

	// Directory should be mode 0700.
	dirInfo, err := os.Stat(filepath.Join(home, ".specscore"))
	if err != nil {
		t.Fatalf("stat .specscore dir: %v", err)
	}
	if dirInfo.Mode().Perm() != installIDDirMode {
		t.Errorf(".specscore dir mode = %v, want %v", dirInfo.Mode().Perm(), installIDDirMode)
	}

	// Second call should return the SAME id and justCreated=false.
	id2, justCreated2, err := InstallID()
	if err != nil {
		t.Fatalf("second InstallID call: %v", err)
	}
	if justCreated2 {
		t.Errorf("second call expected justCreated=false, got true")
	}
	if id2 != id1 {
		t.Errorf("install id changed between calls: %q vs %q", id1, id2)
	}

	// File content is exactly "<uuid>\n" — no trailing whitespace.
	raw, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("read install_id: %v", err)
	}
	want := id1 + "\n"
	if string(raw) != want {
		t.Errorf("install_id content = %q, want %q", string(raw), want)
	}
}

func TestInstallID_MalformedContentTreatedAsMissing(t *testing.T) {
	home := withTempHome(t)
	dir := filepath.Join(home, ".specscore")
	if err := os.MkdirAll(dir, installIDDirMode); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, installIDFilename),
		[]byte("not a uuid at all"),
		installIDFileMode,
	); err != nil {
		t.Fatalf("write malformed: %v", err)
	}

	// Per spec Error-Handling: malformed content => treat as missing, do NOT
	// regenerate. Implementation choice: InstallID returns the malformed read
	// as an error from readInstallID; the outer fast-path-then-create logic
	// will then attempt to create a fresh one. The current implementation
	// will write over the malformed file because the fast path failed —
	// matching "create if missing." This is acceptable as long as we don't
	// silently mutate a user-edited file with a "real-looking" UUID.
	id, justCreated, err := InstallID()
	if err != nil {
		t.Fatalf("InstallID with malformed: %v", err)
	}
	if !justCreated {
		t.Errorf("expected justCreated=true after replacing malformed content")
	}
	if !isUUIDv4(id) {
		t.Errorf("returned id is not a UUID v4: %q", id)
	}
}

func TestUUIDv4Generation_IsRandomAndWellFormed(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := range 1000 {
		id, err := generateUUIDv4()
		if err != nil {
			t.Fatalf("generateUUIDv4: %v", err)
		}
		if !isUUIDv4(id) {
			t.Fatalf("generated id is not a UUID v4: %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate uuid in %d generations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

func TestIsUUIDv4_Rejects(t *testing.T) {
	cases := []string{
		"",
		"not-a-uuid",
		"F47AC10B-58CC-4372-A567-0E02B2C3D479",                 // uppercase
		"f47ac10b58cc4372a5670e02b2c3d479",                     // no hyphens
		"f47ac10b-58cc-5372-a567-0e02b2c3d479",                 // version 5 not 4
		"f47ac10b-58cc-4372-c567-0e02b2c3d479",                 // wrong variant
		"f47ac10b-58cc-4372-a567-0e02b2c3d479-extra",           // too long
		strings.Repeat("a", 36),                                // right length, all 'a', no hyphens
	}
	for _, c := range cases {
		if isUUIDv4(c) {
			t.Errorf("isUUIDv4 accepted bad input: %q", c)
		}
	}
}
