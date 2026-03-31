// Package installer contains component installers for Kobo e-reader software.
package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/google/go-github/v67/github"
	"github.com/seandheath/koboctl/internal/fetch"
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
	return extractZipReader(&r.Reader, destDir, nil)
}

// ExtractZipWithRemap extracts a zip archive to destDir, applying remap to each
// entry name before computing the destination path. If remap returns "", the entry
// is skipped.
func ExtractZipWithRemap(zipPath, destDir string, remap func(string) string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip %q: %w", zipPath, err)
	}
	defer r.Close()
	return extractZipReader(&r.Reader, destDir, remap)
}

// ExtractZipBytes extracts an in-memory zip to destDir.
// Applies the same zip-slip protection as ExtractZip.
func ExtractZipBytes(data []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	return extractZipReader(r, destDir, nil)
}

// ExtractZipBytesWithRemap extracts an in-memory zip to destDir, applying remap
// to each entry name before computing the destination path. If remap returns "",
// the entry is skipped.
func ExtractZipBytesWithRemap(data []byte, destDir string, remap func(string) string) error {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	return extractZipReader(r, destDir, remap)
}

// extractZipReader is the shared implementation for ExtractZip, ExtractZipBytes,
// and ExtractZipWithRemap.
//
// When remap is non-nil, each zip entry name is transformed before computing the
// destination path. If remap returns "", the entry is skipped. Zip-slip validation
// runs after remapping.
func extractZipReader(r *zip.Reader, destDir string, remap func(string) string) error {
	destDir = filepath.Clean(destDir)

	for _, f := range r.File {
		name := f.Name
		if remap != nil {
			name = remap(name)
			if name == "" {
				continue
			}
		}
		destPath, err := safeJoin(destDir, name)
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

// MergeKoboRootTgz merges multiple .tgz byte slices into a single combined archive
// at destPath. If the same path appears in multiple sources, the last source wins.
//
// Both KFMon and NickelMenu deliver their system-partition payloads via
// .kobo/KoboRoot.tgz, but only one file can exist at that path. This function
// combines them so both get installed on the next reboot.
func MergeKoboRootTgz(destPath string, sources ...[]byte) error {
	if len(sources) == 0 {
		return nil
	}

	// Collect all entries, last source wins on duplicates.
	type entry struct {
		header *tar.Header
		data   []byte
	}
	seen := make(map[string]int) // name → index in entries
	var entries []entry

	for i, src := range sources {
		gr, err := gzip.NewReader(bytes.NewReader(src))
		if err != nil {
			return fmt.Errorf("source %d: decompressing: %w", i, err)
		}
		tr := tar.NewReader(gr)

		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				gr.Close()
				return fmt.Errorf("source %d: reading tar: %w", i, err)
			}

			var data []byte
			if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
				data, err = io.ReadAll(tr)
				if err != nil {
					gr.Close()
					return fmt.Errorf("source %d: reading %q: %w", i, hdr.Name, err)
				}
			}

			if idx, ok := seen[hdr.Name]; ok {
				entries[idx] = entry{header: hdr, data: data}
			} else {
				seen[hdr.Name] = len(entries)
				entries = append(entries, entry{header: hdr, data: data})
			}
		}
		gr.Close()
	}

	// Write combined archive.
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating %q: %w", destPath, err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		if err := tw.WriteHeader(e.header); err != nil {
			return fmt.Errorf("writing header for %q: %w", e.header.Name, err)
		}
		if len(e.data) > 0 {
			if _, err := tw.Write(e.data); err != nil {
				return fmt.Errorf("writing %q: %w", e.header.Name, err)
			}
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("closing tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("closing gzip: %w", err)
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

// resolveVersion resolves "latest" to the actual latest release tag, or fetches assets
// for the specified pinned version. Used by KOReader and NickelMenu installers.
func resolveVersion(ctx context.Context, gh *fetch.GitHubClient, owner, repo, version string) (string, []*github.ReleaseAsset, error) {
	if version == "" || version == "latest" {
		return gh.LatestRelease(ctx, owner, repo)
	}
	assets, err := gh.ReleaseByTag(ctx, owner, repo, version)
	if err != nil {
		return "", nil, err
	}
	return version, assets, nil
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
