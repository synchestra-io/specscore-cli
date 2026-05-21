package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/specscore/specscore-cli/internal/telemetry"
)

// telemetryCommand returns the `specscore telemetry` cobra group with three
// verbs (status, enable, disable), each accepting an optional positional
// [channel] argument. Implements cli/telemetry#req:telemetry-subcommand-
// surface.
func telemetryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Inspect or change telemetry preferences (see docs/telemetry.md)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		telemetryStatusCommand(),
		telemetryEnableCommand(),
		telemetryDisableCommand(),
	)
	return cmd
}

// knownChannelArgs returns the user-facing channel names as plain strings,
// for use in cobra arg validation and error messages.
func knownChannelArgs() []string {
	out := make([]string, 0, 2)
	for _, n := range []telemetry.ChannelName{telemetry.ChannelUsageStats, telemetry.ChannelCrashReports} {
		out = append(out, string(n))
	}
	return out
}

// allChannelsSentinel is the explicit "all channels" form accepted by
// `specscore telemetry {status,enable,disable}` per
// cli/telemetry#req:telemetry-subcommand-surface. Equivalent to passing no
// channel argument. Chosen over `*` because `*` is shell-glob-expanded in
// interactive use; `all` is shell-neutral and conventional.
const allChannelsSentinel = "all"

// validateChannelArg parses the optional positional channel argument.
// Returns (channelName, true, nil) for a real channel name.
// Returns ("", false, nil) for no-arg OR the `all` sentinel — both mean
// "operate on all channels."
// Returns ("", false, exitErr{code:2}) for any unrecognized value.
func validateChannelArg(args []string) (telemetry.ChannelName, bool, error) {
	if len(args) == 0 {
		return "", false, nil
	}
	want := args[0]
	if want == allChannelsSentinel {
		return "", false, nil
	}
	known := knownChannelArgs()
	if !slices.Contains(known, want) {
		return "", false, exitErr{
			code: 2,
			msg: fmt.Sprintf("unknown channel %q; known channels: %s (use `all` or omit for all)",
				want, strings.Join(known, ", ")),
		}
	}
	return telemetry.ChannelName(want), true, nil
}

func telemetryStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [channel]",
		Short: "Print current telemetry state for all channels (or a single channel)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			single, hasArg, err := validateChannelArg(args)
			if err != nil {
				return err
			}
			return writeStatus(cmd.OutOrStdout(), single, hasArg)
		},
	}
}

// channelArgHelp is the shared --help long description for the [channel]
// positional on status/enable/disable subcommands. Names the `all` sentinel
// per cli/telemetry#req:telemetry-subcommand-surface.
const channelArgHelp = "" +
	"With no [channel] argument or with the explicit sentinel `all`, operates " +
	"on ALL registered channels. With a channel name (e.g. usage-stats, " +
	"crash-reports), operates only on that channel."

func telemetryEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable [channel]",
		Short: "Enable telemetry (all channels, or a single channel)",
		Long:  "Enable telemetry.\n\n" + channelArgHelp,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			single, hasArg, err := validateChannelArg(args)
			if err != nil {
				return err
			}
			return mutateState(cmd.OutOrStdout(), single, hasArg, true)
		},
	}
}

func telemetryDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable [channel]",
		Short: "Disable telemetry (all channels, or a single channel)",
		Long:  "Disable telemetry.\n\n" + channelArgHelp,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			single, hasArg, err := validateChannelArg(args)
			if err != nil {
				return err
			}
			return mutateState(cmd.OutOrStdout(), single, hasArg, false)
		},
	}
}

// writeStatus prints the channel table to stdout. One channel per line, the
// channel name as the first column, stable enough for grep-based scripting
// (cli/telemetry#ac:telemetry-subcommand-status).
func writeStatus(w io.Writer, single telemetry.ChannelName, hasArg bool) error {
	specZero, doNotTrack, ciDetected := telemetry.CollectOSEnvSignals(nil)
	stateResult, _ := telemetry.ReadState()
	sigs := telemetry.OptOutSignals{
		NoTelemetryFlag:        noTelemetryFlag,
		SpecScoreTelemetryZero: specZero,
		DoNotTrack:             doNotTrack,
		CIDetected:             ciDetected,
		PersistentState:        stateResult.State,
		PersistentStateInvalid: stateResult.InvalidReason != "",
	}
	known := []telemetry.ChannelName{telemetry.ChannelUsageStats, telemetry.ChannelCrashReports}
	decisions := telemetry.ResolveOptOut(sigs, known)

	channels := known
	if hasArg {
		channels = []telemetry.ChannelName{single}
	}
	for _, name := range channels {
		d := decisions[name]
		state := "enabled"
		if !d.Enabled {
			state = "disabled"
		}
		_, _ = fmt.Fprintf(w, "%s: %s (source: %s)\n", name, state, d.Source)
	}
	return nil
}

// mutateState writes the persistent-state file with the requested change.
// When hasArg is false: global enable/disable. With hasArg: per-channel
// override.
func mutateState(w io.Writer, single telemetry.ChannelName, hasArg bool, enable bool) error {
	current, err := telemetry.ReadState()
	if err != nil {
		return err
	}
	next := current.State
	verb := "disabled"
	if enable {
		verb = "enabled"
	}
	target := "all channels"
	if hasArg {
		setChannelOverride(&next, single, enable)
		target = string(single)
	} else {
		next.Enabled = &enable
	}
	if writeErr := telemetry.WriteState(next); writeErr != nil {
		return writeErr
	}
	_, _ = fmt.Fprintf(w, "telemetry: %s %s\n", target, verb)
	return nil
}

func setChannelOverride(s *telemetry.State, name telemetry.ChannelName, enable bool) {
	switch name {
	case telemetry.ChannelUsageStats:
		s.UsageStats = &enable
	case telemetry.ChannelCrashReports:
		s.CrashReports = &enable
	}
}

// exitErr carries an exit code via the exitCoder interface checked by Fatal.
type exitErr struct {
	code int
	msg  string
}

func (e exitErr) Error() string { return e.msg }
func (e exitErr) ExitCode() int { return e.code }
