package telemetry

import (
	"context"
	"slices"
	"testing"
)

// TestUsageStatsChannel_Registered verifies that the usage-stats channel's
// init() ran and registered with the channel registry. Implements (part of)
// cli/telemetry/usage-telemetry#ac:usage-stats-channel-registered.
func TestUsageStatsChannel_Registered(t *testing.T) {
	registered := RegisteredChannels()
	if !slices.Contains(registered, ChannelUsageStats) {
		t.Errorf("expected %q in RegisteredChannels(), got %v", ChannelUsageStats, registered)
	}
}

// TestTransmitUsage_NoKeyIsNoOp covers cli/telemetry/usage-telemetry#ac:
// posthog-write-key-empty-no-op. With an empty posthogWriteKey, the
// usageClient is nil and transmitUsage MUST return without panicking and
// without doing any work.
//
// In dev builds, posthogWriteKey is naturally empty so this exercises the
// real code path. A direct integration test against a live PostHog endpoint
// would require an injected proxy and a real write key — that level of
// verification is deferred to the operational AC:posthog-funnel-defined
// release-checklist step.
func TestTransmitUsage_NoKeyIsNoOp(t *testing.T) {
	if posthogWriteKey != "" {
		t.Skipf("test is meaningful only when posthogWriteKey is empty (dev build); got %q", posthogWriteKey)
	}
	if usageClient != nil {
		t.Fatalf("expected usageClient to be nil with empty write key, got %v", usageClient)
	}
	// Should not panic and should return immediately.
	transmitUsage(context.Background(), Event{
		Command:    "feature.list",
		InstallID:  "test-install-id",
		CLIVersion: "0.0.0-test",
	})
}
