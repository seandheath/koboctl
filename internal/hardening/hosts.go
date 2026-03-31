// Package hardening implements security hardening operations for Kobo e-readers.
//
// All operations that modify the ext4 root filesystem (e.g., /etc/hosts, /etc/resolv.conf)
// are staged as shell scripts on the FAT32 /mnt/onboard partition, which is the only
// partition accessible via USB Mass Storage. These scripts are executed at boot time
// via the KFMon on_boot hook.
package hardening

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// BlocklistEntries returns the domains that should be resolved to 0.0.0.0.
//
// These were identified via packet capture of a stock Kobo and cover Kobo/Rakuten
// telemetry, Google Analytics, Hotjar behavior tracking, and DoubleClick ad tracking.
//
// Domains required for KOReader metadata fetching are NOT included:
//   - openlibrary.org, covers.openlibrary.org (Open Library covers/metadata)
//   - www.googleapis.com, books.google.com (Google Books API)
func BlocklistEntries() []string {
	return []string{
		// Kobo telemetry and store (not needed in sideload mode)
		"storeapi.kobo.com",
		"api.kobobooks.com",
		"auth.kobobooks.com",
		"sync.kobobooks.com",
		"download.kobobooks.com",
		"telemetry.kobo.com",

		// Google Analytics
		"www.google-analytics.com",
		"ssl.google-analytics.com",

		// Google ad tracking
		"stats.g.doubleclick.net",

		// Hotjar behavior analytics
		"script.hotjar.com",
		"static.hotjar.com",
		"vars.hotjar.com",

		// IP geolocation service (fingerprinting)
		"api.ipinfodb.com",
	}
}

// StageHostsBlocklist writes a boot script that appends the telemetry blocklist
// to /etc/hosts on first run. The script is idempotent (checks for a marker line).
//
// The script is placed at <mountPoint>/.adds/koboctl/harden-hosts.sh and executed
// at boot by run-hardening.sh via the KFMon on_boot hook.
func StageHostsBlocklist(mountPoint string, cfg manifest.HardeningConfig) error {
	if !cfg.Privacy.HostsBlocklist {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# koboctl: apply /etc/hosts telemetry blocklist\n")
	sb.WriteString("MARKER=\"# koboctl-blocklist-start\"\n")
	sb.WriteString("if grep -q \"$MARKER\" /etc/hosts 2>/dev/null; then\n")
	sb.WriteString("    exit 0  # Already applied\n")
	sb.WriteString("fi\n")
	sb.WriteString("cat >> /etc/hosts << 'EOF'\n")
	sb.WriteString("# koboctl-blocklist-start\n")

	for _, domain := range BlocklistEntries() {
		fmt.Fprintf(&sb, "0.0.0.0 %s\n", domain)
	}

	sb.WriteString("# koboctl-blocklist-end\n")
	sb.WriteString("EOF\n")

	dest := filepath.Join(mountPoint, ".adds", "koboctl", "harden-hosts.sh")
	return installer.WriteFile(dest, []byte(sb.String()), 0o755)
}
