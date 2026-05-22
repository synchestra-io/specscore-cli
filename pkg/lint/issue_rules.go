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

	"github.com/specscore/specscore-cli/pkg/issue"
)

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
