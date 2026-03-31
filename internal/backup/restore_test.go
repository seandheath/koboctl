package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seandheath/koboctl/internal/device"
)

func TestRestore_RoundTrip(t *testing.T) {
	srcMount := fakeMountPoint(t)
	srcDI := fakeDeviceInfo(srcMount, "SN-RT")
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")

	if _, err := CreateBackup(srcDI, BackupOptions{OutputPath: archive}); err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	br, err := OpenBackup(archive)
	if err != nil {
		t.Fatalf("OpenBackup: %v", err)
	}

	dstMount := t.TempDir()
	dstDI := &device.DeviceInfo{SerialNumber: "SN-RT", MountPoint: dstMount}

	if err := br.Restore(dstMount, dstDI, RestoreOptions{}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	checks := []struct{ rel, want string }{
		{filepath.Join(".kobo", "KoboReader.sqlite"), "sqlite-data"},
		{filepath.Join(".adds", "koreader", "settings.reader.lua"), "-- settings\n"},
		{filepath.Join("My Documents", "book.epub"), "epub-content"},
		{filepath.Join(".kobo", "KoboRoot.tgz"), "fake-tgz-bytes"},
	}
	for _, c := range checks {
		data, err := os.ReadFile(filepath.Join(dstMount, c.rel))
		if err != nil {
			t.Errorf("restored file missing: %s: %v", c.rel, err)
			continue
		}
		if string(data) != c.want {
			t.Errorf("%s content = %q, want %q", c.rel, string(data), c.want)
		}
	}
}

// TestRestore_KoboRootGuardUndo verifies that if the device has converted
// .kobo/KoboRoot.tgz into a directory (the koboroot guard), restoring a backup
// where it was a file removes the directory and writes the file.
func TestRestore_KoboRootGuardUndo(t *testing.T) {
	srcMount := fakeMountPoint(t)
	srcDI := fakeDeviceInfo(srcMount, "SN-GUARD")
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	if _, err := CreateBackup(srcDI, BackupOptions{OutputPath: archive}); err != nil {
		t.Fatal(err)
	}

	// Simulate provisioned device: replace KoboRoot.tgz file with a directory.
	dstMount := t.TempDir()
	koboRootPath := filepath.Join(dstMount, ".kobo", "KoboRoot.tgz")
	os.MkdirAll(koboRootPath, 0o755) // directory where file should be

	br, err := OpenBackup(archive)
	if err != nil {
		t.Fatal(err)
	}
	dstDI := &device.DeviceInfo{SerialNumber: "SN-GUARD", MountPoint: dstMount}
	if err := br.Restore(dstMount, dstDI, RestoreOptions{}); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	fi, err := os.Stat(koboRootPath)
	if err != nil {
		t.Fatalf(".kobo/KoboRoot.tgz missing after restore: %v", err)
	}
	if fi.IsDir() {
		t.Error(".kobo/KoboRoot.tgz should be a file after restore, not a directory")
	}
	data, _ := os.ReadFile(koboRootPath)
	if string(data) != "fake-tgz-bytes" {
		t.Errorf("KoboRoot.tgz content = %q, want \"fake-tgz-bytes\"", string(data))
	}
}

func TestRestore_SerialMismatch_NoForce(t *testing.T) {
	src := fakeMountPoint(t)
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	if _, err := CreateBackup(fakeDeviceInfo(src, "SN-A"), BackupOptions{OutputPath: archive}); err != nil {
		t.Fatal(err)
	}
	br, err := OpenBackup(archive)
	if err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	err = br.Restore(dst, &device.DeviceInfo{SerialNumber: "SN-B", MountPoint: dst}, RestoreOptions{})
	if err == nil {
		t.Fatal("expected serial mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "serial") {
		t.Errorf("error should mention serial, got: %v", err)
	}
}

func TestRestore_SerialMismatch_Force(t *testing.T) {
	src := fakeMountPoint(t)
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	if _, err := CreateBackup(fakeDeviceInfo(src, "SN-A"), BackupOptions{OutputPath: archive}); err != nil {
		t.Fatal(err)
	}
	br, err := OpenBackup(archive)
	if err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := br.Restore(dst, &device.DeviceInfo{SerialNumber: "SN-B", MountPoint: dst}, RestoreOptions{Force: true}); err != nil {
		t.Fatalf("Restore with --force should succeed: %v", err)
	}
}

func TestOpenBackup_InvalidArchive(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.tar.gz")
	os.WriteFile(f, []byte("not a valid gzip"), 0o644)
	if _, err := OpenBackup(f); err == nil {
		t.Error("expected error for invalid archive, got nil")
	}
}

func TestOpenBackup_MissingManifest(t *testing.T) {
	f := filepath.Join(t.TempDir(), "nomanifest.tar.gz")
	out, _ := os.Create(f)
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	content := []byte("hello")
	tw.WriteHeader(&tar.Header{Name: "somefile.txt", Size: int64(len(content))})
	tw.Write(content)
	tw.Close()
	gz.Close()
	out.Close()

	if _, err := OpenBackup(f); err == nil {
		t.Error("expected error for archive missing manifest, got nil")
	}
}

func TestOpenBackup_UnsupportedVersion(t *testing.T) {
	f := filepath.Join(t.TempDir(), "future.tar.gz")
	out, _ := os.Create(f)
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	data := []byte(`{"version":999}`)
	tw.WriteHeader(&tar.Header{Name: manifestFileName, Size: int64(len(data))})
	tw.Write(data)
	tw.Close()
	gz.Close()
	out.Close()

	if _, err := OpenBackup(f); err == nil {
		t.Error("expected error for unsupported schema version, got nil")
	}
}

func TestRestore_TarSlipBlocked(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "slip.tar.gz")
	writeMaliciousArchive(t, archive, "../../etc/evil-file")

	br, err := OpenBackup(archive)
	if err != nil {
		t.Fatalf("OpenBackup: %v", err)
	}

	dst := t.TempDir()
	err = br.Restore(dst, &device.DeviceInfo{MountPoint: dst}, RestoreOptions{Force: true})
	if err == nil {
		t.Error("expected tar-slip error, got nil")
	}
	if !strings.Contains(err.Error(), "tar-slip") {
		t.Errorf("error should mention tar-slip, got: %v", err)
	}
}

func writeMaliciousArchive(t *testing.T, path, evilPath string) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	m := BackupManifest{Version: manifestSchemaVersion}
	manifestData, _ := json.Marshal(m)
	tw.WriteHeader(&tar.Header{Name: manifestFileName, Size: int64(len(manifestData))})
	tw.Write(manifestData)

	evil := []byte("malicious")
	tw.WriteHeader(&tar.Header{Name: evilPath, Size: int64(len(evil))})
	tw.Write(evil)

	tw.Close()
	gz.Close()
	out.Close()
}
