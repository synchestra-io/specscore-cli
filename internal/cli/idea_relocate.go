package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
	"github.com/synchestra-io/specscore-cli/pkg/idea"
	"github.com/synchestra-io/specscore-cli/pkg/idearelocate"
)

// ideaRelocateCommand returns the "idea relocate" subcommand.
// See spec/features/cli/idea/relocate/README.md.
//
// Task 1 of the implementation plan ships the input-validation scaffold
// only: slug auto-resolution (Idea-first, seed-fallback; both → exit 5),
// target-repo resolution (slug-or-path; missing yaml → exit 6; multi-
// match → exit 2). The actual relocation mechanics (file copy, in-file
// rewrite, cross-repo link cleanup, commits) ship in later tasks.
// Until then the happy path prints a one-line resolution summary and
// exits 0; the line is informational scaffolding and will be replaced.
func ideaRelocateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relocate <slug> --to-repo=<target>",
		Short: "Relocate an Idea or sidekick-seed artifact to another SpecScore-managed repo",
		Long: `Relocates spec/ideas/<slug>.md (or spec/ideas/seeds/<slug>.md)
from the current project to a different SpecScore-managed repo.

The current implementation ships the input-validation scaffold only;
mutation (file copy, in-file rewrite, cross-repo link cleanup, commits)
lands in later tasks. For now, the verb exits 0 after resolving the
source artifact and the target repo, with a one-line summary on stdout.

Examples:

  specscore idea relocate foo --to-repo=specscore
  specscore idea relocate foo --to-repo=../specscore
`,
		Args: cobra.ExactArgs(1),
		RunE: runIdeaRelocate,
	}
	cmd.Flags().String("to-repo", "", "target repo: a sibling repo slug (resolved via project.repo "+
		"in each sibling's specscore.yaml) or a relative/absolute path "+
		"(contains '/'). Required.")
	_ = cmd.MarkFlagRequired("to-repo")
	cmd.Flags().Bool("no-commit", false, "stage changes in each affected repo without committing "+
		"(not yet active; reserved for Task 5)")
	cmd.Flags().String("project", "", "source project root (autodetected from cwd if omitted)")
	return cmd
}

func runIdeaRelocate(cmd *cobra.Command, args []string) error {
	slug := args[0]
	if err := idea.ValidateSlug(slug); err != nil {
		return exitcode.InvalidArgsErrorf("invalid slug %q: %v", slug, err)
	}

	toRepo, _ := cmd.Flags().GetString("to-repo")

	projectFlag, _ := cmd.Flags().GetString("project")
	specRoot, err := resolveSpecRoot(projectFlag)
	if err != nil {
		return err
	}

	source, err := idearelocate.ResolveSourceArtifact(specRoot, slug)
	if err != nil {
		return err
	}

	target, err := idearelocate.ResolveTargetRepo(specRoot, toRepo)
	if err != nil {
		return err
	}

	// Task 2: pre-flight clean-tree checks. Abort with exit 7 (DirtyTree)
	// if any path that would be modified has uncommitted changes.
	if err := runPreflight(specRoot, source, target, slug); err != nil {
		return err
	}

	// Task 1 scaffold output. Later tasks replace this with the
	// per-affected-repo lines + summary line specified by
	// cli/idea/relocate#req:stdout-format.
	_, _ = fmt.Fprintf(cmd.OutOrStdout(),
		"resolved: %s (%s) → %s\n",
		source.Path, source.Kind, target.Path)
	return nil
}

// runPreflight assembles the full preflight subject list (source
// artifact, source index, target destination, target index, and every
// referencing file in every sibling repo) and returns a DirtyTreeError
// when any subject has uncommitted changes. Source and target are
// excluded from the sibling scan to avoid double-checking.
func runPreflight(
	specRoot string,
	source idearelocate.SourceArtifact,
	target idearelocate.TargetRepo,
	slug string,
) error {
	allSiblings, err := idearelocate.DiscoverSiblings(specRoot)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering sibling repos: %v", err)
	}
	siblings := excludeRepoPaths(allSiblings, specRoot, target.Path)

	sourceRel, err := filepath.Rel(specRoot, source.Path)
	if err != nil {
		return exitcode.UnexpectedErrorf("computing source relative path: %v", err)
	}
	// Destination preserves the source's relative path within the new
	// repo, so target's preflight path equals the source's relative path.
	targetRel := sourceRel

	subjects, err := idearelocate.PreflightSubjectsForRelocate(
		specRoot, sourceRel,
		target.Path, targetRel,
		siblings, slug,
	)
	if err != nil {
		return exitcode.UnexpectedErrorf("collecting preflight subjects: %v", err)
	}

	dirty, err := idearelocate.CheckPreflight(subjects)
	if err != nil {
		return exitcode.UnexpectedErrorf("preflight check: %v", err)
	}
	return idearelocate.DirtyTreeError(dirty)
}

// excludeRepoPaths returns the subset of siblings whose Path is not
// path-equivalent (after EvalSymlinks) to any of the supplied
// excludePaths. Used to drop the source and target repos from the
// sibling list before scanning for cross-repo references.
func excludeRepoPaths(siblings []idearelocate.TargetRepo, excludePaths ...string) []idearelocate.TargetRepo {
	canon := func(p string) string {
		if abs, err := filepath.Abs(p); err == nil {
			if r, err := filepath.EvalSymlinks(abs); err == nil {
				return filepath.Clean(r)
			}
			return filepath.Clean(abs)
		}
		return filepath.Clean(p)
	}
	excl := make(map[string]struct{}, len(excludePaths))
	for _, p := range excludePaths {
		excl[canon(p)] = struct{}{}
	}
	out := make([]idearelocate.TargetRepo, 0, len(siblings))
	for _, s := range siblings {
		if _, skip := excl[canon(s.Path)]; skip {
			continue
		}
		out = append(out, s)
	}
	return out
}
