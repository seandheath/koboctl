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

// kfmonKOReaderINI is the KFMon trigger config for KOReader.
// The filename paths use the on-device /mnt/onboard/ prefix because KFMon
// reads this config while running on the Kobo itself.
const kfmonKOReaderINI = `[kobo]
name = KOReader
db_title = KOReader
db_author = KOReader Team
db_comment = KOReader is an ebook reader application.
filename = /mnt/onboard/.adds/kfmon/img/koreader.png

[kobo-target]
filename = /mnt/onboard/.adds/koreader/koreader.sh
action = QOBJECT
`

// InstallKOReader downloads, extracts, and configures KOReader on the Kobo filesystem.
//
// KFMon must be installed before calling this function — it writes a KFMon trigger
// config that references KFMon's directory structure.
func InstallKOReader(ctx context.Context, mountPath string, cfg manifest.KOReaderConfig, ghClient *fetch.GitHubClient) error {
	if !cfg.Enabled {
		return nil
	}

	// Resolve version and pick the right pattern based on channel.
	tag, assets, err := resolveVersion(ctx, ghClient, koreaderOwner, koreaderRepo, cfg.Version)
	if err != nil {
		return fmt.Errorf("koreader: resolving version: %w", err)
	}

	// Nightly builds use a slightly different naming scheme; fall back to same
	// pattern and let FindAsset handle it.
	_ = cfg.Channel // used implicitly via version resolution

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

	// Write KFMon trigger config.
	if err := WriteKFMonKOReaderConfig(mountPath); err != nil {
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
func WriteKFMonKOReaderConfig(mountPath string) error {
	dest := filepath.Join(mountPath, ".adds", "kfmon", "config", "koreader.ini")
	return WriteFile(dest, []byte(kfmonKOReaderINI), 0o644)
}
