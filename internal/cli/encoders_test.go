package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/feature"
	"github.com/specscore/specscore-cli/pkg/lint"
)

// --- fail stubs ---

type failYAMLEnc struct{ err error }

func (f *failYAMLEnc) Encode(any) error { return f.err }
func (f *failYAMLEnc) Close() error     { return nil }

type failJSONEnc struct{ err error }

func (f *failJSONEnc) Encode(any) error { return f.err }

// swapYAML replaces newYAMLEnc with a stub returning err and restores on cleanup.
func swapYAML(t *testing.T, err error) {
	t.Helper()
	old := newYAMLEnc
	newYAMLEnc = func(io.Writer) yamlEnc { return &failYAMLEnc{err: err} }
	t.Cleanup(func() { newYAMLEnc = old })
}

// swapJSON replaces newJSONEnc with a stub returning err and restores on cleanup.
func swapJSON(t *testing.T, err error) {
	t.Helper()
	old := newJSONEnc
	newJSONEnc = func(io.Writer) jsonEnc { return &failJSONEnc{err: err} }
	t.Cleanup(func() { newJSONEnc = old })
}

// --- spec.go: outputLintYAML / outputLintJSON ---

func TestOutputLintYAML_EncodeError(t *testing.T) {
	swapYAML(t, fmt.Errorf("yaml boom"))

	var buf bytes.Buffer
	err := outputLintYAML(&buf, []lint.Violation{{File: "a.md", Line: 1, Severity: "error", Rule: "r", Message: "m"}})
	if err == nil {
		t.Fatal("expected error from YAML encode")
	}
	if !strings.Contains(err.Error(), "yaml boom") {
		t.Errorf("error = %q, want it to contain 'yaml boom'", err)
	}
}

func TestOutputLintJSON_EncodeError(t *testing.T) {
	swapJSON(t, fmt.Errorf("json boom"))

	var buf bytes.Buffer
	err := outputLintJSON(&buf, []lint.Violation{{File: "a.md"}})
	if err == nil {
		t.Fatal("expected error from JSON encode")
	}
	if !strings.Contains(err.Error(), "json boom") {
		t.Errorf("error = %q, want it to contain 'json boom'", err)
	}
}

// --- feature.go: writeEnrichedYAML / writeEnrichedJSON / writeFeatureInfo ---

func TestWriteEnrichedYAML_EncodeError(t *testing.T) {
	swapYAML(t, fmt.Errorf("enrich yaml fail"))

	var buf bytes.Buffer
	err := writeEnrichedYAML(&buf, []*feature.EnrichedFeature{{Path: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enrich yaml fail") {
		t.Errorf("error = %q, want 'enrich yaml fail'", err)
	}
}

func TestWriteEnrichedJSON_EncodeError(t *testing.T) {
	swapJSON(t, fmt.Errorf("enrich json fail"))

	var buf bytes.Buffer
	err := writeEnrichedJSON(&buf, []*feature.EnrichedFeature{{Path: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "enrich json fail") {
		t.Errorf("error = %q, want 'enrich json fail'", err)
	}
}

func TestWriteFeatureInfo_YAMLEncodeError(t *testing.T) {
	swapYAML(t, fmt.Errorf("info yaml fail"))

	var buf bytes.Buffer
	err := writeFeatureInfo(&buf, "yaml", &feature.Info{Path: "auth"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "info yaml fail") {
		t.Errorf("error = %q, want 'info yaml fail'", err)
	}
}

func TestWriteFeatureInfo_JSONEncodeError(t *testing.T) {
	swapJSON(t, fmt.Errorf("info json fail"))

	var buf bytes.Buffer
	err := writeFeatureInfo(&buf, "json", &feature.Info{Path: "auth"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "info json fail") {
		t.Errorf("error = %q, want 'info json fail'", err)
	}
}

// --- task.go: runTaskList / runTaskInfo / runTaskNew ---

func TestTaskList_YAMLEncodeError(t *testing.T) {
	root := setupTaskProject(t)
	withCwd(t, root)
	swapYAML(t, fmt.Errorf("task list yaml fail"))

	_, _, err := runTask(t, "list", "--format=yaml")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task list yaml fail") {
		t.Errorf("error = %q, want 'task list yaml fail'", err)
	}
}

func TestTaskList_JSONEncodeError(t *testing.T) {
	root := setupTaskProject(t)
	withCwd(t, root)
	swapJSON(t, fmt.Errorf("task list json fail"))

	_, _, err := runTask(t, "list", "--format=json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task list json fail") {
		t.Errorf("error = %q, want 'task list json fail'", err)
	}
}

func TestTaskInfo_YAMLEncodeError(t *testing.T) {
	root := setupTaskProject(t)
	withCwd(t, root)
	swapYAML(t, fmt.Errorf("task info yaml fail"))

	_, _, err := runTask(t, "info", "--task=setup", "--format=yaml")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task info yaml fail") {
		t.Errorf("error = %q, want 'task info yaml fail'", err)
	}
}

func TestTaskInfo_JSONEncodeError(t *testing.T) {
	root := setupTaskProject(t)
	withCwd(t, root)
	swapJSON(t, fmt.Errorf("task info json fail"))

	_, _, err := runTask(t, "info", "--task=setup", "--format=json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task info json fail") {
		t.Errorf("error = %q, want 'task info json fail'", err)
	}
}

func TestTaskNew_YAMLEncodeError(t *testing.T) {
	root := setupTaskNewProject(t)
	withCwd(t, root)
	swapYAML(t, fmt.Errorf("task new yaml fail"))

	_, _, err := runTask(t, "new", "--task=fresh", "--title=Fresh Task")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task new yaml fail") {
		t.Errorf("error = %q, want 'task new yaml fail'", err)
	}
}

func TestTaskNew_JSONEncodeError(t *testing.T) {
	root := setupTaskNewProject(t)
	withCwd(t, root)
	swapJSON(t, fmt.Errorf("task new json fail"))

	_, _, err := runTask(t, "new", "--task=fresh2", "--title=Fresh Task 2", "--format=json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "task new json fail") {
		t.Errorf("error = %q, want 'task new json fail'", err)
	}
}

// setupTaskNewProject creates a project with a board suitable for "task new".
func setupTaskNewProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "specscore.yaml"), []byte("name: test\n"), 0o644)

	tasksDir := filepath.Join(root, "tasks")
	_ = os.MkdirAll(tasksDir, 0o755)
	board := "# Tasks\n\n" +
		"| Task | Status | Depends on | Branch | Agent | Requester | Time |\n" +
		"|---|---|---|---|---|---|---|\n"
	_ = os.WriteFile(filepath.Join(tasksDir, "README.md"), []byte(board), 0o644)
	return root
}

// --- idea_list.go: runIdeaList ---

func TestIdeaList_YAMLEncodeError_ViaEncoder(t *testing.T) {
	root := setupSpecRoot(t)
	withCwd(t, root)

	// Create an idea so there's data to encode.
	if _, _, err := runIdea(t, "new", "testidea", "--owner", "tester"); err != nil {
		t.Fatalf("idea new: %v", err)
	}

	swapYAML(t, fmt.Errorf("idea list yaml fail"))

	_, _, err := runIdea(t, "list", "--format=yaml")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "idea list yaml fail") {
		t.Errorf("error = %q, want 'idea list yaml fail'", err)
	}
}

func TestIdeaList_JSONEncodeError(t *testing.T) {
	root := setupSpecRoot(t)
	withCwd(t, root)

	if _, _, err := runIdea(t, "new", "testidea", "--owner", "tester"); err != nil {
		t.Fatalf("idea new: %v", err)
	}

	swapJSON(t, fmt.Errorf("idea list json fail"))

	_, _, err := runIdea(t, "list", "--format=json")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "idea list json fail") {
		t.Errorf("error = %q, want 'idea list json fail'", err)
	}
}
