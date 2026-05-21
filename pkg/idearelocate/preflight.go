package idearelocate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// IsPathClean reports whether <relPath> inside <repoRoot> has any
// uncommitted modifications (staged, unstaged, or untracked).
//
// A non-git repoRoot (no .git directory anywhere up the tree) is
// treated as vacuously clean — there is no working tree to check. This
// matches the verb's "best-effort" pre-flight discipline: invocations
// against a non-git project don't fail pre-flight on the absence of git
// itself; the commit phase later will fail loudly if commits are
// requested in such a project.
func IsPathClean(repoRoot, relPath string) (bool, error) {
	if !isGitRepo(repoRoot) {
		return true, nil
	}
	cmd := exec.Command("git", "-C", repoRoot, "status", "--porcelain", "--", relPath)
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status %s: %w", relPath, err)
	}
	return len(bytes.TrimSpace(out)) == 0, nil
}

// isGitRepo reports whether repoRoot is inside a git work tree.
func isGitRepo(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "--is-inside-work-tree")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// FindReferences walks repoRoot/spec/**/*.md and returns the repo-root-
// relative paths of files that reference <slug> per the patterns
// from cli/idea/relocate#req:preflight-clean-tree-siblings-with-references:
//
//   - Bold-prefixed metadata lines naming the slug in any of:
//     **Source Ideas:**, **Related Ideas:**, **Supersedes:**, **Promotes To:**.
//     The slug must appear as a comma-separated value, not as a substring
//     of a different value.
//   - Markdown links whose target ends in /<slug>.md (the slug is the
//     last path component before .md, optionally preceded by /).
//
// Files outside spec/ are ignored. The returned slice is sorted.
func FindReferences(repoRoot, slug string) ([]string, error) {
	specDir := filepath.Join(repoRoot, "spec")
	info, err := os.Stat(specDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	slugLinkRe := regexp.MustCompile(`\[[^\]]*\]\(([^)]*?(?:[/]|^)` + regexp.QuoteMeta(slug) + `\.md)\)`)
	metaPrefixes := []string{
		"**Source Ideas:**",
		"**Related Ideas:**",
		"**Supersedes:**",
		"**Promotes To:**",
	}

	var hits []string
	err = filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries; not a pre-flight error
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if !fileReferencesSlug(body, slug, slugLinkRe, metaPrefixes) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return nil
		}
		hits = append(hits, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// stable order
	sortStrings(hits)
	return hits, nil
}

// fileReferencesSlug checks the body for any of the reference patterns.
func fileReferencesSlug(body []byte, slug string, linkRe *regexp.Regexp, metaPrefixes []string) bool {
	if linkRe.Match(body) {
		return true
	}
	for _, line := range strings.Split(string(body), "\n") {
		for _, prefix := range metaPrefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if rest == "" || rest == "—" {
				continue
			}
			for _, part := range strings.Split(rest, ",") {
				if strings.TrimSpace(part) == slug {
					return true
				}
			}
		}
	}
	return false
}

// PreflightSubject identifies one (repo, file) pair that pre-flight
// must verify is clean.
type PreflightSubject struct {
	RepoRoot string // absolute path to the repo
	RelPath  string // path relative to RepoRoot
}

// CheckPreflight runs IsPathClean against every subject and returns the
// list of dirty (repo, path) subjects, in input order. The error result
// is non-nil only if a git invocation itself fails; a dirty path is
// signaled via the returned slice, not via the error.
func CheckPreflight(subjects []PreflightSubject) ([]PreflightSubject, error) {
	var dirty []PreflightSubject
	for _, s := range subjects {
		clean, err := IsPathClean(s.RepoRoot, s.RelPath)
		if err != nil {
			return nil, err
		}
		if !clean {
			dirty = append(dirty, s)
		}
	}
	return dirty, nil
}

// PreflightSubjectsForRelocate composes the full list of preflight
// subjects for one relocate invocation:
//
//   - Source repo: the artifact file + spec/ideas/README.md.
//   - Target repo: the destination path + spec/ideas/README.md.
//   - Each sibling SpecScore-managed repo (other than source and target)
//     that contains a reference to the slug: every matched file path.
//
// Caller supplies the resolved source and target. siblings is the list
// of additional sibling repos discovered via DiscoverSiblings; the
// caller is responsible for excluding source and target from siblings
// before passing in.
func PreflightSubjectsForRelocate(
	sourceRepoRoot string,
	sourceRelPath string,
	targetRepoRoot string,
	targetRelPath string,
	siblings []TargetRepo,
	slug string,
) ([]PreflightSubject, error) {
	subjects := []PreflightSubject{
		{RepoRoot: sourceRepoRoot, RelPath: sourceRelPath},
		{RepoRoot: sourceRepoRoot, RelPath: "spec/ideas/README.md"},
		{RepoRoot: targetRepoRoot, RelPath: targetRelPath},
		{RepoRoot: targetRepoRoot, RelPath: "spec/ideas/README.md"},
	}
	for _, sib := range siblings {
		refs, err := FindReferences(sib.Path, slug)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			subjects = append(subjects, PreflightSubject{RepoRoot: sib.Path, RelPath: ref})
		}
	}
	return subjects, nil
}

// DirtyTreeError formats a stderr-suitable error message naming every
// dirty subject, then returns an *exitcode.Error with code 7.
func DirtyTreeError(dirty []PreflightSubject) error {
	if len(dirty) == 0 {
		return nil
	}
	var sb strings.Builder
	sb.WriteString("pre-flight: uncommitted changes in paths that would be modified — relocate aborted:\n")
	for _, s := range dirty {
		fmt.Fprintf(&sb, "  %s: %s\n", s.RepoRoot, s.RelPath)
	}
	sb.WriteString("Commit or stash the changes above and re-run.")
	return exitcode.Newf(exitcode.DirtyTree, "%s", sb.String())
}

// DiscoverSiblings returns the same data as discoverSiblings (the
// unexported helper used by ResolveTargetRepo). Exported here so the
// CLI verb can fetch the list once and pass it into both resolution
// and pre-flight, without scanning the workspace twice.
func DiscoverSiblings(specRoot string) ([]TargetRepo, error) {
	return discoverSiblings(specRoot)
}

// sortStrings is a tiny dependency-free in-place sort used to keep the
// FindReferences result deterministic without pulling in sort/slices.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
