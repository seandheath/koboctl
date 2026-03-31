package device

import (
	"fmt"
	"os"
	"path/filepath"
)

// DeviceInfo contains all detected information about a connected Kobo device.
type DeviceInfo struct {
	Model           string
	FirmwareVersion string
	SerialNumber    string
	MountPoint      string
	// Profile is nil if the model is not in the embedded profile database.
	Profile *DeviceProfile
}

// mountSearchRoots are the directories scanned when auto-detecting Kobo mounts.
// /run/media/$USER is added dynamically at runtime.
var mountSearchRoots = []string{"/media", "/mnt"}

// ScanMountPoints scans standard mount locations for Kobo volumes.
// It checks /media/, /mnt/, and /run/media/$USER/ for directories
// that contain a /.kobo/ subdirectory.
func ScanMountPoints() ([]string, error) {
	roots := append([]string{}, mountSearchRoots...)

	// Add /run/media/$USER if $USER is set.
	if user := os.Getenv("USER"); user != "" {
		roots = append(roots, filepath.Join("/run/media", user))
	}

	var found []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			// Directory may not exist; skip silently.
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			candidate := filepath.Join(root, e.Name())
			if IsKoboMount(candidate) {
				found = append(found, candidate)
			}
		}
	}
	return found, nil
}

// IsKoboMount returns true if path contains a /.kobo/ subdirectory.
func IsKoboMount(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".kobo"))
	return err == nil && info.IsDir()
}

// DetectDevice reads device information from the Kobo filesystem at mountPath.
// It tries /.kobo/version first; if the model ID cannot be parsed from it,
// it falls back to /.kobo/.model and /.kobo/serial.
func DetectDevice(mountPath string) (*DeviceInfo, error) {
	if !IsKoboMount(mountPath) {
		return nil, fmt.Errorf("%q does not appear to be a Kobo volume (missing .kobo/ directory)", mountPath)
	}

	fw, err := ReadVersionFiles(mountPath)
	if err != nil {
		return nil, fmt.Errorf("reading device version at %q: %w", mountPath, err)
	}

	di := &DeviceInfo{
		Model:           fw.ModelID,
		FirmwareVersion: fw.FirmwareVersion,
		SerialNumber:    fw.SerialNumber,
		MountPoint:      mountPath,
	}

	// Look up the device profile; unknown models are allowed (Profile stays nil).
	if fw.ModelID != "" {
		if p, err := GetProfile(fw.ModelID); err == nil {
			di.Profile = p
		}
		// Non-fatal: warn at call site if Profile is nil.
	}

	return di, nil
}

// AutoDetect finds the first Kobo volume mounted on the host and returns its DeviceInfo.
// Returns an error if no Kobo volume is found.
func AutoDetect() (*DeviceInfo, error) {
	mounts, err := ScanMountPoints()
	if err != nil {
		return nil, fmt.Errorf("scanning mount points: %w", err)
	}
	if len(mounts) == 0 {
		return nil, fmt.Errorf(
			"no Kobo volume found — check USB connection and that the device is in USB storage mode\n" +
				"searched: /media/, /mnt/, /run/media/$USER/\n" +
				"use --mount to specify the mount point explicitly",
		)
	}
	return DetectDevice(mounts[0])
}
