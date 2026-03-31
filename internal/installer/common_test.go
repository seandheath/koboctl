package installer

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// createTestZip creates a zip archive at zipPath containing entries with the given names.
func createTestZip(t *testing.T, zipPath string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZip_Normal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	createTestZip(t, zipPath, map[string]string{
		".adds/koreader/koreader.sh": "#!/bin/sh\necho hello",
	})

	destDir := filepath.Join(dir, "kobo")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ExtractZip(zipPath, destDir); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, ".adds/koreader/koreader.sh"))
	if err != nil {
		t.Fatalf("reading extracted file: %v", err)
	}
	if string(got) != "#!/bin/sh\necho hello" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestExtractZip_ZipSlipBlocked(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	// This zip entry tries to escape the destination directory.
	createTestZip(t, zipPath, map[string]string{
		"../../../etc/passwd": "malicious content",
	})

	destDir := filepath.Join(dir, "kobo")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := ExtractZip(zipPath, destDir)
	if err == nil {
		t.Error("expected error for zip-slip attempt, got nil")
	}
}

func TestExtractZipWithRemap(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "koreader.zip")
	createTestZip(t, zipPath, map[string]string{
		"koreader.png":              "icon-data",
		"koreader/koreader.sh":      "#!/bin/sh\necho koreader",
		"koreader/COPYING":          "GPLv3",
		"koreader/common/module.lua": "-- lua module",
	})

	destDir := filepath.Join(dir, "kobo")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	remap := func(name string) string {
		if name == "koreader.png" {
			return ".adds/kfmon/img/koreader.png"
		}
		if len(name) >= 9 && name[:9] == "koreader/" {
			return ".adds/" + name
		}
		return name
	}

	if err := ExtractZipWithRemap(zipPath, destDir, remap); err != nil {
		t.Fatalf("ExtractZipWithRemap: %v", err)
	}

	checks := map[string]string{
		".adds/kfmon/img/koreader.png":        "icon-data",
		".adds/koreader/koreader.sh":           "#!/bin/sh\necho koreader",
		".adds/koreader/COPYING":               "GPLv3",
		".adds/koreader/common/module.lua":     "-- lua module",
	}
	for rel, want := range checks {
		got, err := os.ReadFile(filepath.Join(destDir, rel))
		if err != nil {
			t.Errorf("missing %s: %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: got %q, want %q", rel, got, want)
		}
	}
}

func TestSafeJoin_Normal(t *testing.T) {
	base := "/tmp/kobo"
	result, err := safeJoin(base, ".adds/kfmon/bin/kfmon")
	if err != nil {
		t.Fatalf("safeJoin: %v", err)
	}
	expected := "/tmp/kobo/.adds/kfmon/bin/kfmon"
	if result != expected {
		t.Errorf("safeJoin = %q, want %q", result, expected)
	}
}

func TestSafeJoin_Escape(t *testing.T) {
	base := "/tmp/kobo"
	_, err := safeJoin(base, "../../etc/passwd")
	if err == nil {
		t.Error("expected error for path escaping base, got nil")
	}
}

func TestWriteFile_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")
	content := []byte("hello world")

	// Write once.
	if err := WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}

	// Capture mtime.
	info1, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Write again with same content — should not modify the file.
	if err := WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}

	info2, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info1.ModTime() != info2.ModTime() {
		t.Error("WriteFile modified file even though content was identical")
	}
}
