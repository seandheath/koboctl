package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	initcmd "github.com/seandheath/koboctl/internal/init"
	"github.com/seandheath/koboctl/internal/mstore"
)

// mockKobo builds a fake mounted Kobo FAT32 root with a version file and the
// given component markers, returning the mount path.
func mockKobo(t *testing.T, kfmon, koreader, nm bool) string {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, ".kobo"), 0o755))
	// kobo-<serial>-<firmware>-<model>
	must(os.WriteFile(filepath.Join(root, ".kobo", "version"),
		[]byte("kobo-N123456789-4.39.22801-N428"), 0o644))

	if kfmon {
		must(os.MkdirAll(filepath.Join(root, ".adds", "kfmon", "config"), 0o755))
		must(os.WriteFile(filepath.Join(root, ".adds", "kfmon", "config", "kfmon.ini"), []byte("[]"), 0o644))
	}
	if koreader {
		must(os.MkdirAll(filepath.Join(root, ".adds", "koreader"), 0o755))
		must(os.WriteFile(filepath.Join(root, ".adds", "koreader", "koreader.sh"), []byte("#!/bin/sh\n"), 0o755))
		must(os.WriteFile(filepath.Join(root, ".adds", "koreader", "git-rev"), []byte("v2024.11-abcdef0123456789"), 0o644))
	}
	if nm {
		must(os.MkdirAll(filepath.Join(root, ".adds", "nm"), 0o755))
	}
	return root
}

func TestPollDeviceCmd_DetectsMockKobo(t *testing.T) {
	mount := mockKobo(t, true, true, false)
	m := initcmd.SecureDefaults()

	msg := pollDeviceCmd(mount, &m)().(deviceStatusMsg)
	if msg.err != nil {
		t.Fatalf("detect failed: %v", msg.err)
	}
	if msg.di == nil || msg.di.Model != "N428" {
		t.Fatalf("wrong device: %+v", msg.di)
	}
	if msg.di.FirmwareVersion != "4.39.22801" {
		t.Errorf("firmware = %q", msg.di.FirmwareVersion)
	}

	// Component states: KFMon + KOReader installed, NickelMenu not.
	got := map[string]compStatus{}
	for _, c := range msg.comps {
		got[c.name] = c
	}
	if !got["KFMon"].installed {
		t.Error("KFMon should be detected installed")
	}
	if !got["KOReader"].installed || got["KOReader"].version != "v2024.11-abc" {
		t.Errorf("KOReader status wrong: %+v", got["KOReader"])
	}
	if got["NickelMenu"].installed {
		t.Error("NickelMenu should be absent")
	}
}

func TestPollDeviceCmd_NoDevice(t *testing.T) {
	m := initcmd.SecureDefaults()
	// A non-Kobo directory as an explicit mount → detection error.
	msg := pollDeviceCmd(t.TempDir(), &m)().(deviceStatusMsg)
	if msg.err == nil {
		t.Error("expected error for non-Kobo mount")
	}
}

func TestModel_SaveWritesManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "koboctl.toml")
	// Seed a file so LoadManifest succeeds and original matches disk.
	seed, _ := initcmd.Render(initcmd.SecureDefaults())
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newModel(path, "", Actions{})
	m.m.KOReader.Version = "v2024.11" // change something
	m.saveManifest()

	reloaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(reloaded), `version = "v2024.11"`) {
		t.Errorf("saved manifest missing edited version:\n%s", reloaded)
	}
}

func TestModel_DevicePrimaryLoadAndSave(t *testing.T) {
	mount := mockKobo(t, true, true, false)

	// Seed a device manifest distinct from defaults.
	seed := initcmd.SecureDefaults()
	seed.KOReader.Version = "v2024.11"
	if _, err := mstore.WriteToDevice(mount, &seed); err != nil {
		t.Fatal(err)
	}

	// newModel with an (empty) host path + explicit mount → device-primary.
	m := newModel(filepath.Join(t.TempDir(), "host.toml"), mount, Actions{})
	if m.status.di == nil {
		t.Fatal("device not seeded into model")
	}
	if m.m.KOReader.Version != "v2024.11" {
		t.Errorf("did not load device copy: version=%q", m.m.KOReader.Version)
	}
	if m.manifestPath != mstore.DevicePath(mount) {
		t.Errorf("manifestPath = %q, want device path", m.manifestPath)
	}
	if m.saveTarget() != mstore.DevicePath(mount) {
		t.Errorf("saveTarget = %q, want device path", m.saveTarget())
	}

	// Edit + save → writes back to the device copy.
	m.m.KOReader.Version = "v2025.02"
	m.saveManifest()
	r, err := mstore.Load("", mount)
	if err != nil {
		t.Fatal(err)
	}
	if r.Manifest.KOReader.Version != "v2025.02" {
		t.Errorf("save did not persist to device: version=%q", r.Manifest.KOReader.Version)
	}
}
