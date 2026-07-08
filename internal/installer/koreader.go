package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
)

const (
	koreaderOwner   = "koreader"
	koreaderRepo    = "koreader"
	koreaderPattern = "koreader-kobo-*.zip"
	// koreaderScript confirms KOReader is installed.
	koreaderScript = ".adds/koreader/koreader.sh"
)

// kfmonKOReaderINI builds the KFMon trigger config for KOReader.
// The filename paths use the on-device /mnt/onboard/ prefix because KFMon
// reads this config while running on the Kobo itself.
//
// When bootOnStart is true, on_boot/on_boot_trigger are added to the [kobo]
// watch section so KFMon auto-launches KOReader during boot initialisation
// (the same on_boot primitive the hardening hook uses, see
// internal/hardening/runner.go). Exiting KOReader returns to Nickel.
func kfmonKOReaderINI(bootOnStart bool) string {
	boot := ""
	if bootOnStart {
		boot = "on_boot = true\non_boot_trigger = true\n"
	}
	return fmt.Sprintf(`[kobo]
name = KOReader
db_title = KOReader
db_author = KOReader Team
db_comment = KOReader is an ebook reader application.
filename = /mnt/onboard/.adds/kfmon/img/koreader.png
%s
[kobo-target]
filename = /mnt/onboard/.adds/koreader/koreader.sh
action = QOBJECT
`, boot)
}

// InstallKOReader downloads, extracts, and configures KOReader on the Kobo filesystem.
//
// KFMon must be installed before calling this function — it writes a KFMon trigger
// config that references KFMon's directory structure.
func InstallKOReader(ctx context.Context, mountPath string, cfg manifest.KOReaderConfig, ghClient *fetch.GitHubClient) error {
	if !cfg.Enabled {
		return nil
	}

	// Resolve the release (latest or a pinned tag).
	tag, assets, err := resolveVersion(ctx, ghClient, koreaderOwner, koreaderRepo, cfg.Version)
	if err != nil {
		return fmt.Errorf("koreader: resolving version: %w", err)
	}

	asset, err := fetch.FindAsset(assets, koreaderPattern)
	if err != nil {
		return fmt.Errorf("koreader: finding release asset: %w", err)
	}

	zipPath, err := ghClient.FetchAsset(ctx, "koreader", tag, asset)
	if err != nil {
		return fmt.Errorf("koreader: downloading: %w", err)
	}

	fmt.Fprintf(os.Stderr, "koreader: extracting %s...\n", filepath.Base(zipPath))
	if err := ExtractZipWithRemap(zipPath, mountPath, koreaderZipRemap); err != nil {
		return fmt.Errorf("koreader: extracting: %w", err)
	}

	if ok, _ := IsKOReaderInstalled(mountPath); !ok {
		return fmt.Errorf("koreader: installation verification failed: %q not found", koreaderScript)
	}
	fmt.Fprintf(os.Stderr, "koreader: installed %s\n", tag)

	// Write KFMon trigger config (with on_boot autostart when requested).
	if err := WriteKFMonKOReaderConfig(mountPath, cfg.BootIntoKOReader); err != nil {
		return fmt.Errorf("koreader: writing kfmon config: %w", err)
	}

	return nil
}

// koreaderZipRemap transforms KOReader zip entry names to their on-device paths.
// The upstream zip uses "koreader/" as the root prefix, but the Kobo filesystem
// expects files under ".adds/koreader/". The launch icon goes to the KFMon image dir.
func koreaderZipRemap(name string) string {
	if name == "koreader.png" {
		return ".adds/kfmon/img/koreader.png"
	}
	if strings.HasPrefix(name, "koreader/") || name == "koreader" {
		return ".adds/" + name
	}
	return name
}

// IsKOReaderInstalled returns true if the KOReader launch script exists.
func IsKOReaderInstalled(mountPath string) (bool, error) {
	return CheckInstalled(filepath.Join(mountPath, koreaderScript))
}

// WriteKFMonKOReaderConfig writes the KFMon trigger config for KOReader.
// Path: <mountPath>/.adds/kfmon/config/koreader.ini
// This file is read by KFMon on the device; paths use /mnt/onboard/.
// When bootOnStart is true, KFMon auto-launches KOReader at boot.
func WriteKFMonKOReaderConfig(mountPath string, bootOnStart bool) error {
	dest := filepath.Join(mountPath, ".adds", "kfmon", "config", "koreader.ini")
	return WriteFile(dest, []byte(kfmonKOReaderINI(bootOnStart)), 0o644)
}
