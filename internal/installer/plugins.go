package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
	"github.com/seandheath/koboctl/internal/plugins"
)

// koreaderPluginsDir is the on-device (relative to mount) directory where
// KOReader loads plugins from. It only exists after KOReader is installed.
const koreaderPluginsDir = ".adds/koreader/plugins"

// InstallKOReaderPlugins downloads and installs the KOReader plugins listed in
// cfg.Plugins. Each entry is "name" or "name@version" resolved against the
// built-in registry (internal/plugins).
//
// KOReader must be installed first: plugins extract into .adds/koreader/plugins/,
// which is created by the KOReader install. Callers should run this after
// InstallKOReader.
func InstallKOReaderPlugins(ctx context.Context, mountPath string, cfg manifest.KOReaderConfig, ghClient *fetch.GitHubClient) error {
	if !cfg.Enabled || len(cfg.Plugins) == 0 {
		return nil
	}

	// Precondition: KOReader present. Without it the plugins dir is meaningless
	// and the plugin would never load.
	if ok, _ := IsKOReaderInstalled(mountPath); !ok {
		return fmt.Errorf("koreader plugins: KOReader is not installed; install KOReader before plugins")
	}

	for _, entry := range cfg.Plugins {
		if err := installOnePlugin(ctx, mountPath, entry, ghClient); err != nil {
			return err
		}
	}
	return nil
}

// installOnePlugin resolves, downloads, and extracts a single registry plugin.
func installOnePlugin(ctx context.Context, mountPath, entry string, ghClient *fetch.GitHubClient) error {
	name, version := plugins.Parse(entry)
	src, ok := plugins.Lookup(name)
	if !ok {
		return fmt.Errorf("koreader plugins: unknown plugin %q (known: %v)", name, plugins.Names())
	}

	tag, assets, err := resolveVersion(ctx, ghClient, src.Owner, src.Repo, version)
	if err != nil {
		return fmt.Errorf("plugin %s: resolving version: %w", name, err)
	}

	asset, err := fetch.FindAsset(assets, src.AssetPattern)
	if err != nil {
		return fmt.Errorf("plugin %s: finding release asset: %w", name, err)
	}

	zipPath, err := ghClient.FetchAsset(ctx, "plugin-"+name, tag, asset)
	if err != nil {
		return fmt.Errorf("plugin %s: downloading: %w", name, err)
	}

	fmt.Fprintf(os.Stderr, "plugin %s: extracting %s...\n", name, filepath.Base(zipPath))
	if err := ExtractZipWithRemap(zipPath, mountPath, pluginZipRemap); err != nil {
		return fmt.Errorf("plugin %s: extracting: %w", name, err)
	}

	// Verify the plugin directory landed where KOReader expects it. The zip root
	// entry is "<name>.koplugin/", so the installed dir is that name.
	installed := filepath.Join(mountPath, koreaderPluginsDir, name+".koplugin")
	if ok, _ := CheckInstalled(installed); !ok {
		return fmt.Errorf("plugin %s: verification failed: %q not found after extraction", name, installed)
	}
	fmt.Fprintf(os.Stderr, "plugin %s: installed %s\n", name, tag)
	return nil
}

// pluginZipRemap places a plugin zip's entries under .adds/koreader/plugins/.
// The upstream zip root is already "<name>.koplugin/…", so we only prefix the
// KOReader plugins directory.
func pluginZipRemap(name string) string {
	return koreaderPluginsDir + "/" + name
}
