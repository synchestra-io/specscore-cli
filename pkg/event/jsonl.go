package event

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// JsonlWriter is a Subscriber that appends each delivered Event as a single
// JSON line to a file. It is the default subscriber synthesized by the
// config loader when the `events:` block is omitted; see
// spec/features/cli/event/README.md.
//
// Relative configured paths resolve against the project root supplied at
// construction time, NEVER against the current working directory. The
// dispatcher wires the project root in once, at startup.
type JsonlWriter struct {
	// path is the fully-resolved absolute path to the JSONL file.
	path string
}

// NewJsonlWriter constructs a JsonlWriter that will append to `path`. When
// `path` is relative it is joined against `projectRoot`; when it is
// absolute it is used as-is. The resolved path is cached on the struct so
// Deliver and Name remain stable across calls.
func NewJsonlWriter(path string, projectRoot string) *JsonlWriter {
	resolved := path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(projectRoot, resolved)
	}
	return &JsonlWriter{path: resolved}
}

// Deliver serializes e to single-line JSON and appends it (followed by a
// single newline) to the configured file. Parent directories are created
// at mode 0755 if absent; the file itself is opened with O_APPEND|O_CREATE
// |O_WRONLY at mode 0644.
func (w *JsonlWriter) Deliver(_ context.Context, e Event) error {
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	dir := filepath.Dir(w.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir %s: %w", dir, err)
	}

	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open jsonl file %s: %w", w.path, err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("append jsonl line: %w", err)
	}
	return nil
}

// Name returns "jsonl:<resolved-path>" — using the resolved absolute path
// so failure logs point at the actual file on disk.
func (w *JsonlWriter) Name() string {
	return "jsonl:" + w.path
}
