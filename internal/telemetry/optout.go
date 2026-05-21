package telemetry

import "os"

// osGetenv is the default env-var reader: os.Getenv. Exposed as a tiny
// package-private alias so CollectOSEnvSignals can default-inject it.
var osGetenv = os.Getenv

// OptOutSignals carries the inputs to the opt-out evaluator. The struct lets
// callers (the cobra root) decouple signal collection from evaluation: the
// root reads flags + env vars + persistent state once and passes the bundle
// to ResolveOptOut. The evaluator is then a pure function — easy to test in
// isolation against the precedence matrix in cli/telemetry#ac:opt-out-precedence.
type OptOutSignals struct {
	// NoTelemetryFlag is true when the global --no-telemetry flag was set
	// on the current invocation. Highest-precedence signal.
	NoTelemetryFlag bool

	// SpecScoreTelemetryZero is true when env var SPECSCORE_TELEMETRY=0
	// (the literal string "0") was set.
	SpecScoreTelemetryZero bool

	// DoNotTrack is true when env var DO_NOT_TRACK=1 (the literal string
	// "1") was set — the rfc-ish convention.
	DoNotTrack bool

	// CIDetected is true when ANY of the recognised CI markers (CI=true,
	// GITHUB_ACTIONS=true, GITLAB_CI=true, BUILDKITE=true, CIRCLECI=true)
	// were set with their case-sensitive value match.
	CIDetected bool

	// PersistentState is the parsed ~/.specscore/telemetry.yaml content.
	// Zero-value when no preference set; ChannelEnabled() handles the
	// default-opt-in fallback.
	PersistentState State

	// PersistentStateInvalid is true when ~/.specscore/telemetry.yaml
	// existed but failed validation. When true, the evaluator MUST treat
	// every channel as disabled — corrupt config is a fail-safe.
	PersistentStateInvalid bool
}

// OptOutSource identifies which precedence rung caused a channel's decision.
// Used by `specscore telemetry status` to display the source per channel
// (cli/telemetry#ac:telemetry-subcommand-status).
type OptOutSource string

const (
	OptOutSourceFlag             OptOutSource = "flag"
	OptOutSourceEnvVar           OptOutSource = "env var"
	OptOutSourceCI               OptOutSource = "CI auto-disable"
	OptOutSourcePersistentState  OptOutSource = "persistent state"
	OptOutSourcePersistentBroken OptOutSource = "invalid persistent state — see stderr"
	OptOutSourceDefault          OptOutSource = "default"
)

// ChannelDecision is the per-channel result of opt-out evaluation. Source
// names the precedence rung that decided; Enabled is the final answer.
type ChannelDecision struct {
	Enabled bool
	Source  OptOutSource
}

// ResolveOptOut applies the 4-level precedence to produce one ChannelDecision
// per known channel. See cli/telemetry#req:opt-out-signal-precedence for the
// full contract:
//
//  1. --no-telemetry flag → ALL channels disabled (source: flag).
//  2. SPECSCORE_TELEMETRY=0 or DO_NOT_TRACK=1 → ALL channels disabled
//     (source: env var).
//  3. CI auto-disable (any recognised CI marker) → ALL channels disabled
//     (source: CI auto-disable).
//  4. Persistent state from ~/.specscore/telemetry.yaml → per-channel; if
//     the file is malformed, ALL channels disabled (source: invalid
//     persistent state).
//  5. Otherwise: ALL channels enabled (source: default).
//
// The `knownChannels` parameter is taken as a slice argument, NOT imported
// from the channel-registry package — this keeps the evaluator free of any
// code-time dependency on the registry. In practice callers pass
// RegisteredChannels(), but tests can pass any subset.
//
// REQ:opt-out-always-wins: a `--caller` value or SPECSCORE_CALLER env var
// MUST NOT re-enable a disabled channel. This function never consults
// caller signals — opt-out wins by virtue of not having that input.
func ResolveOptOut(sigs OptOutSignals, knownChannels []ChannelName) map[ChannelName]ChannelDecision {
	out := make(map[ChannelName]ChannelDecision, len(knownChannels))

	// Rung 1: --no-telemetry flag (global).
	if sigs.NoTelemetryFlag {
		for _, name := range knownChannels {
			out[name] = ChannelDecision{Enabled: false, Source: OptOutSourceFlag}
		}
		return out
	}
	// Rung 2: env-var opt-out (global).
	if sigs.SpecScoreTelemetryZero || sigs.DoNotTrack {
		for _, name := range knownChannels {
			out[name] = ChannelDecision{Enabled: false, Source: OptOutSourceEnvVar}
		}
		return out
	}
	// Rung 3: CI auto-disable (global).
	if sigs.CIDetected {
		for _, name := range knownChannels {
			out[name] = ChannelDecision{Enabled: false, Source: OptOutSourceCI}
		}
		return out
	}
	// Rung 4a: malformed persistent state — disable all as a fail-safe.
	if sigs.PersistentStateInvalid {
		for _, name := range knownChannels {
			out[name] = ChannelDecision{Enabled: false, Source: OptOutSourcePersistentBroken}
		}
		return out
	}
	// Rung 4b: persistent state per-channel.
	for _, name := range knownChannels {
		enabled, src := sigs.PersistentState.ChannelEnabled(name)
		out[name] = ChannelDecision{
			Enabled: enabled,
			Source:  classifyStateSource(src),
		}
	}
	return out
}

// classifyStateSource maps the freeform source string from State.ChannelEnabled
// into the OptOutSource enum. "default" → OptOutSourceDefault; anything else
// → OptOutSourcePersistentState.
func classifyStateSource(s string) OptOutSource {
	if s == "default" {
		return OptOutSourceDefault
	}
	return OptOutSourcePersistentState
}

// CollectOSEnvSignals reads the recognised env vars from the process
// environment and populates the env-var-related fields of OptOutSignals.
// Callers compose this with the flag and persistent-state inputs.
func CollectOSEnvSignals(getEnv func(string) string) (specZero bool, doNotTrack bool, ciDetected bool) {
	if getEnv == nil {
		getEnv = osGetenv
	}
	specZero = getEnv("SPECSCORE_TELEMETRY") == "0"
	doNotTrack = getEnv("DO_NOT_TRACK") == "1"
	// Case-sensitive value match per REQ:opt-out-signal-precedence step 3.
	ciMarkers := []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI"}
	for _, key := range ciMarkers {
		if getEnv(key) == "true" {
			ciDetected = true
			break
		}
	}
	return
}
