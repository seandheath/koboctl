// Package cmd implements the koboctl CLI command tree.
package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Global flags shared across subcommands.
var (
	// manifestPath is resolved by each subcommand that needs it.
	manifestPath string
	// mountPath overrides auto-detection.
	mountPath string
)

// NewRootCommand creates and returns the root cobra command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "koboctl",
		Short: "Kobo e-reader provisioning and management CLI",
		Long: `koboctl provisions and manages hacked Kobo e-readers from a Linux workstation.

It automates installation of KOReader, KFMon, NickelMenu, and Plato, and applies
security hardening from a declarative TOML manifest.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		// Bare `koboctl` on a TTY launches the interactive TUI; otherwise (piped,
		// scripted, or with unexpected args) it prints help so scripting is
		// unaffected. Real subcommands are dispatched before this RunE.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && isatty.IsTerminal(os.Stdout.Fd()) {
				return runTUI()
			}
			return cmd.Help()
		},
	}

	root.PersistentFlags().StringVar(&manifestPath, "manifest", "koboctl.toml",
		"path to the koboctl manifest file")
	root.PersistentFlags().StringVar(&mountPath, "mount", "",
		"Kobo mount point (auto-detected if not specified)")

	root.AddCommand(
		newInitCommand(),
		newProvisionCommand(),
		newInstallCommand(),
		newStatusCommand(),
		newHardenCommand(),
		newBackupCommand(),
		newRestoreCommand(),
		newTUICommand(),
	)

	return root
}

// fatalf prints a formatted error to stderr and exits with code 1.
// Used by commands that encounter fatal errors after cobra's error handling.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
