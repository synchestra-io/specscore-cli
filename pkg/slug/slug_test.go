package slug

import "testing"

// TestIssueSlug_TruncationContract pins the exact input/output pair from
// the AC `cli/spec/lint/issue-rules#ac:slug-helper-truncation`. The AC
// fixes the output string verbatim, so this test is the contract.
func TestIssueSlug_TruncationContract(t *testing.T) {
	in := "The application crashes intermittently when the user navigates between menus quickly"
	want := "the-application-crashes-intermittently-when-the-user"
	got := IssueSlug(in)
	if got != want {
		t.Errorf("IssueSlug(%q)\n  got  %q (len=%d)\n  want %q (len=%d)", in, got, len(got), want, len(want))
	}
}

// TestIssueSlug_ShortInputNotTruncated covers the no-truncation branch.
func TestIssueSlug_ShortInputNotTruncated(t *testing.T) {
	in := "Short title"
	want := "short-title"
	if got := IssueSlug(in); got != want {
		t.Errorf("IssueSlug(%q) = %q; want %q", in, got, want)
	}
}

// TestIssueSlug_PunctuationCollapsed verifies that runs of non-alphanumeric
// characters collapse to a single hyphen and leading/trailing hyphens are
// trimmed.
func TestIssueSlug_PunctuationCollapsed(t *testing.T) {
	in := "  Hello,   world!!  "
	want := "hello-world"
	if got := IssueSlug(in); got != want {
		t.Errorf("IssueSlug(%q) = %q; want %q", in, got, want)
	}
}

// TestIssueSlug_HardTruncateWhenNoHyphenInWindow guards the fallback path
// where the first 60 chars contain no `-` boundary — the helper must
// hard-truncate at 60 rather than return the full string.
func TestIssueSlug_HardTruncateWhenNoHyphenInWindow(t *testing.T) {
	in := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 73 a's
	got := IssueSlug(in)
	if len(got) != 60 {
		t.Errorf("hard-truncate length = %d; want 60; got %q", len(got), got)
	}
}
