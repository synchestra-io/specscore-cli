package lint

// Issue lint rules (I-001..I-015) per the cli/spec/lint/issue-rules
// Feature. This file holds the single checker that the linter registers
// under all 15 rule IDs (mirroring the planRulesChecker pattern in
// plan_rules.go). Only I-009 (dual-location) has logic in this initial
// scaffold; the remaining 14 rules land in subsequent Plan tasks and
// currently return no violations.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/specscore/specscore-cli/pkg/issue"
	"gopkg.in/yaml.v3"
)

// issueRequiredKeys names the five always-required frontmatter fields
// for an `issue` artifact (rule I-001).
var issueRequiredKeys = []string{
	"type",
	"slug",
	"status",
	"captured_at",
	"captured_by",
}

// issueOptionalKeys names the seven optional frontmatter fields whose
// presence alone is allowed (shape validation lives in later rules).
// Together with issueRequiredKeys these form the closed "known keys"
// set used by I-001's unknown-field check.
var issueOptionalKeys = []string{
	"severity",
	"affected_component",
	"first_seen",
	"github_issue",
	"rejection_reason",
	"rejection_notes",
	"bugs",
}

var issueKnownKeySet = func() map[string]bool {
	m := make(map[string]bool, len(issueRequiredKeys)+len(issueOptionalKeys))
	for _, k := range issueRequiredKeys {
		m[k] = true
	}
	for _, k := range issueOptionalKeys {
		m[k] = true
	}
	return m
}()

// issueStatusValues enumerates the four valid `status` values per
// rule I-002.
var issueStatusValues = []string{"open", "investigating", "resolved", "rejected"}

var issueStatusValueSet = func() map[string]bool {
	m := make(map[string]bool, len(issueStatusValues))
	for _, v := range issueStatusValues {
		m[v] = true
	}
	return m
}()

// issueSeverityValues enumerates the five valid `severity` values per
// rule I-003.
var issueSeverityValues = []string{"low", "medium", "high", "critical", "unset"}

var issueSeverityValueSet = func() map[string]bool {
	m := make(map[string]bool, len(issueSeverityValues))
	for _, v := range issueSeverityValues {
		m[v] = true
	}
	return m
}()

// issueNonEmptyStringOptionals names the optional frontmatter fields
// that I-003 requires to be a non-empty string when present. The
// `severity` enum and `bugs` list are validated separately.
var issueNonEmptyStringOptionals = []string{
	"affected_component",
	"first_seen",
	"github_issue",
	"rejection_reason",
	"rejection_notes",
}

// issueRuleIDs enumerates the 15 canonical rule IDs in order.
var issueRuleIDs = []string{
	"I-001", "I-002", "I-003", "I-004", "I-005",
	"I-006", "I-007", "I-008", "I-009", "I-010",
	"I-011", "I-012", "I-013", "I-014", "I-015",
}

type issueRulesChecker struct{}

func newIssueRulesChecker() checker { return &issueRulesChecker{} }

// name returns the primary rule ID. The checker is registered under all
// 15 IDs in linter.go so that --rules / --ignore work per-rule.
func (c *issueRulesChecker) name() string     { return "I-001" }
func (c *issueRulesChecker) severity() string { return "error" }

func (c *issueRulesChecker) check(specRoot string) ([]Violation, error) {
	discovered, err := issue.DiscoverAll(specRoot)
	if err != nil {
		return nil, fmt.Errorf("discovering issue artifacts: %w", err)
	}

	var violations []Violation
	violations = append(violations, lintI009(specRoot, discovered)...)
	violations = append(violations, lintI001AndI002(specRoot, discovered)...)

	// Stable order: by file, then rule.
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Rule < violations[j].Rule
	})
	return violations, nil
}

// lintI009 enforces dual-location placement per upstream REQ
// issue-dual-location. Any file declaring `type: issue` outside the two
// canonical patterns emits a violation. Feature-scoped issues
// additionally require the parent Feature directory to exist (i.e.
// `spec/features/<feature-slug>/README.md` is present); when the parent
// is missing, the issue is treated as off-pattern.
func lintI009(specRoot string, discovered []issue.Discovered) []Violation {
	var out []Violation
	for _, d := range discovered {
		// Pattern match plus, for Feature-scoped issues, the parent
		// Feature directory must actually be a Feature (README.md present).
		ok := d.MatchesPattern
		if ok && d.FeatureSlug != "" {
			parentReadme := filepath.Join(specRoot, "features", d.FeatureSlug, "README.md")
			if info, statErr := os.Stat(parentReadme); statErr != nil || info.IsDir() {
				ok = false
			}
		}
		if ok {
			continue
		}
		out = append(out, Violation{
			File:     d.RelPath,
			Line:     0,
			Severity: "error",
			Rule:     "I-009",
			Message:  "issue artifact must live under spec/issues/ or spec/features/<feature-slug>/issues/ (and the parent Feature directory must exist)",
		})
	}
	return out
}

// lintI001AndI002 enforces frontmatter schema rules I-001 (required +
// known fields, plus slug-matches-filename) and I-002 (status enum).
//
// I-001 emits up to three distinct violations per file, each under its
// own template so violation taxonomy stays unambiguous when more than
// one defect occurs on the same artifact:
//   - "missing required frontmatter field: <name>"  (one per missing key)
//   - "unknown frontmatter field: <name>"           (one per unknown key)
//   - "slug %q does not match filename %q"          (slug/basename mismatch)
//
// I-002 emits at most one violation per file, listing the four valid
// status values verbatim.
func lintI001AndI002(specRoot string, discovered []issue.Discovered) []Violation {
	var out []Violation
	for _, d := range discovered {
		iss, err := issue.Parse(d.Path)
		if err != nil || iss == nil {
			continue
		}
		out = append(out, checkIssueI001(d.RelPath, iss)...)
		out = append(out, checkIssueI002(d.RelPath, iss)...)
		out = append(out, checkIssueI003(d.RelPath, iss)...)
		out = append(out, checkIssueI004(d.RelPath, iss)...)
	}
	return out
}

func checkIssueI001(relPath string, iss *issue.Issue) []Violation {
	var vs []Violation

	// Missing required fields. Treat empty-string values as missing
	// (a present-but-empty `captured_by:` provides no useful identity).
	for _, k := range issueRequiredKeys {
		v, present := iss.Frontmatter[k]
		if !present || strings.TrimSpace(v) == "" {
			vs = append(vs, Violation{
				File:     relPath,
				Line:     0,
				Severity: "error",
				Rule:     "I-001",
				Message:  fmt.Sprintf("missing required frontmatter field: %s", k),
			})
		}
	}

	// Unknown frontmatter keys (anything outside the closed
	// required+optional set).
	var unknown []string
	for _, k := range iss.FrontmatterKeyOrder {
		if !issueKnownKeySet[k] {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	for _, k := range unknown {
		vs = append(vs, Violation{
			File:     relPath,
			Line:     0,
			Severity: "error",
			Rule:     "I-001",
			Message:  fmt.Sprintf("unknown frontmatter field: %s", k),
		})
	}

	// slug must equal the filename basename (minus `.md`). Only
	// emit when slug is present and non-empty — absence is already
	// covered by the missing-field branch above.
	if slugVal, present := iss.Frontmatter["slug"]; present && strings.TrimSpace(slugVal) != "" {
		if slugVal != iss.Slug {
			vs = append(vs, Violation{
				File:     relPath,
				Line:     0,
				Severity: "error",
				Rule:     "I-001",
				Message:  fmt.Sprintf("slug %q does not match filename %q", slugVal, iss.Slug),
			})
		}
	}

	return vs
}

func checkIssueI002(relPath string, iss *issue.Issue) []Violation {
	status, present := iss.Frontmatter["status"]
	if !present || strings.TrimSpace(status) == "" {
		// Absence is an I-001 missing-field violation; not our concern.
		return nil
	}
	if issueStatusValueSet[status] {
		return nil
	}
	return []Violation{{
		File:     relPath,
		Line:     0,
		Severity: "error",
		Rule:     "I-002",
		Message:  fmt.Sprintf("status %q is not one of {%s}", status, strings.Join(issueStatusValues, ", ")),
	}}
}

// checkIssueI003 validates the shape of optional frontmatter fields.
// Absence of any optional field is always valid; only present-but-
// malformed values emit a violation. Per the Plan, I-003 only checks
// type and non-emptiness for the rejection_* fields — the
// presence/absence wiring against `status: rejected` is I-006's job.
func checkIssueI003(relPath string, iss *issue.Issue) []Violation {
	var vs []Violation

	// `severity` enum check.
	if sev, present := iss.Frontmatter["severity"]; present {
		if !issueSeverityValueSet[sev] {
			vs = append(vs, Violation{
				File:     relPath,
				Line:     0,
				Severity: "error",
				Rule:     "I-003",
				Message:  fmt.Sprintf("severity %q is not one of {%s}", sev, strings.Join(issueSeverityValues, ", ")),
			})
		}
	}

	// Non-empty-string checks for the remaining optional string fields.
	for _, k := range issueNonEmptyStringOptionals {
		v, present := iss.Frontmatter[k]
		if !present {
			continue
		}
		if strings.TrimSpace(v) == "" {
			vs = append(vs, Violation{
				File:     relPath,
				Line:     0,
				Severity: "error",
				Rule:     "I-003",
				Message:  fmt.Sprintf("optional field %q must be a non-empty string when present", k),
			})
		}
	}

	return vs
}

// checkIssueI004 validates the reserved `bugs` field. Absence is valid;
// an empty list is valid; a list whose every element is a string scalar
// is valid. Anything else (scalar value, mapping, or list containing a
// non-string element) emits one violation.
//
// Lint MUST NOT resolve the string elements to bug artifacts — the
// `bug` artifact type does not exist in this MVP and the field is
// opaque by design.
func checkIssueI004(relPath string, iss *issue.Issue) []Violation {
	node := iss.BugsRaw
	if node == nil {
		// Field absent — valid.
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return []Violation{{
			File:     relPath,
			Line:     0,
			Severity: "error",
			Rule:     "I-004",
			Message:  "bugs must be a YAML list whose every element is a string",
		}}
	}
	// Empty list is valid.
	for i, elem := range node.Content {
		if elem.Kind != yaml.ScalarNode || (elem.Tag != "" && elem.Tag != "!!str") {
			return []Violation{{
				File:     relPath,
				Line:     0,
				Severity: "error",
				Rule:     "I-004",
				Message:  fmt.Sprintf("bugs element at index %d (%q) is not a string; every element of bugs must be a string", i, elem.Value),
			}}
		}
	}
	return nil
}
