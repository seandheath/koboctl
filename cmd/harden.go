package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/manifest"
)

func newHardenCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "harden",
		Short: "Apply security hardening to the device",
		Long: `Apply security hardening to a mounted Kobo device.

Operations performed (in order):
  1. Harden Nickel config (disable OTA, enable sideload mode, disable sync)
  2. Install analytics SQLite trigger (blocks telemetry accumulation)
  3. Guard KoboRoot.tgz (prevents rogue firmware extraction)
  4. Remove dangerous KOReader plugins (SSH, WebDAV, web browser)
  5. Stage /etc/hosts blocklist boot script
  6. Stage DNS lockdown boot script
  7. Stage devmode/telnet disable boot script
  8. Write boot hook runner script
  9. Write KFMon on_boot config

Operations that modify the ext4 root filesystem (hosts, DNS, devmode) are staged
as shell scripts on the FAT32 partition and executed at boot via the KFMon on_boot
hook. They take effect after the next reboot.

Also called automatically by 'koboctl provision' when hardening.enabled = true.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manifest.LoadManifest(manifestPath)
			if err != nil {
				return err
			}
			if errs := manifest.ValidateManifest(m); len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "manifest error: %v\n", e)
				}
				return fmt.Errorf("manifest validation failed with %d error(s)", len(errs))
			}

			mp := mountPath
			if mp == "" {
				mp = m.Device.Mount
			}
			var di *device.DeviceInfo
			if mp != "" {
				di, err = device.DetectDevice(mp)
			} else {
				di, err = device.AutoDetect()
			}
			if err != nil {
				return fmt.Errorf("detecting device: %w", err)
			}

			return RunHarden(di.MountPoint, m.Hardening, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be done without making changes")
	return cmd
}

// hardenStep is a single hardening operation with a display name and function.
type hardenStep struct {
	name string
	fn   func() error
}

// RunHarden executes all hardening operations for the given manifest config.
// In dry-run mode, it prints what would be done without writing any files.
func RunHarden(mountPoint string, cfg manifest.HardeningConfig, dryRun bool) error {
	steps := []hardenStep{
		// USB-direct operations (FAT32, written immediately).
		{
			name: "Nickel config",
			fn:   func() error { return hardening.HardenNickelConfig(mountPoint, cfg) },
		},
		{
			name: "Analytics trigger",
			fn:   func() error { return hardening.InstallAnalyticsTrigger(mountPoint) },
		},
		{
			name: "KoboRoot guard",
			fn:   func() error { return hardening.GuardKoboRoot(mountPoint) },
		},
		{
			name: "Plugin removal",
			fn: func() error {
				removed, err := hardening.RemoveDangerousPlugins(mountPoint)
				if err != nil {
					return err
				}
				if len(removed) > 0 {
					for _, p := range removed {
						fmt.Printf("  removed plugin: %s\n", p)
					}
				}
				return nil
			},
		},
		// Staged boot scripts (written to FAT32, executed on boot).
		{
			name: "Hosts blocklist",
			fn:   func() error { return hardening.StageHostsBlocklist(mountPoint, cfg) },
		},
		{
			name: "DNS lockdown",
			fn:   func() error { return hardening.StageDNSLockdown(mountPoint, cfg) },
		},
		{
			name: "Devmode disable",
			fn:   func() error { return hardening.StageDevmodeDisable(mountPoint, cfg) },
		},
		{
			name: "Boot hook runner",
			fn:   func() error { return hardening.StageBootHookRunner(mountPoint) },
		},
		{
			name: "KFMon boot config",
			fn:   func() error { return hardening.StageKFMonBootConfig(mountPoint) },
		},
	}

	for _, step := range steps {
		if dryRun {
			fmt.Printf("[dry-run] Would apply: %s\n", step.name)
			continue
		}
		fmt.Printf("Applying: %s... ", step.name)
		if err := step.fn(); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Println("done")
	}

	if !dryRun {
		hardening.PrintParentalControlsReminder()
	}
	return nil
}
