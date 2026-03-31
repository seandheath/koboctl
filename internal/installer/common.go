// Package installer contains component installers for Kobo e-reader software.
package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ExtractZip extracts a zip archive to destDir.
//
// Every extracted path is validated against destDir to prevent zip-slip attacks.
// Permission bits from the zip are applied but failures are non-fatal (FAT32 ignores them).
func ExtractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip %q: %w", zipPath, err)
	}
	defer r.Close()

	destDir = filepath.Clean(destDir)

	for _, f := range r.File {
		destPath, err := safeJoin(destDir, f.Name)
		if err != nil {
			return fmt.Errorf("zip-slip check failed for %q: %w", f.Name, err)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %q: %w", destPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("creating parent dir for %q: %w", destPath, err)
		}

		if err := extractZipFile(f, destPath); err != nil {
			return err
		}

		// Best-effort permission set; silently ignored on FAT32.
		_ = os.Chmod(destPath, f.Mode())
	}
	return nil
}

// extractZipFile writes a single zip entry to destPath.
func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("opening zip entry %q: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %q: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("extracting %q: %w", f.Name, err)
	}
	return nil
}

// ExtractTar extracts a .tar.gz / .tgz archive to destDir.
// Supports gzip compression only.
//
// Every extracted path is validated against destDir to prevent tar-slip attacks.
// Symlinks are extracted as regular files for safety.
func ExtractTar(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("opening tar %q: %w", tarPath, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("decompressing %q: %w", tarPath, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	destDir = filepath.Clean(destDir)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar %q: %w", tarPath, err)
		}

		destPath, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return fmt.Errorf("tar-slip check failed for %q: %w", hdr.Name, err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return fmt.Errorf("creating directory %q: %w", destPath, err)
			}

		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return fmt.Errorf("creating parent for %q: %w", destPath, err)
			}
			if err := extractTarFile(tr, destPath, hdr.FileInfo().Mode()); err != nil {
				return err
			}

		case tar.TypeSymlink:
			// Resolve symlinks as regular files to avoid escape via symlink chains.
			// We skip them on FAT32 anyway (no symlink support).
			// Log and skip for safety.
			continue

		default:
			// Skip special files (devices, fifos, etc.).
			continue
		}
	}
	return nil
}

// extractTarFile writes a single tar entry to destPath with the given mode.
func extractTarFile(r io.Reader, destPath string, mode os.FileMode) error {
	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating %q: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, r); err != nil {
		return fmt.Errorf("extracting to %q: %w", destPath, err)
	}
	return nil
}

// WriteFile writes content to path, creating parent directories as needed.
// If the file already exists with identical content, it is not rewritten (preserves mtime).
func WriteFile(path string, content []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, content) {
			return nil // already up to date
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, content, perm); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}

// CheckInstalled returns true if the given path exists on the filesystem.
func CheckInstalled(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// CheckDiskSpace returns the available bytes at the filesystem containing path.
// Uses syscall.Statfs (Linux only).
func CheckDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, fmt.Errorf("statfs %q: %w", path, err)
	}
	// Bavail is blocks available to unprivileged user.
	return stat.Bavail * uint64(stat.Bsize), nil
}

// safeJoin joins base and name, returning an error if the result would escape base
// (zip-slip / tar-slip protection).
//
// filepath.Join + filepath.Clean resolves all .. components. If the resulting
// path does not start with base, the entry is attempting to escape.
func safeJoin(base, name string) (string, error) {
	joined := filepath.Clean(filepath.Join(base, name))

	// Ensure the result is within base. Both sides are Cleaned so the comparison is stable.
	prefix := filepath.Clean(base) + string(os.PathSeparator)
	if joined != filepath.Clean(base) && !strings.HasPrefix(joined, prefix) {
		return "", fmt.Errorf("path %q escapes destination directory", name)
	}
	return joined, nil
}
