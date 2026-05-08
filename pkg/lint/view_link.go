package lint

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/synchestra-io/specscore-cli/pkg/gitremote"
	"github.com/synchestra-io/specscore-cli/pkg/projectdef"
)

// viewLinkSuffix is the trailing copy describing what the linked
// surface offers.
const viewLinkSuffix = ") — graph, discussions, approvals"

// legacyViewLinkMarkers are marker prefixes from previous incarnations
// of this rule (when the linked surface was called Synchestra Hub, or
// when the brand was rendered "Spec Studio" with a space). The
// classifier flags them as stale; --fix replaces them with the current
// marker so opted-in repos migrate forward in one CLI run. Append-only
// — never remove an entry until every downstream repo has migrated.
var legacyViewLinkMarkers = []string{
	"> [View in Synchestra Hub](",
	"> [View in Spec Studio](",
}

// viewLinkMarker returns the marker prefix for a given viewer name. The
// prefix uniquely identifies the view-link blockquote (stale or fresh)
// in a feature README.
func viewLinkMarker(viewerName string) string {
	return "> [View in " + viewerName + "]("
}

// viewLinkChecker verifies that every feature README carries a
// "View in {viewer.name}" blockquote directly under its H1.
//
// The rule is always-on with SpecStudio defaults; only `viewer: null`
// in specscore.yaml suppresses it (repo-config#req:viewer-null-opts-out).
type viewLinkChecker struct{}

func newViewLinkChecker() checker { return &viewLinkChecker{} }

func (c *viewLinkChecker) name() string     { return "view-link" }
func (c *viewLinkChecker) severity() string { return "warning" }

// BuildViewURL returns the canonical view URL for a feature README at
// relPath (relative to the project root, e.g. "spec/features/bots").
// host is the base URL of the linked surface, with or without a
// trailing slash — both are normalized.
func BuildViewURL(host string, r gitremote.Remote, relPath string) string {
	host = strings.TrimRight(host, "/")
	id := fmt.Sprintf("%s@%s@%s", r.Repo, r.Owner, r.Host)
	relPath = filepath.ToSlash(relPath)
	return fmt.Sprintf("%s/project/features?id=%s&path=%s",
		host, id, url.QueryEscape(relPath))
}

// viewLinkContext resolves the project-level inputs (effective viewer +
// git remote) once per run. When `viewer: null` is set, enabled is
// false and no walking happens. Any opt-in-but-broken configuration
// (no git remote, non-GitHub remote) is surfaced as a single warning
// at the config file.
type viewLinkContext struct {
	enabled    bool
	viewerName string
	viewerURL  string
	remote     gitremote.Remote
	skipWith   Violation // populated if enabled but remote unusable
	skipSet    bool
}

func resolveViewLinkContext(specRoot string) viewLinkContext {
	projectRoot := filepath.Dir(specRoot)
	cfg, err := projectdef.ReadSpecConfig(projectRoot)
	if err != nil {
		// Treat read failure (missing or malformed config) as
		// rule-disabled; other rules surface those errors directly.
		return viewLinkContext{}
	}
	name, host, suppressed := cfg.EffectiveViewer()
	if suppressed {
		return viewLinkContext{}
	}

	ctx := viewLinkContext{enabled: true, viewerName: name, viewerURL: host}
	originURL, err := gitremote.OriginURL(projectRoot)
	if err != nil {
		ctx.skipSet = true
		ctx.skipWith = Violation{
			File:     projectdef.SpecConfigFile,
			Line:     0,
			Severity: "warning",
			Rule:     "view-link",
			Message:  "viewer is configured but origin remote could not be read; rule skipped",
		}
		return ctx
	}
	remote, ok := gitremote.Parse(originURL)
	if !ok {
		ctx.skipSet = true
		ctx.skipWith = Violation{
			File:     projectdef.SpecConfigFile,
			Line:     0,
			Severity: "warning",
			Rule:     "view-link",
			Message:  "viewer is configured but origin remote is not a supported GitHub URL; rule skipped",
		}
		return ctx
	}
	ctx.remote = remote
	return ctx
}

func (c *viewLinkChecker) check(specRoot string) ([]Violation, error) {
	ctx := resolveViewLinkContext(specRoot)
	if !ctx.enabled {
		return nil, nil
	}
	if ctx.skipSet {
		return []Violation{ctx.skipWith}, nil
	}

	projectRoot := filepath.Dir(specRoot)
	currentMarker := viewLinkMarker(ctx.viewerName)
	var violations []Violation
	err := walkFeatureReadmes(specRoot, func(readmePath string, content []byte) {
		relFromRoot, _ := filepath.Rel(projectRoot, readmePath)
		relFromRoot = filepath.ToSlash(relFromRoot)
		expectedURL := BuildViewURL(ctx.viewerURL, ctx.remote, filepath.Dir(relFromRoot))
		expectedLine := currentMarker + expectedURL + viewLinkSuffix

		status := classifyViewLink(string(content), currentMarker, expectedLine)
		if status == viewLinkOK {
			return
		}
		relFromSpec, _ := filepath.Rel(specRoot, readmePath)
		msg := fmt.Sprintf("missing 'View in %s' blockquote under the H1", ctx.viewerName)
		if status == viewLinkStale {
			msg = fmt.Sprintf("'View in %s' blockquote is out of date (wrong URL or copy)", ctx.viewerName)
		}
		violations = append(violations, Violation{
			File:     relFromSpec,
			Line:     0,
			Severity: "warning",
			Rule:     "view-link",
			Message:  msg,
		})
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func (c *viewLinkChecker) fix(specRoot string) error {
	ctx := resolveViewLinkContext(specRoot)
	if !ctx.enabled || ctx.skipSet {
		return nil
	}
	projectRoot := filepath.Dir(specRoot)
	currentMarker := viewLinkMarker(ctx.viewerName)
	return walkFeatureReadmes(specRoot, func(readmePath string, content []byte) {
		relFromRoot, _ := filepath.Rel(projectRoot, readmePath)
		relFromRoot = filepath.ToSlash(relFromRoot)
		expectedURL := BuildViewURL(ctx.viewerURL, ctx.remote, filepath.Dir(relFromRoot))
		expectedLine := currentMarker + expectedURL + viewLinkSuffix

		updated, changed := applyViewLink(string(content), currentMarker, expectedLine)
		if !changed {
			return
		}
		_ = os.WriteFile(readmePath, []byte(updated), 0o644)
	})
}

type viewLinkStatus int

const (
	viewLinkOK viewLinkStatus = iota
	viewLinkMissing
	viewLinkStale
)

// hasViewLinkMarker reports whether line begins with the current marker
// or any legacy marker. Used to detect both fresh and migration-eligible
// view-link blockquotes in a single pass.
func hasViewLinkMarker(line, currentMarker string) bool {
	if strings.HasPrefix(line, currentMarker) {
		return true
	}
	for _, legacy := range legacyViewLinkMarkers {
		if strings.HasPrefix(line, legacy) {
			return true
		}
	}
	return false
}

func classifyViewLink(content, currentMarker, expectedLine string) viewLinkStatus {
	for _, line := range strings.Split(content, "\n") {
		if !hasViewLinkMarker(line, currentMarker) {
			continue
		}
		if line == expectedLine {
			return viewLinkOK
		}
		return viewLinkStale
	}
	return viewLinkMissing
}

// applyViewLink inserts or replaces the view-link blockquote. The
// blockquote is placed on its own line immediately after the first H1,
// separated by a single blank line above and below. Existing blockquotes
// using either the current marker or any legacy marker are replaced
// in-place — that is what migrates older repos forward on `--fix`.
func applyViewLink(content, currentMarker, expectedLine string) (string, bool) {
	lines := strings.Split(content, "\n")

	// Replace an existing blockquote anywhere in the file (idempotent
	// on stale; migrates legacy markers forward).
	for i, line := range lines {
		if !hasViewLinkMarker(line, currentMarker) {
			continue
		}
		if line == expectedLine {
			return content, false
		}
		lines[i] = expectedLine
		return strings.Join(lines, "\n"), true
	}

	// Insert after the first H1.
	h1 := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			h1 = i
			break
		}
	}
	if h1 < 0 {
		return content, false
	}

	// Build the insertion: [blank][blockquote][blank]. Collapse if the
	// next line is already blank to avoid double blanks.
	insert := []string{"", expectedLine, ""}
	tail := lines[h1+1:]
	if len(tail) > 0 && tail[0] == "" {
		tail = tail[1:]
	}
	newLines := append([]string{}, lines[:h1+1]...)
	newLines = append(newLines, insert...)
	newLines = append(newLines, tail...)
	return strings.Join(newLines, "\n"), true
}

// walkFeatureReadmes invokes fn for every feature README under specRoot,
// skipping reserved _-prefixed subtrees. It mirrors the walk used by
// adherenceFooterChecker so both rules agree on scope.
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
		content, readErr := os.ReadFile(readmePath)
		if readErr != nil {
			return nil
		}
		fn(readmePath, content)
		return nil
	})
}
