package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// withTempHomeForCLI is the cli-package analogue of telemetry/withTempHome.
// Reroutes HOME so the telemetry subcommand reads/writes a scratch path.
func withTempHomeForCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	// Clear opt-out env vars so tests start from a known state.
	t.Setenv("SPECSCORE_TELEMETRY", "")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CIRCLECI", "")
	return dir
}

func TestTelemetryStatus_DefaultsReportEnabled(t *testing.T) {
	withTempHomeForCLI(t)
	var buf bytes.Buffer
	if err := writeStatus(&buf, "", false); err != nil {
		t.Fatalf("writeStatus: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "usage-stats: enabled") {
		t.Errorf("expected usage-stats: enabled line, got %q", out)
	}
	if !strings.Contains(out, "crash-reports: enabled") {
		t.Errorf("expected crash-reports: enabled line, got %q", out)
	}
	if !strings.Contains(out, "(source: default)") {
		t.Errorf("expected source: default, got %q", out)
	}
}

func TestTelemetryStatus_SingleChannel(t *testing.T) {
	withTempHomeForCLI(t)
	var buf bytes.Buffer
	if err := writeStatus(&buf, "usage-stats", true); err != nil {
		t.Fatalf("writeStatus: %v", err)
	}
	out := buf.String()
	if !strings.HasPrefix(out, "usage-stats:") {
		t.Errorf("expected line to start with usage-stats:, got %q", out)
	}
	if strings.Contains(out, "crash-reports") {
		t.Errorf("expected ONLY usage-stats row, got %q", out)
	}
}

func TestValidateChannelArg_AllSentinelMeansAllChannels(t *testing.T) {
	ch, hasArg, err := validateChannelArg([]string{"all"})
	if err != nil {
		t.Fatalf("validateChannelArg(all): unexpected error %v", err)
	}
	if hasArg {
		t.Errorf("`all` should be treated as no-arg (hasArg=false), got hasArg=true with channel=%q", ch)
	}
	if ch != "" {
		t.Errorf("`all` should not yield a real channel name, got %q", ch)
	}
}

func TestValidateChannelArg_StarIsRejected(t *testing.T) {
	// `*` is no longer the sentinel; it should fall through to the
	// unknown-channel error path (exit 2). This guards against
	// inadvertent re-introduction of `*`.
	_, _, err := validateChannelArg([]string{"*"})
	if err == nil {
		t.Fatalf("expected `*` to be rejected as unknown channel")
	}
}

func TestTelemetryDisable_AllSentinel_EquivalentToNoArg(t *testing.T) {
	withTempHomeForCLI(t)
	var buf bytes.Buffer
	// First, disable with no-arg.
	if err := mutateState(&buf, "", false, false); err != nil {
		t.Fatalf("disable no-arg: %v", err)
	}
	want := buf.String()
	buf.Reset()
	// Reset state and disable with `all`.
	withTempHomeForCLI(t)
	single, hasArg, err := validateChannelArg([]string{"all"})
	if err != nil {
		t.Fatalf("validate all: %v", err)
	}
	if err := mutateState(&buf, single, hasArg, false); err != nil {
		t.Fatalf("disable all: %v", err)
	}
	if buf.String() != want {
		t.Errorf("disable all confirmation should match no-arg: got %q want %q", buf.String(), want)
	}
}

func TestValidateChannelArg_UnknownExits2(t *testing.T) {
	_, _, err := validateChannelArg([]string{"unknown-typo"})
	if err == nil {
		t.Fatalf("expected error for unknown channel")
	}
	type exitCoder interface{ ExitCode() int }
	ec, ok := err.(exitCoder)
	if !ok {
		t.Fatalf("error does not implement exitCoder: %T", err)
	}
	if ec.ExitCode() != 2 {
		t.Errorf("exit code: got %d, want 2", ec.ExitCode())
	}
	if !strings.Contains(err.Error(), "known channels:") {
		t.Errorf("error should list known channels, got %q", err.Error())
	}
}

func TestTelemetryEnableDisable_RoundTrip(t *testing.T) {
	home := withTempHomeForCLI(t)

	// Disable globally.
	var buf bytes.Buffer
	if err := mutateState(&buf, "", false, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if !strings.Contains(buf.String(), "all channels disabled") {
		t.Errorf("disable confirmation: got %q", buf.String())
	}
	// File should exist.
	path := filepath.Join(home, ".specscore", "telemetry.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read telemetry.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "enabled: false") {
		t.Errorf("file should contain enabled: false, got %q", string(raw))
	}

	// Status should report both disabled.
	buf.Reset()
	if err := writeStatus(&buf, "", false); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(buf.String(), "usage-stats: disabled") {
		t.Errorf("status should report usage-stats disabled, got %q", buf.String())
	}

	// Re-enable just crash-reports.
	buf.Reset()
	if err := mutateState(&buf, "crash-reports", true, true); err != nil {
		t.Fatalf("enable crash-reports: %v", err)
	}
	if !strings.Contains(buf.String(), "crash-reports enabled") {
		t.Errorf("enable crash-reports confirmation: got %q", buf.String())
	}
	// Status reflects per-channel override beating global.
	buf.Reset()
	if err := writeStatus(&buf, "", false); err != nil {
		t.Fatalf("status: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "usage-stats: disabled") {
		t.Errorf("usage-stats should still be disabled: %q", out)
	}
	if !strings.Contains(out, "crash-reports: enabled") {
		t.Errorf("crash-reports should be enabled (per-channel override): %q", out)
	}
}

func TestPreRun_FirstRunNoticeShownOnce(t *testing.T) {
	home := withTempHomeForCLI(t)
	// Reset module state.
	invocation = runtimeState{}
	noTelemetryFlag = false
	var notice bytes.Buffer
	prevWriter := firstRunNoticeWriter
	firstRunNoticeWriter = &notice
	t.Cleanup(func() { firstRunNoticeWriter = prevWriter })

	cmd := newRootCommandForTest()
	preRun(cmd)
	first := notice.String()
	// Required literal strings per REQ:first-run-notice-content.
	for _, want := range []string{
		"SpecScore",
		"usage-stats",
		"crash-reports",
		"specscore telemetry disable [channel-id]",
		"all",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("first-run notice missing required literal %q; got:\n%s", want, first)
		}
	}

	// Verify install_id was created.
	if _, err := os.Stat(filepath.Join(home, ".specscore", "install_id")); err != nil {
		t.Errorf("install_id should exist after preRun: %v", err)
	}

	// Second invocation: notice should NOT print again.
	notice.Reset()
	invocation = runtimeState{}
	preRun(cmd)
	if notice.Len() > 0 {
		t.Errorf("second preRun should not print first-run notice, got %q", notice.String())
	}
}

func TestPreRun_FirstRunNoticeSuppressedInCI(t *testing.T) {
	home := withTempHomeForCLI(t)
	t.Setenv("CI", "true")
	invocation = runtimeState{}
	noTelemetryFlag = false
	var notice bytes.Buffer
	prevWriter := firstRunNoticeWriter
	firstRunNoticeWriter = &notice
	t.Cleanup(func() { firstRunNoticeWriter = prevWriter })

	cmd := newRootCommandForTest()
	preRun(cmd)
	if notice.Len() > 0 {
		t.Errorf("CI run should NOT print first-run notice, got %q", notice.String())
	}
	// But install_id MUST still be created (so a later interactive run on the
	// same machine doesn't re-trigger).
	if _, err := os.Stat(filepath.Join(home, ".specscore", "install_id")); err != nil {
		t.Errorf("install_id should still be created in CI: %v", err)
	}
}

// newRootCommandForTest constructs a minimal cobra command tree sufficient for
// driving preRun in tests. We deliberately don't go through fang.Execute.
func newRootCommandForTest() *cobra.Command {
	return &cobra.Command{Use: "specscore"}
}
