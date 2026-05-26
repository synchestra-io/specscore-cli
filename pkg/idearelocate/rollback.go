package idearelocate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// rollback.go implements Task 6 of cli/idea/relocate: pre-commit-phase
// rollback. The Task-3 (ApplyMutation) and Task-4 (UpdateCrossRepoLinks)
// phases mutate on-disk state. If anything fails BEFORE Task 5's commit
// phase begins, we must restore the on-disk state to its pre-invocation
// form so the user is not left holding a partial relocation.
//
// Sequence of rollback actions, applied in reverse order of the
// forward-pass mutations:
//
//   1. Revert in-flight link updates via `git -C <repo> checkout --
//      <path>` for every link-update path recorded in any repo.
//   2. Delete the partial destination copy at target if it was created.
//   3. Restore the source artifact at its original path from an in-memory
//      snapshot taken before mutation began.
//
// Failure handling order matters: link updates are reverted first
// (cheapest, idempotent), then the dest is removed (single file), then
// the source is restored (preserves the user's content). Each step is
// best-effort — rollback failures append to the actions list but do
// NOT abort.
//
// The destination-collision case (exit 1) is NOT a rollback trigger:
// ApplyMutation guarantees no mutations occurred when it returns the
// Conflict error, so there is nothing to restore.

// ExecutePreCommitPhase runs Task 3 (file copy + in-file rewrite +
// source delete) followed by Task 4 (cross-repo link cleanup), tracking
// rollback state. On any failure during these phases (other than
// destination collision), it restores on-disk state to its pre-
// invocation form and returns an *exitcode.Error with code 10 naming
// the failed step and the rollback actions performed.
//
// Returns the MutationResult and per-repo link-update results when the
// whole pre-commit phase succeeds. Callers then proceed to Task 5's
// commit phase using those results.
func ExecutePreCommitPhase(
	sourceRepoRoot string,
	source SourceArtifact,
	target TargetRepo,
	scanRepos []TargetRepo,
	slug string,
) (MutationResult, []LinkUpdateResult, error) {
	// Pre-snapshot: read source body and pre-compute destPath so we can
	// stat it during rollback. A read failure here is itself an I/O
	// error worth surfacing — but ApplyMutation will also try to read,
	// so we let it report.
	sourceBody, _ := os.ReadFile(source.Path)
	sourceRel, _ := filepath.Rel(sourceRepoRoot, source.Path)
	expectedDestPath := filepath.Join(target.Path, sourceRel)

	var actions []string

	// Step A: file copy + in-file rewrite + source delete.
	mutation, mutErr := ApplyMutation(sourceRepoRoot, source, target)
	if mutErr != nil {
		// Collision (exit 1) means zero mutations — return as-is.
		var ec *exitcode.Error
		if errors.As(mutErr, &ec) && ec.ExitCode() == exitcode.Conflict {
			return mutation, nil, mutErr
		}
		// Any other failure: clean up whatever partial state exists.
		actions = appendIfPartialDest(actions, expectedDestPath)
		actions = appendIfSourceMissing(actions, source.Path, sourceBody)
		return mutation, nil, ioRollbackError("file copy / source delete", mutErr, actions)
	}

	// Step B: cross-repo link cleanup. UpdateCrossRepoLinks may have
	// returned partial results before an error; we revert whatever it
	// reports as Updated.
	targetRel, _ := filepath.Rel(target.Path, mutation.DestinationPath)
	linkResults, linkErr := updateCrossRepoLinksFn(scanRepos, target, slug, targetRel)
	if linkErr != nil {
		actions = appendCheckoutsForResults(actions, linkResults)
		actions = appendIfPartialDest(actions, mutation.DestinationPath)
		actions = appendIfSourceMissing(actions, source.Path, sourceBody)
		return mutation, nil, ioRollbackError("cross-repo link update", linkErr, actions)
	}
	return mutation, linkResults, nil
}

// appendCheckoutsForResults runs `git checkout -- <path>` for every
// recorded link-update path. Per-path failures append a best-effort
// notice rather than aborting.
func appendCheckoutsForResults(actions []string, results []LinkUpdateResult) []string {
	for _, r := range results {
		for _, p := range r.Updated {
			if !isGitRepoFn(r.RepoPath) {
				actions = append(actions,
					fmt.Sprintf("could not revert %s in %s (not a git repo)", p, r.RepoPath))
				continue
			}
			cmd := exec.Command("git", "-C", r.RepoPath, "checkout", "--", p)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				actions = append(actions,
					fmt.Sprintf("git checkout -- %s in %s FAILED: %v: %s",
						p, r.RepoPath, err, strings.TrimSpace(stderr.String())))
				continue
			}
			actions = append(actions, fmt.Sprintf("reverted %s in %s", p, r.RepoPath))
		}
	}
	return actions
}

// appendIfPartialDest checks whether destPath exists and, if so,
// removes it and records the action.
func appendIfPartialDest(actions []string, destPath string) []string {
	if destPath == "" {
		return actions
	}
	info, err := os.Stat(destPath)
	if err != nil || info.IsDir() {
		return actions
	}
	if err := os.Remove(destPath); err != nil {
		actions = append(actions, fmt.Sprintf("removing partial destination %s FAILED: %v", destPath, err))
		return actions
	}
	actions = append(actions, fmt.Sprintf("removed partial destination %s", destPath))
	return actions
}

// appendIfSourceMissing checks whether the source artifact still
// exists; if not (i.e., ApplyMutation deleted it before failing later
// in the flow), restore it from the in-memory snapshot.
func appendIfSourceMissing(actions []string, sourcePath string, sourceBody []byte) []string {
	if sourcePath == "" || sourceBody == nil {
		return actions
	}
	if _, err := os.Stat(sourcePath); err == nil {
		// Source still in place — nothing to restore.
		return actions
	}
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		actions = append(actions, fmt.Sprintf("recreating dir for %s FAILED: %v", sourcePath, err))
		return actions
	}
	if err := os.WriteFile(sourcePath, sourceBody, 0o644); err != nil {
		actions = append(actions, fmt.Sprintf("restoring source artifact %s FAILED: %v", sourcePath, err))
		return actions
	}
	actions = append(actions, fmt.Sprintf("restored source artifact %s", sourcePath))
	return actions
}

// ioRollbackError formats a stderr-suitable message for a Task-6
// rollback event and wraps it in an *exitcode.Error with code 10.
func ioRollbackError(step string, cause error, actions []string) *exitcode.Error {
	var sb strings.Builder
	fmt.Fprintf(&sb, "pre-commit-phase I/O failure during %s: %v\n", step, cause)
	if len(actions) == 0 {
		sb.WriteString("Rollback actions: none — no partial state detected.")
	} else {
		sb.WriteString("Rollback actions performed:\n")
		for _, a := range actions {
			fmt.Fprintf(&sb, "  - %s\n", a)
		}
	}
	return exitcode.Newf(exitcode.Unexpected, "%s", sb.String())
}
