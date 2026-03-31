package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/device"
)

// fakeMountPoint creates a temporary directory populated with a realistic Kobo
// FAT32 layout (including binaries, config, books) and returns its path.
func fakeMountPoint(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		filepath.Join(".kobo", "KoboReader.sqlite"):             "sqlite-data",
		filepath.Join(".kobo", "Kobo", "Kobo eReader.conf"):     "[DeveloperSettings]\nEnableDebugServices=false\n",
		filepath.Join(".kobo", "version"):                        "4.39.22801,4.39.22801,N428,3,0,,,,",
		filepath.Join(".kobo", "KoboRoot.tgz"):                  "fake-tgz-bytes",
		filepath.Join(".adds", "koreader", "koreader.sh"):        "#!/bin/sh\n",
		filepath.Join(".adds", "koreader", "settings.reader.lua"): "-- settings\n",
		filepath.Join(".adds", "koreader", "history", "book.lua"): "history",
		filepath.Join(".adds", "kfmon", "bin", "kfmon"):          "kfmon-binary",
		filepath.Join(".adds", "kfmon", "config", "kfmon.ini"):   "[kfmon]\n",
		filepath.Join(".adds", "nm", "koboctl.conf"):             "action=",
		filepath.Join(".adds", "koboctl", "run-hardening.sh"):    "#!/bin/sh\n",
		filepath.Join("My Documents", "book.epub"):               "epub-content",
	}

	for rel, content := range files {
		fullPath := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	return root
}

func fakeDeviceInfo(mountPoint, serial string) *device.DeviceInfo {
	return &device.DeviceInfo{
		Model:           "test-model",
		FirmwareVersion: "4.39.22801",
		SerialNumber:    serial,
		MountPoint:      mountPoint,
	}
}

func TestCreateBackup_RoundTrip(t *testing.T) {
	mount := fakeMountPoint(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "backup.tar.gz")
	di := fakeDeviceInfo(mount, "SN123")

	got, err := CreateBackup(di, BackupOptions{OutputPath: outPath})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if got != outPath {
		t.Errorf("returned path = %q, want %q", got, outPath)
	}

	entries := archiveNames(t, outPath)

	if _, ok := entries[manifestFileName]; !ok {
		t.Error("archive is missing koboctl-backup.json")
	}

	// Every file in the fake mount should appear in the archive.
	wantPaths := []string{
		filepath.Join(".kobo", "KoboReader.sqlite"),
		filepath.Join(".kobo", "Kobo", "Kobo eReader.conf"),
		filepath.Join(".kobo", "KoboRoot.tgz"),
		filepath.Join(".adds", "koreader", "koreader.sh"),
		filepath.Join(".adds", "koreader", "history", "book.lua"),
		filepath.Join("My Documents", "book.epub"),
	}
	for _, p := range wantPaths {
		if _, ok := entries[p]; !ok {
			t.Errorf("archive missing expected path: %s", p)
		}
	}

	m := readManifest(t, outPath)
	if m.Version != manifestSchemaVersion {
		t.Errorf("manifest version = %d, want %d", m.Version, manifestSchemaVersion)
	}
	if m.DeviceSerial != "SN123" {
		t.Errorf("manifest serial = %q, want SN123", m.DeviceSerial)
	}
	if m.FileCount == 0 {
		t.Error("manifest FileCount should not be zero")
	}
}

func TestCreateBackup_DefaultOutputName(t *testing.T) {
	mount := fakeMountPoint(t)
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	di := fakeDeviceInfo(mount, "SNDEFAULT")
	got, err := CreateBackup(di, BackupOptions{})
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(got), "koboctl-backup-SNDEFAULT-") {
		t.Errorf("default filename = %q, want prefix koboctl-backup-SNDEFAULT-", got)
	}
	if !strings.HasSuffix(got, ".tar.gz") {
		t.Errorf("default filename = %q, want .tar.gz suffix", got)
	}
}

func TestCreateBackup_RejectsExistingOutputFile(t *testing.T) {
	mount := fakeMountPoint(t)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "existing.tar.gz")
	os.WriteFile(outPath, []byte("old"), 0o644)

	_, err := CreateBackup(fakeDeviceInfo(mount, "SN000"), BackupOptions{OutputPath: outPath})
	if err == nil {
		t.Error("expected error when output file already exists, got nil")
	}
}

func TestCreateBackup_ManifestIsFirstEntry(t *testing.T) {
	mount := fakeMountPoint(t)
	outPath := filepath.Join(t.TempDir(), "backup.tar.gz")

	if _, err := CreateBackup(fakeDeviceInfo(mount, "SN"), BackupOptions{OutputPath: outPath}); err != nil {
		t.Fatal(err)
	}

	f, _ := os.Open(outPath)
	defer f.Close()
	gz, _ := gzip.NewReader(f)
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading first entry: %v", err)
	}
	if hdr.Name != manifestFileName {
		t.Errorf("first archive entry = %q, want %q", hdr.Name, manifestFileName)
	}
}

// archiveNames returns a set of all tar entry names in the archive at path.
func archiveNames(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	names := map[string]struct{}{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names[hdr.Name] = struct{}{}
	}
	return names
}

// readManifest returns the parsed BackupManifest from the first archive entry.
func readManifest(t *testing.T, path string) BackupManifest {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("tar.Next: %v", err)
	}
	if hdr.Name != manifestFileName {
		t.Fatalf("first entry = %q, want %q", hdr.Name, manifestFileName)
	}
	data, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m BackupManifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return m
}
