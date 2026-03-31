package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/seandheath/koboctl/internal/device"
	"github.com/seandheath/koboctl/internal/hardening"
	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// componentStatus is the JSON-serialisable status of a single component.
type componentStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// hardeningStatusJSON is the JSON-serialisable hardening state.
type hardeningStatusJSON struct {
	HostsScriptStaged     bool     `json:"hosts_script_staged"`
	DNSScriptStaged       bool     `json:"dns_script_staged"`
	DevmodeScriptStaged   bool     `json:"devmode_script_staged"`
	BootHookConfigured    bool     `json:"boot_hook_configured"`
	AnalyticsTrigger      bool     `json:"analytics_trigger"`
	KoboRootGuarded       bool     `json:"koboroot_guarded"`
	OTADisabled           bool     `json:"ota_disabled"`
	SyncDisabled          bool     `json:"sync_disabled"`
	SideloadEnabled       bool     `json:"sideload_enabled"`
	LibraryExcludeSet     bool     `json:"library_exclude_set"`
	DangerousPluginsFound []string `json:"dangerous_plugins_found"`
	ParentalEnabled       bool     `json:"parental_enabled"`
}

// deviceStatus is the full JSON-serialisable status output.
type deviceStatus struct {
	Model           string               `json:"model"`
	FirmwareVersion string               `json:"firmware_version"`
	SerialNumber    string               `json:"serial_number"`
	MountPoint      string               `json:"mount_point"`
	Components      []componentStatus    `json:"components"`
	Hardening       *hardeningStatusJSON `json:"hardening,omitempty"`
}

func newStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report current device state",
		Long:  `Detect the connected Kobo device and report installed components and configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			modelName := di.Model
			if di.Profile != nil {
				modelName = di.Profile.Name + " (" + di.Model + ")"
			} else if di.Model != "" {
				modelName = di.Model + " (unknown model)"
			}

			// Gather component status.
			components := []componentStatus{
				checkComponent(di.MountPoint, "KOReader", ".adds/koreader/koreader.sh", koreaderVersion),
				checkComponent(di.MountPoint, "KFMon", ".adds/kfmon/config/kfmon.ini", kfmonVersionFn),
				checkComponent(di.MountPoint, "NickelMenu", ".adds/nm", nil),
			}

			// Gather hardening status if a manifest is available.
			var hstate *hardening.HardeningState
			if m, err := manifest.LoadManifest(manifestPath); err == nil && m.Hardening.Enabled {
				st := hardening.HardeningStatus(di.MountPoint, m.Hardening)
				hstate = &st
			}

			if jsonOutput {
				out := deviceStatus{
					Model:           modelName,
					FirmwareVersion: di.FirmwareVersion,
					SerialNumber:    di.SerialNumber,
					MountPoint:      di.MountPoint,
					Components:      components,
				}
				if hstate != nil {
					out.Hardening = &hardeningStatusJSON{
						HostsScriptStaged:     hstate.HostsScriptStaged,
						DNSScriptStaged:       hstate.DNSScriptStaged,
						DevmodeScriptStaged:   hstate.DevmodeScriptStaged,
						BootHookConfigured:    hstate.BootHookConfigured,
						AnalyticsTrigger:      hstate.AnalyticsTrigger,
						KoboRootGuarded:       hstate.KoboRootGuarded,
						OTADisabled:           hstate.OTADisabled,
						SyncDisabled:          hstate.SyncDisabled,
						SideloadEnabled:       hstate.SideloadEnabled,
						LibraryExcludeSet:     hstate.LibraryExcludeSet,
						DangerousPluginsFound: hstate.DangerousPluginsFound,
						ParentalEnabled:       hstate.ParentalEnabled,
					}
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			// Human-readable output.
			fmt.Printf("Device:       %s\n", modelName)
			fmt.Printf("Firmware:     %s\n", di.FirmwareVersion)
			if di.SerialNumber != "" {
				fmt.Printf("Serial:       %s\n", di.SerialNumber)
			}
			fmt.Printf("Mount:        %s\n", di.MountPoint)
			fmt.Println()
			fmt.Println("Components:")
			for _, c := range components {
				status := "not installed"
				if c.Installed {
					status = "installed"
					if c.Version != "" && c.Version != "unknown" {
						status += " (" + c.Version + ")"
					}
				}
				fmt.Printf("  %-14s %s\n", c.Name, status)
			}

			if hstate != nil {
				fmt.Println()
				fmt.Println("Hardening:")
				printHardeningStatus(hstate)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output status as JSON")
	return cmd
}

// printHardeningStatus prints the human-readable hardening status table.
func printHardeningStatus(s *hardening.HardeningState) {
	check := func(ok bool) string {
		if ok {
			return "ok"
		}
		return "NOT SET"
	}
	staged := func(ok bool) string {
		if ok {
			return "staged (pending boot)"
		}
		return "NOT STAGED"
	}

	fmt.Printf("  %-26s %s\n", "Hosts blocklist", staged(s.HostsScriptStaged))
	fmt.Printf("  %-26s %s\n", "DNS lockdown", staged(s.DNSScriptStaged))
	fmt.Printf("  %-26s %s\n", "Devmode disable", staged(s.DevmodeScriptStaged))
	fmt.Printf("  %-26s %s\n", "Boot hook (KFMon)", check(s.BootHookConfigured))
	fmt.Printf("  %-26s %s\n", "Analytics trigger", check(s.AnalyticsTrigger))
	fmt.Printf("  %-26s %s\n", "KoboRoot guard", check(s.KoboRootGuarded))
	fmt.Printf("  %-26s %s\n", "OTA updates", boolToDisabled(s.OTADisabled))
	fmt.Printf("  %-26s %s\n", "Cloud sync", boolToDisabled(s.SyncDisabled))
	fmt.Printf("  %-26s %s\n", "Sideload mode", boolToEnabled(s.SideloadEnabled))
	fmt.Printf("  %-26s %s\n", "Library exclude folders", check(s.LibraryExcludeSet))
	if len(s.DangerousPluginsFound) > 0 {
		fmt.Printf("  %-26s %v\n", "Dangerous plugins", s.DangerousPluginsFound)
	} else {
		fmt.Printf("  %-26s %s\n", "Dangerous plugins", "0 found")
	}
	if s.ParentalEnabled {
		fmt.Printf("  %-26s %s\n", "Parental controls", "configured")
	} else {
		fmt.Printf("  %-26s %s\n", "Parental controls", "NOT SET")
		fmt.Println("  ! Set manually: More -> Settings -> Accounts -> Parental Controls")
	}
}

func boolToDisabled(v bool) string {
	if v {
		return "disabled"
	}
	return "ENABLED (not hardened)"
}

func boolToEnabled(v bool) string {
	if v {
		return "enabled"
	}
	return "DISABLED (not hardened)"
}

// checkComponent checks whether a component is installed and optionally reads its version.
// markerPath is relative to the mount point and can be a file or directory.
func checkComponent(mountPath, name, markerPath string, versionFn func(string) (string, error)) componentStatus {
	cs := componentStatus{Name: name}
	fullPath := filepath.Join(mountPath, markerPath)
	if _, err := os.Stat(fullPath); err == nil {
		cs.Installed = true
		if versionFn != nil {
			if v, err := versionFn(mountPath); err == nil {
				cs.Version = v
			}
		}
	}
	return cs
}

// kfmonVersionFn wraps installer.KFMonVersion for use as a checkComponent callback.
func kfmonVersionFn(mountPath string) (string, error) {
	return installer.KFMonVersion(mountPath)
}

// koreaderVersion reads the KOReader version from .adds/koreader/git-rev (if present).
func koreaderVersion(mountPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(mountPath, ".adds", "koreader", "git-rev"))
	if err != nil {
		return "unknown", nil
	}
	v := string(data)
	if len(v) > 12 {
		v = v[:12] // Shorten git hashes for display.
	}
	return v, nil
}
