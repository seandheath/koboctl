package installer

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
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
// KFMon's zip contains config files, icons, and a KoboRoot.tgz payload. This
// function extracts only the FAT32 config/icon files. The KoboRoot.tgz is NOT
// placed here — it is merged with other KoboRoot payloads (e.g. NickelMenu) by
// the provision command via MergeKoboRootTgz.
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
// The .kobo/ subtree (containing KoboRoot.tgz) is skipped — it is handled
// separately via KFMonKoboRootTgz + MergeKoboRootTgz.
func kfmonZipRemap(name string) string {
	// Skip .kobo/ entries; KoboRoot.tgz is merged externally.
	if name == ".kobo" || name == ".kobo/" || strings.HasPrefix(name, ".kobo/") {
		return ""
	}
	if name == "kfmon.png" || name == "koreader.png" {
		return ".adds/kfmon/img/" + name
	}
	if strings.HasPrefix(name, "icons/") {
		return ".adds/kfmon/img/" + strings.TrimPrefix(name, "icons/")
	}
	if name == "icons" {
		return ""
	}
	return name
}

// KFMonKoboRootTgz extracts the .kobo/KoboRoot.tgz payload from the embedded KFMon
// zip and returns its raw bytes. The caller is responsible for merging it with other
// KoboRoot payloads via MergeKoboRootTgz before placing on the device.
func KFMonKoboRootTgz() ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(kfmonZip), int64(len(kfmonZip)))
	if err != nil {
		return nil, fmt.Errorf("opening embedded kfmon zip: %w", err)
	}
	for _, f := range r.File {
		if f.Name == ".kobo/KoboRoot.tgz" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("opening KoboRoot.tgz entry: %w", err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("reading KoboRoot.tgz entry: %w", err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("KoboRoot.tgz not found in embedded kfmon zip")
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
