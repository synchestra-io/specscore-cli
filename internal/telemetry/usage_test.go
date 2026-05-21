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

// TestResolveCaller covers cli/telemetry/usage-telemetry#ac:
// caller-resolution-precedence + caller-enum-coercion.
func TestResolveCaller(t *testing.T) {
	cases := []struct {
		name string
		flag string
		env  string
		want string
	}{
		{"default-when-neither-set", "", "", CallerCLI},
		{"flag-only", "claude", "", "claude"},
		{"env-only", "", "codex", "codex"},
		{"flag-beats-env", "claude", "codex", "claude"},
		{"empty-flag-falls-through-to-env", "", "aider", "aider"},
		{"unknown-coerces-to-other-via-flag", "my-custom-tag", "", CallerOther},
		{"unknown-coerces-to-other-via-env", "", "rando-agent", CallerOther},
		{"pi-dev-recognized", "pi.dev", "", "pi.dev"},
		{"antigravity-google-recognized", "antigravity.google", "", "antigravity.google"},
		{"amazon-q-recognized", "amazon-q", "", "amazon-q"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveCaller(tc.flag, tc.env)
			if got != tc.want {
				t.Errorf("ResolveCaller(%q, %q) = %q, want %q", tc.flag, tc.env, got, tc.want)
			}
		})
	}
}

// TestCallerEnumKnown_HasExpectedSize is a regression guard: the closed enum
// is documented in docs/telemetry.md as 20 values. If someone adds or removes
// a value without updating the doc table, this test fires.
func TestCallerEnumKnown_HasExpectedSize(t *testing.T) {
	const want = 20
	if got := len(CallerEnumKnown); got != want {
		t.Errorf("CallerEnumKnown has %d entries, expected %d; "+
			"if the change is intentional, update docs/telemetry.md table AND this test",
			got, want)
	}
}
