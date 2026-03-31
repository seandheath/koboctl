// Package backup implements full backup and restore of the Kobo FAT32 partition.
//
// Archives are gzip-compressed tar files. The first entry is always
// "koboctl-backup.json", a JSON-encoded BackupManifest with device metadata.
// Subsequent entries are every file under the mount point, stored with paths
// relative to that mount.
//
// This package operates on a USB-mounted FAT32 Kobo partition. The device
// must be in USB Mass Storage mode; Nickel is not running in that state so
// KoboReader.sqlite is never locked.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/seandheath/koboctl/internal/build"
	"github.com/seandheath/koboctl/internal/device"
)

// manifestSchemaVersion is incremented on backwards-incompatible manifest changes.
const manifestSchemaVersion = 1

// manifestFileName is the well-known name of the metadata entry within the archive.
// Always written first so OpenBackup can read it in O(1).
const manifestFileName = "koboctl-backup.json"

// BackupManifest is the metadata header written as the first tar entry.
type BackupManifest struct {
	Version         int       `json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	KoboctlVersion  string    `json:"koboctl_version"`
	DeviceSerial    string    `json:"device_serial"`
	DeviceModel     string    `json:"device_model"`
	FirmwareVersion string    `json:"firmware_version"`
	FileCount       int       `json:"file_count"`
	TotalBytes      int64     `json:"total_bytes"`
}

// BackupOptions configures a CreateBackup call.
type BackupOptions struct {
	// OutputPath is the destination file. If empty, a name is derived from the
	// device serial and current timestamp in the working directory.
	OutputPath string
}

// fileEntry is an item collected during the mount walk, ready to stream into the archive.
type fileEntry struct {
	absPath     string
	archiveName string
	size        int64
	mode        fs.FileMode
	modTime     time.Time
}

// CreateBackup creates a gzip-compressed tar archive of the entire FAT32
// partition at di.MountPoint. Every file under the mount is included.
// Returns the path of the written archive.
//
// The archive is written atomically: data goes to <outputPath>.tmp and is
// renamed to <outputPath> only on success. A partial .tmp is removed on failure.
func CreateBackup(di *device.DeviceInfo, opts BackupOptions) (string, error) {
	mountPoint := di.MountPoint

	outPath := opts.OutputPath
	if outPath == "" {
		serial := di.SerialNumber
		if serial == "" {
			serial = di.Model
		}
		if serial == "" {
			serial = "unknown"
		}
		ts := time.Now().UTC().Format("20060102-150405")
		outPath = fmt.Sprintf("koboctl-backup-%s-%s.tar.gz", serial, ts)
	}

	if _, err := os.Stat(outPath); err == nil {
		return "", fmt.Errorf("output file already exists: %s — remove it or choose a different path", outPath)
	}

	// Walk the mount once to collect file list and total size for the manifest.
	// We write the manifest first so OpenBackup can read it without scanning the
	// whole archive.
	var files []fileEntry
	var totalBytes int64

	err := filepath.WalkDir(mountPoint, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("  skipping (unreadable): %s: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil // directories are implicit in tar
		}
		rel, err := filepath.Rel(mountPoint, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %q: %w", path, err)
		}
		fi, err := d.Info()
		if err != nil {
			fmt.Printf("  skipping (stat failed): %s: %v\n", path, err)
			return nil
		}
		files = append(files, fileEntry{
			absPath:     path,
			archiveName: rel,
			size:        fi.Size(),
			mode:        fi.Mode(),
			modTime:     fi.ModTime(),
		})
		totalBytes += fi.Size()
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking mount point: %w", err)
	}

	m := BackupManifest{
		Version:         manifestSchemaVersion,
		CreatedAt:       time.Now().UTC(),
		KoboctlVersion:  build.Version,
		DeviceSerial:    di.SerialNumber,
		DeviceModel:     di.Model,
		FirmwareVersion: di.FirmwareVersion,
		FileCount:       len(files),
		TotalBytes:      totalBytes,
	}
	manifestJSON, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshalling backup manifest: %w", err)
	}

	tmpPath := outPath + ".tmp"
	if err := writeArchive(tmpPath, manifestJSON, files); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	if err := atomicRename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("finalising backup file: %w", err)
	}

	fmt.Printf("Backup written: %s (%d files, %d bytes uncompressed)\n", outPath, len(files), totalBytes)
	return outPath, nil
}

// writeArchive creates the gzip+tar archive at path.
func writeArchive(path string, manifestJSON []byte, files []fileEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating archive: %w", err)
	}
	var closed bool
	defer func() {
		if !closed {
			f.Close()
		}
	}()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	if err := tw.WriteHeader(&tar.Header{
		Name:    manifestFileName,
		Size:    int64(len(manifestJSON)),
		Mode:    0o644,
		ModTime: time.Now().UTC(),
	}); err != nil {
		return fmt.Errorf("writing manifest header: %w", err)
	}
	if _, err := tw.Write(manifestJSON); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	for _, fe := range files {
		if err := addFileToArchive(tw, fe.absPath, fe.archiveName, fe.size, fe.mode, fe.modTime); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("closing gzip writer: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing archive file: %w", err)
	}
	closed = true
	return nil
}

// addFileToArchive writes a single file into the tar stream.
func addFileToArchive(tw *tar.Writer, srcPath, archiveName string, size int64, mode fs.FileMode, modTime time.Time) error {
	src, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File disappeared between walk and open (e.g. device ejected).
			fmt.Printf("  skipping (disappeared): %s\n", archiveName)
			return nil
		}
		return fmt.Errorf("opening %q for backup: %w", archiveName, err)
	}
	defer src.Close()

	if err := tw.WriteHeader(&tar.Header{
		Name:    archiveName,
		Size:    size,
		Mode:    int64(mode),
		ModTime: modTime,
	}); err != nil {
		return fmt.Errorf("writing tar header for %q: %w", archiveName, err)
	}
	if _, err := io.Copy(tw, src); err != nil {
		return fmt.Errorf("writing %q to archive: %w", archiveName, err)
	}
	return nil
}

// atomicRename renames src to dst, falling back to copy+delete on EXDEV.
func atomicRename(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDevice(err) {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp archive for copy: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating destination archive: %w", err)
	}
	defer func() {
		out.Close()
		if err != nil {
			os.Remove(dst)
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("copying archive: %w", err)
	}
	if err = out.Close(); err != nil {
		return fmt.Errorf("closing destination: %w", err)
	}
	return os.Remove(src)
}

func isCrossDevice(err error) bool {
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}
