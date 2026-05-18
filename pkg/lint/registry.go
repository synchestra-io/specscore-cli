package lint

import "strings"

// parseConsumerPath splits a document-types-registry "Consumer Path" cell
// value into the list of globs it represents. The cell is comma-separated;
// whitespace around commas is tolerated; empty entries (leading commas,
// trailing commas, or doubled internal commas like "a,,b") are discarded
// silently regardless of position. The dash placeholder "—", the ASCII
// hyphen "-", an empty cell, or a whitespace-only cell each yield a nil
// slice — never an error.
//
// The parser does NOT compile the globs — it only splits the cell into
// the canonical list of glob strings. Glob matching is the caller's job.
//
// Contract: [cli/spec/lint#req:consumer-path-multi-glob],
//
//	[cli/spec/lint#ac:consumer-path-multi-glob-parsed].
func parseConsumerPath(cell string) []string {
	v := strings.TrimSpace(cell)
	if v == "" || v == "—" || v == "-" {
		return nil
	}
	parts := strings.Split(v, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
