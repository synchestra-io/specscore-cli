// Package idearelocate implements cross-repo relocation of Idea and
// sidekick-seed artifacts per spec/features/cli/idea/relocate/README.md.
// This file houses the slug-and-target resolution primitives used by
// the verb's input-validation phase (Task 1 of the implementation
// plan). Mutation logic — file copy, in-file rewrite, cross-repo link
// cleanup, commits — lives in later tasks.
package idearelocate

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
	"github.com/synchestra-io/specscore-cli/pkg/projectdef"
)

// ArtifactKind distinguishes an Idea document from a sidekick-seed.
type ArtifactKind string

const (
	// KindIdea marks artifacts at spec/ideas/<slug>.md.
	KindIdea ArtifactKind = "idea"
	// KindSeed marks artifacts at spec/ideas/seeds/<slug>.md.
	KindSeed ArtifactKind = "seed"
)

// SourceArtifact is the resolved on-disk location of a relocate-target
// artifact within the source project.
type SourceArtifact struct {
	Path string       // absolute path to the artifact file
	Kind ArtifactKind // idea or seed
}

// ResolveSourceArtifact resolves <slug> per
// cli/idea/relocate#req:slug-resolves-idea-or-seed: Idea path is checked
// first at spec/ideas/<slug>.md; on miss, the seed path
// spec/ideas/seeds/<slug>.md is checked. Exactly one match returns that
// path + kind. Both existing returns an *exitcode.Error with code 5
// (AmbiguousSlug). Neither existing returns code 3 (NotFound).
func ResolveSourceArtifact(specRoot, slug string) (SourceArtifact, error) {
	ideaPath := filepath.Join(specRoot, "spec", "ideas", slug+".md")
	seedPath := filepath.Join(specRoot, "spec", "ideas", "seeds", slug+".md")

	ideaExists := fileExists(ideaPath)
	seedExists := fileExists(seedPath)

	switch {
	case ideaExists && seedExists:
		return SourceArtifact{}, exitcode.Newf(exitcode.AmbiguousSlug,
			"slug %q exists as both an Idea (%s) and a seed (%s); disambiguate by renaming one before relocating",
			slug, ideaPath, seedPath)
	case ideaExists:
		return SourceArtifact{Path: ideaPath, Kind: KindIdea}, nil
	case seedExists:
		return SourceArtifact{Path: seedPath, Kind: KindSeed}, nil
	default:
		return SourceArtifact{}, exitcode.NotFoundErrorf(
			"no artifact found for slug %q (checked %s and %s)",
			slug, ideaPath, seedPath)
	}
}

// TargetRepo is the resolved destination repository.
type TargetRepo struct {
	Path     string // absolute path to the repo root (the dir containing specscore.yaml)
	RepoName string // value of project.repo from the target's specscore.yaml (may be empty)
	Org      string // value of project.org from the target's specscore.yaml (may be empty)
}

// ResolveTargetRepo resolves the --to-repo flag value per
// cli/idea/relocate#req:target-repo-resolution.
//
//   - Value containing no "/" is a repo slug: scan sibling directories
//     of specRoot's parent for specscore.yaml files and match
//     project.repo. Single match returns it. Zero matches → exit 3
//     (NotFound). Multiple matches → exit 2 (InvalidArgs).
//   - Value containing "/" is a path: relative to specRoot, or absolute
//     if starting with "/". The path MUST be a directory containing a
//     specscore.yaml; missing yaml → exit 6 (TargetNotSpecScore);
//     missing directory → exit 3 (NotFound).
func ResolveTargetRepo(specRoot, toRepo string) (TargetRepo, error) {
	if toRepo == "" {
		return TargetRepo{}, exitcode.InvalidArgsErrorf("--to-repo is required")
	}
	if strings.ContainsRune(toRepo, '/') {
		return resolveTargetByPath(specRoot, toRepo)
	}
	return resolveTargetBySlug(specRoot, toRepo)
}

// resolveTargetByPath resolves --to-repo when the value contains "/".
func resolveTargetByPath(specRoot, toRepo string) (TargetRepo, error) {
	var abs string
	if filepath.IsAbs(toRepo) {
		abs = filepath.Clean(toRepo)
	} else {
		abs = filepath.Clean(filepath.Join(specRoot, toRepo))
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return TargetRepo{}, exitcode.NotFoundErrorf(
				"target path does not exist: %s", abs)
		}
		return TargetRepo{}, exitcode.UnexpectedErrorf(
			"stat target path %s: %v", abs, err)
	}
	if !info.IsDir() {
		return TargetRepo{}, exitcode.InvalidArgsErrorf(
			"target path is not a directory: %s", abs)
	}

	yamlPath := filepath.Join(abs, "specscore.yaml")
	if !fileExists(yamlPath) {
		return TargetRepo{}, exitcode.Newf(exitcode.TargetNotSpecScore,
			"target directory %s contains no specscore.yaml; not a SpecScore-managed repo", abs)
	}

	cfg, err := projectdef.ReadSpecConfig(abs)
	if err != nil {
		return TargetRepo{}, exitcode.UnexpectedErrorf(
			"reading %s: %v", yamlPath, err)
	}
	return TargetRepo{Path: abs, RepoName: cfg.Project.Repo, Org: cfg.Project.Org}, nil
}

// resolveTargetBySlug resolves --to-repo when the value contains no "/".
// It scans sibling directories of specRoot's parent (plus specRoot
// itself) for specscore.yaml files and matches project.repo against
// the supplied slug.
func resolveTargetBySlug(specRoot, slug string) (TargetRepo, error) {
	siblings, err := discoverSiblings(specRoot)
	if err != nil {
		return TargetRepo{}, err
	}
	var matches []TargetRepo
	for _, s := range siblings {
		if s.RepoName == slug {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return TargetRepo{}, exitcode.NotFoundErrorf(
			"no sibling SpecScore repo found with project.repo=%q", slug)
	case 1:
		return matches[0], nil
	default:
		paths := make([]string, len(matches))
		for i, m := range matches {
			paths[i] = m.Path
		}
		return TargetRepo{}, exitcode.InvalidArgsErrorf(
			"multiple sibling SpecScore repos declare project.repo=%q (%s); use --to-repo with an explicit path to disambiguate",
			slug, strings.Join(paths, ", "))
	}
}

// discoverSiblings returns every SpecScore-managed repo discoverable
// from specRoot's parent directory: each immediate child dir whose
// `specscore.yaml` parses, plus specRoot itself. Hidden dirs (name
// starts with ".") and symlinks-out-of-parent are skipped.
func discoverSiblings(specRoot string) ([]TargetRepo, error) {
	absSource, err := filepath.Abs(specRoot)
	if err != nil {
		return nil, exitcode.UnexpectedErrorf("resolving spec root: %v", err)
	}
	parent := filepath.Dir(absSource)
	if parent == absSource {
		// specRoot is the filesystem root; no siblings possible.
		return readSelfAsSibling(absSource)
	}

	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, exitcode.UnexpectedErrorf("reading workspace parent %s: %v", parent, err)
	}

	var out []TargetRepo
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := filepath.Join(parent, name)

		// Symlink-out guard. We use Lstat to detect whether the entry
		// itself is a symlink. For regular directories Lstat returns a
		// normal mode and we proceed. For symlinks we resolve and
		// require the target to remain a descendant of parent.
		linfo, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		if linfo.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(parent, resolved)
			if err != nil || strings.HasPrefix(rel, "..") {
				continue
			}
		}

		info, err := os.Stat(fullPath)
		if err != nil || !info.IsDir() {
			continue
		}

		yamlPath := filepath.Join(fullPath, "specscore.yaml")
		if !fileExists(yamlPath) {
			continue
		}
		cfg, err := projectdef.ReadSpecConfig(fullPath)
		if err != nil {
			continue // malformed siblings are ignored; lint surfaces them elsewhere
		}
		out = append(out, TargetRepo{Path: fullPath, RepoName: cfg.Project.Repo, Org: cfg.Project.Org})
	}
	return out, nil
}

// readSelfAsSibling is the degenerate-case helper used when specRoot is
// the filesystem root: only the source itself is a candidate.
func readSelfAsSibling(specRoot string) ([]TargetRepo, error) {
	yamlPath := filepath.Join(specRoot, "specscore.yaml")
	if !fileExists(yamlPath) {
		return nil, nil
	}
	cfg, err := projectdef.ReadSpecConfig(specRoot)
	if err != nil {
		return nil, nil
	}
	return []TargetRepo{{Path: specRoot, RepoName: cfg.Project.Repo, Org: cfg.Project.Org}}, nil
}

// fileExists reports whether the named file exists and is a regular
// file or a symlink resolving to a regular file. Directories return
// false.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
