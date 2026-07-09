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
	// Version is a pinned release tag (e.g., "v2024.11") or "latest".
	Version string `toml:"version"`
	// Plugins is a list of KOReader plugins to install by registry name.
	// Each entry is "name" (latest) or "name@vX.Y.Z" to pin a version.
	// See the plugin registry in internal/installer/plugins.go.
	Plugins []string `toml:"plugins"`
	// BootIntoKOReader launches KOReader automatically at device power-on via
	// KFMon's on_boot hook. Exiting KOReader returns to the stock Kobo UI (Nickel).
	BootIntoKOReader bool `toml:"boot_into_koreader"`
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
}

// HardeningServicesConfig controls which network services are disabled.
// (SSH has no listening service on stock Kobo firmware; it is addressed by
// removing SSH.koplugin and disabling Nickel debug services, not a config flag.)
type HardeningServicesConfig struct {
	DisableTelnet bool `toml:"disable_telnet"`
	DisableFTP    bool `toml:"disable_ftp"`
}

// HardeningFSConfig controls filesystem hardening operations.
type HardeningFSConfig struct {
	// NoexecOnboard is NOT SUPPORTED in v1.
	// All hacked Kobo software (KOReader, KFMon, NickelMenu, Plato)
	// executes from /mnt/onboard/.adds/ on the FAT32 partition.
	// Setting noexec would break all of them.
	NoexecOnboard   bool `toml:"noexec_onboard"`
	DisableKoboRoot bool `toml:"disable_koboroot"`
}

// HardeningPrivacyConfig controls telemetry and analytics blocking.
//
// Note: when hardening is enabled, koboctl always removes dangerous KOReader
// plugins and installs the analytics-blocking SQLite trigger; these are not
// configurable toggles.
type HardeningPrivacyConfig struct {
	HostsBlocklist bool `toml:"hosts_blocklist"`
}
