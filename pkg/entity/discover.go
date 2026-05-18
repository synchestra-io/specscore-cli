package entity

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// EntitySuffix is the canonical filename suffix for entity files.
const EntitySuffix = ".entity.md"

// Discovered is a summary of an entity file found during a Discover walk.
type Discovered struct {
	Slug string // filename stem without ".entity.md"
	Path string // absolute path to the .entity.md file
}

// Discover walks `<specRoot>/features/**/*.entity.md` and returns every
// discovered entity. Hidden directories (path segment starting with ".")
// and reserved underscore-prefixed directories (e.g., "_tests") are
// skipped — same convention used by walkMatchingFiles in
// pkg/lint/adherence_footer.go and required by
// [cli/entity#req:discovery-scope].
//
// Returns ([], nil) when `<specRoot>/features` does not exist. Returns
// absolute paths and is deterministic across invocations (results are
// sorted by path).
func Discover(specRoot string) ([]Discovered, error) {
	featuresDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featuresDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	var out []Discovered
	err = filepath.Walk(featuresDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			if path == featuresDir {
				return nil
			}
			name := info.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), EntitySuffix) {
			return nil
		}
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			abs = path
		}
		out = append(out, Discovered{
			Slug: strings.TrimSuffix(info.Name(), EntitySuffix),
			Path: abs,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
