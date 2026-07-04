package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/fetch"
	"github.com/seandheath/koboctl/internal/manifest"
)

func TestPluginZipRemap(t *testing.T) {
	got := pluginZipRemap("dynamic_panelzoom.koplugin/main.lua")
	want := ".adds/koreader/plugins/dynamic_panelzoom.koplugin/main.lua"
	if got != want {
		t.Errorf("pluginZipRemap = %q, want %q", got, want)
	}
}

// A no-op config must not touch the filesystem or require KOReader.
func TestInstallKOReaderPlugins_Noop(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.KOReaderConfig{Enabled: true} // no plugins listed
	if err := InstallKOReaderPlugins(context.Background(), dir, cfg, fetch.NewGitHubClient()); err != nil {
		t.Errorf("expected no-op with empty plugin list, got: %v", err)
	}

	cfg = manifest.KOReaderConfig{Enabled: false, Plugins: []string{"dynamic_panelzoom"}}
	if err := InstallKOReaderPlugins(context.Background(), dir, cfg, fetch.NewGitHubClient()); err != nil {
		t.Errorf("expected no-op when koreader disabled, got: %v", err)
	}
}

// Installing a plugin without KOReader present must fail the precondition guard
// (before any network access).
func TestInstallKOReaderPlugins_RequiresKOReader(t *testing.T) {
	dir := t.TempDir()
	cfg := manifest.KOReaderConfig{Enabled: true, Plugins: []string{"dynamic_panelzoom"}}
	err := InstallKOReaderPlugins(context.Background(), dir, cfg, fetch.NewGitHubClient())
	if err == nil {
		t.Fatal("expected error when KOReader is not installed")
	}
}

// With the KOReader marker present, the precondition passes; an unknown plugin
// still fails at lookup (again, before network).
func TestInstallKOReaderPlugins_UnknownPlugin(t *testing.T) {
	dir := t.TempDir()
	// Fake KOReader install marker so IsKOReaderInstalled returns true.
	script := filepath.Join(dir, koreaderScript)
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := manifest.KOReaderConfig{Enabled: true, Plugins: []string{"definitely_not_registered"}}
	err := InstallKOReaderPlugins(context.Background(), dir, cfg, fetch.NewGitHubClient())
	if err == nil {
		t.Fatal("expected error for unknown plugin name")
	}
}
