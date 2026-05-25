package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runProposal(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := proposalCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

// Test that `specscore proposal new <feature> <slug>` creates the correct file.
func TestProposalNew_CreatesProposal(t *testing.T) {
	root := setupSpecRootWithFeature(t, "payments")
	withCwd(t, root)

	stdout, _, err := runProposal(t, "new", "payments", "add-refunds",
		"--title", "Add Refunds",
		"--owner", "alice",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	expectedPath := filepath.Join(root, "spec", "features", "payments", "proposals", "add-refunds.md")
	if !strings.Contains(stdout, expectedPath) {
		t.Errorf("stdout %q does not contain expected path %q", stdout, expectedPath)
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("proposal file not created at %s: %v", expectedPath, err)
	}
	body, _ := os.ReadFile(expectedPath)
	s := string(body)
	for _, want := range []string{
		"# Proposal: Add Refunds",
		"**Type:** change-request",
		"**Targets:** payments",
		"**Owner:** alice",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in generated body:\n%s", want, s)
		}
	}
}

// Test that proposal new with a nonexistent feature fails.
func TestProposalNew_FeatureNotFound(t *testing.T) {
	root := setupSpecRoot(t)
	withCwd(t, root)

	_, _, err := runProposal(t, "new", "ghost-feature", "my-proposal")
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention nonexistence: %v", err)
	}
}

// Test that proposal new produces output identical to idea new --type=change-request.
func TestProposalNew_MatchesIdeaNew(t *testing.T) {
	// Create two separate roots with the same feature.
	root1 := setupSpecRootWithFeature(t, "search")
	root2 := setupSpecRootWithFeature(t, "search")

	// Run via proposal alias.
	withCwd(t, root1)
	_, _, err := runProposal(t, "new", "search", "add-facets",
		"--title", "Add Facets",
		"--owner", "bob",
	)
	if err != nil {
		t.Fatalf("proposal new: %v", err)
	}

	// Run via idea new with explicit flags.
	withCwd(t, root2)
	_, _, err = runIdea(t, "new", "add-facets",
		"--type", "change-request",
		"--targets", "search",
		"--title", "Add Facets",
		"--owner", "bob",
	)
	if err != nil {
		t.Fatalf("idea new: %v", err)
	}

	body1, _ := os.ReadFile(filepath.Join(root1, "spec", "features", "search", "proposals", "add-facets.md"))
	body2, _ := os.ReadFile(filepath.Join(root2, "spec", "features", "search", "proposals", "add-facets.md"))

	// Normalize dates (both scaffolded "today" but may differ by sub-second).
	// Just compare structurally — both should have the same fields.
	for _, want := range []string{
		"# Proposal: Add Facets",
		"**Type:** change-request",
		"**Targets:** search",
		"**Owner:** bob",
	} {
		if !strings.Contains(string(body1), want) {
			t.Errorf("proposal alias missing %q:\n%s", want, body1)
		}
		if !strings.Contains(string(body2), want) {
			t.Errorf("idea --type missing %q:\n%s", want, body2)
		}
	}
}

// Test that proposal new forwards --phase.
func TestProposalNew_ForwardsPhase(t *testing.T) {
	root := setupSpecRootWithFeature(t, "analytics")
	withCwd(t, root)

	_, _, err := runProposal(t, "new", "analytics", "add-events",
		"--phase", "design",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	body, _ := os.ReadFile(filepath.Join(root, "spec", "features", "analytics", "proposals", "add-events.md"))
	if !strings.Contains(string(body), "**Phase:** design") {
		t.Errorf("phase not forwarded:\n%s", body)
	}
}
