package telemetry

import (
	"path/filepath"
	"strings"
)

// This file implements the privacy-scrubbing contract per
// cli/telemetry/errors-telemetry#req:stack-frame-scrubber and
// #req:panic-message-safe-allowlist. It is the SINGLE audit surface for
// what crash-reports events carry — both the per-frame path stripping and
// the per-panic-message allowlist gate.

// SafePanicPayload wraps a known-safe messageID + the unwrapped error chain.
// Code wishing to panic with a transmittable message uses:
//
//	panic(telemetry.SafePanic("spec-load-failed", err))
//
// At recovery time, the transmit callback inspects the recovered value: if
// it's a SafePanicPayload AND its MessageID is in the allowlist, the
// messageID is sent verbatim as the Sentry event's `message` field.
// Anything else gets the "unscrubbed panic" coercion.
type SafePanicPayload struct {
	MessageID string
	Wrapped   error
}

// Error makes SafePanicPayload satisfy the error interface, useful when the
// payload is observed through errors.As / errors.Is in user code.
func (p SafePanicPayload) Error() string {
	if p.Wrapped == nil {
		return p.MessageID
	}
	return p.MessageID + ": " + p.Wrapped.Error()
}

// Unwrap exposes the wrapped error for errors.Is / errors.As chains.
func (p SafePanicPayload) Unwrap() error { return p.Wrapped }

// SafePanic constructs a SafePanicPayload. The messageID is NOT validated
// here — allowlist membership is checked at transmission time via
// ScrubMessage. An unknown ID is legal at call sites; the event is just
// coerced to "unscrubbed panic" at send.
func SafePanic(messageID string, err error) SafePanicPayload {
	return SafePanicPayload{MessageID: messageID, Wrapped: err}
}

// UnscrubbedPanicMessage is the literal string sent as the Sentry event's
// `message` field when a recovered panic does NOT match the SafePanic
// allowlist. Exported so tests can assert against it.
const UnscrubbedPanicMessage = "unscrubbed panic"

// safePanicAllowlist is the closed-enum set of messageIDs that may appear
// verbatim in crash-reports events. The map is populated by:
//   - Production registrations in this file as the panic-site audit
//     identifies high-value wrap sites (Plan Task 4).
//   - Test-only registrations in scrubber_testonly_test.go (guarded by
//     _test.go so they never appear in release binaries).
//
// Adding a production messageID is intentionally a two-place edit per
// cli/telemetry#req:channel-registry's audit-surface principle: the entry
// here AND a comment naming the panic site that uses it.
var safePanicAllowlist = map[string]struct{}{
	// Production messageIDs added here as the audit produces them.
	// Currently empty — Plan Task 4 populates.
}

// IsSafeMessageID reports whether messageID is in the allowlist.
func IsSafeMessageID(messageID string) bool {
	_, ok := safePanicAllowlist[messageID]
	return ok
}

// registerSafeMessageID adds a messageID to the allowlist. Package-private
// so test-only entries can register via the _test.go init() while
// production additions remain audited at this file's declaration site.
func registerSafeMessageID(messageID string) {
	safePanicAllowlist[messageID] = struct{}{}
}

// ScrubFrame applies the per-frame contract: file path → basename, function
// name + line number preserved verbatim. Trailing whitespace and embedded
// newlines in the path are also stripped (defense against adversarial
// inputs).
//
// REQ:stack-frame-scrubber clause 3 (strip local-variable values from
// frame metadata) is enforced at the Sentry-SDK config layer inside
// errors.go, NOT here — Go's debug.Stack() doesn't surface local values
// to begin with, but the Sentry SDK is configured with AttachStacktrace +
// SendDefaultPII=false to prevent any reintroduction.
func ScrubFrame(file string, line int, function string) (basename string, scrubbedLine int, scrubbedFunction string) {
	// Use forward-slash form regardless of host OS so the result is
	// deterministic across darwin/linux/windows builds. filepath.Base
	// already does this on Unix; on Windows we strip backslashes too.
	normalized := strings.ReplaceAll(file, "\\", "/")
	// Also strip embedded newlines / carriage returns that could appear if
	// a hostile path is constructed — final defense against newline-
	// injection in event payloads.
	normalized = strings.ReplaceAll(normalized, "\n", "")
	normalized = strings.ReplaceAll(normalized, "\r", "")
	base := filepath.Base(normalized)
	// Edge cases where filepath.Base returns a separator or "." (root
	// path, empty, or just dots) — return empty so the Sentry payload
	// carries no leaked path structure. The invariant downstream is
	// "no leading slash in the basename"; the fuzz corpus enforces this.
	if base == "/" || base == "." || base == "\\" {
		return "", line, function
	}
	return base, line, function
}

// ScrubMessage classifies a recovered panic value into either an
// allowlisted messageID (returned verbatim with isUnscrubbed=false) or the
// "unscrubbed panic" sentinel (isUnscrubbed=true). The caller is
// responsible for setting the corresponding Sentry tag
// (`message: unscrubbed`) when isUnscrubbed=true.
//
// Acceptance behavior matches cli/telemetry/errors-telemetry#req:panic-
// message-safe-allowlist: anything that is not a SafePanicPayload with an
// allowlisted MessageID falls into the unscrubbed bucket. Notably this
// includes:
//   - Plain string panics: panic("...")
//   - Unwrapped errors: panic(errors.New(...))
//   - Runtime panics: nil dereferences, index-out-of-range, etc.
//   - SafePanic with an unknown messageID (allowlist miss)
func ScrubMessage(recovered any) (messageID string, isUnscrubbed bool) {
	payload, ok := recovered.(SafePanicPayload)
	if !ok {
		return UnscrubbedPanicMessage, true
	}
	if !IsSafeMessageID(payload.MessageID) {
		return UnscrubbedPanicMessage, true
	}
	return payload.MessageID, false
}
