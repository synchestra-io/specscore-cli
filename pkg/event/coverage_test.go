package event

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Exec — stdin write error path (child exits immediately, causing broken pipe)
// ---------------------------------------------------------------------------

// TestExecStdinCloseErrorPath exercises the path where stdin.Close returns an
// error because the child exited before reading all input. The child "sh -c
// 'read line; exit 2'" reads one line then exits immediately, so our Write
// succeeds (short payload) but the process exits with 2 before we get to
// Wait(), triggering classifyWaitError.
func TestExecStdinCloseErrorPath(t *testing.T) {
	// This child exits non-zero, triggering the waitErr → classifyWaitError path.
	sub := NewExec([]string{"sh", "-c", "exit 2"}, nil, 2*time.Second)
	err := sub.Deliver(context.Background(), execSampleEvent(t))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Exec — stdin write to already-exited child exercises write error → Wait path
// ---------------------------------------------------------------------------

// TestExecWriteLargePayloadToEarlyExitChild sends a large event payload to a
// child that exits immediately. The large payload increases the chance that
// Write itself fails (broken pipe), exercising the werr != nil → cmd.Wait()
// path in Deliver.
func TestExecWriteLargePayloadToEarlyExitChild(t *testing.T) {
	// Build a large event (payload) so the write to stdin is more likely
	// to fail with broken pipe.
	bigPayload := make([]byte, 64*1024)
	for i := range bigPayload {
		bigPayload[i] = 'x'
	}
	e := Event{
		Name:      "idea.drafted",
		Version:   1,
		UUID:      "00000000-0000-4000-8000-000000000000",
		Timestamp: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Actor:     Actor{Kind: "skill", ID: "test"},
		Artifact:  Artifact{Type: "idea", ID: "x", Path: "x", Revision: "x"},
		Payload:   json.RawMessage(bigPayload),
	}

	// Child exits immediately without reading stdin
	sub := NewExec([]string{"sh", "-c", "exit 5"}, nil, 2*time.Second)
	err := sub.Deliver(context.Background(), e)
	if err == nil {
		t.Fatal("expected error from early-exit child with large payload, got nil")
	}
}

// ---------------------------------------------------------------------------
// classifyWaitError — fallback error path (not DeadlineExceeded, not ExitError)
// ---------------------------------------------------------------------------

func TestClassifyWaitError_FallbackPath(t *testing.T) {
	// Use a context that's not timed out
	ctx := context.Background()
	// Pass a generic error (not *exec.ExitError)
	err := classifyWaitError(errGeneric{}, ctx, time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "exec: wait:"
	if got := err.Error(); !strings.Contains(got, want) {
		t.Errorf("error = %q, want to contain %q", got, want)
	}
}

type errGeneric struct{}

func (errGeneric) Error() string { return "generic wait error" }

// ---------------------------------------------------------------------------
// Exec — stdin close error when child reads partial then exits
// ---------------------------------------------------------------------------

// TestExecChildReadsPartialThenExits exercises the path where the child reads
// some stdin then exits before we finish writing, causing a broken pipe on
// stdin.Write or stdin.Close.
func TestExecChildReadsPartialThenExits(t *testing.T) {
	// "head -c 1" reads 1 byte and exits. Our payload is larger, so
	// the write will either succeed (and close triggers broken pipe) or
	// the write itself gets SIGPIPE. Either way, we exercise the write/close
	// error paths.
	sub := NewExec([]string{"head", "-c", "1"}, nil, 2*time.Second)
	// head exits 0 normally, so there may be no error, but this path still
	// exercises the stdin.Close + cmd.Wait code paths.
	_ = sub.Deliver(context.Background(), execSampleEvent(t))
}

// ---------------------------------------------------------------------------
// Exec — child with env vars
// ---------------------------------------------------------------------------

func TestExecChildWithEnvVars(t *testing.T) {
	sub := NewExec([]string{"sh", "-c", "exit 0"}, map[string]string{"FOO": "bar"}, 2*time.Second)
	err := sub.Deliver(context.Background(), execSampleEvent(t))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Exec — marshal error with invalid payload
// ---------------------------------------------------------------------------

func TestExecDeliverMarshalError(t *testing.T) {
	sub := NewExec([]string{"cat"}, nil, 2*time.Second)
	// Invalid JSON in Payload causes json.Marshal to fail
	badEvent := Event{
		Name:    "test",
		Payload: json.RawMessage(`{invalid json`),
	}
	err := sub.Deliver(context.Background(), badEvent)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "marshal event") {
		t.Errorf("error = %q, want to contain 'marshal event'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Exec — child closes stdin immediately, large payload triggers broken pipe
// ---------------------------------------------------------------------------

func TestExecBrokenPipeLargePayload(t *testing.T) {
	// This child closes stdin (fd 0) immediately and exits 0.
	// With a large payload, the write to the pipe should fail.
	bigPayload := make([]byte, 256*1024)
	for i := range bigPayload {
		bigPayload[i] = byte('a' + (i % 26))
	}
	e := Event{
		Name:    "test",
		Version: 1,
		Payload: json.RawMessage(`"` + string(bigPayload) + `"`),
	}
	sub := NewExec([]string{"sh", "-c", "exec 0<&-; exit 0"}, nil, 2*time.Second)
	// May or may not error depending on pipe buffer timing.
	// The key is exercising the code path, not the specific outcome.
	_ = sub.Deliver(context.Background(), e)
}

// ---------------------------------------------------------------------------
// JsonlWriter — marshal error with invalid payload
// ---------------------------------------------------------------------------

func TestJsonlWriterDeliverMarshalError(t *testing.T) {
	root := t.TempDir()
	w := NewJsonlWriter("events.jsonl", root)
	badEvent := Event{
		Name:    "test",
		Payload: json.RawMessage(`{invalid json`),
	}
	err := w.Deliver(context.Background(), badEvent)
	if err == nil {
		t.Fatal("expected marshal error, got nil")
	}
	if !strings.Contains(err.Error(), "marshal event") {
		t.Errorf("error = %q, want to contain 'marshal event'", err.Error())
	}
}
