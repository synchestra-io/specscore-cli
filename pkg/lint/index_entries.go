package lint

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// indexEntriesChecker verifies that feature README indices match actual child directories.
type indexEntriesChecker struct{}

func newIndexEntriesChecker() checker {
	return &indexEntriesChecker{}
}

func (c *indexEntriesChecker) name() string     { return "index-entries" }
func (c *indexEntriesChecker) severity() string { return "error" }

func (c *indexEntriesChecker) check(specRoot string) ([]Violation, error) {
	var violations []Violation

	featureDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featureDir)
	if err != nil || !info.IsDir() {
		return violations, nil
	}

	err = filepath.Walk(featureDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		readmePath := filepath.Join(path, "README.md")
		if _, statErr := os.Stat(readmePath); statErr != nil {
			return nil
		}

		// Get actual child directories (excluding hidden and _args convention dirs).
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil
		}

		var actualChildren []string
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && !strings.HasPrefix(entry.Name(), "_") {
				actualChildren = append(actualChildren, entry.Name())
			}
		}

		mentioned, parseErr := extractChildRefsFromReadme(readmePath)
		if parseErr != nil {
			return nil
		}

		relPath, _ := filepath.Rel(specRoot, readmePath)

		// Flag index entries that reference non-existent directories.
		actualSet := make(map[string]bool, len(actualChildren))
		for _, a := range actualChildren {
			actualSet[a] = true
		}
		for _, m := range mentioned {
			if !actualSet[m] {
				violations = append(violations, Violation{
					File:     relPath,
					Line:     0,
					Severity: "error",
					Rule:     "index-entries",
					Message:  "Index mentions non-existent directory: " + m,
				})
			}
		}

		// Flag child directories that are not mentioned in the index.
		mentionedSet := make(map[string]bool, len(mentioned))
		for _, m := range mentioned {
			mentionedSet[m] = true
		}
		for _, a := range actualChildren {
			if !mentionedSet[a] {
				violations = append(violations, Violation{
					File:     relPath,
					Line:     0,
					Severity: "error",
					Rule:     "index-entries",
					Message:  "Child directory not listed in index: " + a,
				})
			}
		}

		return nil
	})

	return violations, err
}

// fix implements the index-entries-fix-deletes-phantom-rows REQ: for every
// feature README index that links a child directory which does not exist on
// disk, the row referencing that phantom child is removed. The orphan-child
// direction (a real directory not listed in the index) is NOT autofixed —
// inserting a populated row would require reading Status/Kind/Description
// from the child README, which violates fix-is-safe-subset until a canonical
// feature-metadata parser exists.
func (c *indexEntriesChecker) fix(specRoot string) error {
	featureDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featureDir)
	if err != nil || !info.IsDir() {
		return nil
	}

	return filepath.Walk(featureDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			return nil
		}

		readmePath := filepath.Join(path, "README.md")
		if _, statErr := os.Stat(readmePath); statErr != nil {
			return nil
		}

		// Collect actual child dirs so we know which mentioned slugs are phantoms.
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil
		}
		actualSet := make(map[string]bool)
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && !strings.HasPrefix(e.Name(), "_") {
				actualSet[e.Name()] = true
			}
		}

		content, err := os.ReadFile(readmePath)
		if err != nil {
			return nil
		}
		rewritten, changed := dropPhantomIndexRows(string(content), actualSet)
		if !changed {
			return nil
		}
		return os.WriteFile(readmePath, []byte(rewritten), 0o644)
	})
}

// dropPhantomIndexRows returns content with every table row removed whose
// Markdown link target ends in `<dirname>/README.md` where <dirname> is not
// present in actualSet. Lines outside fenced code blocks are considered. Only
// lines starting with `|` (whitespace-trimmed) are eligible for deletion, so
// inline prose references — which the index-entries check parses for read but
// would never sit on a table row — are left untouched. Returns the same
// content and false when nothing was dropped.
func dropPhantomIndexRows(content string, actualSet map[string]bool) (string, bool) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			out = append(out, line)
			continue
		}
		if inCodeBlock || !strings.HasPrefix(trimmed, "|") {
			out = append(out, line)
			continue
		}
		if dirname, ok := phantomDirInTableRow(line, actualSet); ok {
			changed = true
			_ = dirname // dropped line; no further bookkeeping needed
			continue
		}
		out = append(out, line)
	}
	if !changed {
		return content, false
	}
	return strings.Join(out, "\n"), true
}

// phantomDirInTableRow inspects a single line that is known to start with `|`
// and returns (dirname, true) if it contains a Markdown link whose target is
// of the form `<dirname>/README.md` and <dirname> is NOT in actualSet.
// Returns ("", false) otherwise. If the row links multiple children and any
// one of them is real, the row is kept (false) — the row carries live data
// for the real child and should not be silently deleted.
func phantomDirInTableRow(line string, actualSet map[string]bool) (string, bool) {
	rest := line
	var phantom string
	for {
		idx := strings.Index(rest, "](")
		if idx < 0 {
			break
		}
		after := rest[idx+2:]
		end := strings.Index(after, ")")
		if end < 0 {
			break
		}
		target := after[:end]
		rest = after[end+1:]

		if !strings.HasSuffix(target, "/README.md") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(target, "./"), "/")
		if len(parts) != 2 {
			continue
		}
		dirname := parts[0]
		if dirname == "." || dirname == ".." || strings.HasPrefix(dirname, "_") {
			continue
		}
		if actualSet[dirname] {
			// Row links a real child — keep it even if it also links a phantom;
			// deleting would lose the real link.
			return "", false
		}
		if phantom == "" {
			phantom = dirname
		}
	}
	if phantom == "" {
		return "", false
	}
	return phantom, true
}

// extractChildRefsFromReadme scans a README for markdown links pointing to
// child directories (e.g. `[name](dirname/README.md)`).
func extractChildRefsFromReadme(readmePath string) ([]string, error) {
	file, err := os.Open(readmePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var children []string
	seen := make(map[string]bool)
	inCodeBlock := false

	for scanner.Scan() {
		line := scanner.Text()

		// Skip fenced code blocks.
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Look for links to child README.md: [text](dirname/README.md)
		for {
			idx := strings.Index(line, "](")
			if idx < 0 {
				break
			}
			rest := line[idx+2:]
			end := strings.Index(rest, ")")
			if end < 0 {
				break
			}
			linkTarget := rest[:end]
			line = rest[end+1:] // advance past this link

			// Only consider links ending in README.md and pointing to a direct child.
			if !strings.HasSuffix(linkTarget, "README.md") && !strings.HasSuffix(linkTarget, "README.md)") {
				continue
			}
			parts := strings.Split(strings.TrimPrefix(linkTarget, "./"), "/")
			if len(parts) == 2 {
				dirname := parts[0]
				if dirname != "." && dirname != ".." && !strings.HasPrefix(dirname, "_") && !seen[dirname] {
					seen[dirname] = true
					children = append(children, dirname)
				}
			}
		}
	}

	if len(children) == 0 {
		return nil, nil
	}
	return children, scanner.Err()
}
