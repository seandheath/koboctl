package hardening_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/hardening"
)

// safePlugins are plugins that must NOT be removed.
var safePlugins = []string{
	"bookinfo.koplugin",
	"opds.koplugin",
	"newsdownloader.koplugin",
	"statistics.koplugin",
	"calibre.koplugin",
}

func makeMockPluginDir(t *testing.T, dir string, plugins []string) string {
	t.Helper()
	pluginDir := filepath.Join(dir, ".adds", "koreader", "plugins")
	for _, name := range plugins {
		if err := os.MkdirAll(filepath.Join(pluginDir, name), 0o755); err != nil {
			t.Fatalf("creating plugin dir %s: %v", name, err)
		}
	}
	return pluginDir
}

func TestRemoveDangerousPlugins_RemovesOnlyDangerous(t *testing.T) {
	dir := t.TempDir()

	all := append(hardening.DangerousPlugins(), safePlugins...)
	pluginDir := makeMockPluginDir(t, dir, all)

	removed, err := hardening.RemoveDangerousPlugins(dir)
	if err != nil {
		t.Fatalf("RemoveDangerousPlugins: %v", err)
	}

	// All dangerous plugins should be reported as removed.
	want := hardening.DangerousPlugins()
	if len(removed) != len(want) {
		t.Errorf("removed %d plugins, want %d; got %v", len(removed), len(want), removed)
	}

	// Dangerous plugins must be gone.
	for _, name := range hardening.DangerousPlugins() {
		if _, err := os.Stat(filepath.Join(pluginDir, name)); !os.IsNotExist(err) {
			t.Errorf("dangerous plugin %s should have been removed", name)
		}
	}

	// Safe plugins must survive.
	for _, name := range safePlugins {
		if _, err := os.Stat(filepath.Join(pluginDir, name)); err != nil {
			t.Errorf("safe plugin %s should not have been removed: %v", name, err)
		}
	}
}

func TestRemoveDangerousPlugins_NoPluginsPresent(t *testing.T) {
	dir := t.TempDir()
	// Create the plugin dir with only safe plugins.
	makeMockPluginDir(t, dir, safePlugins)

	removed, err := hardening.RemoveDangerousPlugins(dir)
	if err != nil {
		t.Fatalf("RemoveDangerousPlugins: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removals, got %v", removed)
	}
}

func TestRemoveDangerousPlugins_Idempotent(t *testing.T) {
	dir := t.TempDir()
	makeMockPluginDir(t, dir, hardening.DangerousPlugins())

	if _, err := hardening.RemoveDangerousPlugins(dir); err != nil {
		t.Fatal(err)
	}
	// Second call with nothing to remove.
	removed, err := hardening.RemoveDangerousPlugins(dir)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("second call should remove nothing, got %v", removed)
	}
}
