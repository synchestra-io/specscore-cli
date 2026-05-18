package cli

// Features implemented: cli/entity

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/synchestra-io/specscore-cli/pkg/entity"
	"github.com/synchestra-io/specscore-cli/pkg/exitcode"
	"gopkg.in/yaml.v3"
)

// entityCommand returns the "entity" command group — read-only navigation
// verbs over the entity inheritance graph. Lint enforcement and managed-
// section rendering for *.entity.md files live in pkg/lint; this file
// only wires the three navigation verbs from
// spec/features/cli/entity/README.md ("Navigation verbs").
func entityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entity",
		Short: "Query entities — listing, references, inheritance tree",
	}
	cmd.AddCommand(
		entityListCommand(),
		entityRefsCommand(),
		entityTreeCommand(),
	)
	return cmd
}

// projectRelativePath returns the entity file path relative to the
// project root (i.e., the parent of `spec/`). On any rel-computation
// error we fall back to the absolute path so the verb never panics.
func projectRelativePath(projectRoot, absPath string) string {
	rel, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return rel
}

// --- entity list ---

// entityListItem is the structured representation of a single discovered
// entity, surfaced by `entity list --format yaml|json` per
// [cli/entity#req:entity-list].
type entityListItem struct {
	ID   string `json:"id" yaml:"id"`
	Path string `json:"path" yaml:"path"`
}

func entityListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all entity IDs, one per line",
		Long: `Lists every entity discovered under spec/features/**/*.entity.md, sorted
alphabetically. Default output is plain text — one id per line. With
--format yaml or --format json, emits a structured list whose items
carry id and path (path is project-relative).`,
		Args: cobra.NoArgs,
		RunE: runEntityList,
	}
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	cmd.Flags().String("format", "", "output format: text (default), yaml, json")
	return cmd
}

func runEntityList(cmd *cobra.Command, _ []string) error {
	projectFlag, _ := cmd.Flags().GetString("project")

	format := effectiveFormat(cmd)
	if err := validateFormat(format); err != nil {
		return err
	}

	projectRoot, err := resolveSpecRoot(projectFlag)
	if err != nil {
		return err
	}
	specDir := filepath.Join(projectRoot, "spec")

	discovered, err := entity.Discover(specDir)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering entities: %v", err)
	}

	items := make([]entityListItem, 0, len(discovered))
	for _, d := range discovered {
		items = append(items, entityListItem{
			ID:   d.Slug,
			Path: projectRelativePath(projectRoot, d.Path),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	w := cmd.OutOrStdout()
	switch format {
	case "yaml":
		return writeEntityListYAML(w, items)
	case "json":
		return writeEntityListJSON(w, items)
	default:
		for _, it := range items {
			_, _ = fmt.Fprintln(w, it.ID)
		}
		return nil
	}
}

func writeEntityListYAML(w io.Writer, items []entityListItem) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(items); err != nil {
		return exitcode.UnexpectedErrorf("encoding yaml: %v", err)
	}
	return enc.Close()
}

func writeEntityListJSON(w io.Writer, items []entityListItem) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(items); err != nil {
		return exitcode.UnexpectedErrorf("encoding json: %v", err)
	}
	return nil
}

// --- entity refs ---

// entityRefsOutput is the structured representation surfaced by
// `entity refs <id> --format yaml|json` per [cli/entity#req:entity-refs].
type entityRefsOutput struct {
	Consumers []string `json:"consumers" yaml:"consumers"`
}

func entityRefsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refs <id>",
		Short: "Show consumers of an entity (other entities via inherits:)",
		Long: `Shows every other *.entity.md file whose inherits: resolves to <id>.
Default output is one consumer id per line in plain text (no prefix) —
matching the feature refs exemplar. With --format yaml or --format json,
the output is a structured list under the key consumers. Empty consumer
set exits 0 with empty stdout (text) or "consumers: []" (yaml/json).

In MVP, the only consumer kind is entity-via-inherits:; feature-level
back-references are a separate Idea.`,
		// Manual arg validation so a missing positional yields a typed
		// exit-2 error rather than cobra's default exit-1.
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE:         runEntityRefs,
	}
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	cmd.Flags().String("format", "", "output format: text (default), yaml, json")
	return cmd
}

func runEntityRefs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return exitcode.InvalidArgsError("missing required positional argument: <id>")
	}
	if len(args) > 1 {
		return exitcode.InvalidArgsErrorf(
			"too many positional arguments: entity refs accepts exactly one <id>, got %d", len(args),
		)
	}
	id := args[0]

	projectFlag, _ := cmd.Flags().GetString("project")

	format := effectiveFormat(cmd)
	if err := validateFormat(format); err != nil {
		return err
	}

	projectRoot, err := resolveSpecRoot(projectFlag)
	if err != nil {
		return err
	}
	specDir := filepath.Join(projectRoot, "spec")

	discovered, err := entity.Discover(specDir)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering entities: %v", err)
	}

	// Build slug -> absolute path map for inherits: resolution.
	bySlug := make(map[string]string, len(discovered))
	for _, d := range discovered {
		bySlug[d.Slug] = d.Path
	}
	if _, ok := bySlug[id]; !ok {
		return exitcode.NotFoundErrorf("entity not found: %s", id)
	}

	targetPath := bySlug[id]
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		absTarget = targetPath
	}

	consumers := make([]string, 0)
	for _, d := range discovered {
		if d.Slug == id {
			continue
		}
		doc, parseErr := entity.Parse(d.Path)
		if parseErr != nil {
			return exitcode.UnexpectedErrorf("parsing %s: %v", d.Path, parseErr)
		}
		if doc.Frontmatter == nil || doc.Frontmatter.Inherits == "" {
			continue
		}
		resolved, _, resolveErr := entity.ResolveInherits(specDir, d.Path, doc.Frontmatter.Inherits)
		if resolveErr != nil {
			// URL or unresolvable refs are silently ignored — they
			// can't point to a local entity.
			continue
		}
		if resolved == "" {
			continue
		}
		absResolved, absErr := filepath.Abs(resolved)
		if absErr != nil {
			absResolved = resolved
		}
		if absResolved == absTarget {
			consumers = append(consumers, d.Slug)
		}
	}
	sort.Strings(consumers)

	w := cmd.OutOrStdout()
	switch format {
	case "yaml":
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		if err := enc.Encode(entityRefsOutput{Consumers: consumers}); err != nil {
			return exitcode.UnexpectedErrorf("encoding yaml: %v", err)
		}
		return enc.Close()
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entityRefsOutput{Consumers: consumers}); err != nil {
			return exitcode.UnexpectedErrorf("encoding json: %v", err)
		}
		return nil
	default:
		for _, c := range consumers {
			_, _ = fmt.Fprintln(w, c)
		}
		return nil
	}
}

// --- entity tree ---

func entityTreeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tree",
		Short: "Display the entity inheritance forest as indented text",
		Long: `Prints the inheritance forest as indented plain text. Each entity that
does not declare inherits: is a root at column 0; descendants are
indented two spaces per inheritance step. Siblings are sorted
alphabetically. Cycles are rendered with a (cycle) suffix on the first
edge that closes the cycle, and recursion stops for that subtree.

--format is NOT supported in MVP — entity tree is text-only.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE:         runEntityTree,
	}
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	// --format is registered ONLY so we can surface an exit-2 error in
	// RunE when it is supplied. Per [cli/entity#req:entity-tree], MVP
	// is text-only; any value (including the empty string set by hand)
	// is rejected. cobra would otherwise raise "unknown flag" → exit 1,
	// missing the spec-required exit 2.
	cmd.Flags().String("format", "", "(unsupported on tree — entity tree is text-only)")
	_ = cmd.Flags().MarkHidden("format")
	return cmd
}

func runEntityTree(cmd *cobra.Command, _ []string) error {
	// --format is unsupported on tree; presence (Changed=true) is the
	// signal — an empty literal "--format=" still counts.
	if cmd.Flags().Changed("format") {
		return exitcode.InvalidArgsError(
			"--format is not supported on `entity tree` (text-only in MVP per cli/entity#req:entity-tree)",
		)
	}
	projectFlag, _ := cmd.Flags().GetString("project")

	projectRoot, err := resolveSpecRoot(projectFlag)
	if err != nil {
		return err
	}
	specDir := filepath.Join(projectRoot, "spec")

	discovered, err := entity.Discover(specDir)
	if err != nil {
		return exitcode.UnexpectedErrorf("discovering entities: %v", err)
	}

	// Parse each entity once and build a slug -> parent-slug map.
	type node struct {
		slug   string
		parent string // "" means root
	}
	nodes := make(map[string]*node, len(discovered))
	pathToSlug := make(map[string]string, len(discovered))
	for _, d := range discovered {
		abs, absErr := filepath.Abs(d.Path)
		if absErr != nil {
			abs = d.Path
		}
		pathToSlug[abs] = d.Slug
	}
	for _, d := range discovered {
		doc, parseErr := entity.Parse(d.Path)
		if parseErr != nil {
			return exitcode.UnexpectedErrorf("parsing %s: %v", d.Path, parseErr)
		}
		n := &node{slug: d.Slug}
		if doc.Frontmatter != nil && doc.Frontmatter.Inherits != "" {
			resolved, _, resolveErr := entity.ResolveInherits(specDir, d.Path, doc.Frontmatter.Inherits)
			if resolveErr == nil && resolved != "" {
				if abs, absErr := filepath.Abs(resolved); absErr == nil {
					if slug, ok := pathToSlug[abs]; ok {
						n.parent = slug
					}
				}
			}
		}
		nodes[d.Slug] = n
	}

	// children[parent] = sorted slice of child slugs.
	children := make(map[string][]string, len(nodes))
	var roots []string
	for slug, n := range nodes {
		if n.parent == "" {
			roots = append(roots, slug)
		} else {
			children[n.parent] = append(children[n.parent], slug)
		}
	}
	// Detect cycle members: nodes whose parent chain never reaches a
	// root. These would otherwise never be printed because they have no
	// ancestor that is a root. Walk every node's ancestor chain; if we
	// re-visit a slug, the chain participates in a cycle. Hoist one
	// member of each such cycle into roots so it surfaces with a
	// (cycle) marker on its first re-occurrence per
	// [cli/entity#req:entity-tree].
	reachableFromRoot := make(map[string]bool)
	for _, r := range roots {
		markReachable(r, children, reachableFromRoot)
	}
	cycleRoots := map[string]bool{}
	for slug := range nodes {
		if reachableFromRoot[slug] {
			continue
		}
		// Walk up from slug; if we revisit, slug is in a cycle.
		seen := map[string]bool{}
		cur := slug
		for cur != "" && !seen[cur] {
			seen[cur] = true
			cur = nodes[cur].parent
		}
		if cur != "" && seen[cur] {
			// cur is on a cycle. Compute the strict cycle membership by
			// walking from cur until we return to cur, then pick the
			// alphabetically-smallest slug as the canonical root for
			// that cycle so rendering is deterministic.
			strict := map[string]bool{}
			c := cur
			for {
				strict[c] = true
				c = nodes[c].parent
				if c == cur || c == "" {
					break
				}
			}
			var min string
			for m := range strict {
				if min == "" || m < min {
					min = m
				}
			}
			cycleRoots[min] = true
		}
	}
	for c := range cycleRoots {
		roots = append(roots, c)
	}
	sort.Strings(roots)
	for k := range children {
		sort.Strings(children[k])
	}

	w := cmd.OutOrStdout()
	for _, root := range roots {
		printEntityTreeNode(w, root, children, map[string]bool{}, 0)
	}
	return nil
}

// markReachable performs a DFS over the children map starting at slug
// and records every slug reached. Used to distinguish nodes that have
// a path to a true root from nodes stranded in a cycle.
func markReachable(slug string, children map[string][]string, seen map[string]bool) {
	if seen[slug] {
		return
	}
	seen[slug] = true
	for _, c := range children[slug] {
		markReachable(c, children, seen)
	}
}

// printEntityTreeNode writes one entity node and recurses into its
// children. `onPath` carries the ancestor slugs currently on the
// recursion stack so the first edge that closes a cycle is rendered
// with " (cycle)" and recursion stops for that subtree per
// [cli/entity#req:entity-tree].
func printEntityTreeNode(w io.Writer, slug string, children map[string][]string, onPath map[string]bool, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	if onPath[slug] {
		_, _ = fmt.Fprintf(w, "%s%s (cycle)\n", indent, slug)
		return
	}
	_, _ = fmt.Fprintf(w, "%s%s\n", indent, slug)
	onPath[slug] = true
	defer delete(onPath, slug)
	for _, c := range children[slug] {
		printEntityTreeNode(w, c, children, onPath, depth+1)
	}
}
