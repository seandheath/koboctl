// Package initcmd provides defaults and TOML template rendering for koboctl init.
//
// The package name is initcmd to avoid collision with Go's built-in init() function.
package initcmd

import "github.com/seandheath/koboctl/internal/manifest"

// koreaderNMEntry is the canonical NickelMenu entry for launching KOReader via KFMon.
// This is the ground-truth entry used in tests and generated manifests.
var koreaderNMEntry = manifest.NickelMenuEntry{
	Location: "main",
	Label:    "KOReader",
	Action:   "dbg_toast",
	Arg:      "Starting KOReader...",
	Chain:    "cmd_spawn:quiet:/usr/bin/kfmon-ipc trigger koreader",
}

// cleanBrowsingDNS is the CleanBrowsing Family Filter resolver pair.
// Blocks adult content, malware, and phishing.
var cleanBrowsingDNS = []string{"185.228.168.168", "185.228.169.168"}

// SecureDefaults returns a hardened baseline manifest suitable for most users.
//
// Enabled: KOReader + KFMon + NickelMenu (with KOReader launch entry).
// Hardening: metadata-only network mode, CleanBrowsing DNS, telemetry and OTA blocked,
// telnet/FTP/SSH disabled, KoboRoot guard active, analytics trigger installed.
func SecureDefaults() manifest.Manifest {
	return manifest.Manifest{
		Device: manifest.DeviceConfig{
			Model: "libra-colour",
		},
		KOReader: manifest.KOReaderConfig{
			Enabled: true,
			Channel: "stable",
			Version: "latest",
		},
		KFMon: manifest.KFMonConfig{
			Enabled: true,
		},
		NickelMenu: manifest.NickelMenuConfig{
			Enabled: true,
			Version: "latest",
			Entries: []manifest.NickelMenuEntry{koreaderNMEntry},
		},
		Plato: manifest.PlatoConfig{
			Enabled: false,
		},
		Hardening: manifest.HardeningConfig{
			Enabled: true,
			Network: manifest.HardeningNetworkConfig{
				Mode:           "metadata-only",
				DNSServers:     cleanBrowsingDNS,
				BlockTelemetry: true,
				BlockOTA:       true,
				BlockSync:      true,
			},
			Parental: manifest.HardeningParentalConfig{
				Enabled:     true,
				LockStore:   true,
				LockBrowser: true,
			},
			Services: manifest.HardeningServicesConfig{
				DisableTelnet: true,
				DisableFTP:    true,
				DisableSSH:    true,
			},
			Filesystem: manifest.HardeningFSConfig{
				NoexecOnboard:          false, // NOT SUPPORTED — breaks KOReader/KFMon
				DisableKoboRoot:        true,
				RemoveDangerousPlugins: true,
			},
			Privacy: manifest.HardeningPrivacyConfig{
				BlockAnalyticsDB: true,
				HostsBlocklist:   true,
			},
		},
	}
}

// DefaultNickelMenuEntries returns the standard set of NickelMenu entries.
// If includeKOReader is true, the canonical KOReader launch entry is included.
func DefaultNickelMenuEntries(includeKOReader bool) []manifest.NickelMenuEntry {
	if !includeKOReader {
		return nil
	}
	return []manifest.NickelMenuEntry{koreaderNMEntry}
}

// ChildSafeDefaults returns a manifest with all hardening options maximally enabled.
// Used when the user selects the "child-safe defaults" shortcut during koboctl init.
//
// Differences from SecureDefaults:
//   - Network mode "offline" (all outbound blocked, not just metadata-only)
//   - No DNS servers configured (offline mode doesn't use DNS filtering)
func ChildSafeDefaults() manifest.Manifest {
	m := SecureDefaults()
	m.Hardening.Network.Mode = "offline"
	m.Hardening.Network.DNSServers = nil // not applicable in offline mode
	m.Hardening.Network.BlockTelemetry = true
	m.Hardening.Network.BlockOTA = true
	m.Hardening.Network.BlockSync = true
	return m
}
