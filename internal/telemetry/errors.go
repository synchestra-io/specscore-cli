package telemetry

import (
	"context"

	"github.com/getsentry/sentry-go"
)

// This file is the SOLE place in the repo importing github.com/getsentry/
// sentry-go. boundary_test.go enforces the confinement at build time.
// All crash-reports transmission flows through transmitErrors registered
// below.

// sentryDSN is the Sentry project DSN, compiled into the binary via
// `-ldflags "-X internal/telemetry.sentryDSN=..."` during release builds
// per cli/telemetry/errors-telemetry#req:sentry-dsn-embedded-at-build-time.
// Declaration MUST live in this file so the vendor-SDK import-confinement
// audit surface stays intact.
//
// Empty value (dev build) ⇒ the registered transmit callback silently
// no-ops; the channel still registers so `specscore telemetry status`
// lists it. The DSN itself encodes the EU region: per
// REQ:sentry-eu-region the DSN form is
// https://<key>@<org>.ingest.de.sentry.io/<project-id>.
var sentryDSN = ""

// errorsClientInitialized records whether the Sentry SDK's package-level
// CurrentHub was successfully initialized at init() time. nil DSN or init
// error keeps this false; transmitErrors checks this before sending.
var errorsClientInitialized bool

// transmitErrorsTestPanic is a test-only injection point for verifying the
// defensive recover() in transmitErrors. Production code leaves this nil;
// tests in errors_test.go set it to a panicking function to confirm
// REQ:transmit-callback-must-not-mask-exit-code's invariant. Exposed at
// package level (rather than a build-tag-gated file) so it stays trivially
// observable — production callers never invoke it because transmitErrors
// only calls it under the test hook.
var transmitErrorsTestPanic func()

func init() {
	RegisterChannel(ChannelCrashReports, transmitErrors)
	if sentryDSN == "" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn: sentryDSN,
		// AttachStacktrace=true tells the SDK to attach the Go stack at
		// CaptureMessage / CaptureException call sites. We DO want stack
		// frames (the scrubber strips paths), so this stays true.
		AttachStacktrace: true,
		// SendDefaultPII=false explicitly disables the SDK's automatic
		// collection of OS user, hostname, and other locals from the
		// environment — implements REQ:stack-frame-scrubber clause 3
		// ("strip local-variable values from frame metadata ... disable
		// explicitly").
		SendDefaultPII: false,
		// Release tag is set per-event by transmitErrors (see Task 3)
		// using Event.CLIVersion. Leaving Release empty here lets the
		// per-event tag take effect.
	})
	if err != nil {
		// Init failure ⇒ stay no-op; the channel is still registered for
		// status discoverability.
		return
	}
	errorsClientInitialized = true
}

// transmitErrors converts a typed Event into a Sentry capture call. Honors
// the no-op contract when sentryDSN was not injected at build time.
//
// Per the registry contract (callBounded in registry.go), this function
// already runs inside a 500 ms bounded goroutine with an outer deferred
// recover() at the registry layer. The INNER defer recover() installed
// here is a defense-in-depth measure per
// cli/telemetry/errors-telemetry#req:transmit-callback-must-not-mask-exit-
// code: a scrubber bug, SDK panic, or malformed payload caught here is
// dropped silently, the in-goroutine error is suppressed, and the user's
// original exit code stays intact.
//
// The conditional emit logic (panic-recovered OR exit ≥10 only; panic
// priority; single event per invocation) lands in Task 4. This commit
// adds:
//   - the defensive recover (REQ:transmit-callback-must-not-mask-exit-code)
//   - the release-tag application from Event.CLIVersion
//     (REQ:sentry-release-tag)
func transmitErrors(ctx context.Context, event Event) {
	defer func() { _ = recover() }()
	if !errorsClientInitialized {
		return
	}
	_ = ctx
	// Test injection point — production callers leave this nil. If set,
	// the panic exercises the defer recover() above.
	if transmitErrorsTestPanic != nil {
		transmitErrorsTestPanic()
	}
	// Apply the release tag derived from Event.CLIVersion. Per
	// REQ:sentry-release-tag every event MUST carry release=cli_version
	// so Sentry correlates signatures to releases.
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("release", event.CLIVersion)
		// Conditional emit + ScrubFrame + ScrubMessage wiring lands in
		// Task 4. This commit only proves the release-tag application
		// path runs cleanly with the defensive recover in place.
	})
}
