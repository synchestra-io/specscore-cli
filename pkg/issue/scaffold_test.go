package issue

import (
	"errors"
	"os/user"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateSlug
// ---------------------------------------------------------------------------

func TestValidateSlug_Valid(t *testing.T) {
	valid := []string{
		"a",
		"abc",
		"menu-crashes-on-empty",
		"a1-b2-c3",
		"x",
		"ab",
		// Max length: 60 chars.
		"abcdefghij-abcdefghij-abcdefghij-abcdefghij-abcdefghij-abcde",
	}
	for _, slug := range valid {
		if err := ValidateSlug(slug); err != nil {
			t.Errorf("ValidateSlug(%q) = %v; want nil", slug, err)
		}
	}
}

func TestValidateSlug_Empty(t *testing.T) {
	err := ValidateSlug("")
	if err == nil {
		t.Fatal("ValidateSlug(\"\") should return error")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error = %q; want 'must not be empty'", err.Error())
	}
}

func TestValidateSlug_Uppercase(t *testing.T) {
	err := ValidateSlug("Menu-Crashes")
	if err == nil {
		t.Fatal("ValidateSlug should reject uppercase")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error = %q; want match error", err.Error())
	}
}

func TestValidateSlug_SpecialChars(t *testing.T) {
	cases := []string{"has_underscore", "has.dot", "has space", "has@at", "foo/bar"}
	for _, slug := range cases {
		if err := ValidateSlug(slug); err == nil {
			t.Errorf("ValidateSlug(%q) should return error for special chars", slug)
		}
	}
}

func TestValidateSlug_LeadingHyphen(t *testing.T) {
	err := ValidateSlug("-leading")
	if err == nil {
		t.Fatal("ValidateSlug should reject leading hyphen")
	}
}

func TestValidateSlug_TrailingHyphen(t *testing.T) {
	err := ValidateSlug("trailing-")
	if err == nil {
		t.Fatal("ValidateSlug should reject trailing hyphen")
	}
}

func TestValidateSlug_ConsecutiveHyphens(t *testing.T) {
	err := ValidateSlug("foo--bar")
	if err == nil {
		t.Fatal("ValidateSlug should reject consecutive hyphens")
	}
}

func TestValidateSlug_TooLong(t *testing.T) {
	slug := strings.Repeat("a", 61)
	err := ValidateSlug(slug)
	if err == nil {
		t.Fatal("ValidateSlug should reject slugs > 60 chars")
	}
	if !strings.Contains(err.Error(), "exceeds 60 characters") {
		t.Errorf("error = %q; want 'exceeds 60 characters'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Scaffold: defaults
// ---------------------------------------------------------------------------

func TestScaffold_Defaults(t *testing.T) {
	out, err := Scaffold(ScaffoldOptions{Slug: "menu-crash"})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	s := string(out)

	// Check frontmatter defaults.
	if !strings.Contains(s, "type: issue") {
		t.Error("missing type: issue")
	}
	if !strings.Contains(s, "slug: menu-crash") {
		t.Error("missing slug")
	}
	if !strings.Contains(s, "status: open") {
		t.Error("missing status: open")
	}
	// Title derived from slug.
	if !strings.Contains(s, "# Issue: Menu Crash") {
		t.Error("title not derived from slug")
	}
	// captured_by should be set (from OS user).
	if !strings.Contains(s, "captured_by:") {
		t.Error("missing captured_by")
	}
	// captured_at should be an RFC3339 timestamp (contains T and Z or +).
	idx := strings.Index(s, "captured_at: ")
	if idx < 0 {
		t.Fatal("missing captured_at")
	}
	ts := s[idx+len("captured_at: "):]
	ts = ts[:strings.Index(ts, "\n")]
	if !strings.Contains(ts, "T") {
		t.Errorf("captured_at %q does not look like RFC3339", ts)
	}
}

// ---------------------------------------------------------------------------
// Scaffold: all options supplied
// ---------------------------------------------------------------------------

func TestScaffold_AllOptions(t *testing.T) {
	opts := ScaffoldOptions{
		Slug:              "payment-timeout",
		Title:             "Payment Timeout Bug",
		CapturedBy:        "alice",
		CapturedAt:        "2026-01-15T10:30:00Z",
		Severity:          "high",
		AffectedComponent: "billing",
		FirstSeen:         "2026-01-14",
		GithubIssue:       "https://github.com/org/repo/issues/42",
	}
	out, err := Scaffold(opts)
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	s := string(out)

	checks := []string{
		"slug: payment-timeout",
		"status: open",
		"captured_at: 2026-01-15T10:30:00Z",
		"captured_by: alice",
		"severity: high",
		"affected_component: billing",
		"first_seen: 2026-01-14",
		"github_issue: https://github.com/org/repo/issues/42",
		"# Issue: Payment Timeout Bug",
	}
	for _, chk := range checks {
		if !strings.Contains(s, chk) {
			t.Errorf("output missing %q", chk)
		}
	}
}

// ---------------------------------------------------------------------------
// Scaffold: optional fields omitted when not supplied
// ---------------------------------------------------------------------------

func TestScaffold_OptionalFieldsOmitted(t *testing.T) {
	out, err := Scaffold(ScaffoldOptions{
		Slug:       "minimal-issue",
		CapturedBy: "bob",
		CapturedAt: "2026-02-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	s := string(out)

	// These optional fields must NOT appear.
	absent := []string{"severity:", "affected_component:", "first_seen:", "github_issue:"}
	for _, a := range absent {
		if strings.Contains(s, a) {
			t.Errorf("output should not contain %q when option is empty", a)
		}
	}
}

// ---------------------------------------------------------------------------
// Scaffold: body structure
// ---------------------------------------------------------------------------

func TestScaffold_BodyStructure(t *testing.T) {
	out, err := Scaffold(ScaffoldOptions{
		Slug:       "body-test",
		CapturedBy: "tester",
		CapturedAt: "2026-03-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	s := string(out)

	// Verify required H2 sections in order.
	sections := []string{
		"## Description",
		"## Steps to Reproduce",
		"## Expected vs Actual",
	}
	lastIdx := 0
	for _, sec := range sections {
		idx := strings.Index(s, sec)
		if idx < 0 {
			t.Errorf("missing section %q", sec)
			continue
		}
		if idx < lastIdx {
			t.Errorf("section %q appears before previous section", sec)
		}
		lastIdx = idx
	}

	// HTML comment prompts.
	if !strings.Contains(s, "<!-- TODO: Describe the observed behavior -->") {
		t.Error("missing Description TODO comment")
	}
	if !strings.Contains(s, "<!-- TODO: List the steps to reproduce this issue -->") {
		t.Error("missing Steps to Reproduce TODO comment")
	}
	if !strings.Contains(s, "<!-- TODO: Describe what you expected and what happened instead -->") {
		t.Error("missing Expected vs Actual TODO comment")
	}

	// Footer line.
	if !strings.Contains(s, "https://specscore.md/issue-specification") {
		t.Error("missing specscore specification link")
	}
}

// ---------------------------------------------------------------------------
// Scaffold: title from slug
// ---------------------------------------------------------------------------

func TestScaffold_TitleFromSlug(t *testing.T) {
	cases := []struct {
		slug string
		want string
	}{
		{"menu-crashes-on-empty", "Menu Crashes On Empty"},
		{"a", "A"},
		{"single", "Single"},
		{"one-two-three", "One Two Three"},
	}
	for _, tc := range cases {
		out, err := Scaffold(ScaffoldOptions{
			Slug:       tc.slug,
			CapturedBy: "x",
			CapturedAt: "2026-01-01T00:00:00Z",
		})
		if err != nil {
			t.Fatalf("Scaffold(%q): %v", tc.slug, err)
		}
		expected := "# Issue: " + tc.want
		if !strings.Contains(string(out), expected) {
			t.Errorf("slug %q: expected title %q not found in output", tc.slug, expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Scaffold: invalid slug propagates error
// ---------------------------------------------------------------------------

func TestScaffold_InvalidSlug(t *testing.T) {
	_, err := Scaffold(ScaffoldOptions{Slug: ""})
	if err == nil {
		t.Fatal("Scaffold should return error for empty slug")
	}

	_, err = Scaffold(ScaffoldOptions{Slug: "BAD-SLUG"})
	if err == nil {
		t.Fatal("Scaffold should return error for invalid slug")
	}
}

// ---------------------------------------------------------------------------
// titleCaseFromSlug unit test (via Scaffold output)
// ---------------------------------------------------------------------------

func TestTitleCaseFromSlug(t *testing.T) {
	got := titleCaseFromSlug("payment-timeout")
	if got != "Payment Timeout" {
		t.Errorf("titleCaseFromSlug(\"payment-timeout\") = %q; want \"Payment Timeout\"", got)
	}
}

// TestTitleCaseFromSlug_EmptyParts exercises the `continue` branch when
// strings.Split produces empty parts (e.g., if called with a string
// containing consecutive hyphens — not reachable via Scaffold due to
// ValidateSlug, but the function itself handles it).
func TestTitleCaseFromSlug_EmptyParts(t *testing.T) {
	// Simulate a slug-like string with an empty part (consecutive hyphens).
	got := titleCaseFromSlug("foo--bar")
	// Split produces ["foo", "", "bar"] → after title-casing → "Foo  Bar"
	if got != "Foo  Bar" {
		t.Errorf("titleCaseFromSlug(\"foo--bar\") = %q; want \"Foo  Bar\"", got)
	}
}

// ---------------------------------------------------------------------------
// defaultCapturedBy: just check it returns non-empty
// ---------------------------------------------------------------------------

func TestDefaultCapturedBy(t *testing.T) {
	got := defaultCapturedBy()
	if got == "" {
		t.Error("defaultCapturedBy should return non-empty string")
	}
}

func TestDefaultCapturedBy_UserCurrentFails(t *testing.T) {
	orig := currentUser
	currentUser = func() (*user.User, error) {
		return nil, errors.New("no user")
	}
	t.Cleanup(func() { currentUser = orig })

	got := defaultCapturedBy()
	if got != "unknown" {
		t.Errorf("defaultCapturedBy() = %q; want %q", got, "unknown")
	}
}

func TestDefaultCapturedBy_EmptyUsername(t *testing.T) {
	orig := currentUser
	currentUser = func() (*user.User, error) {
		return &user.User{Username: ""}, nil
	}
	t.Cleanup(func() { currentUser = orig })

	got := defaultCapturedBy()
	if got != "unknown" {
		t.Errorf("defaultCapturedBy() = %q; want %q", got, "unknown")
	}
}

// ---------------------------------------------------------------------------
// Scaffold: whitespace-only Title/CapturedBy/CapturedAt treated as empty
// ---------------------------------------------------------------------------

func TestScaffold_WhitespaceOnlyFieldsTreatedAsEmpty(t *testing.T) {
	out, err := Scaffold(ScaffoldOptions{
		Slug:       "whitespace-test",
		Title:      "   ",
		CapturedBy: "   ",
		CapturedAt: "   ",
	})
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}
	s := string(out)

	// Title should be derived from slug when whitespace-only.
	if !strings.Contains(s, "# Issue: Whitespace Test") {
		t.Error("whitespace-only title should fall back to slug-derived title")
	}
	// CapturedBy should fallback to OS user.
	if strings.Contains(s, "captured_by:    ") {
		t.Error("whitespace-only captured_by should not remain as whitespace")
	}
}

// ---------------------------------------------------------------------------
// ValidSeverityValues: verify expected set
// ---------------------------------------------------------------------------

func TestValidSeverityValues(t *testing.T) {
	expected := []string{"low", "medium", "high", "critical"}
	for _, v := range expected {
		if !ValidSeverityValues[v] {
			t.Errorf("ValidSeverityValues[%q] should be true", v)
		}
	}
	if ValidSeverityValues["invalid"] {
		t.Error("ValidSeverityValues[\"invalid\"] should be false")
	}
}
