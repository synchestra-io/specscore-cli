package idearelocate

import (
	"regexp"
	"strings"
)

// rewrite.go houses the in-file substitution helper used by the
// relocate verb's mutation phase (Task 3 of the implementation Plan).
// Two rules per cli/idea/relocate#req:file-copy-with-rewrite:
//
//  1. Every "synchestra-io/" prefix is rewritten to "specscore/" (org
//     rename from the pre-2026 era). Applied globally — code blocks
//     and tables included.
//  2. Every word-bounded "this repo" in body prose is rewritten to the
//     target repo's slug. Occurrences inside fenced code blocks
//     (```), inline code spans (`...`), and table-cell lines (any
//     line containing `|`) are left untouched per the verb spec.
//
// The line-by-line scanner is intentionally coarse: a fence is any
// line whose leading non-whitespace is "```", and a table line is any
// line containing a `|`. These heuristics match the SpecScore artifact
// corpus and the verb's stated "disambiguation in code/table contexts
// is the user's responsibility post-relocate" rationale.

const (
	orgRenameOld = "synchestra-io/"
	orgRenameNew = "specscore/"
)

var (
	// thisRepoRe matches "this repo" with word boundaries on both sides,
	// case-insensitive.
	thisRepoRe = regexp.MustCompile(`(?i)\bthis repo\b`)

	// fenceRe matches a fenced-code-block delimiter line. Leading
	// whitespace is permitted; the fence must start with three backticks.
	fenceRe = regexp.MustCompile("^\\s*```")
)

// RewriteBody applies the two cross-repo-relocate substitution rules to
// content. targetRepo is the value of project.repo from the target's
// specscore.yaml — it replaces every word-bounded "this repo" occurrence
// in body prose.
func RewriteBody(content, targetRepo string) string {
	// Step 1: org rename. Global, no carve-outs.
	content = strings.ReplaceAll(content, orgRenameOld, orgRenameNew)

	// Step 2: "this repo" rewrite, line-by-line with code/table carve-outs.
	var b strings.Builder
	b.Grow(len(content))
	inFence := false
	for _, line := range splitKeepNewlines(content) {
		bare := strings.TrimRight(line, "\n")
		if fenceRe.MatchString(bare) {
			b.WriteString(line)
			inFence = !inFence
			continue
		}
		if inFence {
			b.WriteString(line)
			continue
		}
		if strings.Contains(line, "|") {
			b.WriteString(line)
			continue
		}
		b.WriteString(rewriteThisRepoSkippingInlineCode(line, targetRepo))
	}
	return b.String()
}

// rewriteThisRepoSkippingInlineCode rewrites "this repo" in line,
// skipping inline code spans delimited by single backticks. The line is
// split on backticks; even-indexed segments (0, 2, ...) are outside
// code, odd-indexed segments are inside. Unmatched backticks degenerate
// gracefully: the trailing segment is treated as "inside" and left
// alone.
func rewriteThisRepoSkippingInlineCode(line, targetRepo string) string {
	segs := strings.Split(line, "`")
	for i := range segs {
		if i%2 == 0 {
			segs[i] = thisRepoRe.ReplaceAllString(segs[i], targetRepo)
		}
	}
	return strings.Join(segs, "`")
}

// splitKeepNewlines splits s into lines, preserving each line's trailing
// "\n". A trailing newline-less remainder (the file's last line if it
// has no terminating newline) is emitted as its own element. Joining
// the result via strings.Join(..., "") reconstructs the input byte-
// for-byte.
func splitKeepNewlines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
