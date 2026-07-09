package hardening

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestOpenKoboDB_SetsBusyTimeout verifies openKoboDB applies PRAGMA
// busy_timeout=5000 so concurrent access waits instead of failing with
// SQLITE_BUSY. Deterministic readback; no concurrency race.
func TestOpenKoboDB_SetsBusyTimeout(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	// Create the file so read-only open (mode=ro) has something to open.
	seed, err := openKoboDB(dbPath, false)
	if err != nil {
		t.Fatalf("seeding db: %v", err)
	}
	if _, err := seed.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("creating table: %v", err)
	}
	seed.Close()

	for _, readOnly := range []bool{false, true} {
		db, err := openKoboDB(dbPath, readOnly)
		if err != nil {
			t.Fatalf("openKoboDB(readOnly=%v): %v", readOnly, err)
		}
		var timeout int
		if err := db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
			db.Close()
			t.Fatalf("reading busy_timeout (readOnly=%v): %v", readOnly, err)
		}
		db.Close()
		if timeout != 5000 {
			t.Errorf("busy_timeout (readOnly=%v) = %d, want 5000", readOnly, timeout)
		}
	}
}
