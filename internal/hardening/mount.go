package hardening

import (
	"fmt"
	"os"
	"path/filepath"
)

// GuardKoboRoot prevents rogue KoboRoot.tgz extraction by replacing any file
// at .kobo/KoboRoot.tgz with a directory of the same name.
//
// The Kobo's init script (rcS) checks for -f .kobo/KoboRoot.tgz and extracts it
// over the root filesystem. A directory at that path fails the -f test, blocking
// both rogue updates and accidental drag-and-drop via USB Mass Storage.
//
// This blocks ALL firmware updates, including legitimate ones. To apply a firmware
// update, the user must remove the guard directory and run koboctl provision again
// afterwards to re-apply hardening.
func GuardKoboRoot(mountPoint string) error {
	guardPath := filepath.Join(mountPoint, ".kobo", "KoboRoot.tgz")

	info, err := os.Stat(guardPath)
	if err == nil && info.IsDir() {
		return nil // Already a directory; guard is active.
	}

	// Remove the file if it exists (could be a legitimate or rogue tgz).
	if err == nil {
		if err := os.Remove(guardPath); err != nil {
			return fmt.Errorf("removing KoboRoot.tgz: %w", err)
		}
	}

	// Create the directory guard.
	if err := os.MkdirAll(guardPath, 0o755); err != nil {
		return fmt.Errorf("creating KoboRoot.tgz guard directory: %w", err)
	}

	return nil
}
