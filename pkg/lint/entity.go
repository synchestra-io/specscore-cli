package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/property"
)

// entityChecker is a dispatch checker that runs every entity-* rule in
// one pass per file so parsing is shared and cross-file rules (e.g.,
// entity-inherits-acyclic, entity-ref-target-exists,
// entity-inherits-target-exists) can share an index built once.
//
// Under --fix, entityChecker also owns the managed-section rewriter:
// the ## Properties table and the ## Referenced by section in every
// entity file are computed once (phase 1) and written to disk only
// after every file has been scanned (phase 2), satisfying
// [cli/entity#req:fix-write-ordering].
type entityChecker struct {
	autofix bool
}

func newEntityChecker() *entityChecker {
	return &entityChecker{}
}

// entityRuleNames is the canonical list of entity-* rule names this
// checker emits. Every name here MUST also appear in allRuleNames.
var entityRuleNames = []string{
	"entity-location",
	"entity-slug-format",
	"entity-single-file",
	"entity-frontmatter-required",
	"entity-frontmatter-required-fields",
	"entity-id-equals-slug",
	"entity-properties-list-shape",
	"entity-ref-target-exists",
	"entity-inherits-additive-only",
	"entity-inherits-target-exists",
	"entity-inherits-acyclic",
	"entity-title-format",
	"entity-required-sections",
	"entity-properties-table-managed",
	"entity-referenced-by-managed",
}

// Required body sections, in order, per
// [entity#req:required-sections].
var entityRequiredSections = []string{
	"Description", "Properties", "Referenced by",
}

func (c *entityChecker) name() string     { return "entity-location" }
func (c *entityChecker) severity() string { return "error" }

// check runs all entity-* rules. Under c.autofix, the managed-section
// rewriter mutates files in two phases (compute, then write) so adding
// a child entity in one pass refreshes the parent's ## Referenced by in
// the same pass (idempotency contract).
func (c *entityChecker) check(specRoot string) ([]Violation, error) {
	var violations []Violation

	// 1. Detect misplaced .entity.md files (entity-location).
	misplaced, err := findMisplacedEntityFiles(specRoot)
	if err != nil {
		return nil, err
	}
	for _, f := range misplaced {
		rel, _ := filepath.Rel(specRoot, f)
		violations = append(violations, Violation{
			File: rel, Line: 0, Severity: "error",
			Rule:    "entity-location",
			Message: fmt.Sprintf("entity files must live under spec/features/**/<slug>.entity.md; got %s", rel),
		})
	}

	// 2. Detect <slug>.entity directories (entity-single-file).
	dirs, err := findEntityDirectories(specRoot)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		rel, _ := filepath.Rel(specRoot, d)
		violations = append(violations, Violation{
			File: rel, Line: 0, Severity: "error",
			Rule:    "entity-single-file",
			Message: fmt.Sprintf("entities must be single markdown files; directory found at %s", rel),
		})
	}

	// 3. Discover and parse entity files under spec/features/**.
	discovered, err := entity.Discover(specRoot)
	if err != nil {
		return nil, err
	}
	parsed := make([]*entity.Doc, 0, len(discovered))
	bySlug := make(map[string]*entity.Doc, len(discovered))
	for _, d := range discovered {
		doc, perr := entity.Parse(d.Path)
		if perr != nil {
			rel, _ := filepath.Rel(specRoot, d.Path)
			violations = append(violations, Violation{
				File: rel, Severity: "error",
				Rule:    "entity-location",
				Message: fmt.Sprintf("cannot read entity file: %v", perr),
			})
			continue
		}
		parsed = append(parsed, doc)
		bySlug[d.Slug] = doc
	}

	// 4. Discover properties (for ref resolution + properties-table rendering).
	propsDiscovered, err := property.Discover(specRoot)
	if err != nil {
		return nil, err
	}
	propByPath := make(map[string]*property.Doc, len(propsDiscovered))
	for _, p := range propsDiscovered {
		pdoc, perr := property.Parse(p.Path)
		if perr != nil {
			continue
		}
		abs, aerr := filepath.Abs(p.Path)
		if aerr != nil {
			abs = p.Path
		}
		propByPath[abs] = pdoc
	}

	// 5. Per-file rules.
	for _, doc := range parsed {
		rel, _ := filepath.Rel(specRoot, doc.Path)
		vs := entityFileRules(doc, rel, specRoot, bySlug, propByPath)
		violations = append(violations, vs...)
	}

	// 6. Cycle detection (entity-inherits-acyclic) across the inheritance graph.
	violations = append(violations, entityInheritsCycleRules(specRoot, parsed)...)

	// 7. Compute descendants for every entity (inherits-based back-refs).
	descendantsByParent := buildInheritsBackrefs(specRoot, parsed)

	// 8. Managed-section content checks (entity-properties-table-managed,
	// entity-referenced-by-managed). Under --fix the rewriter writes
	// every changed file in phase 2 so the second pass is idempotent.
	if c.autofix {
		type pendingEdit struct {
			path    string
			content []byte
		}
		var edits []pendingEdit
		for _, doc := range parsed {
			if doc.Frontmatter == nil {
				continue
			}
			propsBody := renderPropertiesTable(doc, specRoot, bySlug, propByPath)
			refsBody := renderEntityReferencedBy(doc, specRoot, descendantsByParent[doc.Slug])
			newContent, changed, ferr := applyManagedRewrites(doc, propsBody, refsBody)
			if ferr != nil {
				rel, _ := filepath.Rel(specRoot, doc.Path)
				violations = append(violations, Violation{
					File: rel, Severity: "error",
					Rule:    "entity-properties-table-managed",
					Message: fmt.Sprintf("cannot rewrite managed section: %v", ferr),
				})
				continue
			}
			if changed {
				edits = append(edits, pendingEdit{path: doc.Path, content: newContent})
			}
		}
		// Also apply id-equals-slug autofix (separate from managed rewrites).
		// We compute the file's new content for entities where id != slug
		// using yaml.Node round-trip on doc.FmRaw.
		for _, doc := range parsed {
			if doc.Frontmatter == nil {
				continue
			}
			if doc.Frontmatter.ID == doc.Slug {
				continue
			}
			content, changed, ferr := applyIDEqualsSlugFix(doc)
			if ferr != nil || !changed {
				continue
			}
			// Merge with any existing pending edit for the same path.
			merged := false
			for i, e := range edits {
				if e.path == doc.Path {
					edits[i].content = content
					merged = true
					break
				}
			}
			if !merged {
				edits = append(edits, pendingEdit{path: doc.Path, content: content})
			}
		}
		// Also apply title-format autofix.
		for _, doc := range parsed {
			if doc.Frontmatter == nil {
				continue
			}
			if !doc.HasTitle {
				continue
			}
			expected := "# Entity: " + doc.Frontmatter.Singular
			if doc.Title == expected {
				continue
			}
			// Pull the latest pending content if any (so multiple fixes compose).
			var src []byte
			merged := false
			for i, e := range edits {
				if e.path == doc.Path {
					src = e.content
					merged = true
					_ = i
					break
				}
			}
			if src == nil {
				raw, rerr := os.ReadFile(doc.Path)
				if rerr != nil {
					continue
				}
				src = raw
			}
			newSrc, changed := rewriteEntityTitle(src, doc.Frontmatter.Singular)
			if !changed {
				continue
			}
			if merged {
				for i, e := range edits {
					if e.path == doc.Path {
						edits[i].content = newSrc
						break
					}
				}
			} else {
				edits = append(edits, pendingEdit{path: doc.Path, content: newSrc})
			}
		}
		// Phase 2: write every pending edit.
		for _, e := range edits {
			if werr := os.WriteFile(e.path, e.content, 0o644); werr != nil {
				rel, _ := filepath.Rel(specRoot, e.path)
				violations = append(violations, Violation{
					File: rel, Severity: "error",
					Rule:    "entity-properties-table-managed",
					Message: fmt.Sprintf("cannot write rewritten file: %v", werr),
				})
			}
		}
		// After write, drop any reports for managed-section drift /
		// id-equals-slug / title-format since they've been repaired.
		violations = filterAutofixedEntityViolations(violations, parsed)
	} else {
		// Without --fix, emit managed-section drift as violations.
		for _, doc := range parsed {
			if doc.Frontmatter == nil {
				continue
			}
			rel, _ := filepath.Rel(specRoot, doc.Path)
			expectedProps := renderPropertiesTable(doc, specRoot, bySlug, propByPath)
			expectedRefs := renderEntityReferencedBy(doc, specRoot, descendantsByParent[doc.Slug])
			if drift := managedSectionDrift(doc, "Properties", expectedProps); drift {
				violations = append(violations, Violation{
					File: rel, Severity: "error",
					Rule:    "entity-properties-table-managed",
					Message: "## Properties managed body does not match canonical rendering (run `specscore spec lint --fix`)",
				})
			}
			if drift := managedSectionDrift(doc, "Referenced by", expectedRefs); drift {
				violations = append(violations, Violation{
					File: rel, Severity: "error",
					Rule:    "entity-referenced-by-managed",
					Message: "## Referenced by managed body does not match canonical rendering (run `specscore spec lint --fix`)",
				})
			}
		}
	}

	return violations, nil
}

// entityFileRules returns violations for a single parsed entity file that
// do not depend on cross-file state beyond the bySlug + propByPath maps.
func entityFileRules(doc *entity.Doc, rel, specRoot string, bySlug map[string]*entity.Doc, propByPath map[string]*property.Doc) []Violation {
	var vs []Violation

	// entity-slug-format.
	if err := entity.ValidateSlug(doc.Slug); err != nil {
		vs = append(vs, Violation{
			File: rel, Severity: "error",
			Rule:    "entity-slug-format",
			Message: err.Error(),
		})
	}

	// entity-frontmatter-required.
	if doc.Frontmatter == nil {
		vs = append(vs, Violation{
			File: rel, Line: 1, Severity: "error",
			Rule:    "entity-frontmatter-required",
			Message: "entity file MUST begin with a YAML frontmatter block delimited by `---`",
		})
		// Without a frontmatter we can't run the rest of the rules.
		return vs
	}

	// entity-frontmatter-required-fields.
	var missing []string
	if doc.Frontmatter.Kind != "entity" {
		missing = append(missing, "kind")
	}
	if doc.Frontmatter.ID == "" {
		missing = append(missing, "id")
	}
	if doc.Frontmatter.Singular == "" {
		missing = append(missing, "singular")
	}
	if doc.Frontmatter.Plural == "" {
		missing = append(missing, "plural")
	}
	// "properties" is a required key but may be empty. We detect presence
	// via Extras: if "properties" is absent the parser left .Properties as
	// nil AND did not put it in Extras (parser doesn't touch unknown keys).
	// The parser unconditionally sets .Properties to nil or a slice — so
	// we treat nil-AND-no-entry-in-raw as "missing".
	if doc.Frontmatter.Properties == nil && !frontmatterHasKey(doc, "properties") {
		missing = append(missing, "properties")
	}
	if len(missing) > 0 {
		vs = append(vs, Violation{
			File: rel, Severity: "error",
			Rule:    "entity-frontmatter-required-fields",
			Message: fmt.Sprintf("missing required frontmatter field(s): %s", strings.Join(missing, ", ")),
		})
	}

	// entity-id-equals-slug.
	if doc.Frontmatter.ID != "" && doc.Frontmatter.ID != doc.Slug {
		vs = append(vs, Violation{
			File: rel, Severity: "error",
			Rule:    "entity-id-equals-slug",
			Message: fmt.Sprintf("frontmatter id %q must equal filename slug %q", doc.Frontmatter.ID, doc.Slug),
		})
	}

	// entity-title-format.
	if !doc.HasTitle {
		vs = append(vs, Violation{
			File: rel, Line: 1, Severity: "error",
			Rule:    "entity-title-format",
			Message: "missing title (expected `# Entity: <singular>`)",
		})
	} else if !doc.TitleOK {
		vs = append(vs, Violation{
			File: rel, Line: doc.TitleLine, Severity: "error",
			Rule:    "entity-title-format",
			Message: "title must use `# Entity: <singular>` format",
		})
	} else if doc.Frontmatter.Singular != "" && doc.TitleName != doc.Frontmatter.Singular {
		vs = append(vs, Violation{
			File: rel, Line: doc.TitleLine, Severity: "error",
			Rule:    "entity-title-format",
			Message: fmt.Sprintf("title `# Entity: %s` does not match frontmatter singular %q", doc.TitleName, doc.Frontmatter.Singular),
		})
	}

	// entity-properties-list-shape: name required + unique; data_type OR ref required.
	nameSeen := map[string]bool{}
	for i, p := range doc.Frontmatter.Properties {
		if p.Name == "" {
			vs = append(vs, Violation{
				File: rel, Severity: "error",
				Rule:    "entity-properties-list-shape",
				Message: fmt.Sprintf("property item #%d is missing required `name` key", i+1),
			})
			continue
		}
		if nameSeen[p.Name] {
			vs = append(vs, Violation{
				File: rel, Severity: "error",
				Rule:    "entity-properties-list-shape",
				Message: fmt.Sprintf("duplicate property name %q (each `name` must be unique within an entity)", p.Name),
			})
		}
		nameSeen[p.Name] = true
		if p.DataType == "" && p.Ref == "" {
			vs = append(vs, Violation{
				File: rel, Severity: "error",
				Rule:    "entity-properties-list-shape",
				Message: fmt.Sprintf("property %q must declare either `data_type` (inline) or `ref:` (reference)", p.Name),
			})
		}
	}

	// entity-ref-target-exists.
	for _, p := range doc.Frontmatter.Properties {
		if p.Ref == "" {
			continue
		}
		// URLs are tolerated (handled by entity.ResolveRef returning empty path).
		if strings.HasPrefix(p.Ref, "http://") || strings.HasPrefix(p.Ref, "https://") {
			continue
		}
		resolved, _, _ := entity.ResolveRef(specRoot, doc.Path, p.Ref)
		if resolved == "" {
			continue
		}
		if _, ok := propByPath[resolved]; ok {
			continue
		}
		// Also accept on-disk existence (in case the property file lives
		// outside the standard discovery scope).
		if _, statErr := os.Stat(resolved); statErr == nil {
			continue
		}
		vs = append(vs, Violation{
			File: rel, Severity: "error",
			Rule:    "entity-ref-target-exists",
			Message: fmt.Sprintf("property %q ref %q does not resolve to an existing *.property.md file", p.Name, p.Ref),
		})
	}

	// entity-inherits-target-exists.
	if doc.Frontmatter.Inherits != "" {
		raw := doc.Frontmatter.Inherits
		if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
			resolved, _, _ := entity.ResolveInherits(specRoot, doc.Path, raw)
			if resolved != "" {
				if _, statErr := os.Stat(resolved); statErr != nil {
					vs = append(vs, Violation{
						File: rel, Severity: "error",
						Rule:    "entity-inherits-target-exists",
						Message: fmt.Sprintf("inherits target %q does not resolve to an existing *.entity.md file", raw),
					})
				}
			}
		}
	}

	// entity-inherits-additive-only.
	if doc.Frontmatter.Inherits != "" {
		parentDoc := resolveInheritsToDoc(doc, specRoot, bySlug)
		if parentDoc != nil && parentDoc.Frontmatter != nil {
			parentNames := map[string]bool{}
			for _, pp := range parentDoc.Frontmatter.Properties {
				if pp.Name != "" {
					parentNames[pp.Name] = true
				}
			}
			for _, pp := range doc.Frontmatter.Properties {
				if parentNames[pp.Name] {
					vs = append(vs, Violation{
						File: rel, Severity: "error",
						Rule:    "entity-inherits-additive-only",
						Message: fmt.Sprintf("child entity redeclares parent property %q; inheritance is additive-only", pp.Name),
					})
				}
			}
		}
	}

	// entity-required-sections.
	have := map[string]bool{}
	for _, s := range doc.Sections {
		have[s.Title] = true
	}
	var missingSections []string
	for _, name := range entityRequiredSections {
		if !have[name] {
			missingSections = append(missingSections, name)
		}
	}
	if len(missingSections) > 0 {
		vs = append(vs, Violation{
			File: rel, Severity: "error",
			Rule:    "entity-required-sections",
			Message: fmt.Sprintf("missing required section(s): %s", strings.Join(missingSections, ", ")),
		})
	}

	return vs
}

// entityInheritsCycleRules detects cycles in the inheritance graph and
// reports an entity-inherits-acyclic violation for every entity that
// participates in a cycle. Lint MUST terminate safely rather than recurse.
func entityInheritsCycleRules(specRoot string, parsed []*entity.Doc) []Violation {
	var vs []Violation

	// Build absolute-path -> doc map.
	byAbsPath := make(map[string]*entity.Doc, len(parsed))
	for _, d := range parsed {
		abs, err := filepath.Abs(d.Path)
		if err == nil {
			byAbsPath[abs] = d
		}
	}

	// For each doc, walk the inheritance chain up to a fixed bound; a
	// repeated visit signals a cycle.
	for _, start := range parsed {
		if start.Frontmatter == nil || start.Frontmatter.Inherits == "" {
			continue
		}
		visited := map[string]bool{}
		current := start
		for {
			abs, _ := filepath.Abs(current.Path)
			if visited[abs] {
				// Cycle detected — report against the entity that
				// triggered detection.
				rel, _ := filepath.Rel(specRoot, start.Path)
				vs = append(vs, Violation{
					File: rel, Severity: "error",
					Rule:    "entity-inherits-acyclic",
					Message: "inheritance chain contains a cycle",
				})
				break
			}
			visited[abs] = true
			if current.Frontmatter == nil || current.Frontmatter.Inherits == "" {
				break
			}
			parent := resolveInheritsToDoc(current, specRoot, nil)
			if parent == nil {
				// Try resolving via byAbsPath.
				resolved, _, _ := entity.ResolveInherits(specRoot, current.Path, current.Frontmatter.Inherits)
				if resolved == "" {
					break
				}
				p, ok := byAbsPath[resolved]
				if !ok {
					break
				}
				parent = p
			}
			current = parent
		}
	}

	return vs
}

// resolveInheritsToDoc returns the parent entity Doc for the child's
// inherits: value, or nil if it cannot be resolved.
func resolveInheritsToDoc(child *entity.Doc, specRoot string, bySlug map[string]*entity.Doc) *entity.Doc {
	if child.Frontmatter == nil || child.Frontmatter.Inherits == "" {
		return nil
	}
	raw := child.Frontmatter.Inherits
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return nil
	}
	resolved, _, _ := entity.ResolveInherits(specRoot, child.Path, raw)
	if resolved == "" {
		return nil
	}
	// Match by absolute path. A nil map iterates zero times, so we don't
	// need an explicit nil check.
	for _, d := range bySlug {
		abs, _ := filepath.Abs(d.Path)
		if abs == resolved {
			return d
		}
	}
	return nil
}

// buildInheritsBackrefs returns a map slug -> sorted slice of child entity
// Docs that inherit from that slug. Sort order: child id, then relative
// path (tie-break) per [cli/entity#req:referenced-by-from-inheritance].
func buildInheritsBackrefs(specRoot string, parsed []*entity.Doc) map[string][]*entity.Doc {
	bySlug := make(map[string]*entity.Doc, len(parsed))
	for _, d := range parsed {
		bySlug[d.Slug] = d
	}
	out := make(map[string][]*entity.Doc)
	for _, child := range parsed {
		if child.Frontmatter == nil || child.Frontmatter.Inherits == "" {
			continue
		}
		parent := resolveInheritsToDoc(child, specRoot, bySlug)
		if parent == nil {
			continue
		}
		out[parent.Slug] = append(out[parent.Slug], child)
	}
	for slug, children := range out {
		sort.Slice(children, func(i, j int) bool {
			if children[i].Slug != children[j].Slug {
				return children[i].Slug < children[j].Slug
			}
			return children[i].Path < children[j].Path
		})
		out[slug] = children
	}
	return out
}

// frontmatterHasKey reports whether the raw frontmatter contains key.
// Used to distinguish "properties: []" (present, empty) from "properties
// not declared" (absent → required-fields violation).
func frontmatterHasKey(doc *entity.Doc, key string) bool {
	if doc.FmRaw == nil {
		return false
	}
	root := doc.FmRaw
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == key {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------
// Managed-section rendering + rewriting.
// ---------------------------------------------------------------------

// managedStartMarker and managedEndMarker delimit the managed body
// inside both ## Properties and ## Referenced by sections.
const managedStartMarker = "<!-- managed-by: specscore lint --fix -->"
const managedEndMarker = "<!-- end-managed -->"

// renderPropertiesTable returns the canonical body for an entity's
// ## Properties managed section per
// [cli/entity#req:properties-table-rendered]. The body is the four-
// column pipe-table without the surrounding managed markers — those
// are preserved by applyManagedRewrites.
func renderPropertiesTable(doc *entity.Doc, specRoot string, bySlug map[string]*entity.Doc, propByPath map[string]*property.Doc) string {
	if doc.Frontmatter == nil {
		return ""
	}

	// Resolve inherited properties (prepended in parent's order).
	type expandedItem struct {
		item entity.PropertyItem
		// declaredAt is the path of the entity that DECLARED this item
		// (parent for inherited, doc.Path for own). Used to compute the
		// reference path for ref: items — per the REQ, the ref target's
		// PATH is always resolved against the entity being rendered
		// (doc.Path), but the textual ref string lives in the declaring
		// entity. We carry declaredAt because the *target* file location
		// is computed from the declaring entity's directory; we then
		// re-express it as a path relative to doc.Path.
		declaredAt string
	}
	var ordered []expandedItem
	if doc.Frontmatter.Inherits != "" {
		parent := resolveInheritsToDoc(doc, specRoot, bySlug)
		if parent != nil && parent.Frontmatter != nil {
			for _, p := range parent.Frontmatter.Properties {
				ordered = append(ordered, expandedItem{item: p, declaredAt: parent.Path})
			}
		}
	}
	for _, p := range doc.Frontmatter.Properties {
		ordered = append(ordered, expandedItem{item: p, declaredAt: doc.Path})
	}

	var b strings.Builder
	b.WriteString("| Name | Type | Required | Description |\n")
	b.WriteString("|------|------|----------|-------------|\n")
	for _, e := range ordered {
		p := e.item
		name := fmt.Sprintf("`%s`", p.Name)

		typeCell := "—"
		descCell := "—"
		required := "no"

		if p.Ref != "" {
			// Resolve the ref target to an absolute path using the
			// declaring entity's directory (so an inherited "ref:
			// ../shared/email.property.md" from the parent resolves
			// correctly regardless of where the child file lives).
			var refURL bool
			if strings.HasPrefix(p.Ref, "http://") || strings.HasPrefix(p.Ref, "https://") {
				refURL = true
			}
			var resolved string
			if !refURL {
				resolved, _, _ = entity.ResolveRef(specRoot, e.declaredAt, p.Ref)
			}
			dataType := "—"
			refID := p.Name
			var pdoc *property.Doc
			if resolved != "" {
				if pd, ok := propByPath[resolved]; ok {
					pdoc = pd
				} else {
					// Fall back to parsing on demand.
					if pd, perr := property.Parse(resolved); perr == nil {
						pdoc = pd
					}
				}
			}
			if pdoc != nil && pdoc.Frontmatter != nil {
				if pdoc.Frontmatter.DataType != "" {
					dataType = pdoc.Frontmatter.DataType
				}
				if pdoc.Frontmatter.ID != "" {
					refID = pdoc.Frontmatter.ID
				}
				if pdoc.Frontmatter.Description != "" {
					descCell = pdoc.Frontmatter.Description
				}
				if reqVal, ok := pdoc.Frontmatter.Checks["required"]; ok {
					if b, ok := reqVal.(bool); ok && b {
						required = "yes"
					}
				}
			}
			// Relative path: always from doc.Path's directory to the
			// resolved target. Falls back to the literal ref string if
			// the target couldn't be resolved.
			relPath := p.Ref
			if resolved != "" {
				if rp, rerr := filepath.Rel(filepath.Dir(doc.Path), resolved); rerr == nil {
					relPath = filepath.ToSlash(rp)
				}
			} else if refURL {
				relPath = p.Ref
			}
			typeCell = fmt.Sprintf("%s *(via [%s](%s))*", dataType, refID, relPath)
		} else {
			if p.DataType != "" {
				typeCell = p.DataType
			}
			if p.Description != "" {
				descCell = p.Description
			}
			if reqVal, ok := p.Checks["required"]; ok {
				if rb, ok := reqVal.(bool); ok && rb {
					required = "yes"
				}
			}
		}
		// Inline overrides: an item's own description always wins.
		if p.Description != "" {
			descCell = p.Description
		}
		// Pipe-escape any | inside cell text.
		descCell = strings.ReplaceAll(descCell, "|", "\\|")
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", name, typeCell, required, descCell)
	}

	return strings.TrimRight(b.String(), "\n")
}

// renderEntityReferencedBy returns the canonical body of the
// ## Referenced by managed section.
func renderEntityReferencedBy(doc *entity.Doc, specRoot string, descendants []*entity.Doc) string {
	if len(descendants) == 0 {
		return "- _No references yet._"
	}
	parentDir := filepath.Dir(doc.Path)
	var lines []string
	for _, child := range descendants {
		rel, err := filepath.Rel(parentDir, child.Path)
		if err != nil {
			rel = child.Path
		}
		rel = filepath.ToSlash(rel)
		childID := child.Slug
		if child.Frontmatter != nil && child.Frontmatter.ID != "" {
			childID = child.Frontmatter.ID
		}
		lines = append(lines, fmt.Sprintf("- Entity: [%s](%s) *(inherits)*", childID, rel))
	}
	return strings.Join(lines, "\n")
}

// managedSectionDrift reports whether the managed body inside the named
// section diverges from the expected rendering. Missing managed markers
// also count as drift.
func managedSectionDrift(doc *entity.Doc, sectionTitle, expectedBody string) bool {
	sec, ok := doc.SectionByTitle[sectionTitle]
	if !ok {
		// Missing-section is its own rule; do not double-report as drift.
		return false
	}
	actual, found := extractManagedBody(sec.Body)
	if !found {
		return true
	}
	return strings.TrimSpace(actual) != strings.TrimSpace(expectedBody)
}

// extractManagedBody returns the text BETWEEN the managed-by markers in
// a section body, or ("", false) if the markers are absent.
func extractManagedBody(body string) (string, bool) {
	startIdx := strings.Index(body, managedStartMarker)
	if startIdx < 0 {
		return "", false
	}
	rest := body[startIdx+len(managedStartMarker):]
	endIdx := strings.Index(rest, managedEndMarker)
	if endIdx < 0 {
		return "", false
	}
	return strings.Trim(rest[:endIdx], "\n"), true
}

// applyManagedRewrites computes the new file bytes with the ##
// Properties and ## Referenced by managed bodies replaced by the
// canonical rendering. Returns (newBytes, changed, error). The
// section markers (<!-- managed-by: ... -->) MUST be preserved
// byte-for-byte per
// [cli/entity#req:properties-table-rendered] — only the body between
// them is replaced.
func applyManagedRewrites(doc *entity.Doc, propsBody, refsBody string) ([]byte, bool, error) {
	raw, err := os.ReadFile(doc.Path)
	if err != nil {
		return nil, false, err
	}
	source := string(raw)
	hadTrailingNewline := strings.HasSuffix(source, "\n")

	out, propsChanged := rewriteManagedSection(source, "## Properties", propsBody)
	out, refsChanged := rewriteManagedSection(out, "## Referenced by", refsBody)
	if hadTrailingNewline && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out), propsChanged || refsChanged, nil
}

// rewriteManagedSection finds the named ## section, locates the
// managedStartMarker/managedEndMarker pair inside it, and replaces the
// body between them with newBody. If the section has no managed
// markers, the section body is rewritten to contain a fresh marker
// pair surrounding newBody. Returns (newSource, changed).
func rewriteManagedSection(source, sectionHeading, newBody string) (string, bool) {
	lines := strings.Split(source, "\n")
	// Find the section heading.
	headStart := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == sectionHeading {
			headStart = i
			break
		}
	}
	if headStart < 0 {
		return source, false
	}
	// Find the next ## heading (or end of file).
	headEnd := len(lines)
	for i := headStart + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "## ") || strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "---") {
			headEnd = i
			break
		}
	}
	// Inside [headStart+1, headEnd) find the marker pair.
	startIdx := -1
	endIdx := -1
	for i := headStart + 1; i < headEnd; i++ {
		if strings.TrimSpace(lines[i]) == managedStartMarker && startIdx < 0 {
			startIdx = i
		}
		if strings.TrimSpace(lines[i]) == managedEndMarker && endIdx < 0 && startIdx >= 0 {
			endIdx = i
		}
	}
	var newLines []string
	if startIdx >= 0 && endIdx > startIdx {
		// Replace the body between markers.
		newLines = append(newLines, lines[:startIdx+1]...)
		if newBody != "" {
			newLines = append(newLines, strings.Split(newBody, "\n")...)
		}
		newLines = append(newLines, lines[endIdx:]...)
	} else {
		// Markers absent — install canonical body with markers, preserving
		// the rest of the section.
		// Strategy: drop everything between the heading line and the next
		// section/file boundary, then insert: <blank> markers+body.
		newLines = append(newLines, lines[:headStart+1]...)
		newLines = append(newLines, "")
		newLines = append(newLines, managedStartMarker)
		if newBody != "" {
			newLines = append(newLines, strings.Split(newBody, "\n")...)
		}
		newLines = append(newLines, managedEndMarker)
		// Ensure a blank line before the next section.
		if headEnd < len(lines) {
			newLines = append(newLines, "")
			newLines = append(newLines, lines[headEnd:]...)
		}
	}
	out := strings.Join(newLines, "\n")
	return out, out != source
}

// applyIDEqualsSlugFix rewrites the frontmatter id key to match the
// filename slug, using the round-trippable yaml.Node so comments and
// key order are preserved per [cli/entity#req:id-equals-slug-autofix].
func applyIDEqualsSlugFix(doc *entity.Doc) ([]byte, bool, error) {
	if doc.FmRaw == nil {
		return nil, false, nil
	}
	raw, err := os.ReadFile(doc.Path)
	if err != nil {
		return nil, false, err
	}
	source := string(raw)
	// Locate the frontmatter block.
	lines := strings.Split(source, "\n")
	openIdx := -1
	closeIdx := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		if t == "---" {
			openIdx = i
		}
		break
	}
	if openIdx < 0 {
		return nil, false, nil
	}
	for j := openIdx + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			closeIdx = j
			break
		}
	}
	if closeIdx < 0 {
		return nil, false, nil
	}

	// Mutate the yaml.Node so the `id` value matches doc.Slug.
	root := doc.FmRaw
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, false, nil
	}
	changed := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "id" {
			if root.Content[i+1].Value != doc.Slug {
				root.Content[i+1].Value = doc.Slug
				changed = true
			}
			break
		}
	}
	if !changed {
		return nil, false, nil
	}
	out, err := yaml.Marshal(doc.FmRaw)
	if err != nil {
		return nil, false, err
	}
	// yaml.Marshal includes a trailing newline; strip it so we don't
	// inject a blank line inside the frontmatter delimiters.
	newFM := strings.TrimRight(string(out), "\n")

	newLines := append([]string{}, lines[:openIdx+1]...)
	newLines = append(newLines, strings.Split(newFM, "\n")...)
	newLines = append(newLines, lines[closeIdx:]...)
	return []byte(strings.Join(newLines, "\n")), true, nil
}

// rewriteEntityTitle rewrites the `# Entity: <name>` line to
// `# Entity: <singular>`. Returns (newSrc, changed).
func rewriteEntityTitle(content []byte, singular string) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	changed := false
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "# ") {
			newTitle := "# Entity: " + singular
			if t != newTitle {
				lines[i] = newTitle
				changed = true
			}
			break
		}
	}
	if !changed {
		return content, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// filterAutofixedEntityViolations drops violations whose rules are
// fully repaired by the rewriter (managed-section drift, id-equals-slug,
// title-format). Called after the phase-2 write so the post-fix report
// shows zero violations naturally.
func filterAutofixedEntityViolations(in []Violation, parsed []*entity.Doc) []Violation {
	_ = parsed
	auto := map[string]bool{
		"entity-properties-table-managed": true,
		"entity-referenced-by-managed":    true,
		"entity-id-equals-slug":           true,
		"entity-title-format":             true,
	}
	out := in[:0]
	for _, v := range in {
		if auto[v.Rule] {
			continue
		}
		out = append(out, v)
	}
	return out
}

// ---------------------------------------------------------------------
// Discovery helpers (entity-location, entity-single-file).
// ---------------------------------------------------------------------

// findMisplacedEntityFiles returns the paths of *.entity.md files that
// live OUTSIDE spec/features/**.
func findMisplacedEntityFiles(specRoot string) ([]string, error) {
	var out []string
	featuresDir := filepath.Join(specRoot, "features")
	absFeatures, _ := filepath.Abs(featuresDir)
	err := filepath.Walk(specRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if path != specRoot && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), entity.EntitySuffix) {
			return nil
		}
		abs, _ := filepath.Abs(path)
		rel, err := filepath.Rel(absFeatures, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// findEntityDirectories returns paths of directories ending in
// `.entity.md` — i.e., `<slug>.entity/` directories that violate
// entity-single-file.
//
// NOTE: filesystem cannot have a directory whose name is `<slug>.entity.md`
// (technically allowed, but unusual). The most natural interpretation of
// [entity#req:single-file] is that creating a directory at
// `<slug>.entity/` (or `<slug>.entity.md/` as a directory) is the
// violation. We accept BOTH forms here.
func findEntityDirectories(specRoot string) ([]string, error) {
	featuresDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featuresDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var out []string
	err = filepath.Walk(featuresDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if path != featuresDir && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
			return filepath.SkipDir
		}
		if strings.HasSuffix(name, entity.EntitySuffix) || strings.HasSuffix(name, ".entity") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkEntityFiles is the adherence-footer walk function for
// `*.entity.md` files. Registered in docTypeTargets so the shared
// adherence-footer rule covers entity files.
func walkEntityFiles(specRoot string, fn func(path string, content []byte)) error {
	featuresDir := filepath.Join(specRoot, "features")
	return walkMatchingFiles(featuresDir, func(_ string, _ int, name string) bool {
		return strings.HasSuffix(name, entity.EntitySuffix)
	}, fn)
}
