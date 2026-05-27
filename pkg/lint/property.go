package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/property"
)

// propertyChecker is a dispatch checker that runs every property-* rule in
// one pass per file. Parsing is shared across rules and the managed-section
// `## Referenced by` rewriter is invoked under --fix.
//
// The struct mirrors ideaChecker / featureIndexChecker — when --fix is on,
// the autofix pass happens inside `check()` (under the `autofix` flag) so the
// post-fix scan reports zero violations naturally.
type propertyChecker struct {
	autofix bool
}

func newPropertyChecker() *propertyChecker {
	return &propertyChecker{}
}

// propertyRuleNames is the canonical list of rule names this checker emits.
// Every name MUST also appear in lint.go's allRuleNames map.
var propertyRuleNames = []string{
	"property-location",
	"property-slug-format",
	"property-single-file",
	"property-frontmatter-required",
	"property-frontmatter-required-fields",
	"property-id-equals-slug",
	"property-data-type-values",
	"property-checks-shape",
	"property-title-format",
	"property-required-sections",
	"property-referenced-by-managed",
}

func (c *propertyChecker) name() string     { return "property-location" }
func (c *propertyChecker) severity() string { return "error" }

func (c *propertyChecker) check(specRoot string) ([]Violation, error) {
	return checkProperties(specRoot, c.autofix)
}

// fix implements the fixer interface; rewrites managed `## Referenced by`
// bodies in two phases: (1) scan every entity and compute the canonical
// body per property; (2) write each property file once. The fix is also
// invoked implicitly via check() when autofix is on, so calling fix()
// before check() is idempotent.
func (c *propertyChecker) fix(specRoot string) error {
	_, err := runPropertyFix(specRoot)
	return err
}

// checkProperties is the entry point: discover, parse, run every rule,
// (optionally) apply autofixes, then re-discover and re-run for clean post-fix
// reporting under --fix.
func checkProperties(specRoot string, fix bool) ([]Violation, error) {
	var violations []Violation

	// Per-misplacement / per-directory rules first; they don't need parses.
	misplaced, err := findMisplacedPropertyFiles(specRoot)
	if err != nil {
		return nil, err
	}
	for _, p := range misplaced {
		rel, _ := filepath.Rel(specRoot, p)
		violations = append(violations, Violation{
			File:     rel,
			Line:     0,
			Severity: "error",
			Rule:     "property-location",
			Message:  fmt.Sprintf("property files must live at spec/features/**/*.property.md; got %s", rel),
		})
	}

	dirs, err := findPropertyDirectories(specRoot)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		rel, _ := filepath.Rel(specRoot, d)
		violations = append(violations, Violation{
			File:     rel,
			Line:     0,
			Severity: "error",
			Rule:     "property-single-file",
			Message:  fmt.Sprintf("properties must be single markdown files; directory found at %s", rel),
		})
	}

	// Slug-format on filenames (cheap, no parse needed beyond discovery).
	discovered, err := property.Discover(specRoot)
	if err != nil {
		return nil, err
	}

	// Apply autofix BEFORE the violations pass when fix is on.
	// runPropertyFix re-writes id, title, and managed-section bodies.
	if fix {
		if _, err := runPropertyFix(specRoot); err != nil {
			return nil, err
		}
		// Re-discover (in case anything changed; today files don't move
		// during fix but this is a defensive pattern).
		discovered, err = property.Discover(specRoot)
		if err != nil {
			return nil, err
		}
	}

	// Parse each property file.
	type parsed struct {
		doc *property.Doc
		d   property.Discovered
	}
	var docs []parsed
	for _, d := range discovered {
		doc, err := property.Parse(d.Path)
		if err != nil {
			rel, _ := filepath.Rel(specRoot, d.Path)
			violations = append(violations, Violation{
				File:     rel,
				Severity: "error",
				Rule:     "property-location",
				Message:  fmt.Sprintf("cannot read property file: %v", err),
			})
			continue
		}
		docs = append(docs, parsed{doc: doc, d: d})
	}

	// Compute canonical Referenced By bodies once (used for the
	// referenced-by-managed rule). Phase-1 of fix/scan ordering.
	canonical, err := computeReferencedByForAll(specRoot, discovered)
	if err != nil {
		return nil, err
	}

	for _, pd := range docs {
		rel, _ := filepath.Rel(specRoot, pd.doc.Path)
		violations = append(violations, propertyFileRules(pd.doc, rel, canonical[pd.doc.Path])...)
	}

	return violations, nil
}

// propertyFileRules returns every rule violation for one parsed property
// file (excluding location/single-file rules which run against the on-disk
// layout in checkProperties).
func propertyFileRules(doc *property.Doc, relPath string, canonicalReferencedBy string) []Violation {
	var vs []Violation

	// property-slug-format — filename slug must be lowercase-hyphenated.
	if err := property.ValidateSlug(doc.Slug); err != nil {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-slug-format", Message: err.Error(),
		})
	}

	// property-frontmatter-required — first non-empty content must be `---`
	// (parser yields Frontmatter==nil and HasTitle==true for misplaced
	// frontmatter; we directly inspect RawLines for the "first non-empty
	// content" rule).
	hasLeadingFrontmatter := propertyHasLeadingFrontmatter(doc.RawLines)
	if !hasLeadingFrontmatter {
		vs = append(vs, Violation{
			File: relPath, Line: 1, Severity: "error",
			Rule:    "property-frontmatter-required",
			Message: "property file must begin with a YAML frontmatter block delimited by `---`",
		})
		// When the frontmatter block is missing/misplaced we cannot
		// meaningfully run the dependent rules; return early.
		return vs
	}
	if doc.Frontmatter == nil {
		vs = append(vs, Violation{
			File: relPath, Line: 1, Severity: "error",
			Rule:    "property-frontmatter-required",
			Message: "property file frontmatter is malformed or empty",
		})
		return vs
	}

	fm := doc.Frontmatter

	// property-frontmatter-required-fields — kind, id, data_type, checks key.
	if fm.Kind == "" {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-frontmatter-required-fields", Message: "missing required frontmatter key `kind`",
		})
	} else if fm.Kind != "property" {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-frontmatter-required-fields", Message: fmt.Sprintf("frontmatter `kind` MUST be `property`, got %q", fm.Kind),
		})
	}
	if fm.ID == "" {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-frontmatter-required-fields", Message: "missing required frontmatter key `id`",
		})
	}
	if fm.DataType == "" {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-frontmatter-required-fields", Message: "missing required frontmatter key `data_type`",
		})
	}
	if !frontmatterHasChecksKey(doc) {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule: "property-frontmatter-required-fields", Message: "missing required frontmatter key `checks` (may be empty: `checks: {}`)",
		})
	}

	// property-id-equals-slug
	if fm.ID != "" && fm.ID != doc.Slug {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule:    "property-id-equals-slug",
			Message: fmt.Sprintf("frontmatter `id: %s` must equal filename slug %q", fm.ID, doc.Slug),
		})
	}

	// property-data-type-values
	if fm.DataType != "" && !property.LegalDataTypes[fm.DataType] {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule:    "property-data-type-values",
			Message: fmt.Sprintf("`data_type: %s` is not one of %s", fm.DataType, legalDataTypesList()),
		})
	}

	// property-checks-shape — applicability matrix
	if fm.DataType != "" {
		for key := range fm.Checks {
			applies, ok := property.CheckKeyApplicability[key]
			if !ok {
				// Unknown key — warning severity per
				// [cli/property#req:checks-shape-applicability]
				vs = append(vs, Violation{
					File: relPath, Line: 0, Severity: "warning",
					Rule:    "property-checks-shape",
					Message: fmt.Sprintf("unknown check key %q (not in canonical vocabulary)", key),
				})
				continue
			}
			if !applies[fm.DataType] {
				vs = append(vs, Violation{
					File: relPath, Line: 0, Severity: "error",
					Rule:    "property-checks-shape",
					Message: fmt.Sprintf("check key %q is not applicable to data_type %q", key, fm.DataType),
				})
			}
		}
	}

	// property-title-format
	if !doc.HasTitle {
		vs = append(vs, Violation{
			File: relPath, Line: 1, Severity: "error",
			Rule: "property-title-format", Message: "missing title (expected `# Property: <id>`)",
		})
	} else {
		expected := "Property: " + fm.ID
		if fm.ID != "" && doc.Title != expected {
			vs = append(vs, Violation{
				File: relPath, Line: doc.TitleLine, Severity: "error",
				Rule:    "property-title-format",
				Message: fmt.Sprintf("title must be `# Property: %s`, got `# %s`", fm.ID, doc.Title),
			})
		}
	}

	// property-required-sections — Description and Referenced by, in order.
	var missingSections []string
	if _, ok := doc.SectionByTitle["Description"]; !ok {
		missingSections = append(missingSections, "Description")
	}
	if _, ok := doc.SectionByTitle["Referenced by"]; !ok {
		missingSections = append(missingSections, "Referenced by")
	}
	if len(missingSections) > 0 {
		vs = append(vs, Violation{
			File: relPath, Line: 0, Severity: "error",
			Rule:    "property-required-sections",
			Message: fmt.Sprintf("missing required section(s): %s", strings.Join(missingSections, ", ")),
		})
	} else {
		// Order check.
		var sectionOrder []string
		for _, s := range doc.Sections {
			if s.Title == "Description" || s.Title == "Referenced by" {
				sectionOrder = append(sectionOrder, s.Title)
			}
		}
		if len(sectionOrder) == 2 && sectionOrder[0] != "Description" {
			vs = append(vs, Violation{
				File: relPath, Line: 0, Severity: "error",
				Rule:    "property-required-sections",
				Message: "required sections present but not in canonical order (Description, Referenced by)",
			})
		}
	}

	// property-referenced-by-managed — managed body must match canonical scan.
	if sec, ok := doc.SectionByTitle["Referenced by"]; ok {
		actualBody, found := extractPropertyManagedBody(sec.Body)
		expectedBody := canonicalReferencedBy
		if !found || strings.TrimSpace(actualBody) != strings.TrimSpace(expectedBody) {
			vs = append(vs, Violation{
				File: relPath, Line: sec.StartLine, Severity: "error",
				Rule:    "property-referenced-by-managed",
				Message: "managed `## Referenced by` body has drifted from the canonical scan (run `specscore spec lint --fix`)",
			})
		}
	}

	return vs
}

// legalDataTypesList returns a sorted, comma-separated list of legal
// data_type values for error messages.
func legalDataTypesList() string {
	keys := make([]string, 0, len(property.LegalDataTypes))
	for k := range property.LegalDataTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// propertyHasLeadingFrontmatter reports whether the first non-empty line
// of rawLines is a `---` delimiter (per
// [property#req:frontmatter-required]).
func propertyHasLeadingFrontmatter(rawLines []string) bool {
	for _, ln := range rawLines {
		t := strings.TrimSpace(ln)
		if t == "" {
			continue
		}
		return t == "---"
	}
	return false
}

// frontmatterHasChecksKey reports whether the parsed frontmatter contained
// a `checks:` key (which may be an empty mapping). The Frontmatter struct
// always carries a non-nil Checks map, so we must inspect FmRaw to know
// whether the key was actually present in the YAML.
func frontmatterHasChecksKey(doc *property.Doc) bool {
	if doc.FmRaw == nil {
		return false
	}
	root := doc.FmRaw
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return false
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == "checks" {
			return true
		}
	}
	return false
}

// extractPropertyManagedBody returns the text between
// `<!-- managed-by: specscore lint --fix -->` and `<!-- end-managed -->`
// inside a section body, plus a bool indicating whether both markers were
// present. This is a property-package-local helper so the property
// checker stays self-contained (the entity checker has a parallel helper
// in `entity.go`).
func extractPropertyManagedBody(sectionBody string) (string, bool) {
	const open = "<!-- managed-by: specscore lint --fix -->"
	const close = "<!-- end-managed -->"
	openIdx := strings.Index(sectionBody, open)
	if openIdx < 0 {
		return "", false
	}
	rest := sectionBody[openIdx+len(open):]
	closeIdx := strings.Index(rest, close)
	if closeIdx < 0 {
		return "", false
	}
	return strings.Trim(rest[:closeIdx], "\n"), true
}

// findMisplacedPropertyFiles locates `*.property.md` files that live
// outside `spec/features/**` (e.g. `spec/properties/`). Returns absolute
// paths.
func findMisplacedPropertyFiles(specRoot string) ([]string, error) {
	featuresAbs, _ := filepath.Abs(filepath.Join(specRoot, "features"))
	var out []string
	walkErr := filepath.Walk(specRoot, func(path string, info os.FileInfo, err error) error {
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
		if !strings.HasSuffix(info.Name(), ".property.md") {
			return nil
		}
		// In-scope: path is somewhere under spec/features/.
		abs, _ := filepath.Abs(path)
		if pathHasPrefix(abs, featuresAbs+string(filepath.Separator)) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

// findPropertyDirectories returns directories named like `<slug>.property/`
// (i.e. a directory with the .property extension) under `spec/features/`.
// Per [property#req:single-file] a property must be a single markdown
// file, not a directory.
func findPropertyDirectories(specRoot string) ([]string, error) {
	featuresDir := filepath.Join(specRoot, "features")
	info, err := os.Stat(featuresDir)
	if err != nil || !info.IsDir() {
		return nil, nil
	}
	var out []string
	walkErr := filepath.Walk(featuresDir, func(path string, info os.FileInfo, err error) error {
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
		if strings.HasSuffix(name, ".property") {
			out = append(out, path)
			return filepath.SkipDir
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return out, nil
}

func pathHasPrefix(p, prefix string) bool {
	return strings.HasPrefix(p, prefix)
}

// ----------------------------------------------------------------------
// Managed-section: ## Referenced by
// ----------------------------------------------------------------------

// runPropertyFix is the autofix entry point. It rewrites, in order:
//  1. id-mismatch-slug — frontmatter `id` round-tripped via yaml.Node.
//  2. title-format — `# Property: <id>` rewritten from frontmatter.
//  3. managed `## Referenced by` body rewritten from a fresh entity scan.
//
// Step (3) follows the phase-1/phase-2 contract from
// [cli/property#req:fix-write-ordering]: every canonical body is computed
// in a single scan BEFORE any file is written.
func runPropertyFix(specRoot string) (changed bool, err error) {
	discovered, err := property.Discover(specRoot)
	if err != nil {
		return false, err
	}

	// Phase 1: compute every canonical Referenced by body.
	canonical, err := computeReferencedByForAll(specRoot, discovered)
	if err != nil {
		return false, err
	}

	// Phase 2: rewrite each property file.
	for _, d := range discovered {
		fixedFile, err := rewritePropertyFile(d.Path, d.Slug, canonical[d.Path])
		if err != nil {
			return changed, err
		}
		if fixedFile {
			changed = true
		}
	}
	return changed, nil
}

// propertyConsumer is one entity that references a property (via
// `properties[].ref:`). Used to build each property's canonical `##
// Referenced by` body.
type propertyConsumer struct {
	entityID string
	relPath  string
}

// computeReferencedByForAll runs the entity scan once and returns a map
// from each property file's path to its canonical `## Referenced
// by` body (without surrounding marker comments). Properties with no
// consumers receive the `- _No references yet._` fallback.
func computeReferencedByForAll(specRoot string, discovered []property.Discovered) (map[string]string, error) {
	consumers := make(map[string][]propertyConsumer)
	seen := make(map[string]map[string]bool) // dedup by entityID per property

	// Seed every discovered property with an empty list so the
	// no-references fallback kicks in for unreferenced files.
	for _, d := range discovered {
		abs, _ := filepath.Abs(d.Path)
		consumers[abs] = nil
		seen[abs] = make(map[string]bool)
	}

	entities, err := entity.Discover(specRoot)
	if err != nil {
		return nil, err
	}
	for _, e := range entities {
		doc, err := entity.Parse(e.Path)
		if err != nil {
			continue
		}
		if doc.Frontmatter == nil {
			continue
		}
		entityID := doc.Frontmatter.ID
		if entityID == "" {
			entityID = doc.Slug
		}
		for _, pi := range doc.Frontmatter.Properties {
			if pi.Ref == "" {
				continue
			}
			resolved, ok, _ := entity.ResolveRef(specRoot, e.Path, pi.Ref)
			if !ok || resolved == "" {
				continue
			}
			// canonicalize: filepath.Clean + Abs already applied
			// inside ResolveRef.
			if _, tracked := consumers[resolved]; !tracked {
				// Reference resolves to a file we did NOT
				// discover — could be a broken ref. Skip.
				continue
			}
			if seen[resolved][entityID] {
				continue
			}
			seen[resolved][entityID] = true

			// Compute path RELATIVE to the property file.
			rel, relErr := filepath.Rel(filepath.Dir(resolved), e.Path)
			if relErr != nil {
				rel = e.Path
			}
			// Normalize to forward slashes for portability of the
			// rendered markdown link.
			rel = filepath.ToSlash(rel)
			consumers[resolved] = append(consumers[resolved], propertyConsumer{
				entityID: entityID,
				relPath:  rel,
			})
		}
	}

	out := make(map[string]string, len(discovered))
	for _, d := range discovered {
		abs, _ := filepath.Abs(d.Path)
		entries := consumers[abs]
		out[d.Path] = renderReferencedByBody(entries)
	}
	return out, nil
}

// renderReferencedByBody renders the canonical body for a property's
// `## Referenced by` section. Each consumer is one `- Entity: [<id>](<rel>)`
// line. The slice is sorted by (entityID, relPath). Empty input yields the
// fallback `- _No references yet._`.
func renderReferencedByBody(consumers []propertyConsumer) string {
	if len(consumers) == 0 {
		return "- _No references yet._"
	}
	sorted := make([]propertyConsumer, len(consumers))
	copy(sorted, consumers)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].entityID != sorted[j].entityID {
			return sorted[i].entityID < sorted[j].entityID
		}
		return sorted[i].relPath < sorted[j].relPath
	})
	var b strings.Builder
	for i, c := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- Entity: [%s](%s)", c.entityID, c.relPath)
	}
	return b.String()
}

// rewritePropertyFile applies id, title, and managed-section autofixes
// to a single property file. Returns true if any byte changed.
func rewritePropertyFile(path, slug, canonicalReferencedBy string) (bool, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	updated := original

	// (1) id-equals-slug rewrite via yaml.Node round-trip.
	if rewritten, ok := rewritePropertyFrontmatterID(updated, slug); ok {
		updated = rewritten
	}

	// (2) title-format rewrite.
	if rewritten, ok := rewritePropertyTitle(updated, slug); ok {
		updated = rewritten
	}

	// (3) managed Referenced by body.
	if rewritten, ok := rewriteManagedReferencedBy(updated, canonicalReferencedBy); ok {
		updated = rewritten
	}

	if string(updated) == string(original) {
		return false, nil
	}
	return true, os.WriteFile(path, updated, 0o644)
}

// rewritePropertyFrontmatterID rewrites the frontmatter `id` value to
// `slug`, preserving comments + key order + surrounding formatting via
// `yaml.Node` round-trip. Returns the new content and true if a change
// was applied; if the frontmatter is malformed or already correct, returns
// the original content and false.
func rewritePropertyFrontmatterID(content []byte, slug string) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	// Find first non-empty line and verify it's "---".
	first := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			first = i
			break
		}
	}
	if first < 0 || strings.TrimSpace(lines[first]) != "---" {
		return content, false
	}
	end := -1
	for j := first + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) == "---" {
			end = j
			break
		}
	}
	if end < 0 {
		return content, false
	}
	fmContent := strings.Join(lines[first+1:end], "\n")

	var node yaml.Node
	if err := yaml.Unmarshal([]byte(fmContent), &node); err != nil {
		return content, false
	}
	root := &node
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return content, false
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return content, false
	}
	changed := false
	for i := 0; i+1 < len(root.Content); i += 2 {
		k := root.Content[i]
		v := root.Content[i+1]
		if k.Kind == yaml.ScalarNode && k.Value == "id" {
			if v.Kind == yaml.ScalarNode && v.Value != slug {
				v.Value = slug
				v.Style = 0 // plain scalar
				changed = true
			}
			break
		}
	}
	if !changed {
		return content, false
	}
	out, err := yaml.Marshal(&node)
	if err != nil {
		return content, false
	}
	// yaml.Marshal always appends a trailing newline; strip it so we can
	// re-splice into the line slice cleanly.
	rendered := strings.TrimRight(string(out), "\n")
	renderedLines := strings.Split(rendered, "\n")

	var rebuilt []string
	rebuilt = append(rebuilt, lines[:first+1]...)
	rebuilt = append(rebuilt, renderedLines...)
	rebuilt = append(rebuilt, lines[end:]...)
	return []byte(strings.Join(rebuilt, "\n")), true
}

// propertyTitleRe matches a `# Property: <name>` title line.
var propertyTitleRe = regexp.MustCompile(`^#\s+Property:\s*(.+?)\s*$`)

// rewritePropertyTitle rewrites the `# Property: <name>` title line to
// `# Property: <slug>` when it disagrees. Returns the new content and
// true on change.
func rewritePropertyTitle(content []byte, slug string) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if m := propertyTitleRe.FindStringSubmatch(trimmed); m != nil {
			if strings.TrimSpace(m[1]) == slug {
				return content, false
			}
			lines[i] = "# Property: " + slug
			return []byte(strings.Join(lines, "\n")), true
		}
	}
	return content, false
}

// rewriteManagedReferencedBy replaces the body BETWEEN the canonical
// managed markers inside `## Referenced by` with `canonicalBody`. If the
// section is missing or the markers are missing, the content is returned
// unchanged (the property-required-sections rule covers the missing-section
// case).
func rewriteManagedReferencedBy(content []byte, canonicalBody string) ([]byte, bool) {
	const open = "<!-- managed-by: specscore lint --fix -->"
	const close = "<!-- end-managed -->"
	s := string(content)

	// Locate the `## Referenced by` heading.
	headingIdx := strings.Index(s, "\n## Referenced by")
	if headingIdx < 0 {
		// Could be the very first line of the file (rare for property files).
		if strings.HasPrefix(s, "## Referenced by") {
			headingIdx = 0
		} else {
			return content, false
		}
	}
	// Find the open marker after the heading.
	openIdx := strings.Index(s[headingIdx:], open)
	if openIdx < 0 {
		return content, false
	}
	openIdx += headingIdx
	closeIdx := strings.Index(s[openIdx:], close)
	if closeIdx < 0 {
		return content, false
	}
	closeIdx += openIdx

	bodyStart := openIdx + len(open)
	current := s[bodyStart:closeIdx]
	// Always render as "\n<body>\n" between markers.
	desired := "\n" + canonicalBody + "\n"
	if current == desired {
		return content, false
	}
	rebuilt := s[:bodyStart] + desired + s[closeIdx:]
	return []byte(rebuilt), true
}
