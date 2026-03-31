package hardening_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/hardening"
	_ "modernc.org/sqlite"
)

// createTestDB creates a minimal KoboReader.sqlite with an AnalyticsEvents table.
func createTestDB(t *testing.T, dir string) string {
	t.Helper()
	dbPath := filepath.Join(dir, ".kobo", "KoboReader.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("creating test db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS AnalyticsEvents (
		Id INTEGER PRIMARY KEY AUTOINCREMENT,
		Type TEXT,
		Payload TEXT
	)`)
	if err != nil {
		t.Fatalf("creating AnalyticsEvents table: %v", err)
	}
	return dbPath
}

func TestInstallAnalyticsTrigger_InstallsAndBlocks(t *testing.T) {
	dir := t.TempDir()
	dbPath := createTestDB(t, dir)

	if err := hardening.InstallAnalyticsTrigger(dir); err != nil {
		t.Fatalf("InstallAnalyticsTrigger: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Trigger should exist.
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='koboctl_block_analytics'",
	).Scan(&count)
	if count == 0 {
		t.Error("trigger not installed")
	}

	// Insert a row — trigger should delete it immediately.
	if _, err := db.Exec("INSERT INTO AnalyticsEvents (Type, Payload) VALUES ('test', 'data')"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	db.QueryRow("SELECT COUNT(*) FROM AnalyticsEvents").Scan(&count)
	if count != 0 {
		t.Errorf("AnalyticsEvents should be empty after trigger, got %d rows", count)
	}
}

func TestInstallAnalyticsTrigger_Idempotent(t *testing.T) {
	dir := t.TempDir()
	createTestDB(t, dir)

	if err := hardening.InstallAnalyticsTrigger(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := hardening.InstallAnalyticsTrigger(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}
}

func TestInstallAnalyticsTrigger_SkipsIfNoDatabase(t *testing.T) {
	dir := t.TempDir()
	// No database at all — should return nil without error.
	if err := hardening.InstallAnalyticsTrigger(dir); err != nil {
		t.Fatalf("expected no error when database absent, got: %v", err)
	}
}

func TestInstallAnalyticsTrigger_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := createTestDB(t, dir)

	if err := hardening.InstallAnalyticsTrigger(dir); err != nil {
		t.Fatal(err)
	}

	backupPath := dbPath + ".koboctl-backup"
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("backup not created at %s: %v", backupPath, err)
	}
}
