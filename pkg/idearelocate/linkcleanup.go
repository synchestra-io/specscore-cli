package idearelocate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LinkUpdateResult records which files were updated in a given repository.
type LinkUpdateResult struct {
	RepoPath string
	Updated  []string
}

// UpdateCrossRepoLinks updates all markdown link references to the relocated slug
// across all SpecScore repositories.
//
// In-repo links are rewritten to relative paths.
// Cross-repo links are rewritten to full GitHub URL form.
// Bold-prefixed metadata lines are preserved.
func UpdateCrossRepoLinks(allRepos []TargetRepo, targetRepo TargetRepo, slug string, targetRelPath string) ([]LinkUpdateResult, error) {
	targetFileAbs := filepath.Join(targetRepo.Path, targetRelPath)

	// Regex for markdown links whose URL is either bare "<slug>.md"
	// (no path component) or ends in "/<slug>.md". The optional
	// `(?:[^)]*?/)?` prefix handles arbitrary nesting depth. We avoid
	// the preflight regex's `(?:[/]|^)` alternation because `^` in Go
	// RE2 only anchors to the input's position 0, so a bare-URL link
	// not at the line start (e.g. mid-paragraph `[foo](foo.md)`) is
	// silently missed.
	linkRe := regexp.MustCompile(`\[([^\]]*)\]\(((?:[^)]*?/)?` + regexp.QuoteMeta(slug) + `\.md)\)`)

	metaPrefixes := []string{
		"**Source Ideas:**",
		"**Related Ideas:**",
		"**Supersedes:**",
		"**Promotes To:**",
	}

	var results []LinkUpdateResult

	for _, repo := range allRepos {
		specDir := filepath.Join(repo.Path, "spec")
		if _, err := os.Stat(specDir); err != nil {
			continue
		}

		var updatedFiles []string

		err := filepath.Walk(specDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}

			contentBytes, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			content := string(contentBytes)
			lines := strings.Split(content, "\n")
			modified := false

			for i, line := range lines {
				hasMetaPrefix := false
				for _, prefix := range metaPrefixes {
					if strings.HasPrefix(line, prefix) {
						hasMetaPrefix = true
						break
					}
				}
				if hasMetaPrefix {
					continue
				}

				if !linkRe.MatchString(line) {
					continue
				}

				lineModified := false
				newLine := linkRe.ReplaceAllStringFunc(line, func(match string) string {
					submatches := linkRe.FindStringSubmatch(match)
					if len(submatches) < 3 {
						return match
					}
					displayText := submatches[1]

					var newURL string
					if repo.Path == targetRepo.Path {
						refDir := filepath.Dir(path)
						relPath, err := filepath.Rel(refDir, targetFileAbs)
						if err != nil {
							return match
						}
						newURL = filepath.ToSlash(relPath)
					} else {
						newURL = fmt.Sprintf("https://github.com/%s/%s/blob/main/%s",
							targetRepo.Org, targetRepo.RepoName, targetRelPath)
					}

					lineModified = true
					return fmt.Sprintf("[%s](%s)", displayText, newURL)
				})

				if lineModified {
					lines[i] = newLine
					modified = true
				}
			}

			if modified {
				newContent := strings.Join(lines, "\n")
				if err := os.WriteFile(path, []byte(newContent), info.Mode()); err == nil {
					relPath, err := filepath.Rel(repo.Path, path)
					if err == nil {
						updatedFiles = append(updatedFiles, relPath)
					}
				}
			}

			return nil
		})

		if err != nil {
			return nil, err
		}

		// Ensure we always return a result for each repo we processed
		results = append(results, LinkUpdateResult{
			RepoPath: repo.Path,
			Updated:  updatedFiles,
		})
	}

	return results, nil
}
