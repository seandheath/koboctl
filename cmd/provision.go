package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

func newProvisionCommand() *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Full provisioning workflow",
		Long: `Provision a Kobo device from a manifest file.

Executes all enabled install and configure steps in dependency order:
  1. Detect device
  2. Fetch artifacts (parallel)
  3. Install KFMon
  4. Install KOReader
  5. Install NickelMenu
  6. Apply security hardening (if hardening.enabled = true)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load and validate manifest.
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

			// Detect device.
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

			if di.Profile == nil && di.Model != "" {
				fmt.Fprintf(os.Stderr, "warning: device model %q is not in the profile database — proceeding anyway\n", di.Model)
			}

			fmt.Printf("Device:   %s\n", di.Model)
			fmt.Printf("Firmware: %s\n", di.FirmwareVersion)
			fmt.Printf("Mount:    %s\n", di.MountPoint)
			fmt.Println()

			if dryRun {
				fmt.Println("[dry-run] would fetch and install the following components:")
				if m.KFMon.Enabled {
					fmt.Println("  - kfmon")
				}
				if m.KOReader.Enabled {
					fmt.Println("  - koreader")
				}
				if m.NickelMenu.Enabled {
					fmt.Println("  - nickelmenu")
				}
				if m.Hardening.Enabled {
					fmt.Println("  - hardening")
				}
				return nil
			}

			ctx := context.Background()
			gh := fetch.NewGitHubClient()

			// Pre-fetch all artifacts in parallel before installing.
			if err := prefetchArtifacts(ctx, gh, m); err != nil {
				return fmt.Errorf("pre-fetching artifacts: %w", err)
			}

			// Install in dependency order: KFMon → KOReader → NickelMenu.
			if err := installer.InstallKFMon(ctx, di.MountPoint, m.KFMon, gh); err != nil {
				return fmt.Errorf("installing kfmon: %w", err)
			}
			if err := installer.InstallKOReader(ctx, di.MountPoint, m.KOReader, gh); err != nil {
				return fmt.Errorf("installing koreader: %w", err)
			}
			if err := installer.InstallNickelMenu(ctx, di.MountPoint, m.NickelMenu, gh); err != nil {
				return fmt.Errorf("installing nickelmenu: %w", err)
			}

			// Bypass setup wizard so device boots straight to home screen.
			if err := hardening.BypassSetupWizard(di.MountPoint); err != nil {
				return fmt.Errorf("bypassing setup wizard: %w", err)
			}

			// Merge KoboRoot.tgz payloads from KFMon and NickelMenu into a single
			// combined archive. Both deliver system-partition installers via this path,
			// and only one file can exist at .kobo/KoboRoot.tgz.
			if err := mergeAndPlaceKoboRoot(ctx, di.MountPoint, m, gh); err != nil {
				return err
			}

			// Stage a boot script to activate the KoboRoot guard after firmware
			// processes the merged tgz on first reboot. This replaces the immediate
			// guard that would block KFMon/NickelMenu installation.
			if m.Hardening.Enabled && m.Hardening.Filesystem.DisableKoboRoot {
				if err := hardening.StageKoboRootGuard(di.MountPoint); err != nil {
					return fmt.Errorf("staging KoboRoot guard boot script: %w", err)
				}
			}

			// Apply security hardening if enabled.
			// Skip the KoboRoot guard — handled by the boot script above.
			if m.Hardening.Enabled {
				if err := RunHarden(di.MountPoint, m.Hardening, dryRun, true); err != nil {
					return fmt.Errorf("hardening: %w", err)
				}
			}

			printPostProvisionInstructions(m.Hardening.Enabled)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be done without making changes")
	return cmd
}

// prefetchArtifacts downloads all required artifacts in parallel.
// Installation still happens serially; this just warms the cache.
func prefetchArtifacts(ctx context.Context, gh *fetch.GitHubClient, m *manifest.Manifest) error {
	g, ctx := errgroup.WithContext(ctx)

	// KFMon is embedded in the binary; no prefetch needed.

	if m.KOReader.Enabled {
		g.Go(func() error {
			ver := m.KOReader.Version
			if ver == "" {
				ver = "latest"
			}
			tag, assets, err := gh.LatestReleaseOrTag(ctx, "koreader", "koreader", ver)
			if err != nil {
				return fmt.Errorf("koreader: resolving release: %w", err)
			}
			asset, err := fetch.FindAsset(assets, "koreader-kobo-*.zip")
			if err != nil {
				return fmt.Errorf("koreader: %w", err)
			}
			_, err = gh.FetchAsset(ctx, "koreader", tag, asset)
			return err
		})
	}

	if m.NickelMenu.Enabled {
		g.Go(func() error {
			ver := m.NickelMenu.Version
			if ver == "" {
				ver = "latest"
			}
			tag, assets, err := gh.LatestReleaseOrTag(ctx, "pgaskin", "NickelMenu", ver)
			if err != nil {
				return fmt.Errorf("nickelmenu: resolving release: %w", err)
			}
			asset, err := fetch.FindAsset(assets, "KoboRoot.tgz")
			if err != nil {
				return fmt.Errorf("nickelmenu: %w", err)
			}
			_, err = gh.FetchAsset(ctx, "nickelmenu", tag, asset)
			return err
		})
	}

	return g.Wait()
}

// mergeAndPlaceKoboRoot collects KoboRoot.tgz payloads from KFMon and NickelMenu
// and writes a single merged archive to .kobo/KoboRoot.tgz on the device.
func mergeAndPlaceKoboRoot(ctx context.Context, mountPoint string, m *manifest.Manifest, gh *fetch.GitHubClient) error {
	var sources [][]byte

	if m.KFMon.Enabled {
		kfmonTgz, err := installer.KFMonKoboRootTgz()
		if err != nil {
			return fmt.Errorf("extracting kfmon KoboRoot.tgz: %w", err)
		}
		sources = append(sources, kfmonTgz)
	}

	if m.NickelMenu.Enabled {
		installed, err := installer.IsNickelMenuInstalled(mountPoint)
		if err != nil {
			return fmt.Errorf("checking nickelmenu: %w", err)
		}
		if !installed {
			nmTgz, err := installer.FetchNickelMenuTgz(ctx, m.NickelMenu, gh)
			if err != nil {
				return fmt.Errorf("fetching nickelmenu KoboRoot.tgz: %w", err)
			}
			sources = append(sources, nmTgz)
		}
	}

	if len(sources) == 0 {
		return nil
	}

	dest := filepath.Join(mountPoint, ".kobo", "KoboRoot.tgz")
	fmt.Fprintf(os.Stderr, "Merging %d KoboRoot.tgz payload(s)...\n", len(sources))
	if err := installer.MergeKoboRootTgz(dest, sources...); err != nil {
		return fmt.Errorf("merging KoboRoot.tgz: %w", err)
	}
	return nil
}

// printPostProvisionInstructions prints next steps for the user after provisioning.
func printPostProvisionInstructions(hardeningEnabled bool) {
	fmt.Println()
	fmt.Println("Provisioning complete. Next steps:")
	fmt.Println("  1. Safely eject the Kobo")
	fmt.Println("  2. Unplug and reboot the Kobo")
	fmt.Println("  3. On first boot, KFMon and NickelMenu will be installed from KoboRoot.tgz")
	fmt.Println("  4. KOReader can be launched from the NickelMenu or by opening its launcher image")
	if hardeningEnabled {
		fmt.Println("  5. Hardening boot scripts will run automatically via KFMon on_boot hook")
		fmt.Println("  6. KoboRoot guard will activate automatically after first reboot")
		fmt.Println("  7. Set parental controls PIN: More -> Settings -> Accounts -> Parental Controls")
	}
}
