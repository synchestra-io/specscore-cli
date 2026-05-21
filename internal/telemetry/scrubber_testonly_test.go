package telemetry

// testKnownID is the mandatory test-only messageID per
// cli/telemetry/errors-telemetry#req:panic-message-safe-allowlist. Guarded
// by the _test.go file extension so it never appears in release binaries —
// production code in this package CANNOT reference it (the symbol does not
// exist outside test builds).
//
// Test code uses this constant via SafePanic(testKnownID, err) when it
// wants to exercise the allowlisted-message path of ScrubMessage.
const testKnownID = "test-known-id"

func init() {
	registerSafeMessageID(testKnownID)
}
