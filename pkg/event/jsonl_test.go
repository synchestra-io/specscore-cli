package event

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sampleEvent builds a minimal, valid Event envelope used by the JsonlWriter
// tests. The exact field values are not load-bearing — only that the envelope
// round-trips through encoding/json without error.
func sampleEvent(name string) Event {
	return Event{
		Name:      name,
		Version:   1,
		UUID:      "00000000-0000-4000-8000-000000000000",
		Timestamp: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Actor:     Actor{Kind: "skill", ID: "specstudio:test"},
		Artifact: Artifact{
			Type:     "idea",
			ID:       "demo",
			Path:     "spec/ideas/demo.md",
			Revision: "uncommitted",
		},
		Payload: json.RawMessage(`{"k":"v"}`),
	}
}

// TestJsonlWriterAppendsLine verifies AC: jsonl-writer-appends-line.
//
// Two sequential Deliver calls MUST produce a file with exactly two
// newline-separated JSON envelopes, file mode 0644, parent dir mode 0755.
func TestJsonlWriterAppendsLine(t *testing.T) {
	root := t.TempDir()

	w := NewJsonlWriter(".specscore/events.jsonl", root)

	ctx := context.Background()
	if err := w.Deliver(ctx, sampleEvent("idea.drafted")); err != nil {
		t.Fatalf("first Deliver: %v", err)
	}
	if err := w.Deliver(ctx, sampleEvent("idea.approved")); err != nil {
		t.Fatalf("second Deliver: %v", err)
	}

	// Parent directory exists with mode 0755.
	dirInfo, err := os.Stat(filepath.Join(root, ".specscore"))
	if err != nil {
		t.Fatalf("stat parent dir: %v", err)
	}
	if !dirInfo.IsDir() {
		t.Fatalf("%s is not a directory", dirInfo.Name())
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o755 {
		t.Fatalf("parent dir mode = %o, want 0755", perm)
	}

	// File exists with mode 0644.
	filePath := filepath.Join(root, ".specscore", "events.jsonl")
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o644 {
		t.Fatalf("file mode = %o, want 0644", perm)
	}

	// Content is exactly two newline-separated JSON lines and no trailing
	// bytes beyond the final newline.
	body, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n") {
		t.Fatalf("file does not end with newline: %q", string(body))
	}
	// Trim the single trailing newline and split.
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; content=%q", len(lines), string(body))
	}

	// Each line is a valid single-line JSON envelope.
	for i, line := range lines {
		if strings.ContainsRune(line, '\n') {
			t.Fatalf("line %d contains embedded newline", i)
		}
		var got Event
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("line %d is not valid JSON: %v; line=%q", i, err, line)
		}
	}

	// No extraneous bytes: re-serializing the two events with a single
	// trailing newline each MUST equal the file content byte-for-byte.
	want1, _ := json.Marshal(sampleEvent("idea.drafted"))
	want2, _ := json.Marshal(sampleEvent("idea.approved"))
	expected := string(want1) + "\n" + string(want2) + "\n"
	if string(body) != expected {
		t.Fatalf("file content mismatch.\n got: %q\nwant: %q", string(body), expected)
	}
}

// TestJsonlWriterResolvesAgainstProjectRoot verifies AC:
// jsonl-writer-resolves-against-project-root.
//
// A JsonlWriter constructed with a relative path MUST resolve that path
// against the configured project root, not against the current working
// directory at Deliver time.
func TestJsonlWriterResolvesAgainstProjectRoot(t *testing.T) {
	root := t.TempDir()

	// Create a nested subdir under the project root and chdir there so a
	// cwd-relative resolution would land in the wrong place.
	subdir := filepath.Join(root, "sub", "dir")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("chdir subdir: %v", err)
	}

	w := NewJsonlWriter(".specscore/events.jsonl", root)
	if err := w.Deliver(context.Background(), sampleEvent("idea.drafted")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	// Project-root-relative file MUST exist.
	wantPath := filepath.Join(root, ".specscore", "events.jsonl")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected file at project-root path %q, stat err: %v", wantPath, err)
	}

	// cwd-relative file MUST NOT exist.
	cwdPath := filepath.Join(subdir, ".specscore", "events.jsonl")
	if _, err := os.Stat(cwdPath); !os.IsNotExist(err) {
		t.Fatalf("unexpected file at cwd-relative path %q (err=%v)", cwdPath, err)
	}
}

// TestJsonlWriterNameUsesResolvedPath verifies the task's Name() guidance:
// "use the resolved final path, not the configured-relative one".
func TestJsonlWriterNameUsesResolvedPath(t *testing.T) {
	root := t.TempDir()
	w := NewJsonlWriter(".specscore/events.jsonl", root)
	want := "jsonl:" + filepath.Join(root, ".specscore", "events.jsonl")
	if got := w.Name(); got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

// TestJsonlWriterAbsolutePathUnchanged verifies that an absolute configured
// path is used as-is (the project-root anchor only applies to relative paths).
func TestJsonlWriterAbsolutePathUnchanged(t *testing.T) {
	root := t.TempDir()
	abs := filepath.Join(t.TempDir(), "elsewhere", "events.jsonl")

	w := NewJsonlWriter(abs, root)
	if err := w.Deliver(context.Background(), sampleEvent("idea.drafted")); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("expected file at absolute path %q, stat err: %v", abs, err)
	}
}

// TestJsonlWriterDeliverMkdirError verifies that Deliver returns an error when
// parent directory creation fails (e.g. a file exists where the dir should be).
func TestJsonlWriterDeliverMkdirError(t *testing.T) {
	root := t.TempDir()

	// Create a regular file where the parent directory should be, so MkdirAll fails.
	blocker := filepath.Join(root, ".specscore")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	w := NewJsonlWriter(".specscore/events.jsonl", root)
	err := w.Deliver(context.Background(), sampleEvent("idea.drafted"))
	if err == nil {
		t.Fatal("expected error when parent dir cannot be created, got nil")
	}
	if !strings.Contains(err.Error(), "create parent dir") {
		t.Errorf("error message = %q, want to contain 'create parent dir'", err.Error())
	}
}

// TestJsonlWriterDeliverOpenError verifies that Deliver returns an error when
// the file cannot be opened (e.g. the path is a directory).
func TestJsonlWriterDeliverOpenError(t *testing.T) {
	root := t.TempDir()

	// Create .specscore/events.jsonl as a directory so OpenFile fails.
	dirPath := filepath.Join(root, ".specscore", "events.jsonl")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	w := NewJsonlWriter(".specscore/events.jsonl", root)
	err := w.Deliver(context.Background(), sampleEvent("idea.drafted"))
	if err == nil {
		t.Fatal("expected error when file is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "open jsonl file") {
		t.Errorf("error message = %q, want to contain 'open jsonl file'", err.Error())
	}
}
