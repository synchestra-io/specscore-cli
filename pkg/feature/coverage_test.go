package feature

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Helper to build a full spec repo structure for info/plan tests ---

// setupSpecRepo creates a temp directory with a full spec repo layout:
//
//	<root>/spec/features/<id>/README.md  — for each feature
//	<root>/spec/plans/<plan>/README.md   — for each plan
//
// Returns (root, featuresDir).
func setupSpecRepo(t *testing.T, features map[string]string, plans map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	featDir := filepath.Join(root, "spec", "features")
	for id, content := range features {
		dir := filepath.Join(featDir, filepath.FromSlash(id))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	plansDir := filepath.Join(root, "spec", "plans")
	for name, content := range plans {
		dir := filepath.Join(plansDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, featDir
}

// =============================================================================
// info.go — GetInfo
// =============================================================================

func TestGetInfo(t *testing.T) {
	authReadme := `# Feature: Auth

**Status:** Approved

## Summary

Authentication feature.

## Dependencies

## Contents

| Child | Description |
|---|---|
| [sub](sub/README.md) | A sub-feature |

## Open Questions

- How to handle tokens?

---
*This document follows the https://specscore.md/feature-specification*
`
	billingReadme := `# Feature: Billing

**Status:** Draft

## Summary

Billing feature.

## Dependencies

- auth

## Open Questions

None at this time.
`
	subReadme := `# Feature: Auth Sub

**Status:** Draft

## Summary

Sub-feature of auth.

## Open Questions

None at this time.
`
	planReadme := `# Plan: Implement Auth

**Features:**
- [Auth](../../features/auth/README.md)

## Tasks

- Task 1
`

	_, featDir := setupSpecRepo(t, map[string]string{
		"auth":     authReadme,
		"auth/sub": subReadme,
		"billing":  billingReadme,
	}, map[string]string{
		"implement-auth": planReadme,
	})

	info, err := GetInfo(featDir, "auth")
	if err != nil {
		t.Fatal(err)
	}

	if info.Path != "auth" {
		t.Errorf("Path = %q, want %q", info.Path, "auth")
	}
	if info.Status != "Approved" {
		t.Errorf("Status = %q, want %q", info.Status, "Approved")
	}
	// billing depends on auth, so auth should have billing in refs
	if len(info.Refs) != 1 || info.Refs[0] != "billing" {
		t.Errorf("Refs = %v, want [billing]", info.Refs)
	}
	// auth has no deps
	if len(info.Deps) != 0 {
		t.Errorf("Deps = %v, want []", info.Deps)
	}
	// auth has a child sub-feature
	if len(info.Children) != 1 {
		t.Fatalf("Children = %v, want 1 child", info.Children)
	}
	if info.Children[0].Path != "auth/sub" {
		t.Errorf("Children[0].Path = %q, want %q", info.Children[0].Path, "auth/sub")
	}
	if !info.Children[0].InReadme {
		t.Error("Children[0].InReadme = false, want true")
	}
	// auth is referenced by plan "implement-auth"
	if len(info.Plans) != 1 || info.Plans[0] != "implement-auth" {
		t.Errorf("Plans = %v, want [implement-auth]", info.Plans)
	}
	// Sections should include Summary, Dependencies, Contents, Open Questions
	if len(info.Sections) < 3 {
		t.Errorf("Sections count = %d, want >= 3", len(info.Sections))
	}
}

func TestGetInfo_NonexistentFeature(t *testing.T) {
	_, featDir := setupSpecRepo(t, map[string]string{
		"auth": "# Feature: Auth\n\n**Status:** Draft\n",
	}, nil)

	_, err := GetInfo(featDir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
}

// =============================================================================
// info.go — FindFeatureRefs
// =============================================================================

func TestFindFeatureRefs(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"auth":     "# Auth\n\n**Status:** Approved\n\n## Dependencies\n\n## Open Questions\n\nNone.\n",
		"billing":  "# Billing\n\n**Status:** Draft\n\n## Dependencies\n\n- auth\n\n## Open Questions\n\nNone.\n",
		"payments": "# Payments\n\n**Status:** Draft\n\n## Dependencies\n\n- auth\n- billing\n\n## Open Questions\n\nNone.\n",
		"reports":  "# Reports\n\n**Status:** Draft\n\n## Dependencies\n\n## Open Questions\n\nNone.\n",
	})

	// auth is referenced by billing and payments
	refs, err := FindFeatureRefs(featDir, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 {
		t.Fatalf("refs = %v, want [billing payments]", refs)
	}
	if refs[0] != "billing" || refs[1] != "payments" {
		t.Errorf("refs = %v, want [billing payments]", refs)
	}

	// billing is referenced by payments
	refs, err = FindFeatureRefs(featDir, "billing")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != "payments" {
		t.Errorf("refs = %v, want [payments]", refs)
	}

	// reports has no refs
	refs, err = FindFeatureRefs(featDir, "reports")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Errorf("refs = %v, want []", refs)
	}
}

// =============================================================================
// info.go — DiscoverChildFeatures
// =============================================================================

func TestDiscoverChildFeatures(t *testing.T) {
	parentReadme := `# Feature: Parent

**Status:** Stable

## Summary

Parent feature.

## Contents

| Child | Description |
|---|---|
| [child-a](child-a/README.md) | First child |

## Open Questions

None.
`
	featDir := setupTestFeatures(t, map[string]string{
		"parent":         parentReadme,
		"parent/child-a": "# Feature: Child A\n\n**Status:** Draft\n",
		"parent/child-b": "# Feature: Child B\n\n**Status:** Draft\n",
	})

	readmePath := ReadmePath(featDir, "parent")
	children, err := DiscoverChildFeatures(featDir, "parent", readmePath)
	if err != nil {
		t.Fatal(err)
	}

	if len(children) != 2 {
		t.Fatalf("children = %v, want 2 children", children)
	}

	// child-a is in readme, child-b is not
	var childA, childB *ChildInfo
	for i := range children {
		if children[i].Path == "parent/child-a" {
			childA = &children[i]
		} else if children[i].Path == "parent/child-b" {
			childB = &children[i]
		}
	}

	if childA == nil || !childA.InReadme {
		t.Errorf("child-a should be InReadme=true, got %v", childA)
	}
	if childB == nil || childB.InReadme {
		t.Errorf("child-b should be InReadme=false, got %v", childB)
	}
}

func TestDiscoverChildFeatures_SkipsUnderscoreDirs(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"parent": "# Feature: Parent\n\n**Status:** Draft\n",
	})

	// Create a _hidden directory with a README
	hiddenDir := filepath.Join(featDir, "parent", "_hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "README.md"), []byte("# Hidden"), 0o644); err != nil {
		t.Fatal(err)
	}

	readmePath := ReadmePath(featDir, "parent")
	children, err := DiscoverChildFeatures(featDir, "parent", readmePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 0 {
		t.Errorf("expected no children (underscore dirs skipped), got %v", children)
	}
}

// =============================================================================
// info.go — ParseContentsTable
// =============================================================================

func TestParseContentsTable(t *testing.T) {
	content := `# Feature: Parent

## Summary

A parent.

## Contents

| Child | Description |
|---|---|
| [billing](billing/README.md) | Billing system |
| [payments](payments/README.md) | Payments processing |

## Open Questions

None.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := ParseContentsTable(path)
	if err != nil {
		t.Fatal(err)
	}

	if !entries["billing"] {
		t.Error("expected billing in contents table")
	}
	if !entries["payments"] {
		t.Error("expected payments in contents table")
	}
	if entries["nonexistent"] {
		t.Error("nonexistent should not be in contents table")
	}
}

func TestParseContentsTable_NoContentsSection(t *testing.T) {
	content := `# Feature: Simple

## Summary

No contents section here.

## Open Questions

None.
`
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := ParseContentsTable(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %v", entries)
	}
}

// =============================================================================
// info.go — FindLinkedPlans and planReferencesFeature
// =============================================================================

func TestFindLinkedPlans(t *testing.T) {
	planA := `# Plan: Implement Auth

**Features:**
- [Auth](../../features/auth/README.md)
- [Billing](../../features/billing/README.md)

## Tasks

- Task 1
`
	planB := `# Plan: Payment Plan

**Features:**
- [Billing](../../features/billing/README.md)

## Tasks

- Task 1
`
	planC := `# Plan: Unrelated

## Tasks

- Task 1
`

	root, _ := setupSpecRepo(t, map[string]string{
		"auth":    "# Auth\n**Status:** Approved\n",
		"billing": "# Billing\n**Status:** Draft\n",
	}, map[string]string{
		"plan-a":    planA,
		"plan-b":    planB,
		"unrelated": planC,
	})

	// auth is linked by plan-a only
	plans, err := FindLinkedPlans(root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0] != "plan-a" {
		t.Errorf("plans for auth = %v, want [plan-a]", plans)
	}

	// billing is linked by plan-a and plan-b
	plans, err = FindLinkedPlans(root, "billing")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 {
		t.Fatalf("plans for billing = %v, want [plan-a plan-b]", plans)
	}
	if plans[0] != "plan-a" || plans[1] != "plan-b" {
		t.Errorf("plans for billing = %v, want [plan-a plan-b]", plans)
	}

	// nonexistent feature has no plans
	plans, err = FindLinkedPlans(root, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Errorf("plans for nonexistent = %v, want []", plans)
	}
}

func TestFindLinkedPlans_NoPlansDir(t *testing.T) {
	root := t.TempDir()
	plans, err := FindLinkedPlans(root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if plans != nil {
		t.Errorf("expected nil plans when plans dir doesn't exist, got %v", plans)
	}
}

// =============================================================================
// transitive.go — TransitiveRefs, RefsResolver, EnrichTransitiveNodes,
//                  PrintTransitiveText
// =============================================================================

func TestTransitiveRefs(t *testing.T) {
	// c depends on b, b depends on a
	// So: refs of a = [b], refs of b = [c]
	// TransitiveRefs(a) should be: b -> c
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# A\n\n## Dependencies\n\n## Open Questions\n\nNone.\n",
		"b": "# B\n\n## Dependencies\n\n- a\n\n## Open Questions\n\nNone.\n",
		"c": "# C\n\n## Dependencies\n\n- b\n\n## Open Questions\n\nNone.\n",
	})

	nodes := TransitiveRefs(featDir, "a")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 top-level ref node, got %d: %v", len(nodes), nodes)
	}
	if nodes[0].Path != "b" {
		t.Errorf("nodes[0].Path = %q, want %q", nodes[0].Path, "b")
	}
	if len(nodes[0].ChildNodes) != 1 {
		t.Fatalf("expected 1 child of b, got %d", len(nodes[0].ChildNodes))
	}
	if nodes[0].ChildNodes[0].Path != "c" {
		t.Errorf("child path = %q, want %q", nodes[0].ChildNodes[0].Path, "c")
	}
}

func TestTransitiveRefs_CycleDetection(t *testing.T) {
	// a depends on b, b depends on a (mutual dependency)
	// TransitiveRefs(a) should detect cycle
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# A\n\n## Dependencies\n\n- b\n\n## Open Questions\n\nNone.\n",
		"b": "# B\n\n## Dependencies\n\n- a\n\n## Open Questions\n\nNone.\n",
	})

	// TransitiveRefs for "a": refs of a = [b] (because b depends on a),
	// then refs of b = [a] (because a depends on b), but a is already visited
	nodes := TransitiveRefs(featDir, "a")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Path != "b" {
		t.Errorf("nodes[0].Path = %q, want %q", nodes[0].Path, "b")
	}
	// b's children should show a as a cycle
	if len(nodes[0].ChildNodes) != 1 {
		t.Fatalf("expected 1 child of b (cycle node), got %d", len(nodes[0].ChildNodes))
	}
	cycleNode := nodes[0].ChildNodes[0]
	if cycleNode.Path != "a" {
		t.Errorf("cycle node path = %q, want %q", cycleNode.Path, "a")
	}
	if cycleNode.Cycle == nil || !*cycleNode.Cycle {
		t.Error("expected cycle flag to be true")
	}
}

func TestRefsResolver(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"auth":    "# Auth\n\n## Dependencies\n\n## Open Questions\n\nNone.\n",
		"billing": "# Billing\n\n## Dependencies\n\n- auth\n\n## Open Questions\n\nNone.\n",
	})

	refs, err := RefsResolver(featDir, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != "billing" {
		t.Errorf("RefsResolver = %v, want [billing]", refs)
	}
}

func TestEnrichTransitiveNodes(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# Feature: Alpha\n\n**Status:** Draft\n\n## Dependencies\n\n## Open Questions\n\n- Q1?\n",
		"b": "# Feature: Beta\n\n**Status:** Approved\n\n## Dependencies\n\n- a\n\n## Open Questions\n\nNone.\n",
	})

	nodes := []*EnrichedFeature{
		{Path: "a", ChildNodes: []*EnrichedFeature{
			{Path: "b"},
		}},
	}

	EnrichTransitiveNodes(featDir, nodes, []string{"status", "oq"})

	if nodes[0].Status != "Draft" {
		t.Errorf("nodes[0].Status = %q, want %q", nodes[0].Status, "Draft")
	}
	if nodes[0].OQ == nil || *nodes[0].OQ != 1 {
		t.Errorf("nodes[0].OQ = %v, want 1", nodes[0].OQ)
	}
	if nodes[0].ChildNodes[0].Status != "Approved" {
		t.Errorf("child Status = %q, want %q", nodes[0].ChildNodes[0].Status, "Approved")
	}
	if nodes[0].ChildNodes[0].OQ == nil || *nodes[0].ChildNodes[0].OQ != 0 {
		t.Errorf("child OQ = %v, want 0", nodes[0].ChildNodes[0].OQ)
	}
}

func TestEnrichTransitiveNodes_SkipsCycleNodes(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# Feature: Alpha\n\n**Status:** Draft\n\n## Open Questions\n\nNone.\n",
	})

	cycleTrue := true
	nodes := []*EnrichedFeature{
		{Path: "a", Cycle: &cycleTrue},
	}

	EnrichTransitiveNodes(featDir, nodes, []string{"status"})

	// Cycle node should NOT be enriched
	if nodes[0].Status != "" {
		t.Errorf("cycle node should not be enriched, got Status=%q", nodes[0].Status)
	}
}

func TestPrintTransitiveText(t *testing.T) {
	cycleTrue := true
	nodes := []*EnrichedFeature{
		{
			Path: "auth",
			ChildNodes: []*EnrichedFeature{
				{
					Path: "billing",
					ChildNodes: []*EnrichedFeature{
						{Path: "auth", Cycle: &cycleTrue},
					},
				},
			},
		},
	}

	var sb strings.Builder
	PrintTransitiveText(&sb, nodes, 0)

	got := sb.String()
	want := "auth\n\tbilling\n\t\tauth (cycle)\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPrintTransitiveText_Empty(t *testing.T) {
	var sb strings.Builder
	PrintTransitiveText(&sb, nil, 0)
	if sb.String() != "" {
		t.Errorf("expected empty output for nil nodes, got %q", sb.String())
	}
}

// =============================================================================
// tree.go — BuildEnrichedTree
// =============================================================================

func TestBuildEnrichedTree(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"auth":         "# Feature: Auth\n\n**Status:** Approved\n\n## Open Questions\n\nNone.\n",
		"auth/oauth":   "# Feature: OAuth\n\n**Status:** Draft\n\n## Open Questions\n\n- How?\n",
		"auth/session": "# Feature: Session\n\n**Status:** Stable\n\n## Open Questions\n\nNone.\n",
		"billing":      "# Feature: Billing\n\n**Status:** Draft\n\n## Open Questions\n\nNone.\n",
	})

	featureIDs := []string{"auth", "auth/oauth", "auth/session", "billing"}
	fields := []string{"status", "oq"}

	tree := BuildEnrichedTree(featDir, featureIDs, fields, "auth/oauth")

	// Should have 2 roots: auth and billing
	if len(tree) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(tree))
	}

	// auth root
	if tree[0].Path != "auth" {
		t.Errorf("tree[0].Path = %q, want %q", tree[0].Path, "auth")
	}
	if tree[0].Status != "Approved" {
		t.Errorf("tree[0].Status = %q, want %q", tree[0].Status, "Approved")
	}
	// auth should have 2 children
	if len(tree[0].ChildNodes) != 2 {
		t.Fatalf("expected 2 children of auth, got %d", len(tree[0].ChildNodes))
	}

	// focus should be on auth/oauth
	var oauthNode *EnrichedFeature
	for _, c := range tree[0].ChildNodes {
		if c.Path == "auth/oauth" {
			oauthNode = c
			break
		}
	}
	if oauthNode == nil {
		t.Fatal("auth/oauth not found in tree")
	}
	if oauthNode.Focus == nil || !*oauthNode.Focus {
		t.Error("auth/oauth should have Focus=true")
	}
	if oauthNode.Status != "Draft" {
		t.Errorf("oauth Status = %q, want %q", oauthNode.Status, "Draft")
	}

	// billing root
	if tree[1].Path != "billing" {
		t.Errorf("tree[1].Path = %q, want %q", tree[1].Path, "billing")
	}
}

func TestBuildEnrichedTree_NoFocus(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"alpha": "# Feature: Alpha\n\n**Status:** Draft\n",
		"beta":  "# Feature: Beta\n\n**Status:** Stable\n",
	})

	tree := BuildEnrichedTree(featDir, []string{"alpha", "beta"}, []string{"status"}, "")

	if len(tree) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(tree))
	}
	// No focus should be set
	for _, n := range tree {
		if n.Focus != nil {
			t.Errorf("expected no focus, but %q has Focus=%v", n.Path, *n.Focus)
		}
	}
}

func TestBuildEnrichedTree_FiltersChildrenField(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"parent":       "# Feature: Parent\n\n**Status:** Draft\n",
		"parent/child": "# Feature: Child\n\n**Status:** Draft\n",
	})

	// "children" field should be filtered out in tree mode
	tree := BuildEnrichedTree(featDir, []string{"parent", "parent/child"}, []string{"status", "children"}, "")

	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	if tree[0].Path != "parent" {
		t.Errorf("root path = %q, want %q", tree[0].Path, "parent")
	}
	// ChildNodes should be populated via tree nesting
	if len(tree[0].ChildNodes) != 1 {
		t.Errorf("expected 1 child node, got %d", len(tree[0].ChildNodes))
	}
}

// =============================================================================
// discover.go — MarkFocus
// =============================================================================

func TestMarkFocus(t *testing.T) {
	ids := []string{"cli", "cli/task", "cli/task/claim", "cli/feature", "api"}
	nodes := BuildTree(ids)

	MarkFocus(nodes, "cli/task/claim")

	// Verify the target node has focus
	var found bool
	var checkFocus func([]*FeatureNode)
	checkFocus = func(nodes []*FeatureNode) {
		for _, n := range nodes {
			if n.ID == "cli/task/claim" {
				if !n.Focus {
					t.Errorf("cli/task/claim should have Focus=true")
				}
				found = true
			} else {
				if n.Focus {
					t.Errorf("%q should NOT have Focus=true", n.ID)
				}
			}
			checkFocus(n.Children)
		}
	}
	checkFocus(nodes)

	if !found {
		t.Error("cli/task/claim not found in tree")
	}
}

func TestMarkFocus_RootNode(t *testing.T) {
	ids := []string{"alpha", "beta", "gamma"}
	nodes := BuildTree(ids)

	MarkFocus(nodes, "beta")

	if !nodes[1].Focus {
		t.Error("beta should have Focus=true")
	}
	if nodes[0].Focus {
		t.Error("alpha should not have Focus")
	}
	if nodes[2].Focus {
		t.Error("gamma should not have Focus")
	}
}

func TestMarkFocus_NonexistentTarget(t *testing.T) {
	ids := []string{"alpha", "beta"}
	nodes := BuildTree(ids)

	MarkFocus(nodes, "nonexistent")

	// No node should have focus
	for _, n := range nodes {
		if n.Focus {
			t.Errorf("%q should not have Focus", n.ID)
		}
	}
}

// =============================================================================
// fields.go — ValidateFormat
// =============================================================================

func TestValidateFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		format  string
		wantErr bool
	}{
		{"text", false},
		{"yaml", false},
		{"json", false},
		{"xml", true},
		{"csv", true},
		{"", true},
		{"TEXT", true},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			t.Parallel()
			err := ValidateFormat(tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
			}
		})
	}
}

// =============================================================================
// fields.go — BoolPtr
// =============================================================================

func TestBoolPtr(t *testing.T) {
	t.Parallel()

	truePtr := BoolPtr(true)
	if truePtr == nil || !*truePtr {
		t.Error("BoolPtr(true) should return pointer to true")
	}

	falsePtr := BoolPtr(false)
	if falsePtr == nil || *falsePtr {
		t.Error("BoolPtr(false) should return pointer to false")
	}

	// Verify they are distinct pointers
	if truePtr == falsePtr {
		t.Error("BoolPtr should return distinct pointers")
	}
}

// =============================================================================
// fields.go — ResolveFields (additional coverage for uncovered branches)
// =============================================================================

func TestResolveFields_AllFields(t *testing.T) {
	parentReadme := `# Feature: Parent

**Status:** Approved

## Summary

Parent feature.

## Contents

| Child | Description |
|---|---|
| [child](child/README.md) | A child |

## Dependencies

- dep-a

## Open Questions

- Question 1?
- Question 2?
`
	_, featDir := setupSpecRepo(t, map[string]string{
		"parent":       parentReadme,
		"parent/child": "# Feature: Child\n\n**Status:** Draft\n\n## Open Questions\n\nNone.\n",
		"dep-a":        "# Feature: Dep A\n\n**Status:** Stable\n\n## Dependencies\n\n- parent\n\n## Open Questions\n\nNone.\n",
	}, nil)

	ef, err := ResolveFields(featDir, "parent", []string{"status", "oq", "questions", "title", "deps", "refs", "children", "plans", "proposals"})
	if err != nil {
		t.Fatal(err)
	}

	if ef.Path != "parent" {
		t.Errorf("Path = %q, want %q", ef.Path, "parent")
	}
	if ef.Status != "Approved" {
		t.Errorf("Status = %q, want %q", ef.Status, "Approved")
	}
	if ef.Title != "Parent" {
		t.Errorf("Title = %q, want %q", ef.Title, "Parent")
	}
	if ef.OQ == nil || *ef.OQ != 2 {
		t.Errorf("OQ = %v, want 2", ef.OQ)
	}
	if len(ef.Questions) != 2 {
		t.Errorf("Questions = %v, want 2 items", ef.Questions)
	}
	if len(ef.Deps) != 1 || ef.Deps[0] != "dep-a" {
		t.Errorf("Deps = %v, want [dep-a]", ef.Deps)
	}
	if len(ef.Refs) != 1 || ef.Refs[0] != "dep-a" {
		t.Errorf("Refs = %v, want [dep-a]", ef.Refs)
	}
	if len(ef.ChildPaths) != 1 || ef.ChildPaths[0] != "parent/child" {
		t.Errorf("ChildPaths = %v, want [parent/child]", ef.ChildPaths)
	}
}

func TestResolveFields_ErrorHandling(t *testing.T) {
	// Create a features dir with no actual features to trigger errors
	featDir := t.TempDir()

	ef, err := ResolveFields(featDir, "nonexistent", []string{"status", "oq", "title", "deps", "children"})
	// Should return partial result with error
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
	if ef == nil {
		t.Fatal("expected partial result even on error")
	}
	if ef.Path != "nonexistent" {
		t.Errorf("Path = %q, want %q", ef.Path, "nonexistent")
	}
}

// =============================================================================
// transitive.go — TransitiveDeps cycle detection
// =============================================================================

func TestTransitiveDeps_CycleDetection(t *testing.T) {
	// A -> B -> C -> A (cycle)
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# A\n\n## Dependencies\n\n- b\n",
		"b": "# B\n\n## Dependencies\n\n- c\n",
		"c": "# C\n\n## Dependencies\n\n- a\n",
	})

	nodes := TransitiveDeps(featDir, "a")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 top node, got %d", len(nodes))
	}
	if nodes[0].Path != "b" {
		t.Errorf("nodes[0].Path = %q, want %q", nodes[0].Path, "b")
	}

	// Walk to c
	if len(nodes[0].ChildNodes) != 1 {
		t.Fatalf("expected 1 child of b, got %d", len(nodes[0].ChildNodes))
	}
	cNode := nodes[0].ChildNodes[0]
	if cNode.Path != "c" {
		t.Errorf("c node path = %q", cNode.Path)
	}

	// c should have a cycle back to a
	if len(cNode.ChildNodes) != 1 {
		t.Fatalf("expected 1 child of c (cycle), got %d", len(cNode.ChildNodes))
	}
	cycleNode := cNode.ChildNodes[0]
	if cycleNode.Path != "a" {
		t.Errorf("cycle node path = %q, want %q", cycleNode.Path, "a")
	}
	if cycleNode.Cycle == nil || !*cycleNode.Cycle {
		t.Error("expected cycle flag on a")
	}
}

func TestTransitiveDeps_NoDeps(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# A\n\n## Summary\n\nNo deps.\n",
	})

	nodes := TransitiveDeps(featDir, "a")
	if len(nodes) != 0 {
		t.Errorf("expected no nodes for feature with no deps, got %d", len(nodes))
	}
}

func TestTransitiveRefs_NoRefs(t *testing.T) {
	featDir := setupTestFeatures(t, map[string]string{
		"a": "# A\n\n## Summary\n\nNo one depends on this.\n",
	})

	nodes := TransitiveRefs(featDir, "a")
	if len(nodes) != 0 {
		t.Errorf("expected no nodes for feature with no refs, got %d", len(nodes))
	}
}

// =============================================================================
// info.go — planReferencesFeature (tested indirectly via FindLinkedPlans above,
//           but let's also test edge cases)
// =============================================================================

func TestFindLinkedPlans_PlanWithoutFeaturesSection(t *testing.T) {
	root, _ := setupSpecRepo(t, map[string]string{
		"auth": "# Auth\n**Status:** Approved\n",
	}, map[string]string{
		"no-features": "# Plan: No Features\n\n## Tasks\n\n- Task 1\n",
	})

	plans, err := FindLinkedPlans(root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 0 {
		t.Errorf("expected no plans, got %v", plans)
	}
}

func TestFindLinkedPlans_PlanFeaturesEndsAtNewSection(t *testing.T) {
	planContent := `# Plan: Mixed

**Features:**
- [Auth](../../features/auth/README.md)

**Status:** Draft

## Tasks

- Task 1
`
	root, _ := setupSpecRepo(t, map[string]string{
		"auth": "# Auth\n**Status:** Approved\n",
	}, map[string]string{
		"mixed": planContent,
	})

	plans, err := FindLinkedPlans(root, "auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0] != "mixed" {
		t.Errorf("plans = %v, want [mixed]", plans)
	}
}

// =============================================================================
// Integration: GetInfo with plans
// =============================================================================

func TestGetInfo_WithPlans(t *testing.T) {
	billingReadme := `# Feature: Billing

**Status:** Draft

## Summary

Billing module.

## Dependencies

## Open Questions

None at this time.
`
	planReadme := `# Plan: Bill Users

**Features:**
- [Billing](../../features/billing/README.md)

## Tasks

- Implement billing
`

	_, featDir := setupSpecRepo(t, map[string]string{
		"billing": billingReadme,
	}, map[string]string{
		"bill-users": planReadme,
	})

	info, err := GetInfo(featDir, "billing")
	if err != nil {
		t.Fatal(err)
	}

	if len(info.Plans) != 1 || info.Plans[0] != "bill-users" {
		t.Errorf("Plans = %v, want [bill-users]", info.Plans)
	}
}

// =============================================================================
// PrintTree with focus marker (covers the Focus branch in PrintTree)
// =============================================================================

func TestPrintTree_WithFocus(t *testing.T) {
	ids := []string{"alpha", "alpha/child", "beta"}
	nodes := BuildTree(ids)
	MarkFocus(nodes, "alpha/child")

	var sb strings.Builder
	PrintTree(&sb, nodes, 0)

	got := sb.String()
	want := "alpha\n\t* child\nbeta\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
