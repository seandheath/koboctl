package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/backup"
	"github.com/seandheath/koboctl/internal/device"
)

func newRestoreCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore <backup-file>",
		Short: "Restore user data from a koboctl backup archive",
		Long: `Restore user data from a koboctl backup archive to the connected Kobo device.

By default, restore verifies that the backup matches the connected device's
serial number and skips categories whose component is not currently installed
(e.g. KOReader settings when KOReader is absent). Use --force to bypass both
checks.

The device must be mounted in USB Mass Storage mode.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := args[0]

			br, err := backup.OpenBackup(archivePath)
			if err != nil {
				return fmt.Errorf("opening backup: %w", err)
			}

			mp := mountPath
			var di *device.DeviceInfo
			if mp != "" {
				di, err = device.DetectDevice(mp)
			} else {
				di, err = device.AutoDetect()
			}
			if err != nil {
				return fmt.Errorf("detecting device: %w", err)
			}

			return br.Restore(di.MountPoint, di, backup.RestoreOptions{Force: force})
		},
	}

	cmd.Flags().BoolVar(&force, "force", false,
		"bypass serial mismatch check and restore all categories regardless of component installation state")

	return cmd
}
