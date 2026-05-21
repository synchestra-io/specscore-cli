package telemetry

import (
	"context"
	"testing"
)

// TestTransmitErrors_DefensiveRecover covers cli/telemetry/errors-telemetry
// #ac:transmit-panic-does-not-mask-user-exit-code at the transmit-fn level.
// We can't easily inject a panic into the real Sentry SDK without
// replumbing dependency injection, but we CAN verify the function-level
// invariant: the defer recover() inside transmitErrors catches any panic
// originating from within its own body.
//
// Verification strategy: with errorsClientInitialized=false (dev build,
// empty DSN), transmitErrors returns at the first guard. To exercise the
// recover path, the test temporarily forces a panic via a test-only hook —
// the recover MUST suppress it and the test MUST return normally rather
// than failing with a goroutine panic.
func TestTransmitErrors_DefensiveRecover(t *testing.T) {
	// Save original state.
	origInit := errorsClientInitialized
	origHook := transmitErrorsTestPanic
	t.Cleanup(func() {
		errorsClientInitialized = origInit
		transmitErrorsTestPanic = origHook
	})

	// Force-enable the "initialized" guard and install a test hook that
	// panics from inside transmitErrors. The function's defer recover()
	// must catch this.
	errorsClientInitialized = true
	transmitErrorsTestPanic = func() {
		panic("simulated SDK panic for defensive-recover test")
	}

	// If the recover were missing, this call would panic the test
	// goroutine and the framework would print a goroutine traceback.
	transmitErrors(context.Background(), Event{
		Command:    "test.command",
		CLIVersion: "0.0.0-test",
		InstallID:  "test-id",
	})
	// Reaching here means the recover absorbed the panic. Assertion is
	// implicit: no panic propagation.
}

// TestTransmitErrors_NilWithInitializedClient_NoPanic confirms the
// release-tag path runs cleanly with the real Sentry SDK in init-on but
// no real DSN (we synthesize the initialized state). Because sentry.Init
// was not called with a real DSN, the SDK's internal Hub is the default
// no-op hub; sentry.WithScope still runs but no event ships.
func TestTransmitErrors_NilWithInitializedClient_NoPanic(t *testing.T) {
	orig := errorsClientInitialized
	t.Cleanup(func() { errorsClientInitialized = orig })
	errorsClientInitialized = true
	// Should not panic even with empty CLIVersion.
	transmitErrors(context.Background(), Event{})
}
