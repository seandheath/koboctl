// Package manifest handles parsing and validation of koboctl.toml manifest files.
package manifest

// Manifest is the top-level declarative configuration for a Kobo device.
type Manifest struct {
	Device     DeviceConfig     `toml:"device"`
	KOReader   KOReaderConfig   `toml:"koreader"`
	NickelMenu NickelMenuConfig `toml:"nickelmenu"`
	KFMon      KFMonConfig      `toml:"kfmon"`
	Plato      PlatoConfig      `toml:"plato"`
	Hardening  HardeningConfig  `toml:"hardening"`
}

// DeviceConfig specifies which device profile to use and where it is mounted.
type DeviceConfig struct {
	// Model is the device profile name (e.g., "libra-colour"). Auto-detected if empty.
	Model string `toml:"model"`
	// Mount is the host-side mount point. Auto-detected if empty.
	Mount string `toml:"mount"`
}

// KOReaderConfig controls KOReader installation and configuration.
type KOReaderConfig struct {
	Enabled bool `toml:"enabled"`
	// Channel is "stable" or "nightly".
	Channel string `toml:"channel"`
	// Version is a pinned release tag (e.g., "v2024.11") or "latest".
	Version string `toml:"version"`
}

// NickelMenuConfig controls NickelMenu installation and menu entries.
type NickelMenuConfig struct {
	Enabled bool              `toml:"enabled"`
	Version string            `toml:"version"`
	Entries []NickelMenuEntry `toml:"entries"`
}

// NickelMenuEntry is a single NickelMenu DSL entry.
//
// Simple one-action entry:
//
//	menu_item :location :label
//	  chain_success :action :arg
//
// Two-step entry (Chain non-empty):
//
//	menu_item :location :label
//	  chain_success :action :arg
//	  chain_success :<chain-action> :<chain-arg>
//
// Chain is encoded as "action:arg" — split with strings.SplitN(Chain, ":", 2).
type NickelMenuEntry struct {
	Location string `toml:"location"`
	Label    string `toml:"label"`
	Action   string `toml:"action"`
	Arg      string `toml:"arg"`
	Chain    string `toml:"chain"` // optional; "action:arg..." for second chain_success line
}

// KFMonConfig controls KFMon installation.
// KFMon is embedded in the koboctl binary (GPLv3, https://github.com/NiLuJe/kfmon).
// KFMon is a dependency for the hardening on_boot hook.
type KFMonConfig struct {
	Enabled bool `toml:"enabled"`
}

// PlatoConfig controls Plato installation (not implemented in phase 1).
type PlatoConfig struct {
	Enabled bool   `toml:"enabled"`
	Version string `toml:"version"`
}

// HardeningConfig controls all security hardening operations applied to the device.
type HardeningConfig struct {
	Enabled    bool                    `toml:"enabled"`
	Network    HardeningNetworkConfig  `toml:"network"`
	Parental   HardeningParentalConfig `toml:"parental"`
	Services   HardeningServicesConfig `toml:"services"`
	Filesystem HardeningFSConfig       `toml:"filesystem"`
	Privacy    HardeningPrivacyConfig  `toml:"privacy"`
}

// HardeningNetworkConfig controls WiFi and network-level restrictions.
type HardeningNetworkConfig struct {
	// Mode controls what network access is permitted.
	// "metadata-only" allows KOReader metadata fetching only (default).
	// "offline" blocks all outbound traffic via hosts blocklist.
	// "open" applies no network restrictions (hosts blocklist disabled).
	Mode           string   `toml:"mode"`
	DNSServers     []string `toml:"dns_servers"`
	BlockTelemetry bool     `toml:"block_telemetry"`
	BlockOTA       bool     `toml:"block_ota"`
	BlockSync      bool     `toml:"block_sync"`
}

// HardeningParentalConfig controls Nickel parental control settings.
type HardeningParentalConfig struct {
	Enabled     bool `toml:"enabled"`
	LockStore   bool `toml:"lock_store"`
	LockBrowser bool `toml:"lock_browser"`
}

// HardeningServicesConfig controls which network services are disabled.
type HardeningServicesConfig struct {
	DisableTelnet bool `toml:"disable_telnet"`
	DisableFTP    bool `toml:"disable_ftp"`
	DisableSSH    bool `toml:"disable_ssh"`
}

// HardeningFSConfig controls filesystem hardening operations.
type HardeningFSConfig struct {
	// NoexecOnboard is NOT SUPPORTED in v1.
	// All hacked Kobo software (KOReader, KFMon, NickelMenu, Plato)
	// executes from /mnt/onboard/.adds/ on the FAT32 partition.
	// Setting noexec would break all of them.
	NoexecOnboard          bool `toml:"noexec_onboard"`
	DisableKoboRoot        bool `toml:"disable_koboroot"`
	RemoveDangerousPlugins bool `toml:"remove_dangerous_plugins"`
}

// HardeningPrivacyConfig controls telemetry and analytics blocking.
type HardeningPrivacyConfig struct {
	BlockAnalyticsDB bool `toml:"block_analytics_db"`
	HostsBlocklist   bool `toml:"hosts_blocklist"`
}
