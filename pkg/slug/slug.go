// Package slug provides shared slug-derivation helpers used across the
// SpecScore CLI. It is deliberately small and dependency-free so any
// artifact package (issue, idea, feature, …) can import it without
// cycling back through pkg/lint.
package slug

import (
	"strings"
	"unicode"
)

// issueSlugMaxLen is the inclusive upper bound on IssueSlug output
// length. Tracks the AC contract — the result must be ≤ 60 characters
// and, when truncation is needed, must end at a `-` boundary whenever
// one exists within the first 60 characters.
const issueSlugMaxLen = 60

// IssueSlug derives the canonical slug for an `issue` artifact from a
// free-form one-liner. The algorithm:
//
//  1. Lowercase the input (Unicode-aware via strings.ToLower).
//  2. Replace every non-alphanumeric rune with `-`.
//  3. Collapse consecutive `-` runs into a single `-`.
//  4. Trim leading and trailing `-`.
//  5. If the result exceeds 60 chars, truncate at the last `-` boundary
//     at or before index 60 (exclusive of the `-`). If no `-` exists
//     within the first 60 chars, hard-truncate at 60.
//
// The truncation rule pins to the AC
// `cli/spec/lint/issue-rules#ac:slug-helper-truncation`.
func IssueSlug(s string) string {
	s = strings.ToLower(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	out := b.String()

	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")

	if len(out) <= issueSlugMaxLen {
		return out
	}

	// Truncate at the last `-` whose index is < issueSlugMaxLen so the
	// final character of the returned slug sits at index ≤ 59 and the
	// result is ≤ 60 chars overall. If no `-` exists in that window,
	// hard-truncate.
	cut := strings.LastIndex(out[:issueSlugMaxLen], "-")
	if cut <= 0 {
		return out[:issueSlugMaxLen]
	}
	return out[:cut]
}
