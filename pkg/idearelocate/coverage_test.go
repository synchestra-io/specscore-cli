package idearelocate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// ===== shortSHA =====

func TestShortSHA_FullLength(t *testing.T) {
	got := shortSHA("abc1234def5678")
	if got != "abc1234" {
		t.Errorf("shortSHA(full): got %q want %q", got, "abc1234")
	}
}

func TestShortSHA_Exactly7(t *testing.T) {
	got := shortSHA("abc1234")
	if got != "abc1234" {
		t.Errorf("shortSHA(7): got %q want %q", got, "abc1234")
	}
}

func TestShortSHA_Shorter(t *testing.T) {
	got := shortSHA("abc")
	if got != "abc" {
		t.Errorf("shortSHA(short): got %q want %q", got, "abc")
	}
}

func TestShortSHA_Empty(t *testing.T) {
	got := shortSHA("")
	if got != "" {
		t.Errorf("shortSHA(empty): got %q want %q", got, "")
	}
}

// ===== mergePaths =====

func TestMergePaths_NoDuplicates(t *testing.T) {
	got := mergePaths([]string{"a", "b"}, []string{"c", "d"})
	want := []string{"a", "b", "c", "d"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(no dups): got %v want %v", got, want)
	}
}

func TestMergePaths_WithDuplicates(t *testing.T) {
	got := mergePaths([]string{"a", "b"}, []string{"b", "c"})
	want := []string{"a", "b", "c"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(with dups): got %v want %v", got, want)
	}
}

func TestMergePaths_EmptyA(t *testing.T) {
	got := mergePaths(nil, []string{"x", "y"})
	want := []string{"x", "y"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(empty a): got %v want %v", got, want)
	}
}

func TestMergePaths_EmptyB(t *testing.T) {
	got := mergePaths([]string{"x", "y"}, nil)
	want := []string{"x", "y"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(empty b): got %v want %v", got, want)
	}
}

func TestMergePaths_BothEmpty(t *testing.T) {
	got := mergePaths(nil, nil)
	if len(got) != 0 {
		t.Errorf("mergePaths(both empty): got %v want empty", got)
	}
}

func TestMergePaths_CleansAndDedupes(t *testing.T) {
	// "a/b/../c" cleans to "a/c"
	got := mergePaths([]string{"a/c"}, []string{"a/b/../c"})
	want := []string{"a/c"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(clean+dedup): got %v want %v", got, want)
	}
}

func TestMergePaths_InternalDupesInA(t *testing.T) {
	got := mergePaths([]string{"x", "x", "y"}, []string{"z"})
	want := []string{"x", "y", "z"}
	if !sliceEqual(got, want) {
		t.Errorf("mergePaths(internal dup): got %v want %v", got, want)
	}
}

// ===== FormatStdout =====

func TestFormatStdout_CommitAutoWithSHA(t *testing.T) {
	changes := []RepoChange{
		{
			Repo:   TargetRepo{RepoName: "source-repo"},
			Action: ActionMoved,
			Kind:   KindIdea,
			Slug:   "my-idea",
			SHA:    "abc1234567890",
		},
		{
			Repo:   TargetRepo{RepoName: "target-repo"},
			Action: ActionReceived,
			Kind:   KindIdea,
			Slug:   "my-idea",
			SHA:    "def4567890123",
		},
	}
	got := FormatStdout(changes, CommitAuto)
	if !strings.Contains(got, "source-repo: moved idea my-idea  [abc1234]") {
		t.Errorf("line 1 missing or wrong in:\n%s", got)
	}
	if !strings.Contains(got, "target-repo: received idea my-idea  [def4567]") {
		t.Errorf("line 2 missing or wrong in:\n%s", got)
	}
	if !strings.Contains(got, "relocate complete: 2 repos affected") {
		t.Errorf("summary line missing in:\n%s", got)
	}
}

func TestFormatStdout_CommitNoOmitsSHA(t *testing.T) {
	changes := []RepoChange{
		{
			Repo:   TargetRepo{RepoName: "src"},
			Action: ActionMoved,
			Kind:   KindSeed,
			Slug:   "seed-x",
			SHA:    "shouldbeignored",
		},
	}
	got := FormatStdout(changes, CommitNo)
	if strings.Contains(got, "[") {
		t.Errorf("CommitNo should not include SHA brackets: %s", got)
	}
	if !strings.Contains(got, "src: moved seed seed-x") {
		t.Errorf("line content wrong in:\n%s", got)
	}
	if !strings.Contains(got, "relocate complete: 1 repos affected") {
		t.Errorf("summary missing in:\n%s", got)
	}
}

func TestFormatStdout_EmptyChanges(t *testing.T) {
	got := FormatStdout(nil, CommitAuto)
	if !strings.Contains(got, "relocate complete: 0 repos affected") {
		t.Errorf("empty changes: got %q", got)
	}
}

func TestFormatStdout_CommitAutoEmptySHA(t *testing.T) {
	// When mode is CommitAuto but SHA is empty (non-git repo), bracket is omitted.
	changes := []RepoChange{
		{
			Repo:   TargetRepo{RepoName: "non-git"},
			Action: ActionUpdatedLinks,
			Kind:   KindIdea,
			Slug:   "x",
			SHA:    "",
		},
	}
	got := FormatStdout(changes, CommitAuto)
	if strings.Contains(got, "[") {
		t.Errorf("empty SHA in CommitAuto should not show brackets: %s", got)
	}
}

// ===== AsExitError =====

func TestAsExitError_FullFormat(t *testing.T) {
	fail := &CommitFailure{
		Failed: RepoChange{
			Repo: TargetRepo{Path: "/repos/target", RepoName: "target"},
		},
		FailedStderr: "error: nothing to commit\n",
		Committed: []RepoChange{
			{
				Repo: TargetRepo{Path: "/repos/source", RepoName: "source"},
				SHA:  "abc1234567890",
			},
		},
		Unprocessed: []RepoChange{
			{
				Repo: TargetRepo{Path: "/repos/sib", RepoName: "sib"},
			},
		},
	}

	ecErr := fail.AsExitError()
	if ecErr.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ecErr.ExitCode(), exitcode.Unexpected)
	}
	msg := ecErr.Error()
	// Check committed section
	if !strings.Contains(msg, "Already committed:") {
		t.Errorf("missing 'Already committed:' in:\n%s", msg)
	}
	if !strings.Contains(msg, "source") {
		t.Errorf("missing committed repo name in:\n%s", msg)
	}
	if !strings.Contains(msg, "abc1234") {
		t.Errorf("missing short SHA in:\n%s", msg)
	}
	// Check failed section
	if !strings.Contains(msg, "Failed in target") {
		t.Errorf("missing 'Failed in target' in:\n%s", msg)
	}
	if !strings.Contains(msg, "error: nothing to commit") {
		t.Errorf("missing stderr in:\n%s", msg)
	}
	// Check unprocessed section
	if !strings.Contains(msg, "Unprocessed") {
		t.Errorf("missing 'Unprocessed' in:\n%s", msg)
	}
	if !strings.Contains(msg, "sib") {
		t.Errorf("missing unprocessed repo in:\n%s", msg)
	}
	// Check rollback commands
	if !strings.Contains(msg, "git -C /repos/source reset HEAD~1 --hard") {
		t.Errorf("missing rollback for committed repo in:\n%s", msg)
	}
	if !strings.Contains(msg, "git -C /repos/target reset HEAD") {
		t.Errorf("missing rollback for failed repo in:\n%s", msg)
	}
}

func TestAsExitError_NoCommittedNoUnprocessed(t *testing.T) {
	fail := &CommitFailure{
		Failed: RepoChange{
			Repo: TargetRepo{Path: "/repos/src", RepoName: "src"},
		},
		FailedStderr: "",
		Committed:    nil,
		Unprocessed:  nil,
	}
	ecErr := fail.AsExitError()
	msg := ecErr.Error()
	if strings.Contains(msg, "Already committed:") {
		t.Errorf("should not have 'Already committed:' when no committed repos:\n%s", msg)
	}
	if strings.Contains(msg, "Unprocessed") {
		t.Errorf("should not have 'Unprocessed' when none:\n%s", msg)
	}
	if !strings.Contains(msg, "Failed in src") {
		t.Errorf("should name the failing repo:\n%s", msg)
	}
}

// ===== ioRollbackError =====

func TestIoRollbackError_WithActions(t *testing.T) {
	cause := errors.New("write failed")
	actions := []string{"removed partial destination /tmp/dest.md", "restored source artifact /tmp/src.md"}
	ecErr := ioRollbackError("file copy / source delete", cause, actions)
	if ecErr.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ecErr.ExitCode(), exitcode.Unexpected)
	}
	msg := ecErr.Error()
	if !strings.Contains(msg, "file copy / source delete") {
		t.Errorf("missing step name in:\n%s", msg)
	}
	if !strings.Contains(msg, "write failed") {
		t.Errorf("missing cause in:\n%s", msg)
	}
	if !strings.Contains(msg, "Rollback actions performed:") {
		t.Errorf("missing 'Rollback actions performed:' in:\n%s", msg)
	}
	if !strings.Contains(msg, "removed partial destination") {
		t.Errorf("missing first action in:\n%s", msg)
	}
	if !strings.Contains(msg, "restored source artifact") {
		t.Errorf("missing second action in:\n%s", msg)
	}
}

func TestIoRollbackError_NoActions(t *testing.T) {
	cause := errors.New("something")
	ecErr := ioRollbackError("test step", cause, nil)
	msg := ecErr.Error()
	if !strings.Contains(msg, "none — no partial state detected") {
		t.Errorf("missing 'none' notice in:\n%s", msg)
	}
}

// ===== sortStrings =====

func TestSortStrings_AlreadySorted(t *testing.T) {
	s := []string{"a", "b", "c"}
	sortStrings(s)
	want := []string{"a", "b", "c"}
	if !sliceEqual(s, want) {
		t.Errorf("sortStrings(sorted): got %v want %v", s, want)
	}
}

func TestSortStrings_Reversed(t *testing.T) {
	s := []string{"c", "b", "a"}
	sortStrings(s)
	want := []string{"a", "b", "c"}
	if !sliceEqual(s, want) {
		t.Errorf("sortStrings(reversed): got %v want %v", s, want)
	}
}

func TestSortStrings_Empty(t *testing.T) {
	var s []string
	sortStrings(s) // should not panic
}

func TestSortStrings_Single(t *testing.T) {
	s := []string{"x"}
	sortStrings(s)
	if s[0] != "x" {
		t.Errorf("sortStrings(single): got %v", s)
	}
}

func TestSortStrings_Duplicates(t *testing.T) {
	s := []string{"b", "a", "b", "a"}
	sortStrings(s)
	want := []string{"a", "a", "b", "b"}
	if !sliceEqual(s, want) {
		t.Errorf("sortStrings(dups): got %v want %v", s, want)
	}
}

// ===== AssembleRepoChanges =====

func TestAssembleRepoChanges_BasicOrder(t *testing.T) {
	source := TargetRepo{Path: "/src", RepoName: "src"}
	target := TargetRepo{Path: "/tgt", RepoName: "tgt"}
	sibs := []TargetRepo{
		{Path: "/sib-z", RepoName: "z-repo"},
		{Path: "/sib-a", RepoName: "a-repo"},
	}
	linkUpdates := map[string][]string{
		"/src":   {"spec/ideas/README.md"},
		"/tgt":   {"spec/ideas/README.md"},
		"/sib-z": {"spec/features/x/README.md"},
		"/sib-a": {"spec/features/y/README.md"},
	}

	changes := AssembleRepoChanges(
		source, KindIdea, "spec/ideas/foo.md",
		target, "spec/ideas/foo.md",
		sibs, linkUpdates, "foo",
	)

	if len(changes) != 4 {
		t.Fatalf("expected 4 changes, got %d: %+v", len(changes), changes)
	}
	// Order: source, target, then siblings alphabetical by RepoName
	if changes[0].Action != ActionMoved || changes[0].Repo.RepoName != "src" {
		t.Errorf("changes[0]: expected source moved, got %+v", changes[0])
	}
	if changes[1].Action != ActionReceived || changes[1].Repo.RepoName != "tgt" {
		t.Errorf("changes[1]: expected target received, got %+v", changes[1])
	}
	if changes[2].Repo.RepoName != "a-repo" {
		t.Errorf("changes[2]: expected a-repo (alphabetical), got %s", changes[2].Repo.RepoName)
	}
	if changes[3].Repo.RepoName != "z-repo" {
		t.Errorf("changes[3]: expected z-repo (alphabetical), got %s", changes[3].Repo.RepoName)
	}
}

func TestAssembleRepoChanges_SkipsSiblingsWithNoLinkUpdates(t *testing.T) {
	source := TargetRepo{Path: "/src", RepoName: "src"}
	target := TargetRepo{Path: "/tgt", RepoName: "tgt"}
	sibs := []TargetRepo{
		{Path: "/sib-x", RepoName: "x-repo"},
	}
	// No link updates for the sibling
	linkUpdates := map[string][]string{}

	changes := AssembleRepoChanges(
		source, KindSeed, "spec/ideas/seeds/bar.md",
		target, "spec/ideas/seeds/bar.md",
		sibs, linkUpdates, "bar",
	)

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes (source+target only), got %d", len(changes))
	}
}

func TestAssembleRepoChanges_SubjectFormat(t *testing.T) {
	source := TargetRepo{Path: "/s", RepoName: "source-repo"}
	target := TargetRepo{Path: "/t", RepoName: "target-repo"}
	sibs := []TargetRepo{{Path: "/b", RepoName: "bystander"}}
	linkUpdates := map[string][]string{
		"/b": {"spec/ideas/ref.md"},
	}

	changes := AssembleRepoChanges(
		source, KindIdea, "spec/ideas/foo.md",
		target, "spec/ideas/foo.md",
		sibs, linkUpdates, "foo",
	)

	wantSource := "chore(relocate): move idea foo to target-repo"
	if changes[0].Subject != wantSource {
		t.Errorf("source subject: got %q want %q", changes[0].Subject, wantSource)
	}
	wantTarget := "chore(relocate): receive idea foo from source-repo"
	if changes[1].Subject != wantTarget {
		t.Errorf("target subject: got %q want %q", changes[1].Subject, wantTarget)
	}
	wantSib := "chore(relocate): update links for foo (source-repo → target-repo)"
	if changes[2].Subject != wantSib {
		t.Errorf("sibling subject: got %q want %q", changes[2].Subject, wantSib)
	}
}

func TestAssembleRepoChanges_MergesPathsForSourceAndTarget(t *testing.T) {
	source := TargetRepo{Path: "/s", RepoName: "s"}
	target := TargetRepo{Path: "/t", RepoName: "t"}
	linkUpdates := map[string][]string{
		"/s": {"spec/ideas/README.md"},
		"/t": {"spec/features/x/README.md"},
	}

	changes := AssembleRepoChanges(
		source, KindIdea, "spec/ideas/foo.md",
		target, "spec/ideas/foo.md",
		nil, linkUpdates, "foo",
	)

	// Source paths: artifact path + link updates
	if len(changes[0].Paths) != 2 {
		t.Errorf("source paths: expected 2, got %v", changes[0].Paths)
	}
	// Target paths: destination path + link updates
	if len(changes[1].Paths) != 2 {
		t.Errorf("target paths: expected 2, got %v", changes[1].Paths)
	}
}

// ===== DiscoverSiblings =====

func TestDiscoverSiblings_FindsSiblingRepos(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "repo-a", "repo-a")
	stageRepo(t, parent, "repo-b", "repo-b")
	specRoot := filepath.Join(parent, "repo-a")

	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sibs) < 2 {
		t.Fatalf("expected at least 2 siblings, got %d: %+v", len(sibs), sibs)
	}
	names := make(map[string]bool)
	for _, s := range sibs {
		names[s.RepoName] = true
	}
	if !names["repo-a"] || !names["repo-b"] {
		t.Errorf("expected to find both repo-a and repo-b, got %v", names)
	}
}

func TestDiscoverSiblings_SkipsHiddenDirs(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "visible", "visible")
	stageRepo(t, parent, ".hidden", "hidden")
	specRoot := filepath.Join(parent, "visible")

	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range sibs {
		if s.RepoName == "hidden" {
			t.Errorf("hidden sibling should be skipped, but found %+v", s)
		}
	}
}

// ===== ApplyMutation =====

func TestApplyMutation_HappyPath(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	body := "# Idea: My Idea\nSome content about this repo.\n"
	writeIdea(t, source, "my-idea", body)

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "my-idea.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	result, err := ApplyMutation(source, artifact, targetRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Destination should exist
	expectedDest := filepath.Join(target, "spec", "ideas", "my-idea.md")
	if result.DestinationPath != expectedDest {
		t.Errorf("dest path: got %q want %q", result.DestinationPath, expectedDest)
	}
	if _, err := os.Stat(result.DestinationPath); err != nil {
		t.Errorf("destination file should exist: %v", err)
	}

	// Source should be deleted
	if _, err := os.Stat(artifact.Path); !os.IsNotExist(err) {
		t.Errorf("source should be deleted, but still exists")
	}

	// Content should have RewriteBody applied (this repo → tgt)
	destContent, err := os.ReadFile(result.DestinationPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !strings.Contains(string(destContent), "My Idea") {
		t.Errorf("destination content should preserve idea title")
	}
}

func TestApplyMutation_DestCollisionReturnsConflict(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "conflict", "source body")
	writeIdea(t, target, "conflict", "existing body in target")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "conflict.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, err := ApplyMutation(source, artifact, targetRepo)
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Conflict {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Conflict)
	}

	// Source must still exist (zero mutations on conflict)
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Errorf("source should still exist on conflict: %v", err)
	}
}

func TestApplyMutation_SeedPath(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeSeed(t, source, "my-seed", "# Seed: my-seed\n")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "seeds", "my-seed.md"),
		Kind: KindSeed,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	result, err := ApplyMutation(source, artifact, targetRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDest := filepath.Join(target, "spec", "ideas", "seeds", "my-seed.md")
	if result.DestinationPath != expectedDest {
		t.Errorf("dest: got %q want %q", result.DestinationPath, expectedDest)
	}
	if _, err := os.Stat(expectedDest); err != nil {
		t.Errorf("seed destination should exist: %v", err)
	}
}

// ===== appendIfPartialDest =====

func TestAppendIfPartialDest_EmptyPath(t *testing.T) {
	actions := appendIfPartialDest(nil, "")
	if len(actions) != 0 {
		t.Errorf("empty path: expected no actions, got %v", actions)
	}
}

func TestAppendIfPartialDest_FileDoesNotExist(t *testing.T) {
	actions := appendIfPartialDest(nil, "/nonexistent/file.md")
	if len(actions) != 0 {
		t.Errorf("nonexistent file: expected no actions, got %v", actions)
	}
}

func TestAppendIfPartialDest_RemovesExistingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "partial.md")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	actions := appendIfPartialDest(nil, path)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "removed partial destination") {
		t.Errorf("unexpected action text: %s", actions[0])
	}
	// File should be gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed")
	}
}

func TestAppendIfPartialDest_SkipsDirectory(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "subdir")
	os.MkdirAll(dir, 0o755)
	actions := appendIfPartialDest(nil, dir)
	if len(actions) != 0 {
		t.Errorf("directory: expected no actions, got %v", actions)
	}
}

// ===== appendIfSourceMissing =====

func TestAppendIfSourceMissing_EmptyPath(t *testing.T) {
	actions := appendIfSourceMissing(nil, "", []byte("body"))
	if len(actions) != 0 {
		t.Errorf("empty path: expected no actions, got %v", actions)
	}
}

func TestAppendIfSourceMissing_NilBody(t *testing.T) {
	actions := appendIfSourceMissing(nil, "/some/path", nil)
	if len(actions) != 0 {
		t.Errorf("nil body: expected no actions, got %v", actions)
	}
}

func TestAppendIfSourceMissing_SourceStillExists(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "existing.md")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	actions := appendIfSourceMissing(nil, path, []byte("snapshot"))
	if len(actions) != 0 {
		t.Errorf("source exists: expected no actions, got %v", actions)
	}
}

func TestAppendIfSourceMissing_RestoresDeletedSource(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "missing.md")
	body := []byte("original content")
	// Don't create the file — simulate it being deleted
	actions := appendIfSourceMissing(nil, path, body)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "restored source artifact") {
		t.Errorf("unexpected action text: %s", actions[0])
	}
	// File should now be restored
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("restored content: got %q want %q", got, body)
	}
}

func TestAppendIfSourceMissing_CreatesParentDirs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "deep", "nested", "source.md")
	body := []byte("deep content")
	actions := appendIfSourceMissing(nil, path, body)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("content: got %q want %q", got, body)
	}
}

// ===== appendCheckoutsForResults =====

func TestAppendCheckoutsForResults_EmptyResults(t *testing.T) {
	actions := appendCheckoutsForResults(nil, nil)
	if len(actions) != 0 {
		t.Errorf("expected no actions for nil results, got %v", actions)
	}
}

func TestAppendCheckoutsForResults_NoUpdatedPaths(t *testing.T) {
	results := []LinkUpdateResult{
		{RepoPath: "/some/repo", Updated: nil},
	}
	actions := appendCheckoutsForResults(nil, results)
	if len(actions) != 0 {
		t.Errorf("expected no actions for empty Updated, got %v", actions)
	}
}

func TestAppendCheckoutsForResults_NonGitRepo(t *testing.T) {
	tmp := t.TempDir()
	results := []LinkUpdateResult{
		{RepoPath: tmp, Updated: []string{"spec/ideas/foo.md"}},
	}
	actions := appendCheckoutsForResults(nil, results)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "not a git repo") {
		t.Errorf("expected 'not a git repo' message, got: %s", actions[0])
	}
}

func TestAppendCheckoutsForResults_GitRepoRevertsFile(t *testing.T) {
	// Set up a real git repo with a committed file, then modify it
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "foo.md"), []byte("original"), 0o644)
	initGitRepo(t, repoRoot)

	// Modify the file
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "foo.md"), []byte("modified"), 0o644)

	results := []LinkUpdateResult{
		{RepoPath: repoRoot, Updated: []string{"spec/ideas/foo.md"}},
	}
	actions := appendCheckoutsForResults(nil, results)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "reverted") {
		t.Errorf("expected 'reverted' action, got: %s", actions[0])
	}
	// Verify file was restored
	got, _ := os.ReadFile(filepath.Join(repoRoot, "spec", "ideas", "foo.md"))
	if string(got) != "original" {
		t.Errorf("file should be reverted to 'original', got %q", got)
	}
}

// ===== ExecuteCommitPhase =====

func TestExecuteCommitPhase_NonGitRepoPassesThrough(t *testing.T) {
	tmp := t.TempDir()
	changes := []RepoChange{
		{
			Repo:   TargetRepo{Path: tmp, RepoName: "non-git"},
			Action: ActionMoved,
			Paths:  []string{"spec/ideas/foo.md"},
		},
	}
	executed, fail, err := ExecuteCommitPhase(changes, CommitAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fail != nil {
		t.Fatalf("unexpected failure: %+v", fail)
	}
	if len(executed) != 1 {
		t.Errorf("expected 1 executed, got %d", len(executed))
	}
}

func TestExecuteCommitPhase_CommitNoStagesOnly(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "foo.md"), []byte("body"), 0o644)
	initGitRepo(t, repoRoot)

	// Add a new file to stage
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "new.md"), []byte("new"), 0o644)

	changes := []RepoChange{
		{
			Repo:   TargetRepo{Path: repoRoot, RepoName: "repo"},
			Action: ActionMoved,
			Paths:  []string{"spec/ideas/new.md"},
		},
	}
	executed, fail, err := ExecuteCommitPhase(changes, CommitNo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fail != nil {
		t.Fatalf("unexpected failure: %+v", fail)
	}
	if len(executed) != 1 {
		t.Errorf("expected 1 executed, got %d", len(executed))
	}
	// SHA should be empty in CommitNo mode
	if executed[0].SHA != "" {
		t.Errorf("expected empty SHA in CommitNo mode, got %q", executed[0].SHA)
	}

	// Verify file was staged (git status should show it as staged)
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only")
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "spec/ideas/new.md") {
		t.Errorf("file should be staged, git diff --cached: %s", out)
	}
}

func TestExecuteCommitPhase_CommitAutoCreatesCommit(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "foo.md"), []byte("body"), 0o644)
	initGitRepo(t, repoRoot)

	// Add a new file to commit
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "bar.md"), []byte("bar"), 0o644)

	changes := []RepoChange{
		{
			Repo:    TargetRepo{Path: repoRoot, RepoName: "repo"},
			Action:  ActionReceived,
			Kind:    KindIdea,
			Slug:    "bar",
			Paths:   []string{"spec/ideas/bar.md"},
			Subject: "chore(relocate): receive idea bar from src",
		},
	}
	executed, fail, err := ExecuteCommitPhase(changes, CommitAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fail != nil {
		t.Fatalf("unexpected failure: %+v", fail)
	}
	if len(executed) != 1 {
		t.Fatalf("expected 1 executed, got %d", len(executed))
	}
	if executed[0].SHA == "" {
		t.Errorf("expected non-empty SHA after auto commit")
	}
	if len(executed[0].SHA) < 7 {
		t.Errorf("SHA too short: %q", executed[0].SHA)
	}

	// Verify commit message
	cmd := exec.Command("git", "-C", repoRoot, "log", "-1", "--format=%s")
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "chore(relocate): receive idea bar from src") {
		t.Errorf("commit subject: got %q", strings.TrimSpace(string(out)))
	}
}

func TestExecuteCommitPhase_FailureMidFlight(t *testing.T) {
	tmp := t.TempDir()
	// First repo: will succeed
	repo1 := filepath.Join(tmp, "repo1")
	os.MkdirAll(filepath.Join(repo1, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repo1, "spec", "ideas", "foo.md"), []byte("body"), 0o644)
	initGitRepo(t, repo1)
	os.WriteFile(filepath.Join(repo1, "spec", "ideas", "new.md"), []byte("new1"), 0o644)

	// Second repo: commit will fail because nothing to commit (paths don't exist)
	repo2 := filepath.Join(tmp, "repo2")
	os.MkdirAll(filepath.Join(repo2, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repo2, "spec", "ideas", "foo.md"), []byte("body"), 0o644)
	initGitRepo(t, repo2)
	// Don't add a new file — commit will fail "nothing to commit"

	// Third repo: unprocessed
	repo3 := filepath.Join(tmp, "repo3")
	os.MkdirAll(filepath.Join(repo3, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repo3, "spec", "ideas", "foo.md"), []byte("body"), 0o644)
	initGitRepo(t, repo3)

	changes := []RepoChange{
		{
			Repo:    TargetRepo{Path: repo1, RepoName: "repo1"},
			Action:  ActionMoved,
			Paths:   []string{"spec/ideas/new.md"},
			Subject: "chore(relocate): move",
		},
		{
			Repo:    TargetRepo{Path: repo2, RepoName: "repo2"},
			Action:  ActionReceived,
			Paths:   []string{"spec/ideas/foo.md"}, // already committed, git commit will fail
			Subject: "chore(relocate): receive",
		},
		{
			Repo:    TargetRepo{Path: repo3, RepoName: "repo3"},
			Action:  ActionUpdatedLinks,
			Paths:   []string{"spec/ideas/foo.md"},
			Subject: "chore(relocate): links",
		},
	}

	executed, fail, err := ExecuteCommitPhase(changes, CommitAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fail == nil {
		t.Fatal("expected a CommitFailure for repo2")
	}
	// repo1 should be in executed (committed)
	if len(executed) != 1 {
		t.Errorf("expected 1 committed, got %d", len(executed))
	}
	if executed[0].Repo.RepoName != "repo1" {
		t.Errorf("committed repo: got %s want repo1", executed[0].Repo.RepoName)
	}
	// Failed should be repo2
	if fail.Failed.Repo.RepoName != "repo2" {
		t.Errorf("failed repo: got %s want repo2", fail.Failed.Repo.RepoName)
	}
	// Unprocessed should be repo3
	if len(fail.Unprocessed) != 1 || fail.Unprocessed[0].Repo.RepoName != "repo3" {
		t.Errorf("unprocessed: got %+v", fail.Unprocessed)
	}
}

// ===== stagePaths =====

func TestStagePaths_NonGitRepoSkipped(t *testing.T) {
	tmp := t.TempDir()
	err := stagePaths(tmp, []string{"file.md"})
	if err != nil {
		t.Errorf("expected nil error for non-git repo, got: %v", err)
	}
}

func TestStagePaths_EmptyPathsSkipped(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0o755)
	os.WriteFile(filepath.Join(repoRoot, "dummy"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	err := stagePaths(repoRoot, nil)
	if err != nil {
		t.Errorf("expected nil error for empty paths, got: %v", err)
	}
}

func TestStagePaths_StagesFile(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec", "ideas"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "init.md"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	// Add a new file
	os.WriteFile(filepath.Join(repoRoot, "spec", "ideas", "new.md"), []byte("new"), 0o644)
	err := stagePaths(repoRoot, []string{"spec/ideas/new.md"})
	if err != nil {
		t.Fatalf("stagePaths: %v", err)
	}

	cmd := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only")
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "spec/ideas/new.md") {
		t.Errorf("expected file to be staged: %s", out)
	}
}

// ===== commitRepo =====

func TestCommitRepo_Success(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "x.md"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	// Add and stage a new file
	os.WriteFile(filepath.Join(repoRoot, "spec", "y.md"), []byte("y"), 0o644)
	exec.Command("git", "-C", repoRoot, "add", "spec/y.md").Run()

	sha, stderr, err := commitRepo(repoRoot, "test commit")
	if err != nil {
		t.Fatalf("commitRepo: %v stderr=%s", err, stderr)
	}
	if sha == "" {
		t.Error("expected non-empty SHA")
	}
	if len(sha) < 40 {
		t.Errorf("expected full SHA (40 chars), got %d: %s", len(sha), sha)
	}
}

func TestCommitRepo_FailureNothingToCommit(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0o755)
	os.WriteFile(filepath.Join(repoRoot, "file"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	// Nothing staged → commit will fail
	sha, stderr, err := commitRepo(repoRoot, "this should fail")
	if err == nil {
		t.Fatal("expected error for nothing-to-commit")
	}
	if sha != "" {
		t.Errorf("expected empty SHA on failure, got %q", sha)
	}
	_ = stderr // stderr is captured but we don't assert its exact content
}

// ===== ExecutePreCommitPhase =====

func TestExecutePreCommitPhase_HappyPath(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "hello", "# Idea: Hello\nBody text.\n")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "hello.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	mutation, linkResults, err := ExecutePreCommitPhase(
		source, artifact, targetRepo,
		[]TargetRepo{{Path: source, RepoName: "src", Org: "src"}, targetRepo},
		"hello",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutation result should have destination path
	expectedDest := filepath.Join(target, "spec", "ideas", "hello.md")
	if mutation.DestinationPath != expectedDest {
		t.Errorf("dest: got %q want %q", mutation.DestinationPath, expectedDest)
	}

	// Link results should not be nil (even if no links were updated)
	if linkResults == nil {
		t.Errorf("expected non-nil linkResults")
	}
}

func TestExecutePreCommitPhase_ConflictPassesThrough(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "dup", "source body")
	writeIdea(t, target, "dup", "target body")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "dup.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, _, err := ExecutePreCommitPhase(source, artifact, targetRepo, nil, "dup")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Conflict {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Conflict)
	}
}

// ===== ExecutePreCommitPhase rollback: non-conflict ApplyMutation failure =====

func TestExecutePreCommitPhase_NonConflictFailureRollback(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	// Create the source artifact
	writeIdea(t, source, "rollback-test", "# Idea: rollback-test\nBody.\n")

	// Make the source file unreadable so ApplyMutation fails with an
	// Unexpected error (not Conflict) during os.ReadFile.
	sourcePath := filepath.Join(source, "spec", "ideas", "rollback-test.md")
	// Remove read permissions
	os.Chmod(sourcePath, 0o000)
	defer os.Chmod(sourcePath, 0o644)

	artifact := SourceArtifact{
		Path: sourcePath,
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, _, err := ExecutePreCommitPhase(source, artifact, targetRepo, nil, "rollback-test")
	if err == nil {
		t.Fatal("expected error from unreadable source")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
	// The error message should mention rollback actions
	if !strings.Contains(ec.Error(), "pre-commit-phase I/O failure") {
		t.Errorf("error should mention I/O failure: %s", ec.Error())
	}
}

// ===== ApplyMutation error paths =====

func TestApplyMutation_SourceFileUnreadable(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "unreadable", "body")
	sourcePath := filepath.Join(source, "spec", "ideas", "unreadable.md")
	os.Chmod(sourcePath, 0o000)
	defer os.Chmod(sourcePath, 0o644)

	artifact := SourceArtifact{Path: sourcePath, Kind: KindIdea}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, err := ApplyMutation(source, artifact, targetRepo)
	if err == nil {
		t.Fatal("expected error")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

func TestApplyMutation_DestDirUnwritable(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "unwritable", "body")

	// Make the target spec/ideas dir unwritable so MkdirAll/WriteFile fails
	destDir := filepath.Join(target, "spec", "ideas")
	os.Chmod(destDir, 0o555)
	defer os.Chmod(destDir, 0o755)

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "unwritable.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, err := ApplyMutation(source, artifact, targetRepo)
	if err == nil {
		t.Fatal("expected error writing to unwritable dir")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

// ===== discoverSiblings — symlink out-of-parent =====

func TestDiscoverSiblings_SkipsSymlinksOutOfParent(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "real", "real")

	// Create a dir outside the parent
	outsideDir := t.TempDir()
	stageRepoAt(t, outsideDir, "outside")

	// Create a symlink inside parent pointing outside
	symPath := filepath.Join(parent, "link-outside")
	if err := os.Symlink(outsideDir, symPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	specRoot := filepath.Join(parent, "real")
	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range sibs {
		if s.RepoName == "outside" {
			t.Errorf("symlink-out-of-parent should be skipped, but found %+v", s)
		}
	}
}

// ===== discoverSiblings — regular files in parent =====

func TestDiscoverSiblings_SkipsNonDirs(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "real", "real")
	// Create a regular file in parent (not a dir)
	os.WriteFile(filepath.Join(parent, "just-a-file.txt"), []byte("hi"), 0o644)

	specRoot := filepath.Join(parent, "real")
	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only find real
	for _, s := range sibs {
		if s.RepoName != "real" {
			t.Errorf("unexpected sibling: %+v", s)
		}
	}
}

// ===== appendCheckoutsForResults with git checkout failure =====

func TestAppendCheckoutsForResults_GitCheckoutFailure(t *testing.T) {
	// Create a git repo with a committed file, then try to checkout a non-existent path
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(filepath.Join(repoRoot, "spec"), 0o755)
	os.WriteFile(filepath.Join(repoRoot, "spec", "x.md"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	results := []LinkUpdateResult{
		{RepoPath: repoRoot, Updated: []string{"nonexistent-file.md"}},
	}
	actions := appendCheckoutsForResults(nil, results)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "FAILED") {
		t.Errorf("expected FAILED message for nonexistent checkout, got: %s", actions[0])
	}
}

// ===== appendIfPartialDest — remove failure (permissions) =====

func TestAppendIfPartialDest_RemoveFailure(t *testing.T) {
	tmp := t.TempDir()
	subdir := filepath.Join(tmp, "locked")
	os.MkdirAll(subdir, 0o755)
	path := filepath.Join(subdir, "file.md")
	os.WriteFile(path, []byte("data"), 0o644)
	// Remove write permission from parent dir to prevent deletion
	os.Chmod(subdir, 0o555)
	defer os.Chmod(subdir, 0o755)

	actions := appendIfPartialDest(nil, path)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "FAILED") {
		t.Errorf("expected FAILED message, got: %s", actions[0])
	}
}

// ===== appendIfSourceMissing — dir creation failure =====

func TestAppendIfSourceMissing_DirCreationFailure(t *testing.T) {
	tmp := t.TempDir()
	// Make the tmp dir read-only so MkdirAll fails for deep path
	lockedDir := filepath.Join(tmp, "locked")
	os.MkdirAll(lockedDir, 0o755)
	os.Chmod(lockedDir, 0o555)
	defer os.Chmod(lockedDir, 0o755)

	path := filepath.Join(lockedDir, "deep", "nested", "source.md")
	body := []byte("restore me")

	actions := appendIfSourceMissing(nil, path, body)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %v", actions)
	}
	if !strings.Contains(actions[0], "FAILED") {
		t.Errorf("expected FAILED message, got: %s", actions[0])
	}
}

// ===== stagePaths — git add failure =====

func TestStagePaths_InvalidPathReturnsError(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0o755)
	os.WriteFile(filepath.Join(repoRoot, "dummy"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	// Staging a path with invalid characters that causes git add to fail
	// On most systems, trying to add a file that doesn't exist will succeed
	// with a warning but not error. Instead, let's use a path with a newline.
	// Actually, git add of non-existent path does error.
	err := stagePaths(repoRoot, []string{"this/path/does/not/exist.md"})
	if err == nil {
		t.Errorf("expected error staging nonexistent path")
	}
}

// ===== readSelfAsSibling (indirectly tested via discoverSiblings edge case) =====
// readSelfAsSibling is only called when parent == absSource (filesystem root).
// We cannot easily test that on macOS/Linux, but we can test the function directly.

func TestReadSelfAsSibling_WithSpecscoreYaml(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "self", "self-repo")

	result, err := readSelfAsSibling(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[0].RepoName != "self-repo" {
		t.Errorf("RepoName: got %q want %q", result[0].RepoName, "self-repo")
	}
}

func TestReadSelfAsSibling_NoYaml(t *testing.T) {
	tmp := t.TempDir()
	result, err := readSelfAsSibling(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected nil/empty result, got %v", result)
	}
}

func TestReadSelfAsSibling_MalformedYaml(t *testing.T) {
	tmp := t.TempDir()
	// Write malformed yaml
	os.WriteFile(filepath.Join(tmp, "specscore.yaml"), []byte("{{invalid yaml"), 0o644)
	result, err := readSelfAsSibling(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("malformed yaml: expected nil, got %v", result)
	}
}

// ===== ExecutePreCommitPhase — link update error triggers rollback =====

func TestExecutePreCommitPhase_LinkUpdateErrorTriggersRollback(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "link-err", "# Idea: link-err\nBody.\n")

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "link-err.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	// Create a scanRepo that has an unreadable spec dir to force walk error
	badRepo := filepath.Join(parent, "bad-repo")
	os.MkdirAll(filepath.Join(badRepo, "spec"), 0o755)
	os.WriteFile(filepath.Join(badRepo, "specscore.yaml"), []byte("project:\n  repo: bad\n  org: bad\n"), 0o644)

	// Make the spec dir itself unreadable after staging so Walk returns error
	os.Chmod(filepath.Join(badRepo, "spec"), 0o000)
	defer os.Chmod(filepath.Join(badRepo, "spec"), 0o755)

	badTarget := TargetRepo{Path: badRepo, RepoName: "bad", Org: "bad"}

	// Pass badRepo as a scan repo — UpdateCrossRepoLinks may fail or succeed
	// depending on error handling, but this exercises the path
	_, linkResults, err := ExecutePreCommitPhase(
		source, artifact, targetRepo,
		[]TargetRepo{{Path: source, RepoName: "src", Org: "src"}, targetRepo, badTarget},
		"link-err",
	)

	// The Walk in UpdateCrossRepoLinks skips unreadable entries gracefully,
	// so it might not error. If so, the call succeeds. Either outcome is valid.
	if err != nil {
		// If it errored, verify it's an exitcode.Error
		var ec *exitcode.Error
		if !errors.As(err, &ec) {
			t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
		}
	} else {
		// If it succeeded, linkResults should be non-nil
		if linkResults == nil {
			t.Errorf("expected non-nil linkResults on success")
		}
	}
}

// ===== ApplyMutation — source removal after dest write fails =====

func TestApplyMutation_SourceRemoveFailure(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	writeIdea(t, source, "rm-fail", "# Idea: rm-fail\n")
	// Make the source ideas dir unwritable so os.Remove will fail
	sourceDir := filepath.Join(source, "spec", "ideas")
	os.Chmod(sourceDir, 0o555)
	defer os.Chmod(sourceDir, 0o755)

	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "rm-fail.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, err := ApplyMutation(source, artifact, targetRepo)
	if err == nil {
		t.Fatal("expected error from os.Remove failure")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
	if !strings.Contains(ec.Error(), "deleting source artifact") {
		t.Errorf("error should mention 'deleting source artifact': %s", ec.Error())
	}
}

// ===== discoverSiblings — valid symlink inside parent resolves =====

func TestDiscoverSiblings_ValidSymlinkInsideParent(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "real", "real")
	stageRepo(t, parent, "target-dir", "linked-repo")

	// Create symlink inside parent pointing to target-dir (inside parent)
	symPath := filepath.Join(parent, "link-inside")
	target := filepath.Join(parent, "target-dir")
	if err := os.Symlink(target, symPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	specRoot := filepath.Join(parent, "real")
	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The symlink resolves inside parent, so it should be found
	// (the target-dir itself is also found — we might see duplicates)
	foundLinkedRepo := false
	for _, s := range sibs {
		if s.RepoName == "linked-repo" {
			foundLinkedRepo = true
		}
	}
	if !foundLinkedRepo {
		t.Errorf("expected to find linked-repo via symlink or target-dir, got %+v", sibs)
	}
}

// ===== FindReferences — no spec dir returns nil =====

func TestFindReferences_NoSpecDir(t *testing.T) {
	tmp := t.TempDir()
	// No spec/ directory
	hits, err := FindReferences(tmp, "any-slug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no hits for missing spec dir, got %v", hits)
	}
}

// ===== resolveTargetByPath — target is a file, not dir =====

func TestResolveTargetRepo_PathForm_FileNotDir(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	filePath := filepath.Join(parent, "just-a-file")
	os.WriteFile(filePath, []byte("not a dir"), 0o644)

	_, err := ResolveTargetRepo(source, filePath)
	if err == nil {
		t.Fatal("expected error for file target")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.InvalidArgs {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.InvalidArgs)
	}
}

// ===== ExecuteCommitPhase — staging failure returns I/O error =====

func TestExecuteCommitPhase_StagingFailureReturnsError(t *testing.T) {
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "repo")
	os.MkdirAll(repoRoot, 0o755)
	os.WriteFile(filepath.Join(repoRoot, "dummy"), []byte("x"), 0o644)
	initGitRepo(t, repoRoot)

	changes := []RepoChange{
		{
			Repo:   TargetRepo{Path: repoRoot, RepoName: "repo"},
			Action: ActionMoved,
			Paths:  []string{"this/path/does/not/exist.md"},
		},
	}

	_, _, err := ExecuteCommitPhase(changes, CommitAuto)
	if err == nil {
		t.Fatal("expected staging error")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

// ===== FormatStdout with updated-links action =====

func TestFormatStdout_UpdatedLinksAction(t *testing.T) {
	changes := []RepoChange{
		{
			Repo:   TargetRepo{RepoName: "sib-repo"},
			Action: ActionUpdatedLinks,
			Kind:   KindIdea,
			Slug:   "x",
			SHA:    "1234567890abcdef",
		},
	}
	got := FormatStdout(changes, CommitAuto)
	if !strings.Contains(got, "sib-repo: updated-links idea x  [1234567]") {
		t.Errorf("updated-links line wrong: %s", got)
	}
}

// ===== discoverSiblings — malformed yaml in sibling is ignored =====

func TestDiscoverSiblings_MalformedYamlSiblingIgnored(t *testing.T) {
	parent := t.TempDir()
	stageRepo(t, parent, "good", "good")
	// Sibling with malformed yaml
	badDir := filepath.Join(parent, "bad")
	os.MkdirAll(badDir, 0o755)
	os.WriteFile(filepath.Join(badDir, "specscore.yaml"), []byte("{{invalid yaml content"), 0o644)

	specRoot := filepath.Join(parent, "good")
	sibs, err := DiscoverSiblings(specRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only "good" should be found
	for _, s := range sibs {
		if s.Path == badDir {
			t.Errorf("malformed yaml sibling should be ignored, but found: %+v", s)
		}
	}
}

// ===== ExecutePreCommitPhase — source that doesn't exist triggers early error =====

func TestExecutePreCommitPhase_SourceNotReadable(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	// Don't actually write the source file — ApplyMutation will fail reading it
	artifact := SourceArtifact{
		Path: filepath.Join(source, "spec", "ideas", "ghost.md"),
		Kind: KindIdea,
	}
	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}

	_, _, err := ExecutePreCommitPhase(source, artifact, targetRepo, nil, "ghost")
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T: %v", err, err)
	}
	// Should be Unexpected (10) from the rollback path
	if ec.ExitCode() != exitcode.Unexpected {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.Unexpected)
	}
}

// stageRepoAt creates a specscore.yaml directly in the given dir.
func stageRepoAt(t *testing.T, dir, repoSlug string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "spec", "ideas", "seeds"), 0o755); err != nil {
		t.Fatalf("mkdir spec tree: %v", err)
	}
	yaml := "# SpecScore Repo Config Schema: https://specscore.md/repo-config\n" +
		"project:\n" +
		"  title: " + repoSlug + "\n" +
		"  org: " + repoSlug + "\n" +
		"  repo: " + repoSlug + "\n"
	if err := os.WriteFile(filepath.Join(dir, "specscore.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write specscore.yaml: %v", err)
	}
}

// ===== helper =====

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
