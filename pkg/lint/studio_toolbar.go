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

// studioToolbarChecker implements the studio-toolbar lint rule defined
// by the studio-toolbar Feature. The rule validates that file position 3
// of every feature README is a byte-exact canonical toolbar line, given
// the project's resolved studio config and the artifact's path.
//
// Replaces the legacy view-link rule (studio-toolbar#req:studio-toolbar-lint-removes-view-link).
type studioToolbarChecker struct{}

func newStudioToolbarChecker() checker { return &studioToolbarChecker{} }

func (c *studioToolbarChecker) name() string     { return "studio-toolbar" }
func (c *studioToolbarChecker) severity() string { return "error" }

// RenderStudioToolbar returns the canonical toolbar line per
// studio-toolbar#req:toolbar-line-shape. The returned string includes the
// trailing LF. Callers (lint, --fix) compare or replace line 3 with this
// exact byte sequence.
//
// Inputs:
//   - name: studio.name (with or without dots; bold-wrap rules apply
//     per brand-attribution-rendering / brand-attribution-no-dot /
//     brand-attribution-multi-dot).
//   - url: studio.url; MUST end in exactly one "/" — the validation is
//     enforced upstream in projectdef.Validate(). The renderer strips the
//     trailing "/" per url-grammar-trailing-slash before joining with the
//     path grammar.
//   - host, org, repo: resolved from specscore.yaml project.host/.org/.repo,
//     falling back to git origin parsing.
//   - artifactPath: the feature directory path from repo root, e.g.
//     "spec/features/repo-config". MUST NOT include "/README.md".
func RenderStudioToolbar(name, urlStr, host, org, repo, artifactPath string) string {
	brand := renderBrandAttribution(name)
	stripped := strings.TrimSuffix(urlStr, "/")
	prefix := stripped + "/app/p/" + host + "/" + org + "/" + repo + "/" + escapeArtifactPath(artifactPath)
	explore := prefix + "?op=explore"
	edit := prefix + "?op=edit"
	ask := prefix + "?op=ask"
	reqchg := prefix + "?op=request-change"
	return "> " + brand + "](" + stripped + ")" + ":" +
		" | [Explore](" + explore + ")" +
		" | [Edit](" + edit + ")" +
		" | [Ask question](" + ask + ")" +
		" | [Request change](" + reqchg + ")" +
		" |\n"
}

// renderBrandAttribution returns the "[name-prefix**name-product**"
// portion of the brand attribution — the LEADING bracket is included,
// but the closing bracket and the link target are NOT (the caller
// appends `](url):` to complete the markdown link).
//
// Per studio-toolbar#req:brand-attribution-rendering and the no-dot /
// multi-dot variants: only the segment after the LAST `.` is wrapped in
// `**...**`; when no `.` exists, no bold wrapping.
func renderBrandAttribution(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return "[" + name
	}
	prefix := name[:idx+1]
	product := name[idx+1:]
	return "[" + prefix + "**" + product + "**"
}

// escapeArtifactPath escapes only RFC-3986-mandatory chars per segment,
// preserving "/" as a literal path separator (studio-toolbar#req:url-grammar-path).
// url.PathEscape leaves -, _, . alone but does escape "/", which is why
// we split on "/" and escape each segment.
func escapeArtifactPath(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

// resolveProjectIdentity returns (host, org, repo) for toolbar URL
// composition. Explicit project.host/.org/.repo in specscore.yaml win
// over git-origin inference (mirrors the repo-config #req:source-reference-overrides
// precedence). Returns ok=false when neither source yields a usable
// triple — the caller surfaces this as a single skip-violation.
func resolveProjectIdentity(cfg projectdef.SpecConfig, projectRoot string) (host, org, repo string, ok bool) {
	if cfg.Project != nil {
		host, org, repo = cfg.Project.Host, cfg.Project.Org, cfg.Project.Repo
	}
	if host != "" && org != "" && repo != "" {
		return host, org, repo, true
	}
	// Fall back to git origin for any missing field.
	originURL, err := gitremote.OriginURL(projectRoot)
	if err != nil {
		return "", "", "", false
	}
	r, parsed := gitremote.Parse(originURL)
	if !parsed {
		return "", "", "", false
	}
	if host == "" {
		host = r.Host
	}
	if org == "" {
		org = r.Owner
	}
	if repo == "" {
		repo = r.Repo
	}
	if host == "" || org == "" || repo == "" {
		return "", "", "", false
	}
	return host, org, repo, true
}

// check is the lint entry point. Walks spec/features/*/README.md and
// verifies file position 3 byte-equals the canonical toolbar.
func (c *studioToolbarChecker) check(specRoot string) ([]Violation, error) {
	projectRoot := filepath.Dir(specRoot)
	cfg, err := projectdef.ReadSpecConfig(projectRoot)
	if err != nil {
		// The pre-2026-05-19 viewer: block is rejected here. Surface as
		// a single rule violation at specscore.yaml so users see exactly
		// one hard error pointing at the migration step (studio-toolbar#
		// req:studio-toolbar-lint-no-viewer-backcompat).
		//
		// Other parse failures (e.g. missing config in test scaffolds
		// that don't bother to seed specscore.yaml) are silently skipped —
		// other rules surface those issues, and the studio-toolbar rule
		// has nothing meaningful to say without a project identity.
		if strings.Contains(err.Error(), "viewer: block is no longer supported") {
			return []Violation{{
				File:     projectdef.SpecConfigFile,
				Line:     0,
				Severity: "error",
				Rule:     "studio-toolbar",
				Message:  err.Error(),
			}}, nil
		}
		return nil, nil
	}

	if cfg.IsStudioSuppressed() {
		// studio-toolbar#req:studio-toolbar-opt-out — opt-out globally
		// suppresses the rule.
		return nil, nil
	}

	name, urlStr, _ := cfg.EffectiveStudio()
	host, org, repo, identityOK := resolveProjectIdentity(cfg, projectRoot)

	var violations []Violation
	sawFeature := false
	walkErr := walkFeatureReadmesExcludingIndex(specRoot, func(readmePath string, content []byte) {
		sawFeature = true
		if !identityOK {
			// Defer surfacing the no-identity violation until we know
			// there's at least one feature that would need a toolbar.
			// We'll add it once after the walk.
			return
		}
		relFromRoot, _ := filepath.Rel(projectRoot, readmePath)
		relFromRoot = filepath.ToSlash(relFromRoot)
		// artifactPath is the feature directory without README.md.
		artifactPath := strings.TrimSuffix(relFromRoot, "/README.md")
		expected := RenderStudioToolbar(name, urlStr, host, org, repo, artifactPath)
		expectedLine := strings.TrimRight(expected, "\n")

		lines := strings.Split(string(content), "\n")
		relFromSpec, _ := filepath.Rel(specRoot, readmePath)
		relFromSpec = filepath.ToSlash(relFromSpec)

		if len(lines) < 3 {
			violations = append(violations, Violation{
				File:     relFromSpec,
				Line:     3,
				Severity: "error",
				Rule:     "studio-toolbar",
				Message:  fmt.Sprintf("missing studio toolbar at file position 3; expected: %s (studio-toolbar#req:toolbar-position)", expectedLine),
			})
			return
		}
		actual := lines[2]
		if actual == expectedLine {
			return
		}
		violations = append(violations, Violation{
			File:     relFromSpec,
			Line:     3,
			Severity: "error",
			Rule:     "studio-toolbar",
			Message:  classifyDeviation(actual, expectedLine),
		})
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if sawFeature && !identityOK {
		// At least one feature would need a toolbar but the project has
		// no resolvable identity. Surface a single violation pointing at
		// specscore.yaml — the test fixtures without any features (e.g.
		// fresh `idea new` scaffolds) intentionally fall through silent.
		violations = append(violations, Violation{
			File:     projectdef.SpecConfigFile,
			Line:     0,
			Severity: "error",
			Rule:     "studio-toolbar",
			Message:  "studio toolbar requires project host/org/repo (set them in specscore.yaml or configure a GitHub origin remote)",
		})
	}
	return violations, nil
}

// classifyDeviation returns a message identifying the most specific
// REQ violated when actual ≠ expected. Detects the well-known anti-patterns
// (scope tokens, UTM, branch/tag pins) and falls back to a generic
// byte-deviation message citing toolbar-line-shape.
func classifyDeviation(actual, expected string) string {
	switch {
	case strings.Contains(actual, "&section=") || strings.Contains(actual, "&req="):
		return fmt.Sprintf("toolbar URL contains scope token (&section= or &req=); the toolbar operates on whole feature artifacts (studio-toolbar#req:toolbar-file-scope). expected: %s", expected)
	case strings.Contains(actual, "utm_source=") ||
		strings.Contains(actual, "utm_medium=") ||
		strings.Contains(actual, "utm_campaign=") ||
		strings.Contains(actual, "utm_content="):
		return fmt.Sprintf("toolbar URL contains UTM tracking parameter (studio-toolbar#req:url-grammar-no-utm). expected: %s", expected)
	case strings.Contains(actual, "?ref=") || strings.Contains(actual, "&ref=") || containsBranchPin(actual):
		return fmt.Sprintf("toolbar URL contains branch or tag suffix (@branch, ?ref=, etc.); links MUST point at the current working state (studio-toolbar#req:url-grammar-no-branch-tag). expected: %s", expected)
	}
	return fmt.Sprintf("studio toolbar at file position 3 does not match canonical form (studio-toolbar#req:toolbar-line-shape).\n  got:      %s\n  expected: %s", actual, expected)
}

// containsBranchPin returns true if any toolbar URL inside line contains
// an "@<branch>" suffix between the host/org/repo path segments. Matches
// "github.com/owner/repo@main" but not the literal "@" inside a label.
func containsBranchPin(line string) bool {
	// Cheap heuristic: look for "/app/p/...@" — any "@" appearing inside
	// the path component (between /app/p/ and the next ?) is suspicious.
	i := strings.Index(line, "/app/p/")
	for i >= 0 {
		rest := line[i:]
		q := strings.Index(rest, "?")
		end := len(rest)
		if q >= 0 {
			end = q
		}
		if strings.Contains(rest[:end], "@") {
			return true
		}
		next := strings.Index(rest[1:], "/app/p/")
		if next < 0 {
			break
		}
		i = i + 1 + next
	}
	return false
}

// fix implements the fixer interface — rewrites or removes line 3 of
// every feature README to match the resolved studio config. Idempotent.
//
// studio-toolbar#req:studio-toolbar-autofix-artifact-line — replaces
// legacy "> [View in ...]" lines, drifted toolbars, and inserts a
// toolbar when missing (provided studio: is not null).
//
// studio-toolbar#req:studio-toolbar-autofix-blocked-by-viewer — refuses
// to run when specscore.yaml still contains a viewer: block (the parser
// surfaces that error, which propagates up via the fixer return).
func (c *studioToolbarChecker) fix(specRoot string) error {
	projectRoot := filepath.Dir(specRoot)
	cfg, err := projectdef.ReadSpecConfig(projectRoot)
	if err != nil {
		// Distinguish "viewer: block is no longer supported" (hard fail —
		// studio-toolbar#req:studio-toolbar-autofix-blocked-by-viewer)
		// from missing-config / other parse errors (soft no-op so the
		// rule doesn't break tests or callers that lint a bare spec tree
		// without a specscore.yaml).
		if strings.Contains(err.Error(), "viewer: block is no longer supported") {
			return err
		}
		return nil
	}

	suppressed := cfg.IsStudioSuppressed()
	var name, urlStr, host, org, repo string
	if !suppressed {
		name, urlStr, _ = cfg.EffectiveStudio()
		var ok bool
		host, org, repo, ok = resolveProjectIdentity(cfg, projectRoot)
		if !ok {
			// Can't compute URLs — refuse to write garbage. The check()
			// path surfaces this as a violation; here we just no-op.
			return nil
		}
	}

	return walkFeatureReadmesExcludingIndex(specRoot, func(readmePath string, content []byte) {
		// Preserve trailing-LF property of the original file.
		trailingLF := strings.HasSuffix(string(content), "\n")
		// CRLF note: we split on "\n" deliberately. Files with CRLF
		// endings will retain "\r" on each line; the toolbar comparison
		// is byte-exact LF-only per the studio-toolbar contract.
		lines := strings.Split(string(content), "\n")
		// strings.Split adds a trailing "" when the content ends with "\n".
		// Remove it so we operate on logical lines; we'll restore the
		// final LF on writeback if trailingLF.
		if trailingLF && len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}

		if suppressed {
			// Opt-out: remove a line-3 toolbar-shaped blockquote if one
			// exists; do nothing otherwise. Detection is loose — any
			// "> [...](" blockquote at position 3 is treated as a toolbar
			// remnant (legacy view-link OR canonical toolbar).
			if len(lines) >= 3 && isToolbarLikeLine(lines[2]) {
				newLines := append([]string{}, lines[:2]...)
				newLines = append(newLines, lines[3:]...)
				writeBack(readmePath, newLines, trailingLF)
			}
			return
		}

		relFromRoot, _ := filepath.Rel(projectRoot, readmePath)
		relFromRoot = filepath.ToSlash(relFromRoot)
		artifactPath := strings.TrimSuffix(relFromRoot, "/README.md")
		expected := strings.TrimRight(RenderStudioToolbar(name, urlStr, host, org, repo, artifactPath), "\n")

		// Pad to at least 3 lines so we always have a position 3 to operate on.
		for len(lines) < 3 {
			lines = append(lines, "")
		}
		if lines[2] == expected {
			return // idempotent: already canonical
		}
		// If line 3 already looks like a toolbar (canonical drifted or
		// legacy "> [View in ...]"), REPLACE it. Otherwise INSERT a new
		// line at position 3 so we don't clobber genuine content (e.g.
		// "**Status:**" produced by a freshly scaffolded README).
		if isToolbarLikeLine(lines[2]) {
			lines[2] = expected
		} else {
			// Insert: new[0]=lines[0], new[1]=lines[1], new[2]=expected,
			// new[3..]=lines[2..].
			newLines := make([]string, 0, len(lines)+1)
			newLines = append(newLines, lines[:2]...)
			newLines = append(newLines, expected)
			newLines = append(newLines, lines[2:]...)
			lines = newLines
		}
		writeBack(readmePath, lines, trailingLF)
	})
}

// isToolbarLikeLine reports whether s looks like a blockquote that wraps
// a markdown link — that's the visual shape of both the canonical
// toolbar and the legacy "> [View in ...](...)" line.
func isToolbarLikeLine(s string) bool {
	return strings.HasPrefix(s, "> [") && strings.Contains(s, "](")
}

// writeBack joins lines with LF and rewrites readmePath, restoring the
// trailing LF when the original file had one.
func writeBack(readmePath string, lines []string, trailingLF bool) {
	out := strings.Join(lines, "\n")
	if trailingLF {
		out += "\n"
	}
	_ = os.WriteFile(readmePath, []byte(out), 0o644)
}
