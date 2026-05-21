package idearelocate

import (
	"os"
	"path/filepath"

	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
)

// MutationResult records the on-disk effects of ApplyMutation. Returned
// to the CLI verb for use in the per-affected-repo stdout summary.
type MutationResult struct {
	// DestinationPath is the absolute path the artifact was written to
	// inside the target repo.
	DestinationPath string
}

// ApplyMutation performs Task 3 of cli/idea/relocate:
//
//   - Computes the destination path by mirroring source's repo-relative
//     path inside target.Path.
//   - Rejects a pre-existing destination file with exit 1 (Conflict).
//     No mutations are performed in this case.
//   - Reads the source artifact, applies RewriteBody (org rename +
//     "this repo" → target.RepoName), writes the result to the
//     destination (creating parent dirs as needed).
//   - Deletes the source artifact.
//
// Task 6's pre-commit rollback wrapper is responsible for restoring
// state on any mid-sequence I/O failure (file copy or in-file rewrite
// failure). ApplyMutation itself does NOT roll back — it surfaces the
// underlying error so the wrapper can decide. The exception is the
// destination-collision case, which is treated as a pre-check and
// guarantees zero mutations by design.
func ApplyMutation(sourceRepoRoot string, source SourceArtifact, target TargetRepo) (MutationResult, error) {
	sourceRel, err := filepath.Rel(sourceRepoRoot, source.Path)
	if err != nil {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"computing source relative path: %v", err)
	}
	destPath := filepath.Join(target.Path, sourceRel)

	if info, err := os.Stat(destPath); err == nil {
		_ = info
		return MutationResult{}, exitcode.ConflictErrorf(
			"destination already exists in target repo: %s — rename the source artifact or the existing target file before relocating",
			destPath)
	} else if !os.IsNotExist(err) {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"stat destination %s: %v", destPath, err)
	}

	body, err := os.ReadFile(source.Path)
	if err != nil {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"reading source artifact %s: %v", source.Path, err)
	}
	rewritten := RewriteBody(string(body), target.RepoName)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"creating destination dir %s: %v", filepath.Dir(destPath), err)
	}
	if err := os.WriteFile(destPath, []byte(rewritten), 0o644); err != nil {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"writing destination %s: %v", destPath, err)
	}
	if err := os.Remove(source.Path); err != nil {
		return MutationResult{}, exitcode.UnexpectedErrorf(
			"deleting source artifact %s: %v", source.Path, err)
	}

	return MutationResult{DestinationPath: destPath}, nil
}
