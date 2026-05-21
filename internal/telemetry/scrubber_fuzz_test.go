package telemetry

import (
	"strings"
	"testing"
)

// FuzzScrubFrame feeds adversarial paths to ScrubFrame and asserts the
// invariant that the returned basename never leaks the input's path
// structure or any conventional PII-bearing segment.
//
// Per cli/telemetry/errors-telemetry#req:scrubber-fuzz-tests this MUST run
// in CI as part of `go test ./...` (Go 1.18+ fuzzing). A failure
// represents a real privacy leak in the scrubber and MUST fail the build.
//
// The seed corpus covers: long Unix paths under /Users and /home, embedded
// PII-shaped substrings (secret, password, token, email, API key forms),
// multi-byte UTF-8 noise, embedded newlines and carriage returns,
// Windows-style backslash paths, and zero-length / single-character paths.
func FuzzScrubFrame(f *testing.F) {
	seeds := []string{
		"/Users/alice/projects/secret-project/file.go",
		"/home/bob/.config/private/repo/main.go",
		"/Users/...containing-the-word-secret.../file.go",
		`C:\Users\carol\code\file.go`,
		"file with spaces.go",
		"/tmp/⚡emoji-in-path/file.go",
		"/path/with\nembedded\nnewlines/file.go",
		"/Users/api-key-sk_live_abc123/file.go",
		"/Users/user@example.com/file.go",
		"",
		".",
		"/",
		"\x00\x01\x02",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Substrings that MUST NOT appear in any scrubbed basename. Each
	// represents a class of leakage the scrubber's basename-only contract
	// is supposed to prevent. Adding new banned substrings here
	// strengthens the invariant.
	banned := []string{
		"/Users/",
		"/home/",
		"/.config/",
		"\n",
		"\r",
	}

	f.Fuzz(func(t *testing.T, path string) {
		basename, _, _ := ScrubFrame(path, 0, "")

		// Invariant 1: result MUST NOT start with '/' or a Windows drive
		// letter pattern. A basename that does is by definition leaking
		// path structure.
		if strings.HasPrefix(basename, "/") {
			t.Errorf("basename starts with /: input=%q output=%q", path, basename)
		}
		if len(basename) >= 3 && basename[1] == ':' && basename[2] == '\\' {
			t.Errorf("basename retains Windows drive prefix: input=%q output=%q", path, basename)
		}

		// Invariant 2: result MUST NOT contain any of the banned
		// substrings.
		for _, b := range banned {
			if strings.Contains(basename, b) {
				t.Errorf("basename contains banned substring %q: input=%q output=%q",
					b, path, basename)
			}
		}

		// Invariant 3: result MUST NOT be longer than the input. Any
		// growth would indicate something exotic happened (e.g. the
		// scrubber accidentally annotated). filepath.Base on an
		// arbitrary string can return "." for some inputs; "." has
		// length 1 which is <= any non-empty input.
		if len(basename) > len(path)+1 {
			t.Errorf("basename longer than input by more than 1: input=%q (len %d) output=%q (len %d)",
				path, len(path), basename, len(basename))
		}
	})
}

// FuzzScrubMessage feeds arbitrary panic values to ScrubMessage and asserts
// the closed-output invariant: the returned messageID is either the
// UnscrubbedPanicMessage sentinel OR a string in the allowlist. No matter
// what was panicked, no user-authored text leaks.
//
// The fuzzer constructs SafePanicPayload values from string inputs to
// exercise both the allowlist-hit and allowlist-miss paths.
func FuzzScrubMessage(f *testing.F) {
	seeds := []string{
		"",
		"plain string panic with /Users/alice/secret",
		testKnownID,
		"some-unknown-id",
		"id-with-embedded\nnewline",
		"id-with-unicode-⚡",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		// Two paths to cover: the value as a plain string AND as a
		// SafePanic wrapping it as the messageID.

		// Path 1: plain string — always unscrubbed.
		got, isUnscrubbed := ScrubMessage(s)
		if !isUnscrubbed {
			t.Errorf("plain string should always be unscrubbed: input=%q output=%q", s, got)
		}
		if got != UnscrubbedPanicMessage {
			t.Errorf("plain string messageID should be %q: input=%q output=%q",
				UnscrubbedPanicMessage, s, got)
		}

		// Path 2: SafePanic(s, nil). Allowed iff s is in the allowlist.
		got2, isUnscrubbed2 := ScrubMessage(SafePanic(s, nil))
		if IsSafeMessageID(s) {
			if isUnscrubbed2 {
				t.Errorf("allowlisted SafePanic should NOT be unscrubbed: input=%q", s)
			}
			if got2 != s {
				t.Errorf("allowlisted messageID should be verbatim: input=%q output=%q", s, got2)
			}
		} else {
			if !isUnscrubbed2 {
				t.Errorf("non-allowlisted SafePanic should be unscrubbed: input=%q", s)
			}
			if got2 != UnscrubbedPanicMessage {
				t.Errorf("non-allowlisted messageID should be %q: input=%q output=%q",
					UnscrubbedPanicMessage, s, got2)
			}
		}
	})
}
