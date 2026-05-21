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
// runs inside a 500 ms bounded goroutine with its own deferred recover(),
// so any panic here is caught silently and the user's command exit code is
// preserved. Task 3 elaborates the conditional emit + safety properties.
// This stub establishes the dispatch path and the no-op-when-no-DSN
// behavior; Task 3 + Task 4 fill in the trigger conditions and full event
// shape.
func transmitErrors(ctx context.Context, event Event) {
	_ = ctx
	_ = event
	if !errorsClientInitialized {
		return
	}
	// Full conditional-emit + Sentry CaptureEvent logic lands in Task 3
	// and Task 4. This commit only wires the package + registration.
}
