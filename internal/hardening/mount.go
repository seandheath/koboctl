package hardening

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/seandheath/koboctl/internal/installer"
)

// GuardKoboRoot prevents rogue KoboRoot.tgz extraction by replacing any file
// at .kobo/KoboRoot.tgz with a directory of the same name.
//
// The Kobo's init script (rcS) checks for -f .kobo/KoboRoot.tgz and extracts it
// over the root filesystem. A directory at that path fails the -f test, blocking
// both rogue updates and accidental drag-and-drop via USB Mass Storage.
//
// This blocks ALL firmware updates, including legitimate ones. To apply a firmware
// update, the user must remove the guard directory and run koboctl provision again
// afterwards to re-apply hardening.
func GuardKoboRoot(mountPoint string) error {
	guardPath := filepath.Join(mountPoint, ".kobo", "KoboRoot.tgz")

	info, err := os.Stat(guardPath)
	if err == nil && info.IsDir() {
		return nil // Already a directory; guard is active.
	}

	// Remove the file if it exists (could be a legitimate or rogue tgz).
	if err == nil {
		if err := os.Remove(guardPath); err != nil {
			return fmt.Errorf("removing KoboRoot.tgz: %w", err)
		}
	}

	// Create the directory guard.
	if err := os.MkdirAll(guardPath, 0o755); err != nil {
		return fmt.Errorf("creating KoboRoot.tgz guard directory: %w", err)
	}

	return nil
}

// koboRootGuardScript is a boot script that activates the KoboRoot guard after
// the firmware has processed (and removed) the merged KoboRoot.tgz on first reboot.
// On subsequent boots it's a no-op since the guard directory already exists.
const koboRootGuardScript = `#!/bin/sh
# koboctl: guard KoboRoot.tgz against rogue firmware extraction
GUARD="/mnt/onboard/.kobo/KoboRoot.tgz"
if [ -d "$GUARD" ]; then
    exit 0
fi
# Firmware deletes the tgz after processing. If a file still exists, remove it.
if [ -f "$GUARD" ]; then
    rm -f "$GUARD"
fi
mkdir -p "$GUARD"
echo "KoboRoot guard activated"
`

// StageKoboRootGuard writes a boot script that creates the KoboRoot.tgz guard
// directory on the next boot after the firmware has processed the merged tgz.
//
// This allows the provision flow to skip the immediate guard (which would prevent
// KFMon/NickelMenu installation) while still protecting the device on subsequent boots.
func StageKoboRootGuard(mountPoint string) error {
	dest := filepath.Join(mountPoint, ".adds", "koboctl", "harden-koboroot.sh")
	return installer.WriteFile(dest, []byte(koboRootGuardScript), 0o755)
}
