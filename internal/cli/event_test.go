package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runEvent invokes the event command tree in-process with the given args.
func runEvent(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := eventCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// AC: verb-registers-and-helps — bare `event` exits 0 and lists `emit`.
func TestEventCommand_HelpExitsZero(t *testing.T) {
	out, _, err := runEvent(t)
	if err != nil {
		t.Fatalf("bare `event` returned error: %v", err)
	}
	if !strings.Contains(out, "emit") {
		t.Errorf("expected bare `event` help to list `emit` subcommand; got:\n%s", out)
	}
}

// AC: verb-registers-and-helps — `event --help` exits 0.
func TestEventCommand_ExplicitHelpExitsZero(t *testing.T) {
	out, _, err := runEvent(t, "--help")
	if err != nil {
		t.Fatalf("`event --help` returned error: %v", err)
	}
	if !strings.Contains(out, "emit") {
		t.Errorf("expected `event --help` to list `emit`; got:\n%s", out)
	}
}

// AC: verb-registers-and-helps — `event emit --help` includes the canonical docs link.
func TestEventCommand_HelpHasDocsLink(t *testing.T) {
	out, _, err := runEvent(t, "emit", "--help")
	if err != nil {
		t.Fatalf("`event emit --help` returned error: %v", err)
	}
	const want = "https://specscore.md/event-emit"
	if !strings.Contains(out, want) {
		t.Errorf("expected `event emit --help` output to contain %q; got:\n%s", want, out)
	}
}

// AC: verb-registers-and-helps — `event emit --help` enumerates envelope flags.
// NOTE: Full nine-flag completeness (including --payload-json / --payload-file
// from REQ:payload-input-modes) lands in Task 4. This batch only covers the
// seven REQ:envelope-flags.
func TestEventCommand_HelpShowsEnvelopeFlags(t *testing.T) {
	out, _, err := runEvent(t, "emit", "--help")
	if err != nil {
		t.Fatalf("`event emit --help` returned error: %v", err)
	}
	for _, flag := range []string{
		"--name",
		"--actor-kind",
		"--actor-id",
		"--artifact-type",
		"--artifact-id",
		"--artifact-path",
		"--artifact-revision",
	} {
		if !strings.Contains(out, flag) {
			t.Errorf("expected `event emit --help` to mention %s; got:\n%s", flag, out)
		}
	}
}

// AC: required-flag-missing-fails-2.
//
// NOTE: the AC's example invocation includes `--payload-json '{}'`, but
// payload flags are introduced in Task 4. We omit `--payload-json` here —
// the AC's intent is "missing required flag → exit 2 naming the flag and
// envelope field". The full nine-flag invocation lands in a Task-4
// integration test.
func TestEventEmit_MissingNameFlagFails2(t *testing.T) {
	tmp := t.TempDir()
	withCwd(t, tmp)

	_, stderr, err := runEvent(t,
		"emit",
		"--actor-kind", "skill",
		"--actor-id", "skill:t",
		"--artifact-type", "idea",
		"--artifact-id", "x",
		"--artifact-path", "spec/ideas/x.md",
	)
	if err == nil {
		t.Fatal("expected error for missing --name; got nil")
	}

	type exitCoder interface{ ExitCode() int }
	var ec exitCoder
	if !errors.As(err, &ec) {
		t.Fatalf("error does not carry ExitCode: %v", err)
	}
	if got := ec.ExitCode(); got != 2 {
		t.Errorf("exit code = %d; want 2", got)
	}

	combined := err.Error() + stderr
	if !strings.Contains(combined, "--name") {
		t.Errorf("expected error/stderr to name `--name`; got err=%v stderr=%q", err, stderr)
	}
	// AC requires stderr to also name the envelope field the flag supplies (`name`).
	if !strings.Contains(combined, "envelope field") && !strings.Contains(combined, "envelope `name`") &&
		!strings.Contains(combined, "field `name`") && !strings.Contains(combined, "name`") {
		t.Errorf("expected error/stderr to name envelope field `name`; got err=%v stderr=%q", err, stderr)
	}

	// No event MUST be written to .specscore/events.jsonl.
	jsonl := filepath.Join(tmp, ".specscore", "events.jsonl")
	if _, statErr := os.Stat(jsonl); !os.IsNotExist(statErr) {
		t.Errorf("expected no events.jsonl after failure; stat err: %v", statErr)
	}
}

// Reject extra positional args (REQ:envelope-flags: "MUST NOT accept extra
// positional arguments — only flag-form input").
func TestEventEmit_RejectsPositionalArgs(t *testing.T) {
	tmp := t.TempDir()
	withCwd(t, tmp)

	_, _, err := runEvent(t,
		"emit",
		"unexpected-positional",
		"--name", "idea.drafted",
		"--actor-kind", "skill",
		"--actor-id", "skill:t",
		"--artifact-type", "idea",
		"--artifact-id", "x",
		"--artifact-path", "spec/ideas/x.md",
	)
	if err == nil {
		t.Fatal("expected error for unexpected positional arg; got nil")
	}
}
