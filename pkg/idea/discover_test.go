package idea

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// TestFeatureSourceIdeas_NestedFeatures locks in the contract that
// **Source Ideas:** headers on nested feature READMEs are discovered.
// Regression test for the prior bug where only `spec/features/<slug>/`
// (one level deep) was scanned, making nested-feature promotion silently
// no-op in the idea-sync-lint-strict rule.
func TestFeatureSourceIdeas_NestedFeatures(t *testing.T) {
	root := t.TempDir()
	specRoot := filepath.Join(root, "spec")

	files := map[string]string{
		// Top-level feature with a Source Ideas reference.
		"features/auth/README.md": "# Feature: Auth\n\n" +
			"**Status:** Approved\n" +
			"**Source Ideas:** auth-overhaul\n\n",

		// Nested two levels deep — the case the original walker missed.
		"features/cli/lifecycle-transitions/README.md": "# Feature: Lifecycle Transitions\n\n" +
			"**Status:** Approved\n" +
			"**Source Ideas:** lifecycle-verbs-for-idea-and-feature\n\n",

		// Nested three levels deep — also must be picked up.
		"features/cli/spec/lint/README.md": "# Feature: Spec Lint\n\n" +
			"**Status:** Stable\n" +
			"**Source Ideas:** index-entries-autofix\n\n",

		// Feature without Source Ideas — must be omitted from the map.
		"features/cli/README.md": "# Feature: CLI\n\n**Status:** Stable\n\n",

		// Dir convention prefixes that must be skipped entirely.
		"features/_args/README.md":    "# Args\n\n**Source Ideas:** ignored-conventional-dir\n",
		"features/.hidden/README.md":  "# Hidden\n\n**Source Ideas:** ignored-hidden\n",
	}
	for path, content := range files {
		full := filepath.Join(specRoot, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got, err := FeatureSourceIdeas(specRoot)
	if err != nil {
		t.Fatalf("FeatureSourceIdeas: %v", err)
	}

	want := map[string][]string{
		"auth":                       {"auth-overhaul"},
		"cli/lifecycle-transitions":  {"lifecycle-verbs-for-idea-and-feature"},
		"cli/spec/lint":              {"index-entries-autofix"},
	}

	if len(got) != len(want) {
		gotKeys := make([]string, 0, len(got))
		for k := range got {
			gotKeys = append(gotKeys, k)
		}
		sort.Strings(gotKeys)
		t.Fatalf("map size = %d, want %d. got keys = %v", len(got), len(want), gotKeys)
	}
	for slug, wantIdeas := range want {
		gotIdeas, ok := got[slug]
		if !ok {
			t.Errorf("missing slug %q in result", slug)
			continue
		}
		if !reflect.DeepEqual(gotIdeas, wantIdeas) {
			t.Errorf("slug %q: got %v, want %v", slug, gotIdeas, wantIdeas)
		}
	}
	for _, badSlug := range []string{"_args", ".hidden", "cli"} {
		if _, present := got[badSlug]; present {
			t.Errorf("slug %q must NOT be in result", badSlug)
		}
	}
}
