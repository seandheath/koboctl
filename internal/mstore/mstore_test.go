package mstore

import (
	"os"
	"path/filepath"
	"testing"

	initcmd "github.com/seandheath/koboctl/internal/init"
)

// mockKobo builds a fake mounted Kobo FAT32 root with a valid version file.
func mockKobo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".kobo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".kobo", "version"),
		[]byte("kobo-N123456789-4.39.22801-N428"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// hostManifest writes a rendered SecureDefaults manifest to a host path.
func hostManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "koboctl.toml")
	out, err := initcmd.Render(initcmd.SecureDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_PrefersDeviceCopy(t *testing.T) {
	mount := mockKobo(t)
	host := hostManifest(t)

	// Seed a device copy that differs from the host (version pin).
	m := initcmd.SecureDefaults()
	m.KOReader.Version = "v2024.11"
	if _, err := WriteToDevice(mount, &m); err != nil {
		t.Fatal(err)
	}

	r, err := Load(host, mount)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OnDevice {
		t.Error("expected OnDevice=true when device copy exists")
	}
	if r.Path != DevicePath(mount) {
		t.Errorf("Path = %q, want device path", r.Path)
	}
	if r.Manifest.KOReader.Version != "v2024.11" {
		t.Errorf("loaded host copy, not device: version=%q", r.Manifest.KOReader.Version)
	}
	if r.Device == nil {
		t.Error("Device should be set")
	}
}

func TestLoad_FallsBackToHost(t *testing.T) {
	mount := mockKobo(t) // no device manifest written
	host := hostManifest(t)

	r, err := Load(host, mount)
	if err != nil {
		t.Fatal(err)
	}
	if r.OnDevice {
		t.Error("expected host fallback when device has no config")
	}
	if r.Path != host {
		t.Errorf("Path = %q, want host %q", r.Path, host)
	}
	if r.Device == nil {
		t.Error("Device should still be detected for later persistence")
	}
}

func TestSave_DeviceWhenConnected(t *testing.T) {
	mount := mockKobo(t)
	host := hostManifest(t)
	di := Detect(mount)
	if di == nil {
		t.Fatal("mock Kobo not detected")
	}

	m := initcmd.SecureDefaults()
	m.KOReader.Version = "v2025.01"
	path, err := Save(&m, host, di)
	if err != nil {
		t.Fatal(err)
	}
	if path != DevicePath(mount) {
		t.Errorf("Save wrote %q, want device path", path)
	}
	// Round-trips through Load.
	r, err := Load(host, mount)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OnDevice || r.Manifest.KOReader.Version != "v2025.01" {
		t.Errorf("round-trip failed: onDevice=%v version=%q", r.OnDevice, r.Manifest.KOReader.Version)
	}
}

func TestSave_HostWhenNoDevice(t *testing.T) {
	host := hostManifest(t)
	m := initcmd.SecureDefaults()
	path, err := Save(&m, host, nil)
	if err != nil {
		t.Fatal(err)
	}
	if path != host {
		t.Errorf("Save wrote %q, want host %q", path, host)
	}
}

func TestDevicePath(t *testing.T) {
	got := DevicePath("/media/u/KOBOeReader")
	want := filepath.Join("/media/u/KOBOeReader", ".adds", "koboctl", "koboctl.toml")
	if got != want {
		t.Errorf("DevicePath = %q, want %q", got, want)
	}
}
