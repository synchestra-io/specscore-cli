package property

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// LegalDataTypes mirrors the upstream enumeration in
// [property#req:data-type-values]. The parser surfaces the raw `data_type`
// string from frontmatter; lint compares against this map.
var LegalDataTypes = map[string]bool{
	"string":   true,
	"integer":  true,
	"number":   true,
	"boolean":  true,
	"date":     true,
	"datetime": true,
	"object":   true,
	"array":    true,
	"ref":      true,
}

// CheckKeyApplicability is the (check-key → applicable data_types) matrix
// from [property#req:checks-shape]. The `property-checks-shape` lint rule
// uses this to validate (data_type, check) pairs; keys NOT present here
// are "unknown" and reported at severity `warning`.
var CheckKeyApplicability = map[string]map[string]bool{
	"required":    {"string": true, "integer": true, "number": true, "boolean": true, "date": true, "datetime": true, "object": true, "array": true, "ref": true},
	"enum":        {"string": true, "integer": true, "number": true, "boolean": true, "date": true, "datetime": true, "object": true, "array": true, "ref": true},
	"min":         {"integer": true, "number": true, "date": true, "datetime": true},
	"max":         {"integer": true, "number": true, "date": true, "datetime": true},
	"min_length":  {"string": true, "array": true},
	"max_length":  {"string": true, "array": true},
	"pattern":     {"string": true},
	"trim":        {"string": true},
	"lowercase":   {"string": true},
	"uppercase":   {"string": true},
	"items":       {"array": true},
	"json_schema": {"object": true},
	"entity_ref":  {"ref": true},
}

// slugRe matches a valid property slug per
// [property#req:slug-format] — lowercase, hyphen-separated, URL-safe.
var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug returns nil if slug matches `[a-z0-9]+(-[a-z0-9]+)*`.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("slug %q does not match [a-z0-9]+(-[a-z0-9]+)*", slug)
	}
	return nil
}

// Frontmatter mirrors [property#req:frontmatter-required-fields]. The parser
// fills these fields from a decoded YAML map; lint MUST verify required-ness
// and value legality (the parser surfaces values raw).
type Frontmatter struct {
	Kind        string         // MUST equal "property" — lint checks
	ID          string         // MUST equal Doc.Slug — lint checks
	DataType    string         // one of LegalDataTypes — lint checks
	Description string         // OPTIONAL but RECOMMENDED
	Checks      map[string]any // the raw mapping; MAY be empty but key MUST exist
}

// Section captures a single `## <title>` heading and its body.
type Section struct {
	Title     string
	StartLine int // 1-based line number of the `##` heading
	EndLine   int // 1-based line number of the last line of the section
	Body      string
	Items     []string // lines starting with "- "
}

// Doc is a parsed Property artifact. Parse is resilient — Doc is non-nil
// even for malformed files; Frontmatter and FmRaw may be nil when the
// frontmatter block is missing or not the first block in the file.
type Doc struct {
	Path           string
	Slug           string // derived from filename; the contract id
	RawLines       []string
	Title          string // full title line content after `# `
	TitleName      string // the `<id>` portion after `Property: `
	TitleLine      int    // 1-based
	HasTitle       bool
	Frontmatter    *Frontmatter
	FmRaw          *yaml.Node // raw mapping node — used by `id`-rewrite autofix
	Sections       []Section
	SectionByTitle map[string]*Section
}

// Parse reads a property file and returns its parsed representation.
//
// Parse is resilient: a non-nil *Doc is returned for every readable file,
// even when the frontmatter is missing, not first, or malformed. Callers
// (lint rules) MUST inspect Doc.Frontmatter / Doc.HasTitle / etc. to decide
// what is a violation. The only path that returns (nil, err) is an I/O
// failure opening or reading the file.
//
// `Path` and `Slug` are always populated. `RawLines` is preserved byte-
// for-byte so the managed-section rewriter can edit specific line ranges
// without losing surrounding content. `FmRaw` is preserved as a `yaml.Node`
// so the `id`-rewrite autofix can round-trip frontmatter without losing
// comments or key order.
func Parse(path string) (*Doc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	doc := &Doc{
		Path:           path,
		SectionByTitle: make(map[string]*Section),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	doc.RawLines = lines

	// Slug derives from the filename only — the filename is authoritative
	// per [property#req:id-equals-slug].
	base := filepath.Base(path)
	doc.Slug = strings.TrimSuffix(base, ".property.md")

	// Parse the frontmatter only when it is the first non-empty content of
	// the file. A frontmatter block that appears later in the file MUST NOT
	// be picked up; lint's property-frontmatter-required rule fires the
	// diagnostic.
	bodyStart := parseFrontmatter(doc, lines)

	// Parse title + sections from the post-frontmatter body.
	parseBody(doc, lines, bodyStart)

	return doc, nil
}

// parseFrontmatter scans `lines` for an opening `---` on the first non-empty
// line and a matching closing `---`. On success, fills doc.Frontmatter and
// doc.FmRaw and returns the index AFTER the closing delimiter (so body
// parsing starts there). On failure, returns 0 — body parsing covers the
// whole file.
func parseFrontmatter(doc *Doc, lines []string) int {
	// Find the first non-empty line.
	first := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			first = i
			break
		}
	}
	if first < 0 || strings.TrimSpace(lines[first]) != "---" {
		return 0
	}
	// Find the matching closing delimiter.
	end := -1
	for j := first + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			end = j
			break
		}
	}
	if end < 0 {
		return 0
	}

	raw := strings.Join(lines[first+1:end], "\n")
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		// Malformed YAML — parser stays resilient, surfaces nothing.
		return end + 1
	}
	fm := decodeFrontmatter(&node)
	if fm != nil {
		doc.Frontmatter = fm
		doc.FmRaw = &node
	}
	return end + 1
}

// decodeFrontmatter extracts the typed Frontmatter fields from a parsed
// YAML DocumentNode. Returns nil if the document is empty or its root is
// not a mapping.
func decodeFrontmatter(node *yaml.Node) *Frontmatter {
	root := node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil
	}
	fm := &Frontmatter{
		Checks: map[string]any{},
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		val := root.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		switch key.Value {
		case "kind":
			if val.Kind == yaml.ScalarNode {
				fm.Kind = val.Value
			}
		case "id":
			if val.Kind == yaml.ScalarNode {
				fm.ID = val.Value
			}
		case "data_type":
			if val.Kind == yaml.ScalarNode {
				fm.DataType = val.Value
			}
		case "description":
			if val.Kind == yaml.ScalarNode {
				fm.Description = val.Value
			}
		case "checks":
			fm.Checks = decodeChecks(val)
		}
	}
	return fm
}

// yamlNodeDecodeFn is the seam used by decodeChecks. Tests swap it to
// inject a Decode failure that the real yaml.Node API does not surface
// for the shape we always pass.
var yamlNodeDecodeFn = func(n *yaml.Node, out interface{}) error { return n.Decode(out) }

// decodeChecks turns a YAML mapping node into a map[string]any. Non-mapping
// values (including the `checks: ~` shorthand) yield an empty, non-nil map
// so callers can distinguish "missing key" (Frontmatter == nil) from
// "empty mapping" (Frontmatter.Checks == map{}).
func decodeChecks(node *yaml.Node) map[string]any {
	out := map[string]any{}
	if node == nil || node.Kind != yaml.MappingNode {
		return out
	}
	if err := yamlNodeDecodeFn(node, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// parseBody walks `lines[startIdx:]` to extract the `# Property: <id>` title
// line and every `## <heading>` section.
func parseBody(doc *Doc, lines []string, startIdx int) {
	type rawSection struct {
		title string
		start int
		end   int
	}
	var raws []rawSection

	for i := startIdx; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		if !doc.HasTitle && strings.HasPrefix(trimmed, "# ") {
			doc.HasTitle = true
			doc.TitleLine = i + 1
			doc.Title = strings.TrimPrefix(trimmed, "# ")
			if name, ok := strings.CutPrefix(doc.Title, "Property: "); ok {
				doc.TitleName = strings.TrimSpace(name)
			}
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if len(raws) > 0 {
				raws[len(raws)-1].end = i - 1
			}
			raws = append(raws, rawSection{title: title, start: i})
			continue
		}
	}
	if len(raws) > 0 {
		raws[len(raws)-1].end = len(lines) - 1
	}

	for _, r := range raws {
		body := ""
		if r.end >= r.start {
			bodyLines := lines[r.start+1 : r.end+1]
			body = strings.Join(bodyLines, "\n")
		}
		var items []string
		for _, bl := range strings.Split(body, "\n") {
			t := strings.TrimSpace(bl)
			if strings.HasPrefix(t, "- ") {
				items = append(items, strings.TrimPrefix(t, "- "))
			}
		}
		sec := Section{
			Title:     r.title,
			StartLine: r.start + 1,
			EndLine:   r.end + 1,
			Body:      body,
			Items:     items,
		}
		doc.Sections = append(doc.Sections, sec)
	}
	for i := range doc.Sections {
		s := &doc.Sections[i]
		doc.SectionByTitle[s.Title] = s
	}
}
