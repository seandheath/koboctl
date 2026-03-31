package hardening

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO
)

// CheckParentalControls queries KoboReader.sqlite to determine whether
// parental controls appear to be configured on the device.
//
// Returns (false, nil) if the database doesn't exist or the relevant table/column
// is absent — this is expected on a freshly provisioned device before first boot.
func CheckParentalControls(mountPoint string) (bool, error) {
	dbPath := filepath.Join(mountPoint, ".kobo", "KoboReader.sqlite")

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return false, fmt.Errorf("opening kobo database: %w", err)
	}
	defer db.Close()

	// The parental PIN is stored as a hash in the UserData table.
	// If the table doesn't exist (pre-first-boot), return false without error.
	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='UserData'",
	).Scan(&count)
	if err != nil || count == 0 {
		return false, nil
	}

	// Check for a non-empty parental PIN hash.
	var pinHash string
	err = db.QueryRow(
		"SELECT Value FROM UserData WHERE Key='UserParentalControlPIN' LIMIT 1",
	).Scan(&pinHash)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying parental PIN: %w", err)
	}

	return pinHash != "", nil
}

// PrintParentalControlsReminder prints the manual setup instructions for
// enabling parental controls on the device's touchscreen.
// The PIN cannot be set programmatically from USB.
func PrintParentalControlsReminder() {
	fmt.Println()
	fmt.Println("  Parental controls require manual setup on the device:")
	fmt.Println("    1. Boot the Kobo and complete initial setup")
	fmt.Println("    2. Go to: More -> Settings -> Accounts -> Parental Controls")
	fmt.Println("    3. Set a 4-digit PIN (remember it -- factory reset is the only recovery)")
	fmt.Println("    4. Enable \"Lock Kobo Store\" and \"Lock Web Browser\"")
}
