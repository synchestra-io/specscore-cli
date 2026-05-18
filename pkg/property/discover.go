package property

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Discovered is a summary of a Property file found during discovery.
type Discovered struct {
	Slug string
	Path string // path to the *.property.md file
}

// Discover walks `<specRoot>/features/**/*.property.md` and returns every
// property file found, sorted alphabetically by slug.
//
// Discovery scope mirrors [cli/property#req:discovery-scope]:
//   - Hidden directories (any path segment starting with `.`) MUST be skipped.
//   - Reserved underscore-prefixed directories (e.g., `_tests/`) MUST be
//     skipped.
//   - A missing `<specRoot>/features/` directory yields ([], nil) — Discover
//     is not responsible for surfacing project-shape errors.
//
// `specRoot` is the absolute path to the project's `spec/` directory (the
// caller resolves this from `specscore.yaml`); the function appends
// `features/` internally to match the meta-spec's property-location glob.
func Discover(specRoot string) ([]Discovered, error) {
	featuresDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featuresDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	var out []Discovered
	walkErr := filepath.Walk(featuresDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != featuresDir && shouldSkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if !strings.HasSuffix(name, ".property.md") {
			return nil
		}
		slug := strings.TrimSuffix(name, ".property.md")
		out = append(out, Discovered{Slug: slug, Path: path})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

// shouldSkipDir reports whether a directory MUST be excluded from the walk
// per [cli/property#req:discovery-scope] — hidden (`.<anything>`) or
// reserved underscore-prefixed (`_<anything>`).
func shouldSkipDir(name string) bool {
	if name == "" {
		return false
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}
