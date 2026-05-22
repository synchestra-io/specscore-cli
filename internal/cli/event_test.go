package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/specscore/specscore-cli/pkg/event"
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

// uuidV4RE matches the AC regex for a lowercase-hyphenated v4 UUID.
var uuidV4RE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// gitInitWithCommit initialises a real git repo in dir with one empty
// commit so HEAD resolves. Skips the calling test if `git` is missing.
func gitInitWithCommit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "T"},
		{"commit", "--allow-empty", "-q", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
}

// TestAutofillEnvelope_GitRepo (covers AC: envelope-auto-fill-fields).
// Calls autofillEnvelope against a real git repo with no override and
// asserts every auto-filled field matches the AC's contract.
func TestAutofillEnvelope_GitRepo(t *testing.T) {
	dir := t.TempDir()
	gitInitWithCommit(t, dir)

	// Capture the expected SHA via the same command the AC names.
	expectedSHA := func() string {
		cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git rev-parse HEAD: %v", err)
		}
		return strings.TrimSpace(string(out))
	}()

	before := time.Now().UTC()
	var e event.Event
	autofillEnvelope(&e, dir, "")
	after := time.Now().UTC()

	if e.Version != 1 {
		t.Errorf("Version = %d; want 1", e.Version)
	}
	if !uuidV4RE.MatchString(e.UUID) {
		t.Errorf("UUID = %q; does not match v4 regex %q", e.UUID, uuidV4RE.String())
	}
	// Timestamp must be UTC (Go formats UTC times with `Z` suffix when
	// using time.RFC3339) and within ±5s of the call window.
	if e.Timestamp.Location() != time.UTC {
		t.Errorf("Timestamp location = %v; want UTC", e.Timestamp.Location())
	}
	formatted := e.Timestamp.Format(time.RFC3339)
	if !strings.HasSuffix(formatted, "Z") {
		t.Errorf("Timestamp RFC3339 = %q; want `Z` suffix", formatted)
	}
	if e.Timestamp.Before(before.Add(-5*time.Second)) || e.Timestamp.After(after.Add(5*time.Second)) {
		t.Errorf("Timestamp %v outside ±5s window [%v, %v]", e.Timestamp, before, after)
	}
	if e.Artifact.Revision != expectedSHA {
		t.Errorf("Artifact.Revision = %q; want %q (from `git rev-parse HEAD`)",
			e.Artifact.Revision, expectedSHA)
	}
}

// TestAutofillEnvelope_NoGitRepo (covers AC: envelope-auto-fill-revision-no-git).
// Calls autofillEnvelope against a directory that is NOT a git repo and
// asserts Artifact.Revision is the literal string "uncommitted".
func TestAutofillEnvelope_NoGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	dir := t.TempDir() // no `git init`

	var e event.Event
	autofillEnvelope(&e, dir, "")

	if e.Artifact.Revision != "uncommitted" {
		t.Errorf("Artifact.Revision = %q; want %q", e.Artifact.Revision, "uncommitted")
	}
	// The other auto-fill fields should still be populated correctly even
	// when the git path fails.
	if e.Version != 1 {
		t.Errorf("Version = %d; want 1", e.Version)
	}
	if !uuidV4RE.MatchString(e.UUID) {
		t.Errorf("UUID = %q; does not match v4 regex", e.UUID)
	}
}

// TestAutofillEnvelope_RevisionOverride (covers AC:
// envelope-artifact-revision-override). When the override is non-empty
// it MUST be used verbatim — the git path MUST NOT be invoked and MUST
// NOT replace the override.
func TestAutofillEnvelope_RevisionOverride(t *testing.T) {
	dir := t.TempDir()
	gitInitWithCommit(t, dir)

	const override = "deadbeef00000000000000000000000000000000"

	var e event.Event
	autofillEnvelope(&e, dir, override)

	if e.Artifact.Revision != override {
		t.Errorf("Artifact.Revision = %q; want override %q", e.Artifact.Revision, override)
	}
}
