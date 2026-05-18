package property

import (
	"path/filepath"
	"testing"
)

func TestWalk_VisitsEachDiscoveredFile(t *testing.T) {
	root := filepath.Join("_testdata", "walktree")
	var visited []string
	err := Walk(root, func(doc *Doc) error {
		// Walk's contract: never passes nil to the callback. Parse only
		// returns nil on a non-nil error, and Walk would have returned that.
		visited = append(visited, doc.Slug)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := []string{"email", "money"}
	if len(visited) != len(want) {
		t.Fatalf("visited %d files, want %d: %v", len(visited), len(want), visited)
	}
	for i, w := range want {
		if visited[i] != w {
			t.Errorf("visited[%d] = %q, want %q", i, visited[i], w)
		}
	}
}

func TestWalk_PropagatesCallbackError(t *testing.T) {
	root := filepath.Join("_testdata", "walktree")
	sentinel := errSentinel{}
	err := Walk(root, func(doc *Doc) error {
		return sentinel
	})
	if err != sentinel {
		t.Fatalf("Walk did not propagate callback error: got %v", err)
	}
}

func TestWalk_MissingRoot(t *testing.T) {
	err := Walk(filepath.Join("_testdata", "does-not-exist"), func(doc *Doc) error {
		t.Fatal("callback should not be invoked when root is absent")
		return nil
	})
	if err != nil {
		t.Fatalf("Walk on missing root should be a no-op, got %v", err)
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel" }
