package telemetry

import (
	"errors"
	"strings"
	"testing"
)

func TestScrubFrame_PathToBasename(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantBase string
	}{
		{"unix-deep-path", "/Users/alice/projects/secret-project/internal/feature/feature.go", "feature.go"},
		{"linux-home", "/home/bob/repo/main.go", "main.go"},
		{"windows-backslashes", `C:\Users\carol\code\file.go`, "file.go"},
		{"already-bare", "file.go", "file.go"},
		{"path-with-newline", "/some/evil\npath/file.go", "file.go"},
		{"path-with-carriage-return", "/foo\rbar/baz.go", "baz.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, line, fn := ScrubFrame(tc.input, 42, "pkg.Function")
			if base != tc.wantBase {
				t.Errorf("basename = %q, want %q", base, tc.wantBase)
			}
			if line != 42 {
				t.Errorf("line = %d, want 42 (preserved verbatim)", line)
			}
			if fn != "pkg.Function" {
				t.Errorf("function = %q, want pkg.Function (preserved verbatim)", fn)
			}
		})
	}
}

func TestScrubFrame_BasenameNeverLeaksPathPrefix(t *testing.T) {
	// Defense-in-depth: the scrubber's output MUST never contain a leading
	// slash, Windows drive letter, or known PII path-prefix substring.
	adversarial := []string{
		"/Users/alice/something",
		"/home/bob/something",
		`C:\Users\carol\something`,
		"/private/var/folders/abc/T/something",
		"/Users/SECRET_USER_NAME/repo/file.go",
	}
	for _, p := range adversarial {
		base, _, _ := ScrubFrame(p, 1, "")
		if strings.HasPrefix(base, "/") {
			t.Errorf("basename %q starts with slash from input %q", base, p)
		}
		if strings.Contains(base, "/Users/") || strings.Contains(base, "/home/") {
			t.Errorf("basename %q leaks path prefix from input %q", base, p)
		}
		if strings.Contains(base, "SECRET_USER_NAME") {
			t.Errorf("basename %q leaks user-name segment from input %q", base, p)
		}
	}
}

func TestScrubMessage_PlainStringPanicIsUnscrubbed(t *testing.T) {
	got, isUnscrubbed := ScrubMessage("anything user said")
	if !isUnscrubbed {
		t.Errorf("plain string panic should be unscrubbed")
	}
	if got != UnscrubbedPanicMessage {
		t.Errorf("messageID = %q, want %q", got, UnscrubbedPanicMessage)
	}
}

func TestScrubMessage_UnwrappedErrorIsUnscrubbed(t *testing.T) {
	err := errors.New("failed to load spec /Users/alice/secret-project/foo.md")
	got, isUnscrubbed := ScrubMessage(err)
	if !isUnscrubbed {
		t.Errorf("unwrapped error should be unscrubbed")
	}
	if got != UnscrubbedPanicMessage {
		t.Errorf("messageID = %q, want %q", got, UnscrubbedPanicMessage)
	}
	// Confirm the user-authored content does not leak through.
	if strings.Contains(got, "alice") || strings.Contains(got, "secret-project") {
		t.Errorf("messageID leaked user content: %q", got)
	}
}

func TestScrubMessage_NilIsUnscrubbed(t *testing.T) {
	got, isUnscrubbed := ScrubMessage(nil)
	if !isUnscrubbed || got != UnscrubbedPanicMessage {
		t.Errorf("nil should classify as unscrubbed; got (%q, %v)", got, isUnscrubbed)
	}
}

func TestScrubMessage_SafePanicWithAllowlistedIDIsVerbatim(t *testing.T) {
	wrapped := errors.New("failed to load spec /Users/alice/secret-project/foo.md")
	payload := SafePanic(testKnownID, wrapped)
	got, isUnscrubbed := ScrubMessage(payload)
	if isUnscrubbed {
		t.Errorf("allowlisted SafePanic should NOT be unscrubbed")
	}
	if got != testKnownID {
		t.Errorf("messageID = %q, want %q (verbatim)", got, testKnownID)
	}
	// Critical: the wrapped error's content MUST NOT influence the
	// returned messageID — the allowlisted ID is sent verbatim and the
	// wrapped err is discarded by the transmit callback.
	if strings.Contains(got, "alice") || strings.Contains(got, "secret-project") {
		t.Errorf("verbatim messageID leaked wrapped err content: %q", got)
	}
}

func TestScrubMessage_SafePanicWithUnknownIDIsUnscrubbed(t *testing.T) {
	payload := SafePanic("not-in-the-allowlist", nil)
	got, isUnscrubbed := ScrubMessage(payload)
	if !isUnscrubbed {
		t.Errorf("SafePanic with unknown messageID should fall back to unscrubbed")
	}
	if got != UnscrubbedPanicMessage {
		t.Errorf("messageID = %q, want %q", got, UnscrubbedPanicMessage)
	}
}

func TestSafePanicPayload_ErrorInterface(t *testing.T) {
	// Sanity: the payload satisfies the error interface and Unwrap works.
	inner := errors.New("inner")
	payload := SafePanic("some-id", inner)
	var got error = payload
	if got.Error() != "some-id: inner" {
		t.Errorf("Error() = %q, want %q", got.Error(), "some-id: inner")
	}
	if errors.Unwrap(payload) != inner {
		t.Errorf("Unwrap() did not return the wrapped error")
	}
}

func TestIsSafeMessageID_TestOnlyEntryRegistered(t *testing.T) {
	// The _test.go-guarded init() registers testKnownID; confirms the
	// _test.go isolation pattern works.
	if !IsSafeMessageID(testKnownID) {
		t.Errorf("test-only messageID %q should be allowlisted during test runs", testKnownID)
	}
}
