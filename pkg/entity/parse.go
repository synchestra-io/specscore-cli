package entity

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// slugRe matches a valid entity slug (mirrors pkg/idea.slugRe).
var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug returns nil if slug matches `[a-z0-9]+(-[a-z0-9]+)*` per
// [entity#req:slug-format].
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("slug %q does not match [a-z0-9]+(-[a-z0-9]+)*", slug)
	}
	return nil
}

// PropertyItem is a single entry in the entity's frontmatter `properties`
// list. Exactly one of (DataType, Ref) is non-empty in a valid item;
// Parse leaves both empty when neither is present (lint flags this).
type PropertyItem struct {
	Name        string // required
	DataType    string // inline form
	Ref         string // reference form (path or URL)
	Description string
	Checks      map[string]any // free-form per [property#req:checks-shape]
}

// Frontmatter mirrors the typed fields from
// [entity#req:frontmatter-required-fields].
type Frontmatter struct {
	Kind        string // MUST equal "entity"
	ID          string // MUST equal Doc.Slug
	Singular    string
	Plural      string
	Description string
	Inherits    string // optional path or URL; "" when absent
	Properties  []PropertyItem
	// Extras carries any frontmatter keys not listed above so that
	// forward-compatibility (per [entity#req:frontmatter-required-fields]
	// "Additional keys MUST NOT be a lint error in MVP") is preserved.
	Extras map[string]any
}

// Section captures a single `## ` heading and its body. Mirrors
// pkg/idea.Section so lint rules that scan sections by name (## Description,
// ## Properties, ## Referenced by) can share helper code.
type Section struct {
	Title     string
	StartLine int
	EndLine   int
	Body      string
	Items     []string
}

// Doc is a parsed entity file. The struct is intentionally permissive —
// Parse returns a partial Doc even when the file is malformed, so lint
// rules can report every issue they find rather than bailing on the first.
type Doc struct {
	Path           string
	Slug           string
	RawLines       []string // body lines for managed-section rewriting
	Title          string   // full `# Entity: ...` title line, "" if absent
	TitleName      string   // the `<singular>` portion after "Entity: "
	TitleOK        bool     // true if Title matches "# Entity: <Name>"
	TitleLine      int      // 1-based line number of the title
	HasTitle       bool
	Frontmatter    *Frontmatter // nil when frontmatter is absent or unparseable
	FmRaw          *yaml.Node   // round-trippable node for --fix mutations
	Sections       []Section
	SectionByTitle map[string]*Section
	Properties     []PropertyItem // mirrors Frontmatter.Properties for convenience
}

// titleRe matches a "# Entity: <name>" title line.
var titleRe = regexp.MustCompile(`^#\s+Entity:\s*(.+?)\s*$`)

// Parse reads the file at `path` and returns a Doc. Parse is resilient:
// the returned Doc is partial when frontmatter is missing or malformed.
// The caller (lint) decides which issues are violations.
//
// Parse returns a non-nil error only for I/O failures (file not found,
// permission denied). YAML parse errors and structural issues do NOT
// surface as Go errors — they manifest as nil Frontmatter or empty
// fields on the returned Doc.
func Parse(path string) (*Doc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	doc := &Doc{
		Path:           path,
		Slug:           strings.TrimSuffix(baseName(path), EntitySuffix),
		RawLines:       lines,
		SectionByTitle: make(map[string]*Section),
	}

	// Frontmatter must be the very first non-empty content of the file
	// per [entity#req:frontmatter-required]. We scan for a leading "---"
	// line; if anything other than blank lines precedes it, we leave
	// Frontmatter nil so lint can flag the violation.
	bodyStart := 0
	if fmEnd, fmLines, ok := extractLeadingFrontmatter(lines); ok {
		doc.Frontmatter, doc.FmRaw = parseFrontmatter(fmLines)
		if doc.Frontmatter != nil {
			doc.Properties = doc.Frontmatter.Properties
		}
		bodyStart = fmEnd + 1
	}

	// Walk the body lines: find the title and ## sections.
	type rawSection struct {
		title string
		start int
		end   int
	}
	var raws []rawSection
	for i := bodyStart; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if !doc.HasTitle && strings.HasPrefix(trimmed, "# ") {
			doc.HasTitle = true
			doc.TitleLine = i + 1
			doc.Title = trimmed
			if m := titleRe.FindStringSubmatch(trimmed); m != nil {
				doc.TitleOK = true
				doc.TitleName = strings.TrimSpace(m[1])
			}
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if len(raws) > 0 {
				raws[len(raws)-1].end = i - 1
			}
			raws = append(raws, rawSection{title: title, start: i})
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

	return doc, nil
}

// extractLeadingFrontmatter returns the body-start index, the
// frontmatter content lines (excluding the delimiter rows), and a bool
// indicating whether a leading frontmatter block was found.
//
// "Leading" means: blank lines MAY precede the opening "---", but no
// other content. Per [entity#req:frontmatter-required], an entity whose
// first non-blank content is not "---" has no frontmatter — and we
// signal that by returning ok=false so lint can flag the violation.
func extractLeadingFrontmatter(lines []string) (endIdx int, content []string, ok bool) {
	openIdx := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.TrimSpace(ln) == "---" {
			openIdx = i
		}
		break
	}
	if openIdx == -1 {
		return 0, nil, false
	}
	for j := openIdx + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			return j, lines[openIdx+1 : j], true
		}
	}
	// Unclosed frontmatter — treat as absent.
	return 0, nil, false
}

// parseFrontmatter unmarshals the YAML content lines into a typed
// Frontmatter and preserves the raw yaml.Node for round-trip rewrites.
// Returns (nil, nil) when YAML parsing fails — lint surfaces the
// failure as a frontmatter-required-fields violation.
func parseFrontmatter(content []string) (*Frontmatter, *yaml.Node) {
	raw := strings.Join(content, "\n")
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &node); err != nil {
		return nil, nil
	}
	if node.Kind == 0 {
		return nil, nil
	}
	// The top-level node is a DocumentNode wrapping a MappingNode in
	// the well-formed case.
	mapping := &node
	if mapping.Kind == yaml.DocumentNode && len(mapping.Content) > 0 {
		mapping = mapping.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return nil, nil
	}

	fm := &Frontmatter{Extras: map[string]any{}}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		switch keyNode.Value {
		case "kind":
			fm.Kind = valNode.Value
		case "id":
			fm.ID = valNode.Value
		case "singular":
			fm.Singular = valNode.Value
		case "plural":
			fm.Plural = valNode.Value
		case "description":
			fm.Description = valNode.Value
		case "inherits":
			fm.Inherits = valNode.Value
		case "properties":
			fm.Properties = parsePropertiesList(valNode)
		default:
			var raw any
			_ = valNode.Decode(&raw)
			fm.Extras[keyNode.Value] = raw
		}
	}
	return fm, &node
}

// parsePropertiesList walks a sequence node of property items and
// returns the typed slice. Items that are not mappings are skipped
// (lint will already flag the structural issue).
func parsePropertiesList(seq *yaml.Node) []PropertyItem {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]PropertyItem, 0, len(seq.Content))
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		pi := PropertyItem{}
		for i := 0; i+1 < len(item.Content); i += 2 {
			k := item.Content[i].Value
			v := item.Content[i+1]
			switch k {
			case "name":
				pi.Name = v.Value
			case "data_type":
				pi.DataType = v.Value
			case "ref":
				pi.Ref = v.Value
			case "description":
				pi.Description = v.Value
			case "checks":
				var checks map[string]any
				if err := v.Decode(&checks); err == nil {
					pi.Checks = checks
				}
			}
		}
		out = append(out, pi)
	}
	return out
}

// baseName mirrors filepath.Base for a path string without forcing the
// import on every call site that wants the filename stem.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}
