package issue

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// fpRel is filepath.Rel; tests can replace it to simulate Rel failures.
var fpRel = filepath.Rel

// filepathWalkFn is a testable indirection for filepath.Walk.
var filepathWalkFn = filepath.Walk

// Discovered summarizes one file that declares `type: issue` in its
// YAML frontmatter, regardless of whether its path matches PathPatterns.
type Discovered struct {
	// Path is the absolute path to the file.
	Path string
	// RelPath is the path relative to the spec root, with forward slashes.
	RelPath string
	// Slug is the basename minus `.md`.
	Slug string
	// MatchesPattern is true when RelPath matches one of the two
	// canonical PathPatterns. Files declaring `type: issue` outside
	// the patterns have MatchesPattern == false (I-009 candidates).
	MatchesPattern bool
	// FeatureSlug, when non-empty, is the parent Feature slug for a
	// Feature-scoped issue (RelPath of the form
	// `features/<feature-slug>/issues/<slug>.md`). Empty for root-level
	// issues or off-pattern issues.
	FeatureSlug string
}

// DiscoverAll walks the entire spec tree (skipping hidden dirs) and
// returns every markdown file whose YAML frontmatter declares
// `type: issue`. The walk is deliberately broad — restricting to the
// canonical PathPatterns would hide exactly the files I-009 needs to
// flag.
//
// Returns ([], nil) when specRoot does not exist or is not a directory.
func DiscoverAll(specRoot string) ([]Discovered, error) {
	info, err := os.Stat(specRoot)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	var out []Discovered
	walkErr := filepathWalkFn(specRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			// Skip hidden directories (matching the convention used
			// elsewhere in the lint engine).
			if info.Name() != "." && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".md") {
			return nil
		}
		if name == "README.md" {
			return nil
		}
		iss, perr := Parse(path)
		if perr != nil {
			return nil // unreadable files are ignored at discovery
		}
		if iss.Type != TypeValue {
			return nil
		}
		rel, relErr := fpRel(specRoot, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		match, featureSlug := classifyPath(relSlash)
		out = append(out, Discovered{
			Path:           path,
			RelPath:        relSlash,
			Slug:           strings.TrimSuffix(name, ".md"),
			MatchesPattern: match,
			FeatureSlug:    featureSlug,
		})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelPath < out[j].RelPath })
	return out, nil
}

// classifyPath returns (matchesPattern, featureSlug) for a spec-root-
// relative forward-slash path. featureSlug is non-empty only when the
// path matches the Feature-scoped pattern.
func classifyPath(relSlash string) (bool, string) {
	parts := strings.Split(relSlash, "/")
	// Pattern 1: issues/<slug>.md (exactly 2 segments).
	if len(parts) == 2 && parts[0] == "issues" && strings.HasSuffix(parts[1], ".md") {
		return true, ""
	}
	// Pattern 2: features/<feature-slug>/issues/<slug>.md
	// (exactly 4 segments; the feature-slug here is a single segment —
	// nested Feature slugs are out of scope for this MVP per the
	// PathPatterns spec wording `spec/features/*/issues/*.md`).
	if len(parts) == 4 && parts[0] == "features" && parts[2] == "issues" && strings.HasSuffix(parts[3], ".md") {
		return true, parts[1]
	}
	return false, ""
}
