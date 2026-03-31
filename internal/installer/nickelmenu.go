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

// InstallNickelMenu downloads KoboRoot.tgz and places it at .kobo/KoboRoot.tgz.
//
// Installation mechanism: Kobo firmware detects KoboRoot.tgz on the next boot,
// extracts it, and creates .adds/nm/. The tgz file is consumed and removed by the
// firmware after processing.
//
// Idempotency: checked against .adds/nm/ presence, NOT KoboRoot.tgz existence
// (since the tgz disappears after the first boot).
func InstallNickelMenu(ctx context.Context, mountPath string, cfg manifest.NickelMenuConfig, ghClient *fetch.GitHubClient) error {
	if !cfg.Enabled {
		return nil
	}

	// Check idempotency via .adds/nm/ directory.
	installed, err := IsNickelMenuInstalled(mountPath)
	if err != nil {
		return err
	}
	if installed {
		fmt.Fprintf(os.Stderr, "nickelmenu: already installed, skipping KoboRoot.tgz placement\n")
	} else {
		tag, assets, err := resolveVersion(ctx, ghClient, nickelMenuOwner, nickelMenuRepo, cfg.Version)
		if err != nil {
			return fmt.Errorf("nickelmenu: resolving version: %w", err)
		}

		asset, err := fetch.FindAsset(assets, nickelMenuPattern)
		if err != nil {
			return fmt.Errorf("nickelmenu: finding release asset: %w", err)
		}

		tgzPath, err := ghClient.FetchAsset(ctx, "nickelmenu", tag, asset)
		if err != nil {
			return fmt.Errorf("nickelmenu: downloading: %w", err)
		}

		// Place KoboRoot.tgz at .kobo/KoboRoot.tgz for Kobo firmware to process on reboot.
		dest := filepath.Join(mountPath, nickelMenuKoboRoot)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("nickelmenu: creating .kobo/ directory: %w", err)
		}

		fmt.Fprintf(os.Stderr, "nickelmenu: placing KoboRoot.tgz for firmware installation...\n")
		data, err := os.ReadFile(tgzPath)
		if err != nil {
			return fmt.Errorf("nickelmenu: reading downloaded tgz: %w", err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("nickelmenu: placing KoboRoot.tgz: %w", err)
		}
		fmt.Fprintf(os.Stderr, "nickelmenu: %s placed; will be installed on next Kobo reboot\n", tag)
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
