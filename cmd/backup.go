package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/backup"
	"github.com/seandheath/koboctl/internal/device"
)

func newBackupCommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up the entire Kobo FAT32 partition to a tar.gz archive",
		Long: `Back up every file on the connected Kobo's FAT32 partition to a
gzip-compressed tar archive. This includes the reading library, annotations,
device config, installed software, and all ebooks.

The device must be mounted in USB Mass Storage mode.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			mp := mountPath
			var (
				di  *device.DeviceInfo
				err error
			)
			if mp != "" {
				di, err = device.DetectDevice(mp)
			} else {
				di, err = device.AutoDetect()
			}
			if err != nil {
				return fmt.Errorf("detecting device: %w", err)
			}

			_, err = backup.CreateBackup(di, backup.BackupOptions{OutputPath: outputPath})
			return err
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"output file path (default: koboctl-backup-<serial>-<timestamp>.tar.gz in CWD)")

	return cmd
}
