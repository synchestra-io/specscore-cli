package idearelocate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Unit tests for UpdateCrossRepoLinks. Two flavors covered:
//
//   - Cross-repo rewrite: link in repo A pointing at the relocated
//     artifact gets replaced with the full GitHub URL form computed
//     from target.Org / target.RepoName.
//   - Same-repo rewrite: link in the target repo (the new home) gets
//     replaced with a relative path from the referencing file's
//     directory to the new artifact location.
//
// Metadata-line preservation is a third concern: bold-prefixed
// metadata lines (**Source Ideas:**, **Related Ideas:**, …) must NOT
// be rewritten, even if they contain a markdown link to the slug.

func TestUpdateCrossRepoLinks_RewritesCrossRepoLinkToGitHubURL(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "specstudio-skills", "specstudio-skills")
	target := stageRepo(t, parent, "specscore", "specscore")
	sib := stageRepo(t, parent, "specscore-cli", "specscore-cli")

	// Sibling has a Feature linking to the (originally source-resident) Idea.
	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "x"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(sib, "spec", "features", "x", "README.md")
	if err := os.WriteFile(refPath,
		[]byte("# Feature: X\n\nSee [the Idea](../../../specstudio-skills/spec/ideas/foo.md) for details.\n"),
		0o644); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	targetRepo := TargetRepo{Path: target, RepoName: "specscore", Org: "specscore"}
	sourceRepo := TargetRepo{Path: source, RepoName: "specstudio-skills", Org: "specstudio-skills"}
	sibRepo := TargetRepo{Path: sib, RepoName: "specscore-cli", Org: "specscore-cli"}

	results, err := UpdateCrossRepoLinks(
		[]TargetRepo{sourceRepo, targetRepo, sibRepo},
		targetRepo, "foo", "spec/ideas/foo.md",
	)
	if err != nil {
		t.Fatalf("UpdateCrossRepoLinks: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read ref after: %v", err)
	}
	wantURL := "https://github.com/specscore/specscore/blob/main/spec/ideas/foo.md"
	if !strings.Contains(string(got), wantURL) {
		t.Errorf("expected sibling link rewritten to %q; got:\n%s", wantURL, got)
	}
	if !strings.Contains(string(got), "[the Idea]") {
		t.Errorf("expected display text 'the Idea' preserved; got:\n%s", got)
	}

	// Sibling repo should appear in results with the ref file listed.
	var sibResult *LinkUpdateResult
	for i := range results {
		if results[i].RepoPath == sib {
			sibResult = &results[i]
		}
	}
	if sibResult == nil {
		t.Fatalf("expected results to include sibling repo %s; got %+v", sib, results)
	}
	wantRel := filepath.Join("spec", "features", "x", "README.md")
	found := false
	for _, p := range sibResult.Updated {
		if p == wantRel {
			found = true
		}
	}
	if !found {
		t.Errorf("expected sibling Updated to include %q; got %v", wantRel, sibResult.Updated)
	}
}

func TestUpdateCrossRepoLinks_PreservesBoldPrefixedMetadataLines(t *testing.T) {
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")
	sib := stageRepo(t, parent, "sib", "sib")

	// Slug-only references on bold-prefixed metadata lines — even one
	// containing what could parse as a link target — must be untouched.
	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "y"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(sib, "spec", "features", "y", "README.md")
	content := "# Feature: Y\n\n" +
		"**Source Ideas:** foo\n" +
		"**Related Ideas:** foo\n" +
		"**Supersedes:** —\n\n" +
		"Body text without any link.\n"
	if err := os.WriteFile(refPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}
	sourceRepo := TargetRepo{Path: source, RepoName: "src", Org: "src"}
	sibRepo := TargetRepo{Path: sib, RepoName: "sib", Org: "sib"}

	_, err := UpdateCrossRepoLinks(
		[]TargetRepo{sourceRepo, targetRepo, sibRepo},
		targetRepo, "foo", "spec/ideas/foo.md",
	)
	if err != nil {
		t.Fatalf("UpdateCrossRepoLinks: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != content {
		t.Errorf("expected metadata-line-only file unchanged;\nwant:\n%s\ngot:\n%s", content, got)
	}
}

func TestUpdateCrossRepoLinks_RelativePathInTargetRepo(t *testing.T) {
	// A file inside the target repo that links to the relocated artifact
	// should get a relative path, not a full GitHub URL.
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")

	// Target has a Feature with a link to the (about-to-arrive) Idea.
	if err := os.MkdirAll(filepath.Join(target, "spec", "features", "z"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(target, "spec", "features", "z", "README.md")
	if err := os.WriteFile(refPath,
		[]byte("# Feature: Z\n\nSee [the Idea](../../ideas/foo.md) for context.\n"),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}
	sourceRepo := TargetRepo{Path: source, RepoName: "src", Org: "src"}

	if _, err := UpdateCrossRepoLinks(
		[]TargetRepo{sourceRepo, targetRepo},
		targetRepo, "foo", "spec/ideas/foo.md",
	); err != nil {
		t.Fatalf("UpdateCrossRepoLinks: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// In-repo rewrite must NOT produce a full GitHub URL.
	if strings.Contains(string(got), "https://github.com/") {
		t.Errorf("expected relative path inside target repo, not a GitHub URL; got:\n%s", got)
	}
	// Relative path from spec/features/z/ to spec/ideas/foo.md is
	// ../../ideas/foo.md — which happens to be the same as before,
	// so the file content is unchanged. The point is: no GitHub URL.
	if !strings.Contains(string(got), "../../ideas/foo.md") {
		t.Errorf("expected relative path '../../ideas/foo.md' preserved; got:\n%s", got)
	}
}

func TestUpdateCrossRepoLinks_BareSlugFileLinkMatches(t *testing.T) {
	// A markdown link whose URL is bare "<slug>.md" (no path component)
	// should also match — the relative-path regex must handle it.
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")
	sib := stageRepo(t, parent, "sib", "sib")

	refPath := filepath.Join(sib, "spec", "ideas", "neighbor.md")
	if err := os.WriteFile(refPath,
		[]byte("# Idea: neighbor\n\nSee [foo](foo.md) for related context.\n"),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}
	sourceRepo := TargetRepo{Path: source, RepoName: "src", Org: "src"}
	sibRepo := TargetRepo{Path: sib, RepoName: "sib", Org: "sib"}

	if _, err := UpdateCrossRepoLinks(
		[]TargetRepo{sourceRepo, targetRepo, sibRepo},
		targetRepo, "foo", "spec/ideas/foo.md",
	); err != nil {
		t.Fatalf("UpdateCrossRepoLinks: %v", err)
	}

	got, err := os.ReadFile(refPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	wantURL := "https://github.com/tgt/tgt/blob/main/spec/ideas/foo.md"
	if !strings.Contains(string(got), wantURL) {
		t.Errorf("expected bare-URL link rewritten to %q; got:\n%s", wantURL, got)
	}
}

func TestUpdateCrossRepoLinks_LeavesNonSlugLinksAlone(t *testing.T) {
	// A markdown link to a file that happens to end in "Xfoo.md" must
	// NOT be matched — the slug match requires a path-segment boundary.
	parent := t.TempDir()
	source := stageRepo(t, parent, "src", "src")
	target := stageRepo(t, parent, "tgt", "tgt")
	sib := stageRepo(t, parent, "sib", "sib")

	if err := os.MkdirAll(filepath.Join(sib, "spec", "features", "w"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	refPath := filepath.Join(sib, "spec", "features", "w", "README.md")
	original := "# Feature: W\n\nSee [near miss](path/notfoo.md) and [other](barfoo.md).\n"
	if err := os.WriteFile(refPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	targetRepo := TargetRepo{Path: target, RepoName: "tgt", Org: "tgt"}
	sourceRepo := TargetRepo{Path: source, RepoName: "src", Org: "src"}
	sibRepo := TargetRepo{Path: sib, RepoName: "sib", Org: "sib"}

	if _, err := UpdateCrossRepoLinks(
		[]TargetRepo{sourceRepo, targetRepo, sibRepo},
		targetRepo, "foo", "spec/ideas/foo.md",
	); err != nil {
		t.Fatalf("UpdateCrossRepoLinks: %v", err)
	}
	got, _ := os.ReadFile(refPath)
	if string(got) != original {
		t.Errorf("expected near-miss links untouched;\nwant:\n%s\ngot:\n%s", original, got)
	}
}
