package event

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeYAML is a small helper for the config tests; it writes the canonical
// schema-header line followed by `body` to <dir>/specscore.yaml.
func writeYAML(t *testing.T, dir, body string) {
	t.Helper()
	header := "# SpecScore Repo Config Schema: https://specscore.md/repo-config\n\n"
	path := filepath.Join(dir, "specscore.yaml")
	if err := os.WriteFile(path, []byte(header+body), 0o644); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
}

func TestLoadSubscribers_DefaultWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "project:\n  title: Test\n")

	subs, err := LoadSubscribers(dir)
	if err != nil {
		t.Fatalf("LoadSubscribers: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}
	jw, ok := subs[0].(*JsonlWriter)
	if !ok {
		t.Fatalf("expected *JsonlWriter, got %T", subs[0])
	}
	wantSuffix := filepath.Join(".specscore", "events.jsonl")
	if !strings.HasSuffix(jw.Name(), wantSuffix) {
		t.Fatalf("expected Name() to end with %q, got %q", wantSuffix, jw.Name())
	}
}

func TestLoadSubscribers_ExplicitEmpty(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "project:\n  title: Test\nevents:\n  subscribers: []\n")

	subs, err := LoadSubscribers(dir)
	if err != nil {
		t.Fatalf("LoadSubscribers: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("expected 0 subscribers, got %d", len(subs))
	}
}

func TestLoadSubscribers_UnknownType(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `project:
  title: Test
events:
  subscribers:
    - type: webhook
      url: "https://example.com"
`)

	_, err := LoadSubscribers(dir)
	if err == nil {
		t.Fatal("expected error for unknown subscriber type, got nil")
	}
	msg := err.Error()
	for _, want := range []string{
		"specscore.yaml",
		"events.subscribers[0].type",
		"webhook",
		"jsonl",
		"noop",
		"exec",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; got: %s", want, msg)
		}
	}
}

func TestLoadSubscribers_MixedValid(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `project:
  title: Test
events:
  subscribers:
    - type: jsonl
      path: custom/events.jsonl
    - type: exec
      command: [/bin/true]
      timeout_ms: 1500
    - type: noop
`)

	subs, err := LoadSubscribers(dir)
	if err != nil {
		t.Fatalf("LoadSubscribers: %v", err)
	}
	if len(subs) != 3 {
		t.Fatalf("expected 3 subscribers, got %d", len(subs))
	}
	if _, ok := subs[0].(*JsonlWriter); !ok {
		t.Errorf("subs[0]: expected *JsonlWriter, got %T", subs[0])
	}
	if _, ok := subs[1].(*Exec); !ok {
		t.Errorf("subs[1]: expected *Exec, got %T", subs[1])
	}
	if _, ok := subs[2].(NoOp); !ok {
		t.Errorf("subs[2]: expected NoOp, got %T", subs[2])
	}
}

func TestLoadSubscribers_MissingJsonlPath(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `project:
  title: Test
events:
  subscribers:
    - type: jsonl
`)
	_, err := LoadSubscribers(dir)
	if err == nil {
		t.Fatal("expected error for missing jsonl path, got nil")
	}
	msg := err.Error()
	for _, want := range []string{"specscore.yaml", "events.subscribers[0].path"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; got: %s", want, msg)
		}
	}
}

func TestLoadSubscribers_MissingExecCommand(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `project:
  title: Test
events:
  subscribers:
    - type: exec
`)
	_, err := LoadSubscribers(dir)
	if err == nil {
		t.Fatal("expected error for missing exec command, got nil")
	}
	if !strings.Contains(err.Error(), "events.subscribers[0].command") {
		t.Errorf("error message missing key path; got: %s", err.Error())
	}
}

func TestLoadSubscribers_TimeoutOutOfRange(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, `project:
  title: Test
events:
  subscribers:
    - type: exec
      command: [/bin/true]
      timeout_ms: 50
`)
	_, err := LoadSubscribers(dir)
	if err == nil {
		t.Fatal("expected error for timeout_ms < 100, got nil")
	}
	if !strings.Contains(err.Error(), "events.subscribers[0].timeout_ms") {
		t.Errorf("error message missing key path; got: %s", err.Error())
	}
}

func TestLoadSubscribers_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	// No specscore.yaml at all — treat as absent events: block.
	subs, err := LoadSubscribers(dir)
	if err != nil {
		t.Fatalf("LoadSubscribers: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected default JsonlWriter, got %d subs", len(subs))
	}
	if _, ok := subs[0].(*JsonlWriter); !ok {
		t.Fatalf("expected *JsonlWriter, got %T", subs[0])
	}
}
