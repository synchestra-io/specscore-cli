package lint

import (
	"os"
	"path/filepath"
	"strings"
)

// walkFeatureReadmes invokes fn for every feature README under specRoot,
// skipping reserved _-prefixed subtrees. Shared between rules that need
// to iterate over every spec/features/.../README.md (e.g. studio-toolbar).
func walkFeatureReadmes(specRoot string, fn func(readmePath string, content []byte)) error {
	featuresDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featuresDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	return filepath.Walk(featuresDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if path != featuresDir && strings.HasPrefix(info.Name(), "_") {
			return filepath.SkipDir
		}
		readmePath := filepath.Join(path, "README.md")
		readmeInfo, statErr := os.Stat(readmePath)
		if statErr != nil || readmeInfo.IsDir() {
			return nil
		}
		content, readErr := osReadFileFeatureReadme(readmePath)
		if readErr != nil {
			return nil
		}
		fn(readmePath, content)
		return nil
	})
}
