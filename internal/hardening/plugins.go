package hardening

import (
	"fmt"
	"os"
	"path/filepath"
)

// DangerousPlugins returns the KOReader plugin directory names that should be
// removed for a child-safe, metadata-only network configuration.
//
// These plugins provide network services or browsing capabilities that are not
// needed and increase the device's attack surface.
//
// Plugins intentionally NOT removed:
//   - bookinfo.koplugin — fetches covers and metadata (core use case)
//   - opds.koplugin — OPDS catalog browser (user may want Project Gutenberg/Standard Ebooks)
//   - newsdownloader.koplugin — user-initiated RSS fetcher
//   - statistics.koplugin — local reading stats, never phones home
//   - calibre.koplugin — Calibre wireless driver (user may want later)
func DangerousPlugins() []string {
	return []string{
		// Third-party web browser plugin — not installed by default, but remove if present.
		"webbrowser.koplugin",
		// Built-in SSH server — no listening services allowed.
		"SSH.koplugin",
		// WebDAV server — exposes device files over HTTP.
		"MyWebDav.koplugin",
		// Send2Ebook — HTTP receiver for incoming documents.
		"send2ebook.koplugin",
	}
}

// RemoveDangerousPlugins deletes plugins from the DangerousPlugins list.
// Only removes plugins that actually exist; missing plugins are not an error.
// Returns a list of plugin names that were actually removed.
func RemoveDangerousPlugins(mountPoint string) ([]string, error) {
	pluginDir := filepath.Join(mountPoint, ".adds", "koreader", "plugins")

	var removed []string
	for _, name := range DangerousPlugins() {
		path := filepath.Join(pluginDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue // Plugin not present; skip.
		}
		if !info.IsDir() {
			continue // Expected a directory; skip to avoid removing unrelated files.
		}
		if err := os.RemoveAll(path); err != nil {
			return removed, fmt.Errorf("removing plugin %s: %w", name, err)
		}
		removed = append(removed, name)
	}

	return removed, nil
}
