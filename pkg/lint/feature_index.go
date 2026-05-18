package lint

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/synchestra-io/specscore-cli/pkg/feature"
)

// featureIndexChecker dispatches the feature-index-row-sync rule. The
// rule logic lives in `featureIndexRules` and is invoked twice — once
// in report mode from `check()`, once in mutation mode from `fix()` —
// matching the `checker` / `fixer` split used by every other rule in
// this package.
type featureIndexChecker struct{}

func newFeatureIndexChecker() *featureIndexChecker {
	return &featureIndexChecker{}
}

func (c *featureIndexChecker) name() string     { return "feature-index-row-sync" }
func (c *featureIndexChecker) severity() string { return "error" }

func (c *featureIndexChecker) check(specRoot string) ([]Violation, error) {
	vs, _ := featureIndexRules(specRoot, false)
	return vs, nil
}

// fix implements the fixer interface: rewrites drifted Status cells in
// the features-index to match each feature's `**Status:**`. The check
// pass that follows reports zero violations because the rewrite is
// complete; idempotency is satisfied because the second pass finds no
// drift to rewrite.
func (c *featureIndexChecker) fix(specRoot string) error {
	_, _ = featureIndexRules(specRoot, true)
	return nil
}

// featureIndexRules enforces:
//   - feature-index-row-sync: each top-level row's `Status` cell in
//     spec/features/README.md mirrors the corresponding feature's
//     `**Status:**` value at spec/features/<feature_id>/README.md.
//     Drift typically arises after a Status line is rewritten by hand
//     (or by a future `change-status` verb); the index row, being
//     derived state, must follow.
//
// Scope: top-level features only (entries whose slug contains a "/"
// are sub-features and are NOT listed in the features-index).
//
// What --fix does: rewrites the drifted `Status` cell in the index row
// to match the feature README. The `Feature` link and `Description`
// cells are hand-maintained per the features-index meta-spec contract
// and are NOT rewritten.
func featureIndexRules(specRoot string, fix bool) ([]Violation, bool) {
	var vs []Violation
	fixed := false

	featuresDir := filepath.Join(specRoot, "features")
	if info, err := os.Stat(featuresDir); err != nil || !info.IsDir() {
		return nil, false
	}

	indexPath := filepath.Join(featuresDir, "README.md")
	if _, err := os.Stat(indexPath); err != nil {
		return nil, false
	}

	rows, err := readFeatureIndexRows(indexPath)
	if err != nil {
		return nil, false
	}

	type drift struct {
		slug    string
		actual  string
		want    string
		lineNum int
	}
	var drifts []drift
	for _, r := range rows {
		// Top-level features only — sub-features (slug contains "/")
		// are not listed in the features-index.
		if strings.Contains(r.slug, "/") {
			continue
		}
		featureReadme := filepath.Join(featuresDir, r.slug, "README.md")
		if _, err := os.Stat(featureReadme); err != nil {
			// No matching feature directory — that is an
			// orphaned row, which is a different concern from
			// row-sync. Skip silently here; other rules
			// (or future feature-index-completeness) would
			// cover it.
			continue
		}
		want, err := feature.ParseFeatureStatus(featureReadme)
		if err != nil {
			continue
		}
		if r.status != want {
			drifts = append(drifts, drift{
				slug: r.slug, actual: r.status, want: want, lineNum: r.lineNum,
			})
		}
	}

	if len(drifts) == 0 {
		return nil, false
	}

	rel, _ := filepath.Rel(specRoot, indexPath)

	if fix {
		updates := make(map[string]string, len(drifts))
		for _, d := range drifts {
			updates[d.slug] = d.want
		}
		if err := rewriteFeatureIndexStatuses(indexPath, updates); err == nil {
			return nil, true
		}
		// Fall through to reporting if the rewrite failed.
		slugs := make([]string, 0, len(drifts))
		for _, d := range drifts {
			slugs = append(slugs, d.slug)
		}
		sort.Strings(slugs)
		vs = append(vs, Violation{
			File: rel, Line: 0, Severity: "error",
			Rule:    "feature-index-row-sync",
			Message: fmt.Sprintf("features-index Status cells drifted from feature READMEs: %s (fix failed)", strings.Join(slugs, ", ")),
		})
		return vs, false
	}

	sort.Slice(drifts, func(i, j int) bool { return drifts[i].slug < drifts[j].slug })
	for _, d := range drifts {
		vs = append(vs, Violation{
			File: rel, Line: d.lineNum, Severity: "error",
			Rule:    "feature-index-row-sync",
			Message: fmt.Sprintf("features-index row for %q shows Status %q but feature README declares %q (run `specscore spec lint --fix`)", d.slug, d.actual, d.want),
		})
	}
	return vs, fixed
}

// featureIndexRow captures one parsed top-level row of the features
// index table. Only `slug` and `status` participate in the row-sync
// check; the `Feature` link and `Description` cells are hand-maintained
// and are not exposed here.
type featureIndexRow struct {
	slug    string
	status  string
	lineNum int
}

// featureIndexRowRe matches one row of the features-index table:
//
//	| [<slug>](<slug>/README.md) | <status> | <kind> | <description> |
//
// Cells are trimmed; we tolerate any amount of inner whitespace.
// The link target is `<slug>/README.md` (directory-based) rather than
// `<slug>.md` (file-based, used by the ideas index).
var featureIndexRowRe = regexp.MustCompile(`^\|\s*\[[^\]]+\]\(([^)]+)/README\.md\)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*(.*?)\s*\|\s*$`)

// readFeatureIndexRows scans the features-index README and returns one
// featureIndexRow per row of the top-level table. Header and separator
// lines are skipped. Rows whose slug contains "/" (deeper links) are
// retained so the caller can filter; the caller is responsible for
// excluding sub-features from the row-sync check.
func readFeatureIndexRows(path string) ([]featureIndexRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var rows []featureIndexRow
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		m := featureIndexRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		rows = append(rows, featureIndexRow{
			slug:    m[1],
			status:  strings.TrimSpace(m[2]),
			lineNum: lineNum,
		})
	}
	return rows, scanner.Err()
}

// rewriteFeatureIndexStatuses rewrites the `Status` cell of each row in
// the features-index whose slug appears in `updates`. The `Feature`
// link and `Description` cells are preserved verbatim — only the
// `Status` column is touched. Unmatched rows are left alone.
func rewriteFeatureIndexStatuses(path string, updates map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := featureIndexRowRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		slug := m[1]
		newStatus, ok := updates[slug]
		if !ok {
			continue
		}
		if strings.TrimSpace(m[2]) == newStatus {
			continue
		}
		// Reassemble preserving the original kind and description cells.
		kind := strings.TrimSpace(m[3])
		desc := strings.TrimSpace(m[4])
		lines[i] = fmt.Sprintf("| [%s](%s/README.md) | %s | %s | %s |", slug, slug, newStatus, kind, desc)
		changed = true
	}
	if !changed {
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
