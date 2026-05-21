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

// initGitRepo runs git init in root, sets local user.name/email, adds
// all files, and creates an initial commit. After this call, the
// working tree is clean from git's perspective.
func initGitRepo(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), root, err, out)
		}
	}
	run("init", "--initial-branch=main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", "-A")
	run("commit", "--no-gpg-sign", "-m", "initial")
}

// --- IsPathClean ---

func TestIsPathClean_NonGitRepoIsClean(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	writeIdea(t, root, "foo", "body")
	// no git init — root is not a git repo
	clean, err := IsPathClean(root, "spec/ideas/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clean {
		t.Errorf("expected non-git repo to be reported clean")
	}
}

func TestIsPathClean_GitRepoClean(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	writeIdea(t, root, "foo", "body")
	initGitRepo(t, root)
	clean, err := IsPathClean(root, "spec/ideas/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !clean {
		t.Errorf("expected freshly-committed file to be clean")
	}
}

func TestIsPathClean_GitRepoUnstagedDirty(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	writeIdea(t, root, "foo", "original")
	initGitRepo(t, root)
	// Modify in place without staging
	writeIdea(t, root, "foo", "modified after commit")
	clean, err := IsPathClean(root, "spec/ideas/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clean {
		t.Errorf("expected unstaged modification to be reported dirty")
	}
}

func TestIsPathClean_GitRepoUntrackedDirty(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	initGitRepo(t, root)
	// Create untracked file at the target path
	writeIdea(t, root, "foo", "newly added")
	clean, err := IsPathClean(root, "spec/ideas/foo.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if clean {
		t.Errorf("expected untracked file to be reported dirty")
	}
}

// --- FindReferences ---

func TestFindReferences_NoMatches(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	// Body unrelated to slug "foo"
	if err := os.WriteFile(
		filepath.Join(root, "spec", "ideas", "other.md"),
		[]byte("# Idea: Other\n**Source Ideas:** bar\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hits, err := FindReferences(root, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected no hits, got %v", hits)
	}
}

func TestFindReferences_MetadataMatch(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	body := "# Feature: X\n**Source Ideas:** foo, other\n\nbody\n"
	path := filepath.Join(root, "spec", "ideas", "x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hits, err := FindReferences(root, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 || hits[0] != filepath.Join("spec", "ideas", "x.md") {
		t.Errorf("expected one hit at spec/ideas/x.md, got %v", hits)
	}
}

func TestFindReferences_MetadataSubstringDoesNotMatch(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	// 'myfoo' is not the same slug as 'foo' — must not match
	body := "# Feature: X\n**Source Ideas:** myfoo, foo-extended\n"
	if err := os.WriteFile(
		filepath.Join(root, "spec", "ideas", "x.md"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hits, err := FindReferences(root, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("substring should not match, got %v", hits)
	}
}

func TestFindReferences_MarkdownLinkMatch(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	body := "# Feature: X\n\nSee [the Idea](../../specstudio-skills/spec/ideas/foo.md) for context.\n"
	path := filepath.Join(root, "spec", "ideas", "x.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hits, err := FindReferences(root, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("expected one hit, got %v", hits)
	}
}

func TestFindReferences_MarkdownLinkSubstringDoesNotMatch(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	// link target ends in 'myfoo.md', not '/foo.md' — must not match
	body := "# Feature: X\n\nSee [it](../../other/spec/ideas/myfoo.md).\n"
	if err := os.WriteFile(
		filepath.Join(root, "spec", "ideas", "x.md"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	hits, err := FindReferences(root, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("substring link should not match, got %v", hits)
	}
}

// --- CheckPreflight + DirtyTreeError ---

func TestCheckPreflight_AllClean(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	writeIdea(t, root, "foo", "body")
	initGitRepo(t, root)
	dirty, err := CheckPreflight([]PreflightSubject{
		{RepoRoot: root, RelPath: "spec/ideas/foo.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirty) != 0 {
		t.Errorf("expected no dirty subjects, got %v", dirty)
	}
}

func TestCheckPreflight_SomeDirty(t *testing.T) {
	parent := t.TempDir()
	root := stageRepo(t, parent, "src", "src")
	writeIdea(t, root, "foo", "original")
	writeIdea(t, root, "bar", "original")
	initGitRepo(t, root)
	// Dirty foo, leave bar clean
	writeIdea(t, root, "foo", "modified")
	dirty, err := CheckPreflight([]PreflightSubject{
		{RepoRoot: root, RelPath: "spec/ideas/foo.md"},
		{RepoRoot: root, RelPath: "spec/ideas/bar.md"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirty) != 1 || dirty[0].RelPath != "spec/ideas/foo.md" {
		t.Errorf("expected one dirty subject (foo.md), got %v", dirty)
	}
}

func TestDirtyTreeError_FormatAndExitCode(t *testing.T) {
	err := DirtyTreeError([]PreflightSubject{
		{RepoRoot: "/r1", RelPath: "spec/ideas/foo.md"},
		{RepoRoot: "/r2", RelPath: "spec/ideas/bar.md"},
	})
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var ec *exitcode.Error
	if !errors.As(err, &ec) {
		t.Fatalf("expected *exitcode.Error, got %T", err)
	}
	if ec.ExitCode() != exitcode.DirtyTree {
		t.Errorf("exit code: got %d want %d", ec.ExitCode(), exitcode.DirtyTree)
	}
	msg := ec.Error()
	for _, want := range []string{"/r1", "spec/ideas/foo.md", "/r2", "spec/ideas/bar.md"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message should contain %q; got: %s", want, msg)
		}
	}
}

func TestDirtyTreeError_EmptyReturnsNil(t *testing.T) {
	if err := DirtyTreeError(nil); err != nil {
		t.Errorf("expected nil for empty dirty list, got: %v", err)
	}
}

// --- PreflightSubjectsForRelocate ---

func TestPreflightSubjectsForRelocate_IncludesSiblingReferences(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")
	sib := stageRepo(t, parent, "sib", "sib")
	writeIdea(t, source, "foo", "")
	// sibling references the slug
	if err := os.WriteFile(
		filepath.Join(sib, "spec", "ideas", "ref.md"),
		[]byte("**Source Ideas:** foo\n"), 0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	subjects, err := PreflightSubjectsForRelocate(
		source, "spec/ideas/foo.md",
		target, "spec/ideas/foo.md",
		[]TargetRepo{{Path: sib, RepoName: "sib"}},
		"foo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// source artifact + source index + target dest + target index + 1 sibling ref = 5
	if len(subjects) != 5 {
		t.Errorf("expected 5 subjects, got %d: %v", len(subjects), subjects)
	}
	// Last subject is the sibling reference
	last := subjects[len(subjects)-1]
	if last.RepoRoot != sib || last.RelPath != filepath.Join("spec", "ideas", "ref.md") {
		t.Errorf("unexpected sibling subject: %+v", last)
	}
}
