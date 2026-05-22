package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/specscore/specscore-cli/pkg/event"
	"github.com/specscore/specscore-cli/pkg/exitcode"
	"github.com/specscore/specscore-cli/pkg/gitremote"
	"github.com/spf13/cobra"
)

// eventCommand returns the "event" command group — emits SpecScore events
// onto the JSONL bus and (in later tasks) dispatches them to configured
// subscribers. See spec/features/cli/event/README.md.
func eventCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Event emission — append SpecScore events to .specscore/events.jsonl",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(eventEmitCommand())
	return cmd
}

// eventEmitCommand returns `specscore event emit` — the user-facing emission
// verb. See spec/features/cli/event/emit/README.md.
//
// This task wires only the seven REQ:envelope-flags and cobra plumbing
// (help text, required-flag enforcement, no-positional-args). Payload
// reading (REQ:payload-input-modes), envelope auto-fill, and dispatch
// land in subsequent tasks.
func eventEmitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "emit",
		Short: "Emit a SpecScore event",
		Long: `Emits a SpecScore event by constructing an envelope from the supplied
flags and dispatching it to configured subscribers (per spec/features/
cli/event/README.md).

The envelope's stable fields come from the seven flags below. Bookkeeping
fields (version, uuid, timestamp, artifact.revision) are auto-filled when
not supplied. The payload bytes are read via one of three input modes
(--payload-json, --payload-file, or stdin).

The verb accepts flag-form input only; positional arguments are rejected
to keep the call shape stable across shells.

Docs: https://specscore.md/event-emit
`,
		Args: cobra.NoArgs,
		RunE: runEventEmit,
	}

	// REQ:envelope-flags — the seven envelope flags.
	// Required-flag enforcement is handled manually in RunE so that the
	// error path maps to exitcode.InvalidArgs (exit 2) per the AC, rather
	// than cobra's default exit-1 "required flag(s) not set" path. The
	// stderr message names both the flag and the envelope field it
	// supplies (per REQ:envelope-flags last paragraph).
	cmd.Flags().String("name", "", "event name (e.g. idea.drafted); supplies envelope field `name` (required)")
	cmd.Flags().String("actor-kind", "", "one of skill|user|external; supplies envelope field `actor.kind` (required)")
	cmd.Flags().String("actor-id", "", "actor identifier; supplies envelope field `actor.id` (required)")
	cmd.Flags().String("artifact-type", "", "one of idea|feature|plan|task|idea-seed|consilium-review; supplies envelope field `artifact.type` (required)")
	cmd.Flags().String("artifact-id", "", "artifact slug or id; supplies envelope field `artifact.id` (required)")
	cmd.Flags().String("artifact-path", "", "repo-relative path to the artifact; supplies envelope field `artifact.path` (required)")
	cmd.Flags().String("artifact-revision", "", "git SHA or the literal `uncommitted`; supplies envelope field `artifact.revision` (optional; overrides auto-fill)")

	// REQ:payload-input-modes — the two payload-source flags. Both are
	// optional in the cobra wiring; mode arbitration (at most one of the
	// two; stdin only when neither is set) lands in Task 5.
	cmd.Flags().String("payload-json", "", "inline JSON payload; the flag value IS the envelope `payload` bytes")
	cmd.Flags().String("payload-file", "", "path to a file containing the JSON payload (relative paths resolve against the project root)")

	return cmd
}

// resolvePayload returns the payload bytes for the envelope, using the
// first non-empty source in priority order: --payload-json flag value,
// --payload-file contents, then stdin to EOF.
//
// This task (4) covers the three happy-path ACs only. Mode conflicts
// (--payload-json AND --payload-file both set), TTY-stdin rejection,
// and JSON-parse validation land in Task 5 — so this function does
// NOT inspect the bytes nor detect mutually-exclusive flag usage.
//
// File-path resolution: relative --payload-file paths join against
// projectRoot; absolute paths are used verbatim.
func resolvePayload(payloadJSON, payloadFile string, stdin io.Reader, projectRoot string) (json.RawMessage, error) {
	if payloadJSON != "" {
		return json.RawMessage(payloadJSON), nil
	}
	if payloadFile != "" {
		path := payloadFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(projectRoot, path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	}
	b, err := io.ReadAll(stdin)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// envelopeRequiredFlag pairs a CLI flag with the envelope field it supplies,
// so the missing-required error message names both per REQ:envelope-flags.
type envelopeRequiredFlag struct {
	flag  string // e.g. "name"
	field string // e.g. "name" or "actor.kind"
}

var envelopeRequiredFlags = []envelopeRequiredFlag{
	{"name", "name"},
	{"actor-kind", "actor.kind"},
	{"actor-id", "actor.id"},
	{"artifact-type", "artifact.type"},
	{"artifact-id", "artifact.id"},
	{"artifact-path", "artifact.path"},
}

// autofillEnvelope populates the four bookkeeping fields of e that the
// caller is not required to supply: version, uuid, timestamp, and
// artifact.revision. Per cli/event/emit#ac:envelope-auto-fill-*:
//
//   - version is always 1.
//   - uuid is a fresh lowercase-hyphenated v4 (matches the AC regex).
//   - timestamp is time.Now().UTC() formatted RFC 3339 with the `Z`
//     suffix (Go's time.RFC3339 format yields `Z` for UTC times).
//   - artifact.revision is, in priority order: (1) the override
//     argument when non-empty, else (2) the output of `git rev-parse
//     HEAD` run from projectRoot, else (3) the literal string
//     "uncommitted" when git fails (no .git/, no commits, etc.).
//
// The function is intentionally pure-ish — it mutates only e and reads
// the filesystem only through gitremote.HeadSHA. End-to-end wiring
// (RunE → dispatch) lands in Task 6; this function is exercised
// directly by unit tests in this batch.
func autofillEnvelope(e *event.Event, projectRoot string, revisionOverride string) {
	e.Version = 1
	e.UUID = uuid.NewString()
	e.Timestamp = time.Now().UTC()

	switch {
	case revisionOverride != "":
		e.Artifact.Revision = revisionOverride
	default:
		sha, err := gitremote.HeadSHA(projectRoot)
		if err != nil {
			e.Artifact.Revision = "uncommitted"
		} else {
			e.Artifact.Revision = sha
		}
	}
}

func runEventEmit(cmd *cobra.Command, _ []string) error {
	for _, rf := range envelopeRequiredFlags {
		v, _ := cmd.Flags().GetString(rf.flag)
		if v == "" {
			return exitcode.InvalidArgsErrorf(
				"missing required flag --%s (supplies envelope field `%s`)",
				rf.flag, rf.field)
		}
	}
	// Payload reading, envelope auto-fill, and dispatch land in later tasks.
	// For now, returning nil here is unreachable: at least one required flag
	// will always be missing until Task 4 wires payload modes (and the AC
	// for this batch only covers the missing-flag path).
	return nil
}
