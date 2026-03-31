// Package fetch handles artifact downloading and caching for koboctl.
package fetch

import (
	"fmt"
	"os"
	"path/filepath"
)

// CacheDir returns the root cache directory for koboctl artifacts.
// Uses os.UserCacheDir() so it resolves correctly on Linux (~/.cache),
// macOS (~/Library/Caches), and Windows (%LocalAppData%).
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving user cache dir: %w", err)
	}
	return filepath.Join(base, "koboctl"), nil
}

// CachedPath returns the full path for a cached artifact.
// Layout: <cacheDir>/<component>/<version>/<filename>
func CachedPath(component, version, filename string) (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, component, version, filename), nil
}

// IsCached returns true if the artifact file exists in the cache.
func IsCached(component, version, filename string) (bool, error) {
	p, err := CachedPath(component, version, filename)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// EnsureCacheDir creates the cache subdirectory for component+version if it
// does not already exist. Returns the directory path.
func EnsureCacheDir(component, version string) (string, error) {
	base, err := CacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, component, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating cache dir %q: %w", dir, err)
	}
	return dir, nil
}
