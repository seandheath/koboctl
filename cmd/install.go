package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
	"github.com/seandheath/koboctl/internal/installer"
)

func newInstallCommand() *cobra.Command {
	var (
		version  string
		channel  string
		noVerify bool
	)

	cmd := &cobra.Command{
		Use:   "install <component>",
		Short: "Install or update a single component",
		Long: `Install or update a single component on the connected Kobo device.

Components: koreader, nickelmenu, kfmon`,
		Args:    cobra.ExactArgs(1),
		ValidArgs: []string{"koreader", "nickelmenu", "kfmon"},
		RunE: func(cmd *cobra.Command, args []string) error {
			component := args[0]

			// Detect device.
			mp := mountPath
			var di *device.DeviceInfo
			var err error
			if mp != "" {
				di, err = device.DetectDevice(mp)
			} else {
				di, err = device.AutoDetect()
			}
			if err != nil {
				return fmt.Errorf("detecting device: %w", err)
			}
			_ = noVerify // TODO: pass to fetch layer when checksum support is added

			ctx := context.Background()
			gh := fetch.NewGitHubClient()

			ver := version
			if ver == "" {
				ver = "latest"
			}

			switch component {
			case "kfmon":
				cfg := manifest.KFMonConfig{Enabled: true, Version: ver}
				return installer.InstallKFMon(ctx, di.MountPoint, cfg, gh)

			case "koreader":
				cfg := manifest.KOReaderConfig{
					Enabled: true,
					Version: ver,
					Channel: channel,
				}
				return installer.InstallKOReader(ctx, di.MountPoint, cfg, gh)

			case "nickelmenu":
				cfg := manifest.NickelMenuConfig{Enabled: true, Version: ver}
				return installer.InstallNickelMenu(ctx, di.MountPoint, cfg, gh)

			default:
				return fmt.Errorf("unknown component %q — valid components: koreader, nickelmenu, kfmon", component)
			}
		},
	}

	cmd.Flags().StringVar(&version, "version", "", `component version to install (default: "latest")`)
	cmd.Flags().StringVar(&channel, "channel", "stable", `KOReader channel: "stable" or "nightly"`)
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "skip SHA256 checksum verification")

	return cmd
}
