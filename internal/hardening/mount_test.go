package hardening_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/hardening"
)

func TestGuardKoboRoot_ReplacesFileWithDirectory(t *testing.T) {
	dir := t.TempDir()
	koboDir := filepath.Join(dir, ".kobo")
	if err := os.MkdirAll(koboDir, 0o755); err != nil {
		t.Fatal(err)
	}

	guardPath := filepath.Join(koboDir, "KoboRoot.tgz")
	// Place a regular file at the guard path.
	if err := os.WriteFile(guardPath, []byte("fake firmware"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := hardening.GuardKoboRoot(dir); err != nil {
		t.Fatalf("GuardKoboRoot: %v", err)
	}

	info, err := os.Stat(guardPath)
	if err != nil {
		t.Fatalf("guard path does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("guard path should be a directory, not a file")
	}
}

func TestGuardKoboRoot_CreatesDirectoryWhenAbsent(t *testing.T) {
	dir := t.TempDir()

	if err := hardening.GuardKoboRoot(dir); err != nil {
		t.Fatalf("GuardKoboRoot: %v", err)
	}

	guardPath := filepath.Join(dir, ".kobo", "KoboRoot.tgz")
	info, err := os.Stat(guardPath)
	if err != nil {
		t.Fatalf("guard not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("guard should be a directory")
	}
}

func TestGuardKoboRoot_Idempotent(t *testing.T) {
	dir := t.TempDir()

	if err := hardening.GuardKoboRoot(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Second call with directory already in place.
	if err := hardening.GuardKoboRoot(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}

	guardPath := filepath.Join(dir, ".kobo", "KoboRoot.tgz")
	info, err := os.Stat(guardPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("guard should still be a directory after second call")
	}
}
