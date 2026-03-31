package installer

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
)

// KFMon v1.4.6-179-ge000d65 is embedded directly. NiLuJe/kfmon does not publish
// GitHub releases; the binary is distributed via MobileRead forum. GPLv3 license
// permits redistribution with source available at https://github.com/NiLuJe/kfmon.
//
//go:embed kfmon_dist/KFMon-v1.4.6-179-ge000d65.zip
var kfmonZip []byte

const (
	// kfmonVersion is the embedded KFMon build identifier.
	kfmonVersion = "v1.4.6-179-ge000d65"

	// kfmonMarker is a KFMon-specific config file placed on FAT32 by zip extraction.
	// The kfmon binary itself lives inside KoboRoot.tgz and installs to ext4 on first reboot.
	kfmonMarker = ".adds/kfmon/config/kfmon.ini"

	// kfmonRevision is where KFMon writes its version string (if present).
	kfmonRevision = ".adds/kfmon/REVISION"
)

// InstallKFMon extracts the embedded KFMon zip to the Kobo filesystem.
//
// KFMon's zip extracts config files and icons to .adds/kfmon/ and places
// .kobo/KoboRoot.tgz on the FAT32 partition. The KoboRoot.tgz is processed by
// Kobo firmware on the next reboot to install KFMon's binary and kernel hooks
// to the ext4 system partition — do not extract it manually.
func InstallKFMon(ctx context.Context, mountPath string, cfg manifest.KFMonConfig, _ *fetch.GitHubClient) error {
	if !cfg.Enabled {
		return nil
	}

	// Check idempotency.
	installed, err := IsKFMonInstalled(mountPath)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(os.Stderr, "kfmon: already installed, skipping\n")
		return nil
	}

	// Extract embedded zip to Kobo root.
	fmt.Fprintf(os.Stderr, "kfmon: extracting embedded %s...\n", kfmonVersion)
	if err := ExtractZipBytesWithRemap(kfmonZip, mountPath, kfmonZipRemap); err != nil {
		return fmt.Errorf("kfmon: extracting: %w", err)
	}

	// Verify the marker config exists post-extraction.
	if ok, _ := IsKFMonInstalled(mountPath); !ok {
		return fmt.Errorf("kfmon: installation verification failed: %q not found after extraction", kfmonMarker)
	}

	fmt.Fprintf(os.Stderr, "kfmon: installed %s\n", kfmonVersion)
	return nil
}

// kfmonZipRemap transforms KFMon zip entry names to their on-device paths.
// The upstream zip places launcher icons (kfmon.png, koreader.png, icons/plato.png)
// at the root; they belong under .adds/kfmon/img/.
func kfmonZipRemap(name string) string {
	if name == "kfmon.png" || name == "koreader.png" {
		return ".adds/kfmon/img/" + name
	}
	if strings.HasPrefix(name, "icons/") {
		return ".adds/kfmon/img/" + strings.TrimPrefix(name, "icons/")
	}
	if name == "icons" {
		// Bare directory entry — skip, parent dirs are created on demand.
		return ""
	}
	return name
}

// IsKFMonInstalled returns true if KFMon's config marker exists on the FAT32 partition.
// The kfmon binary itself lives on ext4 and is only accessible on-device.
func IsKFMonInstalled(mountPath string) (bool, error) {
	return CheckInstalled(filepath.Join(mountPath, kfmonMarker))
}

// KFMonVersion reads the installed KFMon version from .adds/kfmon/REVISION.
// Returns ("unknown", nil) if the REVISION file does not exist.
func KFMonVersion(mountPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(mountPath, kfmonRevision))
	if os.IsNotExist(err) {
		return "unknown", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading kfmon revision: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
