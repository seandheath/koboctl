package hardening

import (
	"path/filepath"

	"github.com/seandheath/koboctl/internal/installer"
)

// runHardeningScript is the master boot hook that executes all harden-*.sh scripts.
// It is placed on the FAT32 partition and invoked at boot via the KFMon on_boot hook.
const runHardeningScript = `#!/bin/sh
# koboctl hardening hook -- runs all hardening scripts on boot
LOGFILE="/mnt/onboard/.adds/koboctl/hardening.log"
echo "$(date): koboctl hardening starting" >> "$LOGFILE"

HOOK_DIR="/mnt/onboard/.adds/koboctl"
for script in "$HOOK_DIR"/harden-*.sh; do
    [ -f "$script" ] || continue
    echo "$(date): running $(basename $script)" >> "$LOGFILE"
    sh "$script" >> "$LOGFILE" 2>&1
done

echo "$(date): koboctl hardening complete" >> "$LOGFILE"
`

// kfmonBootINI is the KFMon config that triggers the hardening hook runner on boot.
// KFMon's on_boot=true / on_boot_trigger=true causes it to run the command once
// during KFMon initialisation, before Nickel finishes loading.
//
// A trigger image at /mnt/onboard/koboctl.png is required by KFMon — it uses the
// image path as the database entry key.
const kfmonBootINI = `[koboctl]
filename = /mnt/onboard/koboctl.png
label = koboctl-hardening
db_title = koboctl Hardening
db_author = koboctl
db_comment = System hardening hooks -- do not delete
command = /bin/sh /mnt/onboard/.adds/koboctl/run-hardening.sh
on_boot = true
on_boot_trigger = true
`

// minimalPNG is a 1x1 transparent PNG. KFMon requires a trigger image file
// at the configured filename path. This minimal image satisfies that requirement
// without adding a visible book to the library.
//
// Generated from: convert -size 1x1 xc:none PNG32:koboctl.png | xxd -i
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk length + type
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // width=1, height=1
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, // bit depth=8, color type=6 (RGBA)
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, // IHDR CRC, IDAT length + type
	0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02, // IDAT compressed data
	0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00, 0x00, // IDAT CRC
	0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, // IEND chunk
	0x60, 0x82, // IEND CRC
}

// StageBootHookRunner writes the master run-hardening.sh script to the FAT32 partition.
// This script is called by the KFMon on_boot hook and iterates over all harden-*.sh scripts.
func StageBootHookRunner(mountPoint string) error {
	dest := filepath.Join(mountPoint, ".adds", "koboctl", "run-hardening.sh")
	return installer.WriteFile(dest, []byte(runHardeningScript), 0o755)
}

// StageKFMonBootConfig writes the KFMon boot config and the required trigger PNG.
//
// The config at .adds/kfmon/config/koboctl.ini tells KFMon to run run-hardening.sh
// once during boot initialisation. The PNG at koboctl.png is required by KFMon as
// the database trigger image; it is a 1x1 transparent pixel.
func StageKFMonBootConfig(mountPoint string) error {
	iniDest := filepath.Join(mountPoint, ".adds", "kfmon", "config", "koboctl.ini")
	if err := installer.WriteFile(iniDest, []byte(kfmonBootINI), 0o644); err != nil {
		return err
	}

	pngDest := filepath.Join(mountPoint, "koboctl.png")
	return installer.WriteFile(pngDest, minimalPNG, 0o644)
}
