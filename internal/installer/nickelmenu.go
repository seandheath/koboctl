package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/seandheath/koboctl/internal/config"
	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
)

const (
	nickelMenuOwner   = "pgaskin"
	nickelMenuRepo    = "NickelMenu"
	nickelMenuPattern = "KoboRoot.tgz"
	// nickelMenuDir is the directory that confirms NickelMenu is installed
	// (created after Kobo processes KoboRoot.tgz on first reboot).
	nickelMenuDir = ".adds/nm"
	// nickelMenuKoboRoot is where the KoboRoot.tgz is placed for the firmware to pick up.
	nickelMenuKoboRoot = ".kobo/KoboRoot.tgz"
)

// InstallNickelMenu writes NickelMenu configuration to the Kobo filesystem.
//
// The KoboRoot.tgz payload is NOT placed here — it is fetched separately via
// FetchNickelMenuTgz and merged with other KoboRoot payloads (e.g. KFMon) by
// the provision command.
//
// Idempotency: checked against .adds/nm/ presence (created after Kobo processes
// KoboRoot.tgz on first reboot).
func InstallNickelMenu(ctx context.Context, mountPath string, cfg manifest.NickelMenuConfig, ghClient *fetch.GitHubClient) error {
	if !cfg.Enabled {
		return nil
	}

	installed, err := IsNickelMenuInstalled(mountPath)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(os.Stderr, "nickelmenu: already installed\n")
	}

	// Write menu config (always — can be written before .adds/nm/ exists;
	// NickelMenu reads it at runtime from .adds/nm/config).
	if len(cfg.Entries) > 0 {
		if err := config.WriteNickelMenuConfig(mountPath, cfg.Entries); err != nil {
			return fmt.Errorf("nickelmenu: writing config: %w", err)
		}
	}

	return nil
}

// FetchNickelMenuTgz downloads the NickelMenu KoboRoot.tgz and returns its raw bytes.
// The caller is responsible for merging it with other KoboRoot payloads via
// MergeKoboRootTgz before placing on the device.
func FetchNickelMenuTgz(ctx context.Context, cfg manifest.NickelMenuConfig, ghClient *fetch.GitHubClient) ([]byte, error) {
	tag, assets, err := resolveVersion(ctx, ghClient, nickelMenuOwner, nickelMenuRepo, cfg.Version)
	if err != nil {
		return nil, fmt.Errorf("nickelmenu: resolving version: %w", err)
	}

	asset, err := fetch.FindAsset(assets, nickelMenuPattern)
	if err != nil {
		return nil, fmt.Errorf("nickelmenu: finding release asset: %w", err)
	}

	tgzPath, err := ghClient.FetchAsset(ctx, "nickelmenu", tag, asset)
	if err != nil {
		return nil, fmt.Errorf("nickelmenu: downloading: %w", err)
	}

	data, err := os.ReadFile(tgzPath)
	if err != nil {
		return nil, fmt.Errorf("nickelmenu: reading downloaded tgz: %w", err)
	}

	fmt.Fprintf(os.Stderr, "nickelmenu: fetched %s KoboRoot.tgz\n", tag)
	return data, nil
}

// IsNickelMenuInstalled returns true if the .adds/nm/ directory exists.
// Note: this is only true after Kobo has rebooted and processed KoboRoot.tgz.
func IsNickelMenuInstalled(mountPath string) (bool, error) {
	info, err := os.Stat(filepath.Join(mountPath, nickelMenuDir))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}
