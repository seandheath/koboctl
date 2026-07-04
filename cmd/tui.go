package cmd

import (
	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/tui"
)

// newTUICommand returns the `koboctl tui` subcommand.
func newTUICommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive configuration TUI",
		Long: `Launch the interactive terminal UI: a live device dashboard, a full
manifest editor, and action runners (provision, install, harden, backup) with
streamed output.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI()
		},
	}
}

// runTUI wires the cmd-package orchestration entry points into the TUI. The
// callbacks avoid an import cycle (tui cannot import cmd).
func runTUI() error {
	return tui.Run(manifestPath, mountPath, tui.Actions{
		Provision: RunProvision,
		Harden:    RunHarden,
	})
}
