package telemetry

import (
	"context"

	"github.com/posthog/posthog-go"
)

// This file is the SOLE place in the repo importing github.com/posthog/posthog-go.
// The boundary_test.go AST walk verifies this confinement at build time.
// All product-analytics transmission for the usage-stats channel flows through
// the transmitUsage callback registered here.

// posthogEUEndpoint is the PostHog EU regional ingest endpoint. Encoded as a
// constant per cli/telemetry/usage-telemetry#req:posthog-eu-region; changing
// region requires a code change.
const posthogEUEndpoint = "https://eu.i.posthog.com"

// usageStatsEventName is the literal PostHog event name every usage-stats
// emission uses, per cli/telemetry/usage-telemetry#req:usage-stats-event-name.
const usageStatsEventName = "cli.command.completed"

// posthogWriteKey is the PostHog project API key, compiled into the binary
// via Go's -ldflags "-X internal/telemetry.posthogWriteKey=..." during release
// builds per cli/telemetry/usage-telemetry#req:posthog-write-key-embedded-at-
// build-time. The declaration MUST live in this file so the vendor-SDK import-
// confinement audit surface (REQ:vendor-sdk-import-confinement) stays intact —
// the package-scoped Go variable is the ldflags target, but its declaration
// site is what the boundary test cares about.
//
// Empty value (developer build with no key injected) ⇒ the registered transmit
// callback silently no-ops; the channel still registers so `specscore telemetry
// status` lists it.
var posthogWriteKey = ""

// usageClient is the lazily-constructed PostHog client. nil when posthogWriteKey
// is empty.
var usageClient posthog.Client

func init() {
	RegisterChannel(ChannelUsageStats, transmitUsage)
	if posthogWriteKey == "" {
		return
	}
	client, err := posthog.NewWithConfig(posthogWriteKey, posthog.Config{
		Endpoint: posthogEUEndpoint,
	})
	if err != nil {
		// Init failure is non-fatal — the transmit-fn checks for nil and no-ops.
		return
	}
	usageClient = client
}

// transmitUsage converts a typed Event into a PostHog capture call. Honors
// the no-op contract when posthogWriteKey was not injected at build time.
//
// Per the registry contract (callBounded in registry.go), this function runs
// inside a 500 ms bounded goroutine with its own deferred recover(), so any
// panic here is caught silently and the user's command exit code is preserved.
//
// Per cli/telemetry/usage-telemetry#req:usage-stats-event-properties this
// function populates all 10 properties from the Event struct. The caller-enum
// coercion and the full property population body land in the next task; this
// stub establishes the dispatch path and the no-op-when-no-key behavior.
func transmitUsage(ctx context.Context, event Event) {
	_ = ctx
	if usageClient == nil {
		return
	}
	// Full Enqueue with property population lands with the event-emission task.
	// For now: register the channel, no-op the transmit unless the key is
	// present. When the key IS present and the client is up, we send a stub
	// capture so the AC:posthog-client-eu-region test can observe the
	// outbound HTTPS host. The full property set wires in next.
	_ = usageClient.Enqueue(posthog.Capture{
		DistinctId: event.InstallID,
		Event:      usageStatsEventName,
	})
}
