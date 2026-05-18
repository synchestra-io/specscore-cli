package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validEntityBody is a lint-clean minimal entity used as the "happy path"
// baseline for entity tests. Callers swap individual lines to drive each
// negative case.
const validEntityBody = `---
kind: entity
id: user
singular: User
plural: Users
properties:
  - name: id
    data_type: string
    description: Stable identifier.
    checks:
      required: true
---

# Entity: User

## Description

A registered human (or service) account in the system.

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ` + "`id`" + ` | string | yes | Stable identifier. |
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->

---
*This document follows the https://specscore.md/entity-specification*
`

// writeEntityProject creates a project tree under t.TempDir() with the
// given relative paths populated. Returns the project's spec/ root.
func writeEntityProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	specRoot := filepath.Join(dir, "spec")
	for rel, content := range files {
		full := filepath.Join(specRoot, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return specRoot
}

// runEntityCheck runs the entity checker against the given spec root and
// returns the violations.
func runEntityCheck(t *testing.T, specRoot string, fix bool) []Violation {
	t.Helper()
	c := newEntityChecker()
	c.autofix = fix
	vs, err := c.check(specRoot)
	if err != nil {
		t.Fatalf("entity check: %v", err)
	}
	return vs
}

// hasRuleViolation reports whether the slice contains a violation with
// the given rule name.
func hasRuleViolation(vs []Violation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------
// One test per rule in entityRuleNames.
// -----------------------------------------------------------------

func TestEntityChecker_EntityLocation(t *testing.T) {
	// A .entity.md file under spec/entities/ (not spec/features/) triggers
	// entity-location.
	specRoot := writeEntityProject(t, map[string]string{
		"entities/foo.entity.md": validEntityBody,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-location") {
		t.Errorf("expected entity-location violation, got: %+v", vs)
	}
}

func TestEntityChecker_SlugFormat(t *testing.T) {
	specRoot := writeEntityProject(t, map[string]string{
		"features/foo/Foo_Bar.entity.md": validEntityBody,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-slug-format") {
		t.Errorf("expected entity-slug-format violation, got: %+v", vs)
	}
}

func TestEntityChecker_SingleFile(t *testing.T) {
	dir := t.TempDir()
	specRoot := filepath.Join(dir, "spec")
	dirPath := filepath.Join(specRoot, "features", "foo", "user.entity")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-single-file") {
		t.Errorf("expected entity-single-file violation, got: %+v", vs)
	}
}

func TestEntityChecker_FrontmatterRequired(t *testing.T) {
	body := strings.Replace(validEntityBody, "---\nkind: entity", "# Entity: User\n\n---\nkind: entity", 1)
	// Above swap pushes the title before frontmatter — frontmatter is no
	// longer the first non-empty block.
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-frontmatter-required") {
		t.Errorf("expected entity-frontmatter-required violation, got: %+v", vs)
	}
}

func TestEntityChecker_FrontmatterRequiredFields(t *testing.T) {
	body := `---
kind: entity
id: user
singular: User
---

# Entity: User

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-frontmatter-required-fields") {
		t.Errorf("expected entity-frontmatter-required-fields violation, got: %+v", vs)
	}
}

func TestEntityChecker_IDEqualsSlug(t *testing.T) {
	body := strings.Replace(validEntityBody, "id: user", "id: usr", 1)
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-id-equals-slug") {
		t.Errorf("expected entity-id-equals-slug violation, got: %+v", vs)
	}
}

func TestEntityChecker_PropertiesListShape(t *testing.T) {
	body := `---
kind: entity
id: user
singular: User
plural: Users
properties:
  - name: id
    data_type: string
  - name: id
    data_type: string
---

# Entity: User

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-properties-list-shape") {
		t.Errorf("expected entity-properties-list-shape violation, got: %+v", vs)
	}
}

func TestEntityChecker_RefTargetExists(t *testing.T) {
	body := `---
kind: entity
id: user
singular: User
plural: Users
properties:
  - name: email
    ref: ./does-not-exist.property.md
---

# Entity: User

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-ref-target-exists") {
		t.Errorf("expected entity-ref-target-exists violation, got: %+v", vs)
	}
}

func TestEntityChecker_InheritsAdditiveOnly(t *testing.T) {
	parent := `---
kind: entity
id: parent
singular: Parent
plural: Parents
properties:
  - name: shared
    data_type: string
---

# Entity: Parent

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
| ` + "`shared`" + ` | string | no | — |
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	child := `---
kind: entity
id: child
singular: Child
plural: Children
inherits: ./parent.entity.md
properties:
  - name: shared
    data_type: string
---

# Entity: Child

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/family/parent.entity.md": parent,
		"features/family/child.entity.md":  child,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-inherits-additive-only") {
		t.Errorf("expected entity-inherits-additive-only violation, got: %+v", vs)
	}
}

func TestEntityChecker_InheritsTargetExists(t *testing.T) {
	body := `---
kind: entity
id: orphan
singular: Orphan
plural: Orphans
inherits: ./missing.entity.md
properties: []
---

# Entity: Orphan

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/family/orphan.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-inherits-target-exists") {
		t.Errorf("expected entity-inherits-target-exists violation, got: %+v", vs)
	}
}

func TestEntityChecker_InheritsAcyclic(t *testing.T) {
	a := `---
kind: entity
id: a
singular: A
plural: As
inherits: ./b.entity.md
properties: []
---

# Entity: A

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	b := `---
kind: entity
id: b
singular: B
plural: Bs
inherits: ./a.entity.md
properties: []
---

# Entity: B

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/cycle/a.entity.md": a,
		"features/cycle/b.entity.md": b,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-inherits-acyclic") {
		t.Errorf("expected entity-inherits-acyclic violation, got: %+v", vs)
	}
}

func TestEntityChecker_TitleFormat(t *testing.T) {
	body := strings.Replace(validEntityBody, "# Entity: User", "# Entity: WrongName", 1)
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-title-format") {
		t.Errorf("expected entity-title-format violation, got: %+v", vs)
	}
}

func TestEntityChecker_RequiredSections(t *testing.T) {
	// Drop the ## Properties section.
	body := strings.Replace(validEntityBody, `## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
| `+"`id`"+` | string | yes | Stable identifier. |
<!-- end-managed -->

`, "", 1)
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-required-sections") {
		t.Errorf("expected entity-required-sections violation, got: %+v", vs)
	}
}

func TestEntityChecker_PropertiesTableManaged(t *testing.T) {
	// Hand-edit the managed body so it diverges from canonical rendering.
	body := strings.Replace(validEntityBody,
		"| `id` | string | yes | Stable identifier. |",
		"| `id` | string | yes | hand-edited description |", 1)
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-properties-table-managed") {
		t.Errorf("expected entity-properties-table-managed violation, got: %+v", vs)
	}
}

func TestEntityChecker_ReferencedByManaged(t *testing.T) {
	// Hand-edit the managed body of ## Referenced by.
	body := strings.Replace(validEntityBody,
		"- _No references yet._",
		"- a hand-edited line", 1)
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	vs := runEntityCheck(t, specRoot, false)
	if !hasRuleViolation(vs, "entity-referenced-by-managed") {
		t.Errorf("expected entity-referenced-by-managed violation, got: %+v", vs)
	}
}

// -----------------------------------------------------------------
// Rendering tests.
// -----------------------------------------------------------------

func TestRenderPropertiesTableInlineAndRef(t *testing.T) {
	parentEntity := `---
kind: entity
id: user
singular: User
plural: Users
properties:
  - name: id
    data_type: string
    description: Stable identifier.
    checks:
      required: true
  - name: email
    ref: ../shared/email.property.md
---

# Entity: User

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	emailProperty := `---
kind: property
id: email
data_type: string
description: An RFC 5322 email address.
checks:
  required: true
---

# Property: email

## Description

X

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md":      parentEntity,
		"features/shared/email.property.md": emailProperty,
	})
	// Apply --fix.
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(specRoot, "features/user/user.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotS := string(got)
	if !strings.Contains(gotS, "| `id` | string | yes | Stable identifier. |") {
		t.Errorf("missing inline row: %s", gotS)
	}
	if !strings.Contains(gotS, "string *(via [email](../shared/email.property.md))*") {
		t.Errorf("missing ref-style Type cell: %s", gotS)
	}
}

func TestRenderEntityReferencedByInheritance(t *testing.T) {
	parent := `---
kind: entity
id: parent
singular: Parent
plural: Parents
properties: []
---

# Entity: Parent

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	child := `---
kind: entity
id: child
singular: Child
plural: Children
inherits: ./parent.entity.md
properties: []
---

# Entity: Child

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/family/parent.entity.md": parent,
		"features/family/child.entity.md":  child,
	})
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	parentGot, err := os.ReadFile(filepath.Join(specRoot, "features/family/parent.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parentGot), "- Entity: [child](child.entity.md) *(inherits)*") {
		t.Errorf("parent missing back-reference to child: %s", parentGot)
	}
}

func TestRenderEntityReferencedByNoReferencesFallback(t *testing.T) {
	// No descendants → exactly "- _No references yet._"
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": strings.Replace(validEntityBody,
			"- _No references yet._",
			"- a hand-edited line", 1),
	})
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(specRoot, "features/user/user.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "- _No references yet._") {
		t.Errorf("expected fallback line in output: %s", got)
	}
}

func TestEntityIDEqualsSlugAutofix(t *testing.T) {
	body := `---
# top-of-file comment
kind: entity
id: usr           # this comment must survive
singular: User
plural: Users
description: A user.
properties: []
---

# Entity: User

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/user/user.entity.md": body,
	})
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(specRoot, "features/user/user.entity.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotS := string(got)
	if !strings.Contains(gotS, "id: user") {
		t.Errorf("id not rewritten: %s", gotS)
	}
	if strings.Contains(gotS, "id: usr") {
		t.Errorf("old id still present: %s", gotS)
	}
	// Comment survival: the top-of-file comment and the inline comment
	// must both still be present.
	if !strings.Contains(gotS, "top-of-file comment") {
		t.Errorf("top-of-file comment lost: %s", gotS)
	}
	if !strings.Contains(gotS, "this comment must survive") {
		t.Errorf("inline comment lost: %s", gotS)
	}
	// Adjacent key survives (singular/plural should appear with original
	// order).
	idxSingular := strings.Index(gotS, "singular:")
	idxPlural := strings.Index(gotS, "plural:")
	if idxSingular < 0 || idxPlural < 0 || idxSingular >= idxPlural {
		t.Errorf("key order broken: singular=%d plural=%d", idxSingular, idxPlural)
	}
}

func TestEntityFixIsIdempotent(t *testing.T) {
	// Drive an entity through --fix twice; assert the second pass writes
	// no further changes (byte-compare).
	parent := `---
kind: entity
id: parent
singular: Parent
plural: Parents
properties: []
---

# Entity: Parent

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	child := `---
kind: entity
id: child
singular: Child
plural: Children
inherits: ./parent.entity.md
properties: []
---

# Entity: Child

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/family/parent.entity.md": parent,
		"features/family/child.entity.md":  child,
	})
	// First pass.
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	firstParent, _ := os.ReadFile(filepath.Join(specRoot, "features/family/parent.entity.md"))
	firstChild, _ := os.ReadFile(filepath.Join(specRoot, "features/family/child.entity.md"))
	// Second pass.
	c2 := newEntityChecker()
	c2.autofix = true
	if _, err := c2.check(specRoot); err != nil {
		t.Fatal(err)
	}
	secondParent, _ := os.ReadFile(filepath.Join(specRoot, "features/family/parent.entity.md"))
	secondChild, _ := os.ReadFile(filepath.Join(specRoot, "features/family/child.entity.md"))
	if string(firstParent) != string(secondParent) {
		t.Errorf("parent.entity.md changed on second --fix pass\nfirst:\n%s\nsecond:\n%s", firstParent, secondParent)
	}
	if string(firstChild) != string(secondChild) {
		t.Errorf("child.entity.md changed on second --fix pass\nfirst:\n%s\nsecond:\n%s", firstChild, secondChild)
	}
}

func TestEntityFixWriteOrdering(t *testing.T) {
	// A single --fix pass MUST refresh the parent's ## Referenced by
	// after a new child is added, in the same pass.
	parent := `---
kind: entity
id: parent
singular: Parent
plural: Parents
properties: []
---

# Entity: Parent

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	// Child entity inherits from parent.
	child := `---
kind: entity
id: child
singular: Child
plural: Children
inherits: ./parent.entity.md
properties: []
---

# Entity: Child

## Description

X

## Properties

<!-- managed-by: specscore lint --fix -->
| Name | Type | Required | Description |
|------|------|----------|-------------|
<!-- end-managed -->

## Referenced by

<!-- managed-by: specscore lint --fix -->
- _No references yet._
<!-- end-managed -->
`
	specRoot := writeEntityProject(t, map[string]string{
		"features/family/parent.entity.md": parent,
		"features/family/child.entity.md":  child,
	})
	c := newEntityChecker()
	c.autofix = true
	if _, err := c.check(specRoot); err != nil {
		t.Fatal(err)
	}
	parentGot, _ := os.ReadFile(filepath.Join(specRoot, "features/family/parent.entity.md"))
	if !strings.Contains(string(parentGot), "- Entity: [child](child.entity.md) *(inherits)*") {
		t.Errorf("parent's ## Referenced by not refreshed in single --fix pass: %s", parentGot)
	}
}

func TestAllEntityRuleNamesRegistered(t *testing.T) {
	all := AllRuleNames()
	for _, name := range entityRuleNames {
		if !all[name] {
			t.Errorf("entityRuleNames entry %q is not registered in allRuleNames", name)
		}
	}
}
