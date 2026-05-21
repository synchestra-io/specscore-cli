// Package telemetry is the single audit surface for all telemetry transmission
// from the specscore CLI. Every outbound telemetry HTTP request originates from
// this package; no other package may import a vendor telemetry SDK (see
// boundary_test.go for enforcement).
//
// The public Event struct is a closed-enum wrapper: the field set is fixed,
// and arbitrary string-keyed maps are NOT accepted from callers. Adding a key
// requires a code change in this package and a corresponding update to
// docs/telemetry.md. This is the implementation of
// cli/telemetry#req:fixed-event-property-keys.
package telemetry

// Event is the typed payload for a single CLI invocation. The ten primary
// fields below are the closed-enum property set enumerated in
// cli/telemetry#req:fixed-event-property-keys; the additional context fields
// (Panic, RecoveredAt) are dispatch hints consumed by per-channel transmit-fns
// to decide whether to send.
//
// Callers MUST construct Event by field name. The package does NOT expose any
// API that accepts map[string]any — see AC:fixed-event-property-keys-enforced-
// at-compile-time.
type Event struct {
	// Command is the cobra command path of the executed command, dot-separated
	// (e.g. "feature.create", "spec.lint"). Not space-separated.
	Command string

	// Success is true iff ExitCode == 0.
	Success bool

	// DurationMs is the wall-clock duration from PersistentPreRun start to
	// PersistentPostRun emission, in integer milliseconds.
	DurationMs int64

	// ExitCode is the integer exit code returned by the command.
	ExitCode int

	// CLIVersion is the version string from `specscore --version`, embedded at
	// build time via goreleaser.
	CLIVersion string

	// OS is runtime.GOOS (e.g. "darwin", "linux", "windows").
	OS string

	// Arch is runtime.GOARCH (e.g. "amd64", "arm64").
	Arch string

	// Caller is the resolved AI-agent identifier, per the usage-stats channel's
	// caller-resolution contract. Empty in invocations where no caller was set;
	// the usage-stats channel applies the closed-enum guard before transmission.
	Caller string

	// InstallID is the per-machine UUID v4 created on first invocation.
	InstallID string

	// IsFirstRun is true iff the install_id file was created during THIS
	// invocation.
	IsFirstRun bool

	// Panic carries a recovered panic value when a panic was caught by the
	// CLI's top-level recover handler. nil when no panic occurred. Consumed by
	// the crash-reports channel; ignored by the usage-stats channel.
	Panic *PanicInfo
}

// PanicInfo carries the context of a recovered panic for the crash-reports
// channel. The Stack field contains the raw output of runtime/debug.Stack().
// Scrubbing happens inside the crash-reports transmit-fn (see scrubber.go),
// NOT here — this struct is just the carrier.
type PanicInfo struct {
	// Value is the value passed to panic(). May be a string, an error, or a
	// telemetry.SafePanic payload (see scrubber.go) that wraps a safe
	// messageID.
	Value any

	// Stack is the unmodified output of runtime/debug.Stack() at the moment
	// of recovery, including the panicking goroutine's stack frames.
	Stack []byte
}
