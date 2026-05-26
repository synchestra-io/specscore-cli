package idearelocate

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// commit.go implements Task 5 of cli/idea/relocate: the post-mutation
// commit phase. Three behaviors per spec:
//
//   - Default (auto-commit): for each affected repo in canonical order
//     (source, target, then siblings alphabetically by RepoName),
//     stage paths and `git commit -m <subject>` with one of three
//     canonical subjects. Capture each commit SHA for stdout.
//   - --no-commit: stage paths in every affected repo, no commits.
//   - Stop-on-first-failure: if any commit fails mid-flight, exit 10
//     with a stderr report naming committed repos+SHAs, failing repo +
//     reason, unprocessed repos, and explicit rollback commands. No
//     automatic rollback.

// CommitMode toggles the post-mutation commit behavior.
type CommitMode int

const (
	// CommitAuto stages and commits every affected repo.
	CommitAuto CommitMode = iota
	// CommitNo stages only — no commits.
	CommitNo
)

// RepoAction labels a repo's role in the relocate. Used both in stdout
// per-repo lines and to derive commit-subject formatting.
type RepoAction string

const (
	// ActionMoved is the source repo's role: the artifact was deleted
	// from it.
	ActionMoved RepoAction = "moved"
	// ActionReceived is the target repo's role: the artifact landed in
	// it.
	ActionReceived RepoAction = "received"
	// ActionUpdatedLinks is a sibling repo's role: only link rewrites.
	ActionUpdatedLinks RepoAction = "updated-links"
)

// RepoChange is one affected repo's slice of work — paths to stage and,
// in auto-commit mode, a subject line for `git commit`.
type RepoChange struct {
	Repo    TargetRepo // includes Path, RepoName, Org
	Action  RepoAction
	Kind    ArtifactKind // idea or seed
	Slug    string
	Paths   []string // repo-relative paths to `git add`
	Subject string   // commit subject (computed)
	SHA     string   // populated after a successful commit; empty in CommitNo
}

// AssembleRepoChanges builds the ordered slice of affected-repo work.
// Order:
//  1. Source (action=moved)
//  2. Target (action=received)
//  3. Siblings (action=updated-links), alphabetically by RepoName,
//     ONLY when their Updated[] is non-empty.
//
// linkUpdates maps each affected repo's path to its updated repo-rel
// paths. The source and target repos additionally carry the artifact-
// move path: sourceRelPath in source (a `git add` of a deleted file
// stages the deletion), and targetRelPath in target.
func AssembleRepoChanges(
	source TargetRepo, sourceKind ArtifactKind, sourceRelPath string,
	target TargetRepo, targetRelPath string,
	siblings []TargetRepo,
	linkUpdates map[string][]string,
	slug string,
) []RepoChange {
	sourcePaths := mergePaths([]string{sourceRelPath}, linkUpdates[source.Path])
	targetPaths := mergePaths([]string{targetRelPath}, linkUpdates[target.Path])

	changes := []RepoChange{
		{
			Repo:    source,
			Action:  ActionMoved,
			Kind:    sourceKind,
			Slug:    slug,
			Paths:   sourcePaths,
			Subject: fmt.Sprintf("chore(relocate): move %s %s to %s", sourceKind, slug, target.RepoName),
		},
		{
			Repo:    target,
			Action:  ActionReceived,
			Kind:    sourceKind,
			Slug:    slug,
			Paths:   targetPaths,
			Subject: fmt.Sprintf("chore(relocate): receive %s %s from %s", sourceKind, slug, source.RepoName),
		},
	}

	// Siblings, alphabetical by RepoName.
	sorted := append([]TargetRepo(nil), siblings...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].RepoName < sorted[j].RepoName })
	for _, sib := range sorted {
		paths := linkUpdates[sib.Path]
		if len(paths) == 0 {
			// No link rewrites in this sibling → not an affected repo.
			continue
		}
		changes = append(changes, RepoChange{
			Repo:    sib,
			Action:  ActionUpdatedLinks,
			Kind:    sourceKind,
			Slug:    slug,
			Paths:   paths,
			Subject: fmt.Sprintf("chore(relocate): update links for %s (%s → %s)", slug, source.RepoName, target.RepoName),
		})
	}
	return changes
}

// ExecuteCommitPhase stages each change's paths and, in CommitAuto
// mode, commits them. On a commit failure mid-flight it returns a
// CommitFailure with the committed-so-far state, the failing change,
// and the unprocessed remainder.
//
// Non-git repos are vacuously processed: no staging, no commit, no
// SHA — the verb tolerates non-git project roots in the same way
// preflight does, so a mid-experiment workspace not yet under VCS
// can still relocate artifacts on disk.
func ExecuteCommitPhase(changes []RepoChange, mode CommitMode) ([]RepoChange, *CommitFailure, error) {
	executed := make([]RepoChange, 0, len(changes))
	for i, ch := range changes {
		if !isGitRepoFn(ch.Repo.Path) {
			executed = append(executed, ch)
			continue
		}
		if err := stagePaths(ch.Repo.Path, ch.Paths); err != nil {
			// Staging failures bubble up as I/O errors (exit 10) with
			// no partial-commit cleanup needed (nothing committed yet
			// for this change).
			return executed, nil, exitcode.UnexpectedErrorf(
				"staging paths in %s: %v", ch.Repo.Path, err)
		}
		if mode == CommitNo {
			executed = append(executed, ch)
			continue
		}
		sha, stderr, err := commitRepo(ch.Repo.Path, ch.Subject)
		if err != nil {
			fail := &CommitFailure{
				Failed:       ch,
				FailedStderr: stderr,
				Committed:    executed,
				Unprocessed:  append([]RepoChange(nil), changes[i+1:]...),
			}
			return executed, fail, nil
		}
		ch.SHA = sha
		executed = append(executed, ch)
	}
	return executed, nil, nil
}

// CommitFailure carries the post-mortem details for a stop-on-first-
// failure event: which change failed, the captured stderr from
// git-commit, the changes already committed (with SHAs), and the
// changes that were never attempted.
type CommitFailure struct {
	Failed       RepoChange
	FailedStderr string
	Committed    []RepoChange
	Unprocessed  []RepoChange
}

// AsExitError formats the CommitFailure into a stderr-suitable message
// and wraps it in an *exitcode.Error with code 10 per
// cli/idea/relocate#req:stop-on-first-commit-failure.
func (f *CommitFailure) AsExitError() *exitcode.Error {
	var sb strings.Builder
	sb.WriteString("commit failed mid-flight; cross-repo rollback is the user's responsibility:\n\n")
	if len(f.Committed) > 0 {
		sb.WriteString("Already committed:\n")
		for _, c := range f.Committed {
			fmt.Fprintf(&sb, "  %s  %s  %s\n", c.Repo.RepoName, shortSHA(c.SHA), c.Repo.Path)
		}
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "Failed in %s (%s):\n", f.Failed.Repo.RepoName, f.Failed.Repo.Path)
	if f.FailedStderr != "" {
		for _, line := range strings.Split(strings.TrimRight(f.FailedStderr, "\n"), "\n") {
			fmt.Fprintf(&sb, "  | %s\n", line)
		}
	}
	if len(f.Unprocessed) > 0 {
		sb.WriteString("\nUnprocessed (mutations in-tree, not committed):\n")
		for _, u := range f.Unprocessed {
			fmt.Fprintf(&sb, "  %s  %s\n", u.Repo.RepoName, u.Repo.Path)
		}
	}
	sb.WriteString("\nRollback commands:\n")
	for _, c := range f.Committed {
		fmt.Fprintf(&sb, "  git -C %s reset HEAD~1 --hard\n", c.Repo.Path)
	}
	fmt.Fprintf(&sb, "  git -C %s reset HEAD && git -C %s checkout -- .\n",
		f.Failed.Repo.Path, f.Failed.Repo.Path)
	return exitcode.Newf(exitcode.Unexpected, "%s", sb.String())
}

// FormatStdout produces the canonical per-affected-repo lines and the
// summary line per cli/idea/relocate#req:stdout-format. When
// mode=CommitNo, the [<sha7>] bracket is omitted.
func FormatStdout(changes []RepoChange, mode CommitMode) string {
	var sb strings.Builder
	for _, c := range changes {
		line := fmt.Sprintf("%s: %s %s %s", c.Repo.RepoName, c.Action, c.Kind, c.Slug)
		if mode == CommitAuto && c.SHA != "" {
			line += fmt.Sprintf("  [%s]", shortSHA(c.SHA))
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "relocate complete: %d repos affected\n", len(changes))
	return sb.String()
}

// --- helpers ---

// mergePaths returns a deduplicated, order-preserving union of a and b.
// Duplicates use a's earlier position.
func mergePaths(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, p := range a {
		p = filepath.ToSlash(filepath.Clean(p))
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range b {
		p = filepath.ToSlash(filepath.Clean(p))
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// stagePaths runs `git -C <repo> add <paths...>` to stage modifications,
// new files, AND deletions. A repo with no .git directory is silently
// skipped (the verb tolerates non-git project roots).
func stagePaths(repoRoot string, paths []string) error {
	if !isGitRepoFn(repoRoot) || len(paths) == 0 {
		return nil
	}
	args := append([]string{"-C", repoRoot, "add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git add: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// defaultGitRevParseHEAD runs `git rev-parse HEAD` and returns the SHA.
func defaultGitRevParseHEAD(repoRoot string) (string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// commitRepo runs `git -C <repo> commit -m <subject>` and returns the
// new HEAD SHA on success, or the captured stderr + error on failure.
func commitRepo(repoRoot, subject string) (string, string, error) {
	cmd := exec.Command("git", "-C", repoRoot, "commit", "-m", subject)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", stderr.String(), err
	}
	sha, err := gitRevParseHEADFn(repoRoot)
	if err != nil {
		return "", "", err
	}
	return sha, "", nil
}

// shortSHA returns the 7-character abbreviation of a full SHA.
func shortSHA(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
