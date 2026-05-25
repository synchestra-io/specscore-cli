package issue

import (
	"fmt"
	"os/user"
	"regexp"
	"strings"
	"time"
)

// ScaffoldOptions controls the content emitted by Scaffold.
type ScaffoldOptions struct {
	Slug              string
	Title             string // defaults to title-cased slug
	CapturedBy        string // defaults to $USER
	CapturedAt        string // RFC 3339 timestamp, defaults to now UTC
	Severity          string // optional: low|medium|high|critical
	AffectedComponent string // optional: Feature slug
	FirstSeen         string // optional
	GithubIssue       string // optional
}

// ValidSeverityValues is the set of valid --severity values for issue new.
var ValidSeverityValues = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

// Testable indirection for user lookup.
var currentUser = user.Current

// slugRe matches a valid issue slug: lowercase alphanumeric with hyphens,
// no leading/trailing hyphens, max 60 chars.
var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug checks that the slug conforms to issue slug rules:
// lowercase, [a-z0-9-] only, no leading/trailing hyphens, max 60 chars.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if len(slug) > 60 {
		return fmt.Errorf("slug %q exceeds 60 characters", slug)
	}
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("slug %q does not match [a-z0-9]+(-[a-z0-9]+)*", slug)
	}
	return nil
}

// titleCaseFromSlug turns "payment-timeout" into "Payment Timeout".
func titleCaseFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// defaultCapturedBy returns a reasonable fallback (current OS user).
func defaultCapturedBy() string {
	if u, err := currentUser(); err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}

// Scaffold returns a lint-clean Issue file body for the given options.
func Scaffold(opts ScaffoldOptions) ([]byte, error) {
	if err := ValidateSlug(opts.Slug); err != nil {
		return nil, err
	}

	title := opts.Title
	if strings.TrimSpace(title) == "" {
		title = titleCaseFromSlug(opts.Slug)
	}
	capturedBy := strings.TrimSpace(opts.CapturedBy)
	if capturedBy == "" {
		capturedBy = defaultCapturedBy()
	}
	capturedAt := strings.TrimSpace(opts.CapturedAt)
	if capturedAt == "" {
		capturedAt = time.Now().UTC().Format(time.RFC3339)
	}

	var out strings.Builder

	// Frontmatter
	out.WriteString("---\n")
	out.WriteString("type: issue\n")
	fmt.Fprintf(&out, "slug: %s\n", opts.Slug)
	out.WriteString("status: open\n")
	fmt.Fprintf(&out, "captured_at: %s\n", capturedAt)
	fmt.Fprintf(&out, "captured_by: %s\n", capturedBy)
	if sev := strings.TrimSpace(opts.Severity); sev != "" {
		fmt.Fprintf(&out, "severity: %s\n", sev)
	}
	if comp := strings.TrimSpace(opts.AffectedComponent); comp != "" {
		fmt.Fprintf(&out, "affected_component: %s\n", comp)
	}
	if fs := strings.TrimSpace(opts.FirstSeen); fs != "" {
		fmt.Fprintf(&out, "first_seen: %s\n", fs)
	}
	if gh := strings.TrimSpace(opts.GithubIssue); gh != "" {
		fmt.Fprintf(&out, "github_issue: %s\n", gh)
	}
	out.WriteString("---\n")

	// Body
	fmt.Fprintf(&out, "\n# Issue: %s\n", title)
	out.WriteString("\n## Description\n\n")
	out.WriteString("<!-- TODO: Describe the observed behavior -->\n")
	out.WriteString("\n## Steps to Reproduce\n\n")
	out.WriteString("<!-- TODO: List the steps to reproduce this issue -->\n")
	out.WriteString("\n## Expected vs Actual\n\n")
	out.WriteString("<!-- TODO: Describe what you expected and what happened instead -->\n")
	out.WriteString("\n---\n")
	out.WriteString("*This document follows the https://specscore.md/issue-specification*\n")

	return []byte(out.String()), nil
}
