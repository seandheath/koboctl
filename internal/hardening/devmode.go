package hardening

import (
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/installer"
	"github.com/seandheath/koboctl/internal/manifest"
)

// StageDevmodeDisable writes a boot script that kills telnetd, removes telnet
// from inetd, and disables FTP if configured.
//
// On the Kobo, typing "devmodeon" in the search bar enables telnet on port 23
// with root access and no password. This script ensures telnet is not running
// and removes it from inetd on each boot.
//
// The script is placed at <mountPoint>/.adds/koboctl/harden-devmode.sh and
// executed at boot by run-hardening.sh via the KFMon on_boot hook.
func StageDevmodeDisable(mountPoint string, cfg manifest.HardeningConfig) error {
	if !cfg.Services.DisableTelnet && !cfg.Services.DisableFTP {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("# koboctl: ensure devmode/telnet/ftp are disabled\n")

	if cfg.Services.DisableTelnet {
		// Kill any running telnetd process.
		sb.WriteString("pkill -f telnetd 2>/dev/null\n")
		// Remove telnet from inetd configuration.
		sb.WriteString("if [ -f /etc/inetd.conf ]; then\n")
		sb.WriteString("    sed -i '/telnet/d' /etc/inetd.conf 2>/dev/null\n")
		sb.WriteString("    kill -HUP $(pidof inetd) 2>/dev/null\n")
		sb.WriteString("fi\n")
		sb.WriteString("if [ -f /etc/inetd.conf.local ]; then\n")
		sb.WriteString("    sed -i '/telnet/d' /etc/inetd.conf.local 2>/dev/null\n")
		sb.WriteString("fi\n")
	}

	if cfg.Services.DisableFTP {
		// Remove FTP from inetd configuration.
		sb.WriteString("if [ -f /etc/inetd.conf ]; then\n")
		sb.WriteString("    sed -i '/ftp/d' /etc/inetd.conf 2>/dev/null\n")
		sb.WriteString("    kill -HUP $(pidof inetd) 2>/dev/null\n")
		sb.WriteString("fi\n")
	}

	dest := filepath.Join(mountPoint, ".adds", "koboctl", "harden-devmode.sh")
	return installer.WriteFile(dest, []byte(sb.String()), 0o755)
}
