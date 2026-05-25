package telemetry

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlMarshal is a testable indirection over yaml.Marshal so that tests can
// inject marshaling failures to exercise the defensive error paths in
// WriteState.
var yamlMarshal = yaml.Marshal

// stateFilename is the YAML file under the SpecScore per-user directory that
// holds the user's persistent telemetry preferences. See
// cli/telemetry#req:persistent-state-file-shape for the schema.
const stateFilename = "telemetry.yaml"

// stateFileMode is the mode bits for the preferences file. Same as install_id
// — user-readable, not world-visible.
const stateFileMode = installIDFileMode

// stateSchemaPointer is the canonical schema-pointer comment that MUST appear
// on line 1 of every persistent state file written by the CLI. Read-time
// validation is lenient (we don't require the comment to match exactly when
// reading user-edited files); write-time emission is strict (we always emit
// this exact line).
const stateSchemaPointer = "# SpecScore Telemetry Preferences: https://specscore.md/telemetry-preferences"

// allowedStateKeys enumerates the closed-set of valid keys in the YAML root
// per cli/telemetry#req:persistent-state-file-shape. Adding a key requires a
// code change here AND a spec amendment to the parent Feature's REQ.
var allowedStateKeys = map[string]struct{}{
	"enabled":       {},
	"usage-stats":   {},
	"crash-reports": {},
}

// State is the in-memory shape of telemetry.yaml. A nil-valued pointer field
// means "key absent from the file" — distinct from "key present with false."
// The opt-out evaluator uses presence-vs-absent semantics to implement the
// per-channel-overrides-global rule.
type State struct {
	Enabled      *bool `yaml:"enabled,omitempty"`
	UsageStats   *bool `yaml:"usage-stats,omitempty"`
	CrashReports *bool `yaml:"crash-reports,omitempty"`
}

// StateReadResult is what ReadState returns to its caller. The InvalidReason
// field is non-empty iff the file existed but failed validation — callers
// MUST treat InvalidReason!="" the same as "telemetry disabled for this
// invocation" while continuing the user's command. The split between
// (File-Not-Found ⇒ no preference set) and (File-Malformed ⇒ disable + warn)
// implements cli/telemetry#ac:persistent-state-file-shape-rejected.
type StateReadResult struct {
	// State is the parsed preferences. Zero-value when the file doesn't exist
	// or when InvalidReason is non-empty.
	State State
	// FileExisted is true iff the file was readable (regardless of content
	// validity).
	FileExisted bool
	// InvalidReason names the validation failure (unknown key, wrong type,
	// non-YAML). Empty when State is usable.
	InvalidReason string
}

// StatePath returns the absolute path of telemetry.yaml. Used by tests and by
// the `specscore telemetry status` subcommand.
func StatePath() (string, error) {
	dir, err := userStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFilename), nil
}

// ReadState reads telemetry.yaml. See StateReadResult docs for the three
// outcomes:
//   - File absent (no preference set) → FileExisted=false, no error.
//   - File present and valid → State populated, FileExisted=true, no error.
//   - File present and malformed → FileExisted=true, InvalidReason set, no
//     error returned from ReadState (the malformed-file case is a user-
//     facing warning, not an I/O failure from the CLI's perspective).
//
// Genuine I/O errors (permission denied on a present file, etc.) are
// returned as the error result.
func ReadState() (StateReadResult, error) {
	path, err := StatePath()
	if err != nil {
		return StateReadResult{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StateReadResult{FileExisted: false}, nil
		}
		return StateReadResult{}, fmt.Errorf("reading %s: %w", path, err)
	}
	parsed, invalidReason := parseStateBytes(raw)
	return StateReadResult{
		State:         parsed,
		FileExisted:   true,
		InvalidReason: invalidReason,
	}, nil
}

// WriteState writes telemetry.yaml atomically with the canonical schema-
// pointer comment on line 1. Per cli/telemetry#req:persistent-state-file-
// shape, writes that would produce a non-conforming file (unknown keys we
// shouldn't be able to write in the first place — defense in depth) fail
// at write time with a stderr-bound error.
//
// Callers (`specscore telemetry enable|disable [channel]`) construct a
// State, possibly merging the freshly-read state with a single field
// change, and call WriteState. The function does NOT auto-create the file
// — only writes when the caller explicitly persists a preference.
func WriteState(s State) error {
	path, err := StatePath()
	if err != nil {
		return err
	}
	if mkErr := os.MkdirAll(filepath.Dir(path), installIDDirMode); mkErr != nil {
		return fmt.Errorf("creating state directory: %w", mkErr)
	}
	body, err := yamlMarshal(&s)
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	// `yaml.Marshal` produces just the document body. Prepend the schema
	// pointer comment.
	content := []byte(stateSchemaPointer + "\n" + string(body))
	// Defense-in-depth: re-parse what we're about to write and reject if
	// the result wouldn't pass our own read-time validation.
	if _, reason := parseStateBytes(content); reason != "" {
		return fmt.Errorf("refusing to write malformed state: %s", reason)
	}
	return atomicWriteFile(path, content, stateFileMode)
}

// parseStateBytes decodes the YAML and validates the key set. Returns
// (State, "") on success; (zero-State, reason) on validation failure. A
// genuine YAML syntax error counts as a validation failure with reason
// like "yaml syntax error: <detail>".
func parseStateBytes(raw []byte) (State, string) {
	// Read into a generic map first so we can detect unknown keys; yaml.v3's
	// KnownFields(true) on a Decoder also catches this, but we want a
	// caller-friendly reason string naming the offending key.
	var generic map[string]any
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&generic); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty file — treat as zero State (equivalent to no preferences).
			return State{}, ""
		}
		return State{}, "yaml syntax error: " + err.Error()
	}
	for key := range generic {
		if _, ok := allowedStateKeys[key]; !ok {
			return State{}, fmt.Sprintf("unknown key %q", key)
		}
	}
	// Re-decode into the typed struct now that keys are vetted. yaml.v3's
	// strict mode is engaged via KnownFields.
	var typed State
	strictDec := yaml.NewDecoder(strings.NewReader(string(raw)))
	strictDec.KnownFields(true)
	if err := strictDec.Decode(&typed); err != nil && !errors.Is(err, io.EOF) {
		return State{}, "type error: " + err.Error()
	}
	return typed, ""
}

// ChannelEnabled resolves the effective enabled state for a single channel
// from the persistent state alone. The full opt-out precedence (flag → env →
// CI → persistent state) is implemented by the opt-out evaluator (Task 4);
// this helper only answers the "what does the persistent state say about
// this channel?" question.
//
// Semantics:
//   - Per-channel override (UsageStats or CrashReports) takes precedence over
//     the global Enabled when set.
//   - When neither per-channel nor global is set, returns (true, "default")
//     meaning "default-opt-in applies."
//   - When per-channel is set, returns (*, "persistent state per-channel").
//   - When only global is set, returns (*, "persistent state global").
func (s State) ChannelEnabled(name ChannelName) (enabled bool, source string) {
	switch name {
	case ChannelUsageStats:
		if s.UsageStats != nil {
			return *s.UsageStats, "persistent state per-channel"
		}
	case ChannelCrashReports:
		if s.CrashReports != nil {
			return *s.CrashReports, "persistent state per-channel"
		}
	}
	if s.Enabled != nil {
		return *s.Enabled, "persistent state global"
	}
	return true, "default"
}
