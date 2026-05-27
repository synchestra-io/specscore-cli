package cli

// Features implemented: cli/property

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/specscore/specscore-cli/pkg/entity"
	"github.com/specscore/specscore-cli/pkg/exitcode"
	"github.com/specscore/specscore-cli/pkg/property"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// propertyCommand returns the "property" command group.
func propertyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "property",
		Short: "Query properties — listing and references",
	}
	cmd.AddCommand(
		propertyListCommand(),
		propertyRefsCommand(),
	)
	return cmd
}

// validatePropertyFormat checks the --format flag value is valid (text|yaml|json).
// Returns nil for "" (default) so callers can apply their own default.
func validatePropertyFormat(format string) error {
	switch format {
	case "", "text", "yaml", "json":
		return nil
	default:
		return exitcode.InvalidArgsErrorf("invalid --format: %s (valid: text, yaml, json)", format)
	}
}

// resolvePropertySpecRoot returns the absolute path to the project's `spec/`
// directory (the parent of every property/entity file). Errors are typed
// exitcode.Errors so the runner maps them to the right exit code.
func resolvePropertySpecRoot(projectFlag string) (string, error) {
	repoRoot, err := resolveSpecRoot(projectFlag)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, "spec"), nil
}

// --- property list ---

// propertyListEntry is the structured form of a discovered property emitted
// when `--format yaml|json` is requested.
type propertyListEntry struct {
	ID   string `json:"id" yaml:"id"`
	Path string `json:"path" yaml:"path"`
}

func propertyListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all property IDs, one per line",
		Long: `Lists every property id discovered under the project's
spec/features/**/*.property.md, sorted alphabetically. Default output is
plain text (one id per line); --format yaml|json emits a structured list
with id + project-relative path per item.`,
		Args: cobra.NoArgs,
		RunE: runPropertyList,
	}
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	cmd.Flags().String("format", "", "output format: text (default), yaml, json")
	return cmd
}

func runPropertyList(cmd *cobra.Command, _ []string) error {
	projectFlag, _ := cmd.Flags().GetString("project")
	formatFlag, _ := cmd.Flags().GetString("format")
	if err := validatePropertyFormat(formatFlag); err != nil {
		return err
	}

	specRoot, err := resolvePropertySpecRoot(projectFlag)
	if err != nil {
		return err
	}

	props, err := property.Discover(specRoot)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering properties: %v", err)
	}

	// Project-relative paths for stable, repo-portable output. specRoot
	// is `<repo>/spec/`; we relativize against its parent so paths read
	// `spec/features/...` rather than `features/...`.
	repoRoot := filepath.Dir(specRoot)
	entries := make([]propertyListEntry, 0, len(props))
	for _, p := range props {
		rel, relErr := filepathRelFn(repoRoot, p.Path)
		if relErr != nil {
			rel = p.Path
		}
		entries = append(entries, propertyListEntry{
			ID:   p.Slug,
			Path: filepath.ToSlash(rel),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	w := cmd.OutOrStdout()
	switch formatFlag {
	case "yaml":
		return writePropertyListYAML(w, entries)
	case "json":
		return writePropertyListJSON(w, entries)
	default:
		return writePropertyListText(w, entries)
	}
}

func writePropertyListText(w io.Writer, entries []propertyListEntry) error {
	bw := bufio.NewWriter(w)
	for _, e := range entries {
		_, _ = fmt.Fprintln(bw, e.ID)
	}
	return bw.Flush()
}

func writePropertyListYAML(w io.Writer, entries []propertyListEntry) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(entries); err != nil {
		return exitcode.UnexpectedErrorf("encoding yaml: %v", err)
	}
	return enc.Close()
}

func writePropertyListJSON(w io.Writer, entries []propertyListEntry) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		return exitcode.UnexpectedErrorf("encoding json: %v", err)
	}
	return nil
}

// --- property refs ---

// propertyRefsOutput is the structured form for --format yaml|json. The
// `consumers` key is fixed-shape — empty list when no consumers exist —
// per [REQ: property-refs] (`consumers: []`).
type propertyRefsOutput struct {
	Consumers []string `json:"consumers" yaml:"consumers"`
}

func propertyRefsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refs <id>",
		Short: "Show consumers of a property (entities whose properties[].ref: resolves to <id>)",
		Long: `Shows every entity whose frontmatter properties[] list contains a
ref: that resolves to the property identified by <id>. Output is one
entity id per line in text mode (bare ids, no prefix — matching
` + "`feature refs`" + `); --format yaml|json emits {consumers: [<entity-id>, ...]}.
Consumer entities are deduplicated: an entity that references the same
property under multiple property names appears exactly once. Entries are
sorted alphabetically by entity id.

Inline property definitions (those embedded directly in an entity's
frontmatter with data_type + checks, without a ref:) have no addressable
property id and therefore MUST NOT appear in this list — only ref:-style
references count. This matches [REQ: property-refs] from the
cli/property Feature.

Exit codes:
  0  success (including the no-consumers case)
  2  missing <id> argument, invalid --format value
  3  no project found, or <id> does not resolve to a discovered property
  10 unexpected I/O error`,
		Args: cobra.ExactArgs(1),
		RunE: runPropertyRefs,
	}
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	cmd.Flags().String("format", "", "output format: text (default), yaml, json")
	return cmd
}

func runPropertyRefs(cmd *cobra.Command, args []string) error {
	propertyID := args[0]
	projectFlag, _ := cmd.Flags().GetString("project")
	formatFlag, _ := cmd.Flags().GetString("format")
	if err := validatePropertyFormat(formatFlag); err != nil {
		return err
	}

	specRoot, err := resolvePropertySpecRoot(projectFlag)
	if err != nil {
		return err
	}

	// Discover every property and confirm <id> resolves. Per
	// [REQ: verb-exit-codes], an unknown id is exit 3.
	props, err := property.Discover(specRoot)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering properties: %v", err)
	}
	var targetPath string
	for _, p := range props {
		if p.Slug == propertyID {
			absTarget, absErr := filepathAbsCLI(p.Path)
			if absErr != nil {
				absTarget = p.Path
			}
			targetPath = absTarget
			break
		}
	}
	if targetPath == "" {
		return exitcode.NotFoundErrorf("property not found: %s", propertyID)
	}

	// Scan every entity. For each, parse properties[] and check whether
	// any ref: resolves (via entity.ResolveRef) to targetPath. De-dupe
	// entities whose frontmatter references the same property under
	// multiple names.
	entities, err := entityDiscoverCLI(specRoot)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering entities: %v", err)
	}

	seen := map[string]bool{}
	var consumers []string
	for _, e := range entities {
		doc, parseErr := entity.Parse(e.Path)
		if parseErr != nil {
			// Per the parse contract, only I/O errors surface here;
			// malformed frontmatter yields a partial Doc with no
			// Properties. Treat genuine I/O failures as exit 10 so
			// callers see them.
			return exitcode.UnexpectedErrorf("parsing %s: %v", e.Path, parseErr)
		}
		if doc == nil || doc.Frontmatter == nil {
			continue
		}
		for _, pi := range doc.Frontmatter.Properties {
			if pi.Ref == "" {
				// Inline definition — skip per [REQ: property-refs].
				continue
			}
			resolved, isLocal, resErr := entity.ResolveRef(specRoot, e.Path, pi.Ref)
			if resErr != nil || !isLocal {
				continue
			}
			absResolved, absErr := filepathAbsCLI(resolved)
			if absErr != nil {
				absResolved = resolved
			}
			if absResolved != targetPath {
				continue
			}
			// Frontmatter `id` is the addressable entity id; fall
			// back to the filename slug if frontmatter is malformed.
			entityID := doc.Frontmatter.ID
			if entityID == "" {
				entityID = e.Slug
			}
			if seen[entityID] {
				break // already recorded this entity
			}
			seen[entityID] = true
			consumers = append(consumers, entityID)
			break
		}
	}
	sort.Strings(consumers)

	w := cmd.OutOrStdout()
	switch formatFlag {
	case "yaml":
		return writePropertyRefsYAML(w, consumers)
	case "json":
		return writePropertyRefsJSON(w, consumers)
	default:
		return writePropertyRefsText(w, consumers)
	}
}

func writePropertyRefsText(w io.Writer, consumers []string) error {
	bw := bufio.NewWriter(w)
	for _, c := range consumers {
		_, _ = fmt.Fprintln(bw, c)
	}
	return bw.Flush()
}

func writePropertyRefsYAML(w io.Writer, consumers []string) error {
	// Force `consumers: []` (not `consumers: null`) for the empty case
	// per [REQ: property-refs].
	if consumers == nil {
		consumers = []string{}
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(propertyRefsOutput{Consumers: consumers}); err != nil {
		return exitcode.UnexpectedErrorf("encoding yaml: %v", err)
	}
	return enc.Close()
}

func writePropertyRefsJSON(w io.Writer, consumers []string) error {
	if consumers == nil {
		consumers = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(propertyRefsOutput{Consumers: consumers}); err != nil {
		return exitcode.UnexpectedErrorf("encoding json: %v", err)
	}
	return nil
}
