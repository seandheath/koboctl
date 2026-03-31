package hardening

import (
	"bufio"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/manifest"
	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO
)

// HardeningState reports the current hardening status of a mounted Kobo device.
// Fields set to true indicate the corresponding measure is active/configured.
// Checks that depend on ext4 (hosts, resolv.conf) can only verify that the boot
// scripts are staged — actual application requires a reboot.
type HardeningState struct {
	HostsScriptStaged     bool
	DNSScriptStaged       bool
	DevmodeScriptStaged   bool
	BootHookConfigured    bool
	AnalyticsTrigger      bool
	KoboRootGuarded       bool
	OTADisabled           bool
	SyncDisabled          bool
	SideloadEnabled       bool
	DangerousPluginsFound []string
	ParentalEnabled       bool
}

// HardeningStatus reads the current state of hardening on the mounted device.
// It cannot verify ext4 changes (hosts, DNS) from USB — those are reported as
// "staged (pending boot)" based on script presence.
func HardeningStatus(mountPoint string, cfg manifest.HardeningConfig) HardeningState {
	var s HardeningState

	// Boot scripts (staged on FAT32, applied at boot).
	s.HostsScriptStaged = fileExists(filepath.Join(mountPoint, ".adds", "koboctl", "harden-hosts.sh"))
	s.DNSScriptStaged = fileExists(filepath.Join(mountPoint, ".adds", "koboctl", "harden-dns.sh"))
	s.DevmodeScriptStaged = fileExists(filepath.Join(mountPoint, ".adds", "koboctl", "harden-devmode.sh"))
	s.BootHookConfigured = fileExists(filepath.Join(mountPoint, ".adds", "kfmon", "config", "koboctl.ini"))

	// Analytics trigger in SQLite.
	s.AnalyticsTrigger = checkAnalyticsTrigger(mountPoint)

	// KoboRoot guard: must be a directory, not a file.
	guardPath := filepath.Join(mountPoint, ".kobo", "KoboRoot.tgz")
	if info, err := os.Stat(guardPath); err == nil && info.IsDir() {
		s.KoboRootGuarded = true
	}

	// Nickel config values.
	s.OTADisabled, s.SyncDisabled, s.SideloadEnabled = checkNickelConfig(mountPoint)

	// Dangerous plugin scan.
	s.DangerousPluginsFound = findDangerousPlugins(mountPoint)

	// Parental controls.
	s.ParentalEnabled, _ = CheckParentalControls(mountPoint)

	return s
}

// fileExists returns true if path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// checkAnalyticsTrigger returns true if the koboctl_block_analytics trigger exists.
func checkAnalyticsTrigger(mountPoint string) bool {
	dbPath := filepath.Join(mountPoint, ".kobo", "KoboReader.sqlite")
	if _, err := os.Stat(dbPath); err != nil {
		return false
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return false
	}
	defer db.Close()

	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='koboctl_block_analytics'",
	).Scan(&count) //nolint:errcheck
	return count > 0
}

// checkNickelConfig reads the Nickel config and returns whether OTA is disabled,
// sync is disabled, and sideload mode is enabled.
func checkNickelConfig(mountPoint string) (otaDisabled, syncDisabled, sideloadEnabled bool) {
	confPath := filepath.Join(mountPoint, ".kobo", "Kobo", "Kobo eReader.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.IndexByte(line, '='); idx > 0 {
			k := strings.TrimSpace(line[:idx])
			v := strings.TrimSpace(line[idx+1:])
			switch strings.ToLower(k) {
			case "autoupdateenabled":
				otaDisabled = strings.EqualFold(v, "false")
			case "autosync":
				syncDisabled = strings.EqualFold(v, "false")
			case "sideloadedmode":
				sideloadEnabled = strings.EqualFold(v, "true")
			}
		}
	}
	return
}

// findDangerousPlugins returns any dangerous plugins currently present on the device.
func findDangerousPlugins(mountPoint string) []string {
	pluginDir := filepath.Join(mountPoint, ".adds", "koreader", "plugins")
	var found []string
	for _, name := range DangerousPlugins() {
		path := filepath.Join(pluginDir, name)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			found = append(found, name)
		}
	}
	return found
}
