package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v67/github"
	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
)

const (
	kfmonOwner   = "NiLuJe"
	kfmonRepo    = "kfmon"
	kfmonPattern = "KFMon-*.zip"
	// kfmonBinary is the path (relative to mount) that confirms KFMon is installed.
	kfmonBinary = ".adds/kfmon/bin/kfmon"
	// kfmonRevision is where KFMon writes its version string (if present).
	kfmonRevision = ".adds/kfmon/REVISION"
)

// InstallKFMon downloads, extracts, and verifies KFMon on the Kobo filesystem.
//
// KFMon's zip extracts to both .adds/kfmon/ and .kobo/ at the Kobo root.
// The .kobo/KoboRoot.tgz inside the zip is processed by Kobo firmware on the
// next reboot to install KFMon's kernel-level hooks — do not extract it manually.
func InstallKFMon(ctx context.Context, mountPath string, cfg manifest.KFMonConfig, ghClient *fetch.GitHubClient) error {
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

	var zipPath string
	if cfg.URL != "" {
		// Direct URL path: NiLuJe/kfmon has no GitHub releases; binaries are
		// distributed via MobileRead forum. Use the URL from the manifest directly.
		var err error
		zipPath, err = ghClient.FetchURL(ctx, "kfmon", cfg.URL)
		if err != nil {
			return fmt.Errorf("kfmon: downloading from url: %w", err)
		}
	} else {
		tag, assets, err := resolveVersion(ctx, ghClient, kfmonOwner, kfmonRepo, cfg.Version)
		if err != nil {
			return fmt.Errorf("kfmon: resolving version: %w", err)
		}
		asset, err := fetch.FindAsset(assets, kfmonPattern)
		if err != nil {
			return fmt.Errorf("kfmon: finding release asset: %w", err)
		}
		zipPath, err = ghClient.FetchAsset(ctx, "kfmon", tag, asset)
		if err != nil {
			return fmt.Errorf("kfmon: downloading: %w", err)
		}
		fmt.Fprintf(os.Stderr, "kfmon: resolved %s\n", tag)
	}

	// Extract zip to Kobo root.
	fmt.Fprintf(os.Stderr, "kfmon: extracting %s...\n", filepath.Base(zipPath))
	if err := ExtractZip(zipPath, mountPath); err != nil {
		return fmt.Errorf("kfmon: extracting: %w", err)
	}

	// Verify the binary exists post-extraction.
	if ok, _ := IsKFMonInstalled(mountPath); !ok {
		return fmt.Errorf("kfmon: installation verification failed: %q not found after extraction", kfmonBinary)
	}

	fmt.Fprintf(os.Stderr, "kfmon: installed\n")
	return nil
}

// IsKFMonInstalled returns true if the KFMon binary exists at the expected path.
func IsKFMonInstalled(mountPath string) (bool, error) {
	return CheckInstalled(filepath.Join(mountPath, kfmonBinary))
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

// resolveVersion resolves "latest" to the actual latest tag, or fetches assets
// for the specified pinned version. Used by all component installers.
func resolveVersion(ctx context.Context, gh *fetch.GitHubClient, owner, repo, version string) (string, []*github.ReleaseAsset, error) {
	if version == "" || version == "latest" {
		return gh.LatestRelease(ctx, owner, repo)
	}
	assets, err := gh.ReleaseByTag(ctx, owner, repo, version)
	if err != nil {
		return "", nil, err
	}
	return version, assets, nil
}
