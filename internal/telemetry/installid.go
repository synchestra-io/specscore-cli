package telemetry

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Testable indirection points — production code uses the real implementations;
// tests swap these to inject failures on specific I/O paths.
var (
	runtimeGOOS  = runtime.GOOS
	randRead     = rand.Read
	osCreateTemp = os.CreateTemp
	osRename     = os.Rename
	fileChmod    = (*os.File).Chmod
	fileSync     = (*os.File).Sync
	fileClose    = (*os.File).Close
)

// installIDFilename is the name of the install-ID file inside the
// SpecScore-per-user directory. Per cli/telemetry#req:install-id-file-path
// the file lives at ~/.specscore/install_id on Unix and the platform-
// appropriate user-state directory equivalent on Windows.
const installIDFilename = "install_id"

// installIDDirName is the per-user SpecScore directory under the user's home.
const installIDDirName = ".specscore"

// installIDDirMode and installIDFileMode are the mandated mode bits per
// cli/telemetry#req:install-id-file-path.
const (
	installIDDirMode  fs.FileMode = 0o700
	installIDFileMode fs.FileMode = 0o600
)

// InstallID returns the per-machine install identifier, creating it on first
// invocation. The boolean return value is true iff the file was created during
// THIS call (used by callers to drive IsFirstRun semantics + first-run-notice
// suppression).
//
// Behavior summary (cli/telemetry#req:install-id-*):
//   - Path: ~/.specscore/install_id on Unix; the platform-appropriate
//     equivalent on Windows (uses os.UserConfigDir).
//   - Format: UUID v4, lowercase, hyphenated, no surrounding whitespace, a
//     single trailing newline.
//   - Creation: atomic write-then-rename so a concurrent read never observes
//     a partial file.
//   - Mode bits: directory 0700, file 0600.
//   - Per-machine, NOT per-project: a single developer working across N
//     repositories counts as one install.
//   - Immutability: once created, the CLI MUST NOT regenerate or rotate the
//     file in normal operation. Users delete the file manually to rotate.
//
// On any I/O error (no home dir, permission denied, read-only fs), the
// function returns ("", false, err); callers MUST treat this as "telemetry
// disabled for this invocation" — never abort the user's command.
func InstallID() (id string, justCreated bool, err error) {
	path, err := InstallIDPath()
	if err != nil {
		return "", false, err
	}

	// Fast path: file exists and is well-formed.
	if existing, readErr := readInstallID(path); readErr == nil {
		return existing, false, nil
	}

	// Slow path: create. Ensure parent dir exists with the right mode.
	if mkErr := os.MkdirAll(filepath.Dir(path), installIDDirMode); mkErr != nil {
		return "", false, fmt.Errorf("creating install-id directory: %w", mkErr)
	}
	// Best-effort fix of dir mode if it pre-existed with wider permissions.
	_ = os.Chmod(filepath.Dir(path), installIDDirMode)

	newID, genErr := generateUUIDv4()
	if genErr != nil {
		return "", false, fmt.Errorf("generating install id: %w", genErr)
	}

	if writeErr := atomicWriteFile(path, []byte(newID+"\n"), installIDFileMode); writeErr != nil {
		return "", false, fmt.Errorf("writing install-id: %w", writeErr)
	}
	return newID, true, nil
}

// InstallIDPath returns the absolute path of the install-id file without
// reading or creating it. Used by tests, by the first-run-notice trigger
// (which only checks existence), and by `specscore telemetry status`.
func InstallIDPath() (string, error) {
	dir, err := userStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, installIDFilename), nil
}

// userStateDir resolves the parent directory of install_id per
// cli/telemetry#req:install-id-file-path.
//
// On Unix: $HOME/.specscore
// On Windows: %LOCALAPPDATA%\specscore (via os.UserConfigDir, which on
//
//	Windows returns %APPDATA%; we route Windows through os.UserConfigDir
//	for safety and use a subdir "specscore" — matching the Feature's
//	"%LOCALAPPDATA%\specscore\install_id" example in spirit).
func userStateDir() (string, error) {
	if runtimeGOOS == "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolving user config dir: %w", err)
		}
		return filepath.Join(base, "specscore"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home dir: %w", err)
	}
	return filepath.Join(home, installIDDirName), nil
}

// readInstallID reads and validates the existing install_id file. Returns
// (id, nil) if the file exists and the content is a syntactically-valid
// UUID v4 (lowercase, hyphenated, optional single trailing newline).
//
// If the file exists but content is malformed (NOT a UUID v4), readInstallID
// returns ("", error) — per cli/telemetry README Error-Handling row
// "malformed content (not a UUID)": treat as missing, do NOT regenerate
// (REQ:install-id-immutability). Callers SHOULD disable telemetry for the
// invocation and surface at --verbose.
func readInstallID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.TrimRight(string(raw), "\n")
	if !isUUIDv4(content) {
		return "", errors.New("install_id content is not a UUID v4")
	}
	return content, nil
}

// generateUUIDv4 returns a freshly-generated random UUID v4 in lowercase
// hyphenated form (e.g. "f47ac10b-58cc-4372-a567-0e02b2c3d479"). Uses
// crypto/rand for entropy — no external dependency.
func generateUUIDv4() (string, error) {
	var b [16]byte
	if _, err := randRead(b[:]); err != nil {
		return "", err
	}
	// Set version (4) in the high nibble of byte 6.
	b[6] = (b[6] & 0x0f) | 0x40
	// Set variant (10x) in the high two bits of byte 8.
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16],
	), nil
}

// isUUIDv4 validates the canonical 8-4-4-4-12 lowercase-hex form with the
// version-4 nibble in the right place. Strict — refuses uppercase, missing
// hyphens, or wrong version digits. This is the format generateUUIDv4
// produces; any other content is treated as "not ours."
func isUUIDv4(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isLowerHex(byte(c)) {
				return false
			}
		}
	}
	// Version nibble at index 14.
	if s[14] != '4' {
		return false
	}
	// Variant nibble at index 19 ∈ {8, 9, a, b}.
	switch s[19] {
	case '8', '9', 'a', 'b':
		return true
	default:
		return false
	}
}

func isLowerHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

// atomicWriteFile writes data to path via a temp file in the same directory
// followed by os.Rename, so a concurrent reader never observes a partial
// write (cli/telemetry#req:install-id-creation).
func atomicWriteFile(path string, data []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := osCreateTemp(dir, ".install_id-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// On any failure below, remove the temp file.
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := fileChmod(tmp, mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := fileSync(tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := fileClose(tmp); err != nil {
		return err
	}
	return osRename(tmpPath, path)
}
