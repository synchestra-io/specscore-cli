package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Transition table: legal arcs per the Meta contract.
// Keep this in sync with transitionMatrix in lifecycle.go; the tests below
// derive their expected behavior from THIS table, not from the production
// table, so a drift between the two will be caught explicitly.
// ---------------------------------------------------------------------------

var expectedLegal = map[Kind][]transitionRow{
	KindIdea: {
		{From: IdeaDraft, To: IdeaApproved},
		{From: IdeaDraft, To: IdeaArchived},
		{From: IdeaUnderReview, To: IdeaArchived},
		{From: IdeaApproved, To: IdeaArchived},
		{From: IdeaImplementing, To: IdeaArchived},
		{From: IdeaSpecified, To: IdeaArchived},
	},
	KindFeature: {
		{From: FeatureDraft, To: FeatureUnderReview},
		{From: FeatureDraft, To: FeatureApproved},
		{From: FeatureUnderReview, To: FeatureApproved},
		{From: FeatureApproved, To: FeatureImplementing},
		{From: FeatureImplementing, To: FeatureStable},
		{From: FeatureStable, To: FeatureDeprecated},
	},
}

// allKindStatuses lists every status that should appear as either From or To
// in the kind's matrix. Used to exhaustively enumerate ILLEGAL transitions
// (every (from, to) where (from, to) is NOT in expectedLegal[kind]).
var allKindStatuses = map[Kind][]Status{
	KindIdea: {
		IdeaDraft,
		IdeaUnderReview,
		IdeaApproved,
		IdeaImplementing,
		IdeaSpecified,
		IdeaArchived,
	},
	KindFeature: {
		FeatureDraft,
		FeatureUnderReview,
		FeatureApproved,
		FeatureImplementing,
		FeatureStable,
		FeatureDeprecated,
	},
}

// ---------------------------------------------------------------------------
// Tests for Transition: legal AND illegal triples.
// ---------------------------------------------------------------------------

func TestTransition_LegalArcs(t *testing.T) {
	t.Parallel()
	count := 0
	for kind, rows := range expectedLegal {
		for _, r := range rows {
			count++
			if err := Transition(kind, r.From, r.To); err != nil {
				t.Errorf("Transition(%q, %q → %q) returned error, want nil: %v",
					kind, r.From, r.To, err)
			}
		}
	}
	if count == 0 {
		t.Fatal("expectedLegal is empty; no legal arcs to test")
	}
	t.Logf("verified %d legal arcs across all kinds", count)
}

func TestTransition_IllegalArcs(t *testing.T) {
	t.Parallel()
	count := 0
	for kind, statuses := range allKindStatuses {
		legalSet := make(map[transitionRow]bool, len(expectedLegal[kind]))
		for _, r := range expectedLegal[kind] {
			legalSet[r] = true
		}
		for _, from := range statuses {
			for _, to := range statuses {
				if legalSet[transitionRow{From: from, To: to}] {
					continue
				}
				count++
				err := Transition(kind, from, to)
				if err == nil {
					t.Errorf("Transition(%q, %q → %q) returned nil, want ErrInvalidTransition",
						kind, from, to)
					continue
				}
				if !errors.Is(err, ErrInvalidTransition) {
					t.Errorf("Transition(%q, %q → %q): err does not wrap ErrInvalidTransition: %v",
						kind, from, to, err)
				}
				var ite *InvalidTransitionError
				if !errors.As(err, &ite) {
					t.Errorf("Transition(%q, %q → %q): err is not *InvalidTransitionError: %T",
						kind, from, to, err)
					continue
				}
				if ite.Kind != kind || ite.From != from || ite.To != to {
					t.Errorf("InvalidTransitionError context wrong: got (%q, %q, %q), want (%q, %q, %q)",
						ite.Kind, ite.From, ite.To, kind, from, to)
				}
			}
		}
	}
	if count == 0 {
		t.Fatal("no illegal arcs enumerated; test logic broken")
	}
	t.Logf("verified %d illegal arcs across all kinds", count)
}

// TestTransition_UnknownKind covers the defensive branch for a kind that has
// no entry in transitionMatrix.
func TestTransition_UnknownKind(t *testing.T) {
	t.Parallel()
	err := Transition(Kind("totally-bogus"), Status("X"), Status("Y"))
	if err == nil {
		t.Fatal("Transition with unknown kind returned nil error")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err does not wrap ErrInvalidTransition: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests for LegalTargets / LegalSources / LegalStatuses.
// ---------------------------------------------------------------------------

func TestLegalTargets_AllSources(t *testing.T) {
	t.Parallel()
	for kind, rows := range expectedLegal {
		want := map[Status][]Status{}
		for _, r := range rows {
			want[r.From] = append(want[r.From], r.To)
		}
		for from, ts := range want {
			sort.Slice(ts, func(i, j int) bool { return string(ts[i]) < string(ts[j]) })
			got := LegalTargets(kind, from)
			if !reflect.DeepEqual(got, ts) {
				t.Errorf("LegalTargets(%q, %q) = %v, want %v", kind, from, got, ts)
			}
		}
	}
}

// Archived for Idea / Deprecated for Feature are terminal: no legal targets
// exist FROM them. Verify the function returns an empty (non-nil) slice.
func TestLegalTargets_Terminal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind Kind
		from Status
	}{
		{KindIdea, IdeaArchived},
		{KindFeature, FeatureDeprecated},
	}
	for _, c := range cases {
		got := LegalTargets(c.kind, c.from)
		if got == nil {
			t.Errorf("LegalTargets(%q, %q) returned nil, want empty []Status", c.kind, c.from)
		}
		if len(got) != 0 {
			t.Errorf("LegalTargets(%q, %q) = %v, want empty", c.kind, c.from, got)
		}
	}
}

func TestLegalSources(t *testing.T) {
	t.Parallel()
	for kind, rows := range expectedLegal {
		want := map[Status][]Status{}
		for _, r := range rows {
			want[r.To] = append(want[r.To], r.From)
		}
		for to, ss := range want {
			sort.Slice(ss, func(i, j int) bool { return string(ss[i]) < string(ss[j]) })
			got := LegalSources(kind, to)
			if !reflect.DeepEqual(got, ss) {
				t.Errorf("LegalSources(%q, %q) = %v, want %v", kind, to, got, ss)
			}
		}
	}
}

func TestLegalStatuses(t *testing.T) {
	t.Parallel()
	for kind, want := range allKindStatuses {
		sorted := append([]Status(nil), want...)
		sort.Slice(sorted, func(i, j int) bool { return string(sorted[i]) < string(sorted[j]) })
		got := LegalStatuses(kind)
		if !reflect.DeepEqual(got, sorted) {
			t.Errorf("LegalStatuses(%q) = %v, want %v", kind, got, sorted)
		}
	}
}

func TestLegalStatuses_UnknownKind(t *testing.T) {
	t.Parallel()
	got := LegalStatuses(Kind("totally-bogus"))
	if got == nil {
		t.Error("LegalStatuses(unknown) returned nil, want empty []Status")
	}
	if len(got) != 0 {
		t.Errorf("LegalStatuses(unknown) = %v, want empty", got)
	}
}

// TestLegalStatuses_ReturnsCopy guards against callers mutating the
// package-level slice via the return value.
func TestLegalStatuses_ReturnsCopy(t *testing.T) {
	t.Parallel()
	got := LegalStatuses(KindIdea)
	if len(got) == 0 {
		t.Fatal("LegalStatuses(KindIdea) returned empty slice")
	}
	got[0] = Status("MUTATED")
	got2 := LegalStatuses(KindIdea)
	if got2[0] == Status("MUTATED") {
		t.Error("LegalStatuses mutation leaked to package-level state")
	}
}

// ---------------------------------------------------------------------------
// ParseStatus: case-insensitive flag parsing.
// ---------------------------------------------------------------------------

func TestParseStatus_CaseInsensitive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind  Kind
		raw   string
		want  Status
		ok    bool
		label string
	}{
		// Exact match.
		{KindIdea, "Draft", IdeaDraft, true, "exact"},
		{KindIdea, "Approved", IdeaApproved, true, "exact"},
		// Lower-case.
		{KindIdea, "draft", IdeaDraft, true, "lower"},
		{KindFeature, "stable", FeatureStable, true, "lower"},
		// Upper-case.
		{KindFeature, "DEPRECATED", FeatureDeprecated, true, "upper"},
		// Mixed.
		{KindIdea, "ApPrOvEd", IdeaApproved, true, "mixed"},
		// Multi-word.
		{KindIdea, "under review", IdeaUnderReview, true, "lower multi-word"},
		{KindIdea, "UNDER REVIEW", IdeaUnderReview, true, "upper multi-word"},
		{KindIdea, "Under Review", IdeaUnderReview, true, "exact multi-word"},
		{KindFeature, "Under Review", FeatureUnderReview, true, "feature multi-word"},
		// Whitespace tolerance.
		{KindIdea, "  Draft  ", IdeaDraft, true, "padded"},
		{KindIdea, "\tApproved\t", IdeaApproved, true, "tab-padded"},
		// Negative cases.
		{KindIdea, "", "", false, "empty"},
		{KindIdea, "Bogus", "", false, "unknown name"},
		{KindIdea, "Stable", "", false, "wrong kind: Stable is Feature-only"},
		{KindFeature, "Specified", "", false, "wrong kind: Specified is Idea-only"},
		{KindIdea, "Underreview", "", false, "missing space in multi-word"},
		{Kind("bogus-kind"), "Draft", "", false, "unknown kind"},
	}
	for _, c := range cases {
		got, ok := ParseStatus(c.kind, c.raw)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseStatus(%q, %q) [%s] = (%q, %v), want (%q, %v)",
				c.kind, c.raw, c.label, got, ok, c.want, c.ok)
		}
	}
}

// ---------------------------------------------------------------------------
// validateMatrix: the not-idempotent init-time invariant.
//
// Per the task spec: "test asserts the panic by constructing a deliberately
// bad table in a test-internal helper — e.g. a helper that calls a private
// validateMatrix(matrix) function with a bad row, NOT by mutating the
// production matrix."
// ---------------------------------------------------------------------------

func TestValidateMatrix_RejectsSelfLoop(t *testing.T) {
	t.Parallel()
	bad := []transitionRow{
		{From: Status("Draft"), To: Status("Approved")},
		{From: Status("Approved"), To: Status("Approved")}, // self-loop!
	}
	err := validateMatrix(bad)
	if err == nil {
		t.Fatal("validateMatrix accepted a self-loop row")
	}
	if !strings.Contains(err.Error(), "self-loop") {
		t.Errorf("validateMatrix error does not mention self-loop: %v", err)
	}
}

func TestValidateMatrix_AcceptsLegalTable(t *testing.T) {
	t.Parallel()
	for kind, rows := range expectedLegal {
		if err := validateMatrix(rows); err != nil {
			t.Errorf("validateMatrix(%q rows) returned error on production table: %v", kind, err)
		}
	}
}

// TestValidateMatrix_PanicSimulation demonstrates that init() would panic if a
// bad table were registered. We simulate the exact recover() + panic()
// signature init uses, without touching the production map.
func TestValidateMatrix_PanicSimulation(t *testing.T) {
	t.Parallel()
	bad := []transitionRow{
		{From: Status("Draft"), To: Status("Draft")},
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("simulated init did not panic on self-loop")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value is not string: %T = %v", r, r)
		}
		if !strings.Contains(msg, "self-loop") {
			t.Errorf("panic message does not mention self-loop: %s", msg)
		}
	}()

	// Reproduce the init-time check exactly.
	if err := validateMatrix(bad); err != nil {
		panic("lifecycle: transition matrix for kind \"simulated\" is invalid: " + err.Error())
	}
	t.Fatal("validateMatrix did not return an error for self-loop row")
}

// TestProductionMatrix_NoSelfLoops is the belt-and-suspenders direct
// verification of REQ: not-idempotent against the actual production table.
func TestProductionMatrix_NoSelfLoops(t *testing.T) {
	t.Parallel()
	for kind, rows := range transitionMatrix {
		for _, r := range rows {
			if r.From == r.To {
				t.Errorf("production matrix for kind %q contains self-loop %q → %q",
					kind, r.From, r.To)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Validate: combines readStatus + Transition. Uses a synthetic Idea file.
// ---------------------------------------------------------------------------

const ideaFixture = "# Idea: Sample\n\n**Status:** Draft\n**Date:** 2026-01-01\n**Owner:** alice\n\n## Problem Statement\n\nSomething to fix.\n"

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestValidate_LegalTransition(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ideaFixture)
	from, err := Validate(KindIdea, path, IdeaApproved)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if from != IdeaDraft {
		t.Errorf("Validate returned from=%q, want %q", from, IdeaDraft)
	}
}

func TestValidate_IllegalTransition(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ideaFixture)
	// Idea: Draft → Stable is bogus (Stable doesn't exist for Idea).
	from, err := Validate(KindIdea, path, Status("Stable"))
	if err == nil {
		t.Fatal("Validate accepted illegal transition Draft → Stable for Idea")
	}
	if from != IdeaDraft {
		t.Errorf("Validate returned from=%q, want %q (even on failure)", from, IdeaDraft)
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err does not wrap ErrInvalidTransition: %v", err)
	}
}

func TestValidate_MissingStatusLine(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, "# Idea: Bogus\n\nNo status here.\n")
	_, err := Validate(KindIdea, path, IdeaApproved)
	if !errors.Is(err, ErrStatusLineNotFound) {
		t.Errorf("Validate err = %v, want ErrStatusLineNotFound", err)
	}
}

func TestValidate_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := Validate(KindIdea, filepath.Join(t.TempDir(), "does-not-exist.md"), IdeaApproved)
	if err == nil {
		t.Fatal("Validate accepted missing file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Validate err = %v, want os.IsNotExist", err)
	}
}

// ---------------------------------------------------------------------------
// Rewrite + Rollback round-trip.
// ---------------------------------------------------------------------------

func TestRewrite_DraftToApproved(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ideaFixture)
	origLine, err := Rewrite(path, IdeaApproved)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if !strings.Contains(origLine, "Draft") {
		t.Errorf("returned originalStatusLine missing Draft: %q", origLine)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after Rewrite: %v", err)
	}
	want := strings.Replace(ideaFixture, "**Status:** Draft", "**Status:** Approved", 1)
	if string(got) != want {
		t.Errorf("file after Rewrite is not byte-identical except for status line.\nGot:\n%q\nWant:\n%q", got, want)
	}
}

func TestRewriteRollback_RoundTrip(t *testing.T) {
	t.Parallel()
	path := writeFixture(t, ideaFixture)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	origLine, err := Rewrite(path, IdeaApproved)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	// Sanity-check the file actually changed.
	mutated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, mutated) {
		t.Fatal("Rewrite did not change the file")
	}

	if err := Rollback(path, origLine); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("Rollback did not restore byte-identical content.\nBefore:\n%q\nAfter:\n%q", before, after)
	}
}

// TestRewrite_PreservesCRLF guarantees REQ: status-line-rewrite holds for
// files using Windows-style line endings.
func TestRewrite_PreservesCRLF(t *testing.T) {
	t.Parallel()
	content := "# Idea: Sample\r\n\r\n**Status:** Draft\r\n**Owner:** alice\r\n"
	path := writeFixture(t, content)
	origLine, err := Rewrite(path, IdeaApproved)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(content, "**Status:** Draft", "**Status:** Approved", 1)
	if string(got) != want {
		t.Errorf("CRLF file not preserved byte-for-byte except for status value.\nGot:\n%q\nWant:\n%q", got, want)
	}
	if !strings.HasSuffix(origLine, "\r\n") {
		t.Errorf("originalStatusLine lost CRLF terminator: %q", origLine)
	}
}

// TestRewrite_PreservesTrailingWhitespace ensures the rewrite preserves
// trailing whitespace on the status line (a subtle byte-identity case).
func TestRewrite_PreservesTrailingWhitespace(t *testing.T) {
	t.Parallel()
	// Use spaces after the status value AND a leading indent (atypical but
	// the contract says we preserve it).
	content := "# Idea: Sample\n\n  **Status:** Draft   \n**Owner:** alice\n"
	path := writeFixture(t, content)
	if _, err := Rewrite(path, IdeaApproved); err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Idea: Sample\n\n  **Status:** Approved   \n**Owner:** alice\n"
	if string(got) != want {
		t.Errorf("Rewrite did not preserve indent/trailing-space.\nGot:\n%q\nWant:\n%q", got, want)
	}
}

func TestRewrite_NoStatusLine(t *testing.T) {
	t.Parallel()
	content := "# Idea: Sample\n\nNo header.\n"
	path := writeFixture(t, content)
	_, err := Rewrite(path, IdeaApproved)
	if !errors.Is(err, ErrStatusLineNotFound) {
		t.Errorf("Rewrite err = %v, want ErrStatusLineNotFound", err)
	}
	// File must be untouched.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Errorf("Rewrite mutated file despite missing status line.\nGot:\n%q\nWant:\n%q", got, content)
	}
}

func TestRollback_NoStatusLine(t *testing.T) {
	t.Parallel()
	content := "# Idea: Sample\n\nNo header.\n"
	path := writeFixture(t, content)
	err := Rollback(path, "**Status:** Draft\n")
	if !errors.Is(err, ErrStatusLineNotFound) {
		t.Errorf("Rollback err = %v, want ErrStatusLineNotFound", err)
	}
}

// TestRewrite_FeatureFixture covers a Feature README (no kind dependency in
// Rewrite, but we want to be sure the regex matches the canonical Feature
// README shape).
func TestRewrite_FeatureFixture(t *testing.T) {
	t.Parallel()
	content := "# Feature: Sample\n\n**Status:** Draft\n**Deps:** —\n\n## Summary\n"
	path := writeFixture(t, content)
	if _, err := Rewrite(path, FeatureUnderReview); err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(content, "**Status:** Draft", "**Status:** Under Review", 1)
	if string(got) != want {
		t.Errorf("Feature Rewrite not byte-clean.\nGot:\n%q\nWant:\n%q", got, want)
	}
}
