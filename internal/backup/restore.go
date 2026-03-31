package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/seandheath/koboctl/internal/device"
)

// BackupReader holds an opened backup archive and its parsed manifest.
type BackupReader struct {
	Manifest BackupManifest
	path     string
}

// RestoreOptions configures a Restore call.
type RestoreOptions struct {
	// Force bypasses the device serial mismatch error.
	Force bool
}

// OpenBackup opens a backup archive and reads its manifest from the first entry.
func OpenBackup(archivePath string) (*BackupReader, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("opening backup archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	hdr, err := tr.Next()
	if err != nil {
		return nil, fmt.Errorf("reading archive: %w", err)
	}
	if hdr.Name != manifestFileName {
		return nil, fmt.Errorf("backup archive does not start with %q (got %q) — may be corrupt", manifestFileName, hdr.Name)
	}

	data, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("reading backup manifest: %w", err)
	}
	var m BackupManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing backup manifest: %w", err)
	}
	if m.Version != manifestSchemaVersion {
		return nil, fmt.Errorf("unsupported backup schema version %d (expected %d)", m.Version, manifestSchemaVersion)
	}

	return &BackupReader{Manifest: m, path: archivePath}, nil
}

// Restore extracts all files from the backup archive to mountPoint.
//
// A serial mismatch between the backup and the target device is an error
// unless opts.Force is set.
//
// If an archive entry is a regular file but a directory exists at the
// destination path (e.g. the KoboRoot guard converted .kobo/KoboRoot.tgz
// from a file to a directory), the directory is removed before the file
// is written.
func (br *BackupReader) Restore(mountPoint string, di *device.DeviceInfo, opts RestoreOptions) error {
	m := br.Manifest

	// Serial check.
	if m.DeviceSerial != "" && di.SerialNumber != "" && m.DeviceSerial != di.SerialNumber {
		if !opts.Force {
			return fmt.Errorf(
				"backup serial %q does not match device serial %q\n"+
					"use --force to restore to a different device",
				m.DeviceSerial, di.SerialNumber,
			)
		}
		fmt.Printf("warning: serial mismatch (backup: %s, device: %s) — proceeding due to --force\n",
			m.DeviceSerial, di.SerialNumber)
	}

	// Pre-restore summary.
	fmt.Printf("Backup created:  %s\n", m.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Device model:    %s\n", m.DeviceModel)
	fmt.Printf("Device serial:   %s", m.DeviceSerial)
	switch {
	case m.DeviceSerial == di.SerialNumber:
		fmt.Printf(" (matches)\n")
	case m.DeviceSerial == "" || di.SerialNumber == "":
		fmt.Printf(" (cannot verify — serial unknown)\n")
	default:
		fmt.Printf(" (MISMATCH with device %s)\n", di.SerialNumber)
	}
	fmt.Printf("Firmware:        %s\n", m.FirmwareVersion)
	fmt.Printf("Files in backup: %d\n", m.FileCount)
	fmt.Println()

	// Extract.
	f, err := os.Open(br.path)
	if err != nil {
		return fmt.Errorf("opening backup archive: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("reading gzip stream: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	restored := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading archive entry: %w", err)
		}

		if hdr.Name == manifestFileName {
			continue
		}

		// Tar-slip guard.
		destPath := filepath.Join(mountPoint, hdr.Name)
		if !strings.HasPrefix(destPath+string(filepath.Separator), filepath.Clean(mountPoint)+string(filepath.Separator)) {
			return fmt.Errorf("tar-slip: archive entry %q would escape mount point — archive may be corrupt or malicious", hdr.Name)
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating parent directory for %q: %w", hdr.Name, err)
		}

		// If a directory exists where a file should go (e.g. KoboRoot guard
		// converted .kobo/KoboRoot.tgz from a file to a directory), remove it.
		if fi, err := os.Lstat(destPath); err == nil && fi.IsDir() {
			if err := os.RemoveAll(destPath); err != nil {
				return fmt.Errorf("removing directory at %q to restore file: %w", hdr.Name, err)
			}
		}

		if err := writeFile(destPath, tr); err != nil {
			return fmt.Errorf("restoring %q: %w", hdr.Name, err)
		}
		_ = os.Chtimes(destPath, hdr.ModTime, hdr.ModTime)
		restored++
	}

	fmt.Printf("Restored %d files to %s\n", restored, mountPoint)
	return nil
}

func writeFile(destPath string, r io.Reader) error {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Close()
}
