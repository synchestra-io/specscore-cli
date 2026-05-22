// Package issue provides parsing and discovery for SpecScore `issue`
// artifacts. Issues are markdown files declaring `type: issue` in their
// YAML frontmatter and living at one of two canonical locations:
//
//   - spec/issues/<slug>.md                     (root-level)
//   - spec/features/<feature-slug>/issues/<slug>.md (Feature-scoped)
//
// See spec/features/cli/spec/lint/issue-rules/README.md for the full
// contract.
package issue

import "gopkg.in/yaml.v3"

// PathPatterns names the two glob patterns (relative to the spec root)
// where `issue` artifacts may live. Files outside these patterns that
// declare `type: issue` violate rule I-009 (dual-location).
//
// The patterns are written without the leading `spec/` prefix because
// the lint engine always operates relative to the discovered spec root.
var PathPatterns = []string{
	"issues/*.md",
	"features/*/issues/*.md",
}

// RuleFamilyPrefix is the canonical ID prefix for every lint rule that
// validates `issue` artifacts (I-001 .. I-015).
const RuleFamilyPrefix = "I-"

// TypeValue is the literal frontmatter value identifying an issue artifact.
const TypeValue = "issue"

// Issue is the parsed representation of an issue artifact. It is
// deliberately minimal at this Task: only the fields the I-009 rule
// (and downstream rules in later tasks) need.
type Issue struct {
	// Path is the absolute (or otherwise caller-supplied) path to the file.
	Path string
	// Slug is the basename minus `.md`.
	Slug string
	// Type is the frontmatter `type` value (empty when missing or
	// unparseable). Only files where Type == TypeValue are considered
	// issue artifacts.
	Type string
	// Frontmatter holds the top-level YAML mapping keys (values
	// stringified). nil when no frontmatter could be parsed.
	Frontmatter map[string]string
	// FrontmatterKeyOrder preserves the order keys appeared in the
	// source YAML.
	FrontmatterKeyOrder []string
	// Body is the markdown body after the closing `---`.
	Body string
	// HasFrontmatter is true when the file begins with `---` and a
	// closing `---` line is found.
	HasFrontmatter bool
	// BugsRaw is the raw YAML node for the `bugs` frontmatter field, or
	// nil when the field is absent. Exposed so list-aware rules (I-004)
	// can inspect the node's Kind and elements without re-parsing.
	BugsRaw *yaml.Node
}
