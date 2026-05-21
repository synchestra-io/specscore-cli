package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDebugError_HonorsOptOutByDefault covers cli/telemetry/errors-telemetry
// #ac:debug-error-honors-optout: when crash-reports is opted out via
// persistent state, `specscore debug error --text foo` (no --force) MUST
// no-op with a stdout pointer at `specscore telemetry enable crash-reports`.
func TestDebugError_HonorsOptOutByDefault(t *testing.T) {
	home := withTempHomeForCLI(t)
	// Pre-set crash-reports opt-out.
	if err := os.MkdirAll(filepath.Join(home, ".specscore"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := "# SpecScore Telemetry Preferences: https://specscore.md/telemetry-preferences\ncrash-reports: false\n"
	if err := os.WriteFile(filepath.Join(home, ".specscore", "telemetry.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	if err := runDebugError(&buf, "test-known-id", false); err != nil {
		t.Fatalf("runDebugError: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "crash-reports opt-out is in effect") {
		t.Errorf("expected opt-out message, got %q", out)
	}
	if !strings.Contains(out, "specscore telemetry enable crash-reports") {
		t.Errorf("expected pointer to enable command, got %q", out)
	}
	if strings.Contains(out, "sent:") {
		t.Errorf("opt-out path should NOT have 'sent:' marker: %q", out)
	}
}

// TestDebugError_ForceBypassesOptOut covers cli/telemetry/errors-telemetry
// #ac:debug-error-force-bypasses-optout. With --force, the command MUST
// emit even when opted out, AND the persistent telemetry.yaml MUST be
// byte-identical before/after.
func TestDebugError_ForceBypassesOptOut(t *testing.T) {
	home := withTempHomeForCLI(t)
	if err := os.MkdirAll(filepath.Join(home, ".specscore"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yamlPath := filepath.Join(home, ".specscore", "telemetry.yaml")
	original := []byte("# SpecScore Telemetry Preferences: https://specscore.md/telemetry-preferences\ncrash-reports: false\n")
	if err := os.WriteFile(yamlPath, original, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	if err := runDebugError(&buf, "test-known-id", true); err != nil {
		t.Fatalf("runDebugError: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "sent:") {
		t.Errorf("--force should produce 'sent:' marker, got %q", out)
	}
	if !strings.Contains(out, "debug=true") {
		t.Errorf("--force output should include debug=true, got %q", out)
	}
	// NOTE on the verbatim path: the test-only `test-known-id` allowlist
	// entry registers via scrubber_testonly_test.go's init(), which only
	// runs under `go test ./internal/telemetry/...` — not cross-package
	// from internal/cli/. So from this test's perspective, ANY --text
	// value is non-allowlisted and coerces to "unscrubbed panic" with
	// unscrubbed=true. The verbatim-allowlist contract itself is verified
	// by TestScrubMessage_SafePanicWithAllowlistedIDIsVerbatim inside the
	// telemetry package. Here we only assert the bypass + tag.
	if !strings.Contains(out, "unscrubbed=true") {
		t.Errorf("--force with non-allowlisted text should coerce to unscrubbed: %q", out)
	}

	// Persistent state byte-identical.
	after, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if !bytes.Equal(original, after) {
		t.Errorf("telemetry.yaml mutated by --force; before=%q after=%q", original, after)
	}
}

// TestDebugError_UnknownMessageIDCoercesToUnscrubbed: when --text is not in
// the allowlist, the output must reflect the coercion to "unscrubbed
// panic" with unscrubbed=true.
func TestDebugError_UnknownMessageIDCoercesToUnscrubbed(t *testing.T) {
	withTempHomeForCLI(t)
	var buf bytes.Buffer
	if err := runDebugError(&buf, "definitely-not-in-allowlist", true); err != nil {
		t.Fatalf("runDebugError: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `message="unscrubbed panic"`) {
		t.Errorf("unknown messageID should coerce; got %q", out)
	}
	if !strings.Contains(out, "unscrubbed=true") {
		t.Errorf("output should mark unscrubbed=true; got %q", out)
	}
	if strings.Contains(out, `message="definitely-not-in-allowlist"`) {
		t.Errorf("unknown messageID should NEVER appear in output: %q", out)
	}
}

// TestDebugError_CISmokeTestPath covers cli/telemetry/errors-telemetry#ac:
// debug-error-ci-smoke-test: under CI=true (auto-disable), running without
// --force MUST take the opt-out path — no network, exit 0, with the
// no-op message.
func TestDebugError_CISmokeTestPath(t *testing.T) {
	withTempHomeForCLI(t)
	t.Setenv("CI", "true")
	var buf bytes.Buffer
	if err := runDebugError(&buf, "test-known-id", false); err != nil {
		t.Fatalf("runDebugError: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "crash-reports opt-out is in effect") {
		t.Errorf("CI smoke-test should hit opt-out path, got %q", out)
	}
	if strings.Contains(out, "sent:") {
		t.Errorf("CI smoke-test should NOT emit: %q", out)
	}
}
