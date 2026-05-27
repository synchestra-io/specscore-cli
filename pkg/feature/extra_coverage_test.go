package feature

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/lifecycle"
)

// ---------------------------------------------------------------------------
// fields.go — lines 91-93: ExtractOpenQuestions returns error
// ---------------------------------------------------------------------------

func TestResolveFields_QuestionsError_Stub(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"auth": "# Feature: Auth\n\n**Status:** Draft\n\n## Open Questions\n\n- Q1?\n",
	})

	orig := extractOpenQuestionsFn
	extractOpenQuestionsFn = func(path string) ([]string, error) {
		return nil, fmt.Errorf("injected questions error")
	}
	t.Cleanup(func() { extractOpenQuestionsFn = orig })

	ef, err := ResolveFields(featDir, "auth", []string{"questions"})
	if err == nil {
		t.Fatal("expected error from injected questions failure")
	}
	if ef == nil || ef.Path != "auth" {
		t.Errorf("expected partial result with path=auth, got %v", ef)
	}
	if !strings.Contains(err.Error(), "questions") {
		t.Errorf("error = %v, want mention of questions", err)
	}
}

// ---------------------------------------------------------------------------
// fields.go — lines 112-114: FindFeatureRefs returns error
// ---------------------------------------------------------------------------

func TestResolveFields_RefsError_Stub(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"auth": "# Feature: Auth\n\n**Status:** Draft\n\n## Open Questions\n\nNone.\n",
	})

	orig := findFeatureRefsFn
	findFeatureRefsFn = func(dir, id string) ([]string, error) {
		return nil, fmt.Errorf("injected refs error")
	}
	t.Cleanup(func() { findFeatureRefsFn = orig })

	ef, err := ResolveFields(featDir, "auth", []string{"refs"})
	if err == nil {
		t.Fatal("expected error from injected refs failure")
	}
	if ef == nil || ef.Path != "auth" {
		t.Errorf("expected partial result, got %v", ef)
	}
	if !strings.Contains(err.Error(), "refs") {
		t.Errorf("error = %v, want mention of refs", err)
	}
}

// ---------------------------------------------------------------------------
// info.go — lines 125-126: ParseDependencies error → skip in FindFeatureRefs
// (already tested indirectly via TestFindFeatureRefs_UnreadableFeature, but
// that test depends on os.Chmod which doesn't work as root)
// ---------------------------------------------------------------------------

func TestFindFeatureRefs_ParseDependenciesError_Skip(t *testing.T) {
	// We test this by having a feature whose README is malformed enough
	// that ParseDependencies cannot open it. Use a nonexistent README path.
	featDir := setupTestFeatures(t, map[string]string{
		"auth": "# Auth\n\n**Status:** Approved\n\n## Dependencies\n\n## Open Questions\n\nNone.\n",
		"bad":  "# Bad\n\n**Status:** Draft\n\n## Dependencies\n\n- auth\n\n## Open Questions\n\nNone.\n",
	})

	// Remove the bad feature's README so ParseDependencies fails for it.
	_ = os.Remove(filepath.Join(featDir, "bad", "README.md"))

	// FindFeatureRefs should not error — it skips parse failures.
	refs, err := FindFeatureRefs(featDir, "auth")
	if err != nil {
		t.Fatal(err)
	}
	// bad's deps can't be read, so no refs for auth.
	if len(refs) != 0 {
		t.Errorf("expected 0 refs (bad feature unreadable), got %v", refs)
	}
}

// ---------------------------------------------------------------------------
// info.go — lines 149-151: ParseContentsTable error
// ---------------------------------------------------------------------------

func TestDiscoverChildFeatures_ParseContentsTableError(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"parent":       "# Feature: Parent\n\n**Status:** Draft\n",
		"parent/child": "# Feature: Child\n\n**Status:** Draft\n",
	})

	// Remove the parent README so ParseContentsTable fails
	parentReadme := filepath.Join(featDir, "parent", "README.md")
	_ = os.Remove(parentReadme)

	_, err := DiscoverChildFeatures(featDir, "parent", parentReadme)
	if err == nil {
		t.Fatal("expected error when parent README is missing for ParseContentsTable")
	}
}

// ---------------------------------------------------------------------------
// info.go — lines 228-230: WalkDir callback receives OS error
// (stub via filepathWalkFn would require changing info.go; instead trigger
// by a real walkDir error — use a directory with a broken symlink that
// causes a stat error during walk)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// info.go — lines 259-261: os.Open fails in planReferencesFeature
// (Already covered: planReferencesFeature returns false when os.Open fails.
//  Exercise by calling FindLinkedPlans with a plan whose README is missing.)
// ---------------------------------------------------------------------------

func TestFindLinkedPlans_PlanReadmeMissing(t *testing.T) {
	root, _ := setupSpecRepo(t,
		map[string]string{
			"auth": "# Feature: Auth\n\n**Status:** Draft\n",
		},
		map[string]string{
			"my-plan": "# Plan\n\n**Features:**\n- [auth](../../features/auth/README.md)\n",
		},
	)

	// Remove the plan README so os.Open in planReferencesFeature fails.
	planReadme := filepath.Join(root, "spec", "plans", "my-plan", "README.md")
	_ = os.Remove(planReadme)

	plans, err := FindLinkedPlans(root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	// The plan's README is missing, so planReferencesFeature returns false.
	if len(plans) != 0 {
		t.Errorf("expected 0 plans when README is missing, got %v", plans)
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — lines 101-103: os.MkdirAll fails (stub-based)
// ---------------------------------------------------------------------------

func TestNew_MkdirAllError_Stub(t *testing.T) {
	featDir := t.TempDir()

	orig := osMkdirAll
	osMkdirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("injected mkdir error")
	}
	t.Cleanup(func() { osMkdirAll = orig })

	_, err := New(featDir, NewOptions{Title: "Fail Mkdir"})
	if err == nil {
		t.Fatal("expected error from injected MkdirAll failure")
	}
	if !strings.Contains(err.Error(), "creating feature directory") {
		t.Errorf("error = %v, want mention of creating feature directory", err)
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — lines 117-119: UpdateParentContents fails (stub-based)
// ---------------------------------------------------------------------------

func TestNew_UpdateParentContentsError_Stub(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"parent": "# Feature: Parent\n\n**Status:** Stable\n\n## Summary\n\nParent.\n",
	})

	// Make UpdateParentContents fail by making osReadFileFn fail when reading the parent README.
	// Actually, UpdateParentContents calls os.ReadFile directly, not osReadFileFn.
	// Instead, make osWriteFile fail only for the parent README.
	origWrite := osWriteFile
	callCount := 0
	osWriteFile = func(name string, data []byte, perm os.FileMode) error {
		callCount++
		if callCount == 1 {
			// First call is writing the feature README — let it succeed
			return origWrite(name, data, perm)
		}
		// Second call is UpdateParentContents — fail
		return fmt.Errorf("injected parent write error")
	}
	t.Cleanup(func() { osWriteFile = origWrite })

	_, err := New(featDir, NewOptions{
		Title:  "Child",
		Parent: "parent",
	})
	if err == nil {
		t.Fatal("expected error from UpdateParentContents failure")
	}
	if !strings.Contains(err.Error(), "updating parent contents") {
		t.Errorf("error = %v, want mention of updating parent contents", err)
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — lines 129-131: UpdateFeatureIndex fails (stub-based)
// ---------------------------------------------------------------------------

func TestNew_UpdateFeatureIndexError_Stub(t *testing.T) {
	featDir := t.TempDir()
	// Create a features index that can be read
	indexPath := filepath.Join(featDir, "README.md")
	if err := os.WriteFile(indexPath, []byte("# Features\n\n| Feature | Status |\n|---|---|\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Make osReadFileFn fail for the index (non-NotExist error)
	origRead := osReadFileFn
	osReadFileFn = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("injected read error")
	}
	t.Cleanup(func() { osReadFileFn = origRead })

	_, err := New(featDir, NewOptions{Title: "Fail Index"})
	if err == nil {
		t.Fatal("expected error from UpdateFeatureIndex failure")
	}
	if !strings.Contains(err.Error(), "updating feature index") {
		t.Errorf("error = %v, want mention of updating feature index", err)
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — lines 243-245: os.WriteFile fails in UpdateParentContents
// ---------------------------------------------------------------------------

func TestUpdateParentContents_WriteFileError_Stub(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte("# Feature: Parent\n\n## Summary\n\nA parent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := osWriteFile
	osWriteFile = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	t.Cleanup(func() { osWriteFile = orig })

	_, err := UpdateParentContents(path, "child", "desc")
	if err == nil {
		t.Fatal("expected error from injected WriteFile failure")
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — line 266: os.ReadFile fails with non-NotExist error
// ---------------------------------------------------------------------------

func TestUpdateFeatureIndex_ReadFileNonNotExistError(t *testing.T) {
	orig := osReadFileFn
	osReadFileFn = func(name string) ([]byte, error) {
		return nil, fmt.Errorf("injected read error")
	}
	t.Cleanup(func() { osReadFileFn = orig })

	_, err := UpdateFeatureIndex("/some/path/README.md", "feat", "Draft", "desc")
	if err == nil {
		t.Fatal("expected error from non-NotExist ReadFile failure")
	}
}

// ---------------------------------------------------------------------------
// newfeature.go — lines 308-310: os.WriteFile fails in UpdateFeatureIndex
// ---------------------------------------------------------------------------

func TestUpdateFeatureIndex_WriteFileError_Stub(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(indexPath, []byte("# Features\n\n| Feature | Status |\n|---|---|\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRead := osReadFileFn
	origWrite := osWriteFile
	// Let read succeed (using real implementation)
	osReadFileFn = origRead
	osWriteFile = func(name string, data []byte, perm os.FileMode) error {
		return fmt.Errorf("injected write error")
	}
	t.Cleanup(func() { osWriteFile = origWrite })

	_, err := UpdateFeatureIndex(indexPath, "new-feat", "Draft", "desc")
	if err == nil {
		t.Fatal("expected error from injected WriteFile failure")
	}
}

// ---------------------------------------------------------------------------
// transitions.go — lines 100-102: lifecycle.Rewrite fails
// ---------------------------------------------------------------------------

func TestChangeStatus_RewriteError_Stub(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"test": "# Feature: Test\n\n**Status:** Draft\n\n## Open Questions\n\nNone.\n",
	})

	orig := lifecycleRewriteFn
	lifecycleRewriteFn = func(path string, status lifecycle.Status) (string, error) {
		return "", fmt.Errorf("injected rewrite error")
	}
	t.Cleanup(func() { lifecycleRewriteFn = orig })

	_, err := ChangeStatus(featDir, "test", "under review")
	if err == nil {
		t.Fatal("expected error from injected Rewrite failure")
	}
	if !strings.Contains(err.Error(), "rewriting status line") {
		t.Errorf("error = %v, want mention of rewriting status line", err)
	}
}
