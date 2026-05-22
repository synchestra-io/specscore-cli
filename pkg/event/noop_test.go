package event

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestNoOpDeliverReturnsNil verifies the NoOp subscriber returns nil from
// Deliver and exposes the literal name "noop". See AC:noop-discards.
func TestNoOpDeliverReturnsNil(t *testing.T) {
	var sub Subscriber = NoOp{}
	if got := sub.Name(); got != "noop" {
		t.Fatalf("Name() = %q, want %q", got, "noop")
	}
	if err := sub.Deliver(context.Background(), Event{}); err != nil {
		t.Fatalf("Deliver returned error: %v", err)
	}
}

// TestNoOpNoSideEffects asserts the side-effect contract: redirecting stdout
// and stderr during Deliver yields zero bytes, and the per-test temp dir
// remains empty after the call. Network behavior is not exercised here — the
// trivial implementation has no network code.
func TestNoOpNoSideEffects(t *testing.T) {
	tmp := t.TempDir()

	// Redirect os.Stdout and os.Stderr through pipes so we can detect any
	// write performed during the Deliver call.
	origStdout, origStderr := os.Stdout, os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stderr: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	e := Event{
		Name:      "idea.drafted",
		Version:   1,
		UUID:      "00000000-0000-4000-8000-000000000000",
		Timestamp: time.Now().UTC(),
		Actor:     Actor{Kind: "skill", ID: "specstudio:ideate"},
		Artifact:  Artifact{Type: "idea", ID: "demo", Path: "spec/ideas/demo.md", Revision: "uncommitted"},
		Payload:   json.RawMessage(`{}`),
	}

	deliverErr := NoOp{}.Deliver(context.Background(), e)

	// Restore and close write ends so the readers see EOF.
	os.Stdout = origStdout
	os.Stderr = origStderr
	if err := stdoutW.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	if err := stderrW.Close(); err != nil {
		t.Fatalf("close stderr pipe: %v", err)
	}

	if deliverErr != nil {
		t.Fatalf("Deliver returned error: %v", deliverErr)
	}

	stdoutBytes, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if len(stdoutBytes) != 0 {
		t.Fatalf("stdout wrote %d bytes: %q", len(stdoutBytes), string(stdoutBytes))
	}
	stderrBytes, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("read stderr pipe: %v", err)
	}
	if len(stderrBytes) != 0 {
		t.Fatalf("stderr wrote %d bytes: %q", len(stderrBytes), string(stderrBytes))
	}

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("read temp dir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, filepath.Join(tmp, e.Name()))
		}
		t.Fatalf("temp dir not empty after Deliver: %v", names)
	}
}
