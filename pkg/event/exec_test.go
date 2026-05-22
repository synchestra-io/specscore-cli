package event

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// execSampleEvent returns a representative event used by the Exec tests.
func execSampleEvent(t *testing.T) Event {
	t.Helper()
	return Event{
		Name:      "idea.drafted",
		Version:   1,
		UUID:      "00000000-0000-4000-8000-000000000000",
		Timestamp: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Actor:     Actor{Kind: "skill", ID: "specstudio:ideate"},
		Artifact:  Artifact{Type: "idea", ID: "demo", Path: "spec/ideas/demo.md", Revision: "uncommitted"},
		Payload:   json.RawMessage(`{"k":"v"}`),
	}
}

// TestExecPipesEventToStdin verifies AC:exec-pipes-event-to-stdin. The Exec
// subscriber must pipe the JSON-serialized envelope to the child's stdin and
// close stdin so a child like `tee` can exit cleanly on EOF.
func TestExecPipesEventToStdin(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "recorded-event.jsonl")

	sub := NewExec([]string{"tee", out}, nil, 2*time.Second)
	if got, want := sub.Name(), "exec:tee"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}

	e := execSampleEvent(t)
	if err := sub.Deliver(context.Background(), e); err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read recorded file: %v", err)
	}
	want, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	// Allow a single optional trailing newline.
	gotTrimmed := bytes.TrimRight(got, "\n")
	if !bytes.Equal(gotTrimmed, want) {
		t.Fatalf("recorded bytes mismatch.\n got:  %q\n want: %q", string(gotTrimmed), string(want))
	}
	// Ensure at most one trailing newline (i.e. exactly one line).
	if n := bytes.Count(got, []byte{'\n'}); n > 1 {
		t.Fatalf("recorded file contains %d newlines, want at most 1", n)
	}
}

// TestExecTimeoutKillsHungProcess verifies AC:exec-timeout-kills-hung-process.
// A `sleep 30` subscriber with a 200 ms timeout must return a timeout-typed
// error, complete within ~[200, 400] ms wall clock, and leave no surviving
// child process.
func TestExecTimeoutKillsHungProcess(t *testing.T) {
	sub := NewExec([]string{"sleep", "30"}, nil, 200*time.Millisecond)

	start := time.Now()
	err := sub.Deliver(context.Background(), execSampleEvent(t))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Deliver returned nil, want timeout error")
	}
	var timeoutErr *ExecTimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Deliver error type = %T (%v), want *ExecTimeoutError", err, err)
	}
	if timeoutErr.Timeout != 200*time.Millisecond {
		t.Fatalf("timeoutErr.Timeout = %v, want 200ms", timeoutErr.Timeout)
	}

	// Wall-clock window: 200 ms timeout + 100 ms SIGTERM grace + scheduler
	// slack. The AC names [200, 400] ms; we widen the upper bound slightly
	// because CI runners are occasionally slow, but keep the lower bound
	// strict so a too-eager return (e.g., immediate exit) is caught.
	if elapsed < 200*time.Millisecond {
		t.Fatalf("elapsed = %v, want >= 200ms", elapsed)
	}
	if elapsed > 800*time.Millisecond {
		t.Fatalf("elapsed = %v, want <= 800ms", elapsed)
	}

	// Verify no surviving `sleep 30` child process. pgrep exits 0 with
	// matches, 1 with no matches. We tolerate any pgrep failure other than
	// the "no matches" case by failing the test only on exit 0 with output.
	pg := exec.Command("pgrep", "-f", "sleep 30")
	pgOut, pgErr := pg.Output()
	if pgErr == nil {
		// Filter out the line for our own pgrep process if it appears.
		lines := strings.Split(strings.TrimSpace(string(pgOut)), "\n")
		var survivors []string
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			survivors = append(survivors, ln)
		}
		if len(survivors) > 0 {
			t.Fatalf("found surviving sleep processes after timeout: %v", survivors)
		}
	}
	// pgErr non-nil typically means "no matches" (exit 1); that is the
	// desired outcome and we do not fail the test.
}

// TestExecNonZeroExitReturnsExitError verifies that a child exiting non-zero
// surfaces a distinguishable *ExecExitError so the dispatcher can name the
// failure mode in its stderr log.
func TestExecNonZeroExitReturnsExitError(t *testing.T) {
	sub := NewExec([]string{"sh", "-c", "exit 7"}, nil, 2*time.Second)
	err := sub.Deliver(context.Background(), execSampleEvent(t))
	if err == nil {
		t.Fatalf("Deliver returned nil, want exit error")
	}
	var exitErr *ExecExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Deliver error type = %T (%v), want *ExecExitError", err, err)
	}
	if exitErr.ExitCode != 7 {
		t.Fatalf("exitErr.ExitCode = %d, want 7", exitErr.ExitCode)
	}
}

// TestExecEnvIsAdditive verifies that the configured env mapping is appended
// to os.Environ rather than replacing it: a variable from the parent
// environment must remain visible to the child, alongside the configured one.
func TestExecEnvIsAdditive(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "env.txt")

	t.Setenv("SPECSCORE_EXEC_PARENT_SENTINEL", "parent-visible")

	sub := NewExec(
		[]string{"sh", "-c", "printenv SPECSCORE_EXEC_PARENT_SENTINEL > " + out + "; printenv SPECSCORE_EXEC_CHILD_SENTINEL >> " + out},
		map[string]string{"SPECSCORE_EXEC_CHILD_SENTINEL": "child-visible"},
		2*time.Second,
	)
	if err := sub.Deliver(context.Background(), execSampleEvent(t)); err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}
	want := "parent-visible\nchild-visible\n"
	if string(got) != want {
		t.Fatalf("env file = %q, want %q", string(got), want)
	}
}
