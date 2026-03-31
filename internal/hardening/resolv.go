package hardening

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// StageDNSLockdown writes a boot script that sets /etc/resolv.conf to point at
// the configured DNS servers and makes the file immutable via chattr +i so that
// BusyBox udhcpc cannot overwrite it on WiFi connect.
//
// Default DNS servers: CleanBrowsing Family Filter (185.228.168.168 / 185.228.169.168),
// which blocks adult content, malware, and phishing.
//
// The immutable flag (chattr +i) sets the ext4 immutable bit. Even root cannot
// modify the file until chattr -i is run. BusyBox udhcpc fails silently — harmless.
//
// The script is placed at <mountPoint>/.adds/koboctl/harden-dns.sh and executed
// at boot by run-hardening.sh via the KFMon on_boot hook.
func StageDNSLockdown(mountPoint string, cfg manifest.HardeningConfig) error {
	if cfg.Network.Mode == "" && !cfg.Network.BlockTelemetry {
		return nil
	}

	servers := cfg.Network.DNSServers
	if len(servers) == 0 {
		// Default: CleanBrowsing Family Filter
		servers = []string{"185.228.168.168", "185.228.169.168"}
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# koboctl: lock DNS to family-safe resolver\n")
	sb.WriteString("# CleanBrowsing Family Filter blocks adult content, malware, and phishing.\n")
	sb.WriteString("# chattr +i makes resolv.conf immutable so udhcpc cannot overwrite it on WiFi connect.\n")
	sb.WriteString("chattr -i /etc/resolv.conf 2>/dev/null\n")
	sb.WriteString("cat > /etc/resolv.conf << 'EOF'\n")
	for _, s := range servers {
		fmt.Fprintf(&sb, "nameserver %s\n", s)
	}
	sb.WriteString("EOF\n")
	sb.WriteString("chattr +i /etc/resolv.conf\n")

	dest := filepath.Join(mountPoint, ".adds", "koboctl", "harden-dns.sh")
	return installer.WriteFile(dest, []byte(sb.String()), 0o755)
}
