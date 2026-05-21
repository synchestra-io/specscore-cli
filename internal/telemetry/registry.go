package telemetry

import (
	"context"
	"slices"
	"sync"
	"time"
)

// transmitTimeout is the hard upper bound on every external transmission path,
// per cli/telemetry#req:transmission-hard-timeout. On timeout, the in-flight
// transmission is dropped silently — no retry, no panic, no error returned to
// the caller. CLI behavior is identical whether the telemetry endpoint is
// reachable, timing out, or returning 5xx.
const transmitTimeout = 500 * time.Millisecond

// ChannelName is the closed-enum type for registered telemetry channels.
// New names require a code change in this package per
// cli/telemetry#req:channel-registry.
type ChannelName string

const (
	// ChannelUsageStats is the PostHog product-analytics channel, implemented
	// by internal/telemetry/usage.go and specified by cli/telemetry/
	// usage-telemetry.
	ChannelUsageStats ChannelName = "usage-stats"

	// ChannelCrashReports is the Sentry crash-reporting channel, implemented
	// by internal/telemetry/errors.go and specified by cli/telemetry/
	// errors-telemetry.
	ChannelCrashReports ChannelName = "crash-reports"
)

// knownChannelNames returns the closed-enum set in stable order. The list is
// the single source of truth that specscore telemetry status|enable|disable
// consults to enumerate channels (cli/telemetry#req:channel-registry).
func knownChannelNames() []ChannelName {
	return []ChannelName{ChannelUsageStats, ChannelCrashReports}
}

// TransmitFunc is the signature each channel's transmit callback must satisfy.
// Transmission failure is silent — channels do not propagate errors to the
// caller. Per-channel triggering decisions (e.g. crash-reports skipping when
// Event.Panic is nil and ExitCode < 10) happen inside the TransmitFunc body.
type TransmitFunc func(ctx context.Context, event Event)

var (
	registryMu sync.RWMutex
	registry   = map[ChannelName]TransmitFunc{}
)

// RegisterChannel attaches a channel's transmit-fn to the registry. Channels
// MUST register from a Go init() function. Panics on:
//   - an unknown channel name (one not in knownChannelNames())
//   - duplicate registration of the same channel
//
// The init()-time panic surfaces at startup, not at runtime, which makes
// invalid registrations a build/test-time failure rather than a user-visible
// crash.
func RegisterChannel(name ChannelName, transmit TransmitFunc) {
	if !isKnownChannel(name) {
		panic("telemetry: unknown channel: " + string(name))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[name]; exists {
		panic("telemetry: duplicate registration: " + string(name))
	}
	registry[name] = transmit
}

func isKnownChannel(name ChannelName) bool {
	return slices.Contains(knownChannelNames(), name)
}

// RegisteredChannels returns the sorted list of currently-registered channels.
// Used by `specscore telemetry status` and by Emit to iterate channels.
func RegisteredChannels() []ChannelName {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]ChannelName, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Emit transmits the event through every registered channel. Each channel's
// transmit-fn is invoked in its own goroutine bounded by transmitTimeout
// (500 ms hard cap, per cli/telemetry#req:transmission-hard-timeout). On
// timeout the goroutine continues to run but its result is discarded; the
// Event struct is the only thing callers may pass. A map[string]any payload
// MUST NOT compile — the function signature is the closed-enum boundary.
func Emit(ctx context.Context, event Event) {
	for _, name := range RegisteredChannels() {
		registryMu.RLock()
		transmit := registry[name]
		registryMu.RUnlock()
		if transmit == nil {
			continue
		}
		callBounded(ctx, transmit, event)
	}
}

// callBounded invokes a single transmit-fn with the per-transmission hard
// timeout. An inner deferred recover() guards against panics inside the
// transmit-fn so a scrubber bug or SDK panic can never mask the user's
// command exit code (see cli/telemetry/errors-telemetry#req:transmit-
// callback-must-not-mask-exit-code).
func callBounded(ctx context.Context, fn TransmitFunc, event Event) {
	done := make(chan struct{})
	timeoutCtx, cancel := context.WithTimeout(ctx, transmitTimeout)
	defer cancel()
	go func() {
		defer close(done)
		defer func() { _ = recover() }()
		fn(timeoutCtx, event)
	}()
	select {
	case <-done:
	case <-timeoutCtx.Done():
		// Drop the in-flight transmission silently.
	}
}
