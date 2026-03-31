package hardening

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO
)

// InstallAnalyticsTrigger installs a SQLite trigger in KoboReader.sqlite that
// automatically deletes any rows inserted into the AnalyticsEvents table.
//
// This prevents Kobo telemetry from accumulating even if the /etc/hosts blocklist
// is somehow bypassed. The trigger name is "koboctl_block_analytics" and is
// identifiable in sqlite_master for verification.
//
// The function is idempotent: it checks for the trigger before installing,
// and creates a backup of the database on first run.
func InstallAnalyticsTrigger(mountPoint string) error {
	dbPath := filepath.Join(mountPoint, ".kobo", "KoboReader.sqlite")

	// Database may not exist yet (pre-first-boot). Skip without error.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	// Back up the database before modification (one-time; skip if backup exists).
	backupPath := dbPath + ".koboctl-backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		data, err := os.ReadFile(dbPath)
		if err != nil {
			return fmt.Errorf("reading database for backup: %w", err)
		}
		if err := os.WriteFile(backupPath, data, 0o644); err != nil {
			return fmt.Errorf("writing database backup: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening kobo database: %w", err)
	}
	defer db.Close()

	// Check if the trigger already exists (idempotent).
	var triggerCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='trigger' AND name='koboctl_block_analytics'",
	).Scan(&triggerCount)
	if err != nil {
		return fmt.Errorf("checking for existing trigger: %w", err)
	}
	if triggerCount > 0 {
		return nil // Already installed.
	}

	// Check that the AnalyticsEvents table exists.
	// Older firmware versions may not have it.
	var tableCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='AnalyticsEvents'",
	).Scan(&tableCount)
	if err != nil {
		return fmt.Errorf("checking for AnalyticsEvents table: %w", err)
	}
	if tableCount == 0 {
		return nil // Table absent on this firmware version; nothing to do.
	}

	// Install the trigger: delete all rows after any insert.
	_, err = db.Exec(`
		CREATE TRIGGER koboctl_block_analytics
		AFTER INSERT ON AnalyticsEvents
		BEGIN
			DELETE FROM AnalyticsEvents;
		END
	`)
	if err != nil {
		return fmt.Errorf("creating analytics trigger: %w", err)
	}

	// Nuke any existing analytics data accumulated before provisioning.
	if _, err := db.Exec("DELETE FROM AnalyticsEvents"); err != nil {
		return fmt.Errorf("clearing existing analytics: %w", err)
	}

	return nil
}
