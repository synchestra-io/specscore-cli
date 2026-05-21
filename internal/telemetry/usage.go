package telemetry

import (
	"context"
	"slices"

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

// CallerEnumKnown is the closed-enum set of recognized values for the `caller`
// event property per cli/telemetry/usage-telemetry#req:caller-enum-known-values.
// Any caller value not in this set is coerced to CallerOther before
// transmission. Order matches the spec's table; extending requires a code
// change here AND a spec amendment.
var CallerEnumKnown = []string{
	CallerCLI,
	CallerClaude,
	CallerCodex,
	CallerAider,
	CallerOpenCode,
	CallerGoose,
	CallerCursor,
	CallerGemini,
	CallerCopilot,
	CallerDevin,
	CallerCline,
	CallerRoo,
	CallerContinue,
	CallerWindsurf,
	CallerZed,
	CallerAmazonQ,
	CallerTabnine,
	CallerPiDev,
	CallerAntigravityGoogle,
	CallerOther,
}

// Caller constants. The user-facing values are the strings PostHog ever sees.
// CallerCLI is the default for invocations without --caller or
// SPECSCORE_CALLER set. CallerOther is the coercion target for any value not
// in CallerEnumKnown.
const (
	CallerCLI               = "cli"
	CallerClaude            = "claude"
	CallerCodex             = "codex"
	CallerAider             = "aider"
	CallerOpenCode          = "opencode"
	CallerGoose             = "goose"
	CallerCursor            = "cursor"
	CallerGemini            = "gemini"
	CallerCopilot           = "copilot"
	CallerDevin             = "devin"
	CallerCline             = "cline"
	CallerRoo               = "roo"
	CallerContinue          = "continue"
	CallerWindsurf          = "windsurf"
	CallerZed               = "zed"
	CallerAmazonQ           = "amazon-q"
	CallerTabnine           = "tabnine"
	CallerPiDev             = "pi.dev"
	CallerAntigravityGoogle = "antigravity.google"
	CallerOther             = "other"
)

// ResolveCaller computes the final caller value to attach to a usage-stats
// event, per cli/telemetry/usage-telemetry#req:caller-resolution. Precedence:
//
//  1. flagValue (from --caller on the current invocation)
//  2. envValue (from SPECSCORE_CALLER env var)
//  3. default literal "cli"
//
// The resolved string is then passed through the closed-enum coercion: if it
// matches a known value, return it verbatim; otherwise return CallerOther
// (REQ:caller-enum-known-values). The coercion happens here, NOT at the
// cobra-flag-parsing layer — the flag accepts arbitrary strings so a script
// passing --caller my-custom-tag never fails the user's command, only the
// transmitted value is constrained.
//
// Empty strings at either source are treated as absent (fall through to the
// next precedence rung).
func ResolveCaller(flagValue, envValue string) string {
	resolved := CallerCLI
	switch {
	case flagValue != "":
		resolved = flagValue
	case envValue != "":
		resolved = envValue
	}
	return coerceCaller(resolved)
}

// coerceCaller is the closed-enum guard. Any value not in CallerEnumKnown
// becomes CallerOther — never reaches PostHog with arbitrary content.
func coerceCaller(s string) string {
	if slices.Contains(CallerEnumKnown, s) {
		return s
	}
	return CallerOther
}

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
