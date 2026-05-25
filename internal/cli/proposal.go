package cli

import (
	"github.com/spf13/cobra"
)

// proposalCommand returns the "proposal" command group — a convenience alias
// for creating change-request Ideas that target an existing Feature.
func proposalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proposal",
		Short: "Proposal management — convenience aliases for change-request Ideas",
	}
	cmd.AddCommand(proposalNewCommand())
	return cmd
}

// proposalNewCommand is a convenience alias for:
//
//	specscore idea new <slug> --type change-request --targets <feature-slug>
//
// Usage: specscore proposal new <feature-slug> <slug>
func proposalNewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <feature-slug> <slug>",
		Short: "Scaffold a new Proposal (change-request) targeting a Feature",
		Long: `Creates a lint-clean Proposal at spec/features/<feature-slug>/proposals/<slug>.md.

This is a convenience alias for:

  specscore idea new <slug> --type change-request --targets <feature-slug>

All flags from "idea new" are forwarded.`,
		Args: cobra.ExactArgs(2),
		RunE: runProposalNew,
	}
	cmd.Flags().String("title", "", "Proposal title (defaults to title-cased slug)")
	cmd.Flags().String("owner", "", "author identifier (defaults to $USER)")
	cmd.Flags().String("hmw", "", "Problem Statement (How Might We...) sentence")
	cmd.Flags().String("context", "", "Context section content")
	cmd.Flags().String("recommended-direction", "", "Recommended Direction content")
	cmd.Flags().String("mvp", "", "MVP Scope content")
	cmd.Flags().StringArray("not-doing", nil, "Not Doing exclusion (repeatable). Format: `<thing> — <reason>`")
	cmd.Flags().String("project", "", "project root (autodetected from current directory if omitted)")
	cmd.Flags().BoolP("interactive", "i", false, "prompt for each field on stdin")
	cmd.Flags().Bool("force", false, "overwrite an existing proposal file at that slug")
	cmd.Flags().String("phase", "", "optional lifecycle phase to pre-populate")
	return cmd
}

// runProposalNew delegates to runIdeaNew with --type=change-request and
// --targets set from the first positional argument.
func runProposalNew(cmd *cobra.Command, args []string) error {
	featureSlug := args[0]
	slug := args[1]

	// Build an ideaCommand with the translated arguments and forward execution.
	ideaCmd := ideaCommand()
	ideaArgs := []string{"new", slug, "--type", "change-request", "--targets", featureSlug}

	// Forward all flags that were explicitly set.
	forwardStringFlag(cmd, "title", &ideaArgs)
	forwardStringFlag(cmd, "owner", &ideaArgs)
	forwardStringFlag(cmd, "hmw", &ideaArgs)
	forwardStringFlag(cmd, "context", &ideaArgs)
	forwardStringFlag(cmd, "recommended-direction", &ideaArgs)
	forwardStringFlag(cmd, "mvp", &ideaArgs)
	forwardStringFlag(cmd, "project", &ideaArgs)
	forwardStringFlag(cmd, "phase", &ideaArgs)

	notDoing, _ := cmd.Flags().GetStringArray("not-doing")
	for _, nd := range notDoing {
		ideaArgs = append(ideaArgs, "--not-doing", nd)
	}

	if interactive, _ := cmd.Flags().GetBool("interactive"); interactive {
		ideaArgs = append(ideaArgs, "-i")
	}
	if force, _ := cmd.Flags().GetBool("force"); force {
		ideaArgs = append(ideaArgs, "--force")
	}

	ideaCmd.SetOut(cmd.OutOrStdout())
	ideaCmd.SetErr(cmd.ErrOrStderr())
	ideaCmd.SetIn(cmd.InOrStdin())
	ideaCmd.SetArgs(ideaArgs)
	return ideaCmd.Execute()
}

// forwardStringFlag appends --name=value to args if the flag was set.
func forwardStringFlag(cmd *cobra.Command, name string, args *[]string) {
	if v, _ := cmd.Flags().GetString(name); v != "" {
		*args = append(*args, "--"+name, v)
	}
}
