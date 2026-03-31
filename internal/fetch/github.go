package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/google/go-github/v67/github"
)

// GitHubClient wraps the go-github client with helpers for release asset fetching.
// Uses unauthenticated access (60 req/hr limit). Downloads use the CDN
// (BrowserDownloadURL) which does not count against the API rate limit.
type GitHubClient struct {
	client *github.Client
}

// NewGitHubClient creates an unauthenticated GitHub API client.
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{client: github.NewClient(nil)}
}

// LatestRelease returns the tag name and asset list for the latest release of owner/repo.
func (c *GitHubClient) LatestRelease(ctx context.Context, owner, repo string) (string, []*github.ReleaseAsset, error) {
	rel, _, err := c.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return "", nil, fmt.Errorf("fetching latest release for %s/%s: %w", owner, repo, err)
	}
	return rel.GetTagName(), rel.Assets, nil
}

// ReleaseByTag returns the asset list for a specific tagged release.
func (c *GitHubClient) ReleaseByTag(ctx context.Context, owner, repo, tag string) ([]*github.ReleaseAsset, error) {
	rel, _, err := c.client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return nil, fmt.Errorf("fetching release %s for %s/%s: %w", tag, owner, repo, err)
	}
	return rel.Assets, nil
}

// FindAsset returns the first release asset whose name matches the given glob pattern.
// Uses path.Match (not filepath.Match) for cross-platform consistency.
func FindAsset(assets []*github.ReleaseAsset, pattern string) (*github.ReleaseAsset, error) {
	for _, a := range assets {
		matched, err := path.Match(pattern, a.GetName())
		if err != nil {
			return nil, fmt.Errorf("invalid asset pattern %q: %w", pattern, err)
		}
		if matched {
			return a, nil
		}
	}
	return nil, fmt.Errorf("no release asset matches pattern %q", pattern)
}

// DownloadAsset downloads the release asset at the given CDN URL to destPath.
// Uses the BrowserDownloadURL which bypasses GitHub API rate limits.
// Progress is written to stderr as plain lines (works in both TTY and pipe).
func (c *GitHubClient) DownloadAsset(ctx context.Context, downloadURL, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("creating download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %q: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download of %q returned HTTP %d", downloadURL, resp.StatusCode)
	}

	// Write to a temp file in the same directory, then rename atomically.
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating destination directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".koboctl-download-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpPath) // no-op if rename succeeded
	}()

	filename := filepath.Base(destPath)
	fmt.Fprintf(os.Stderr, "downloading %s...\n", filename)

	written, err := io.Copy(tmp, resp.Body)
	if err != nil {
		return fmt.Errorf("writing download to %q: %w", tmpPath, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("moving download to %q: %w", destPath, err)
	}

	fmt.Fprintf(os.Stderr, "downloaded %s (%d bytes)\n", filename, written)
	return nil
}

// LatestReleaseOrTag is a convenience wrapper: if version is "latest" or empty it
// calls LatestRelease; otherwise it calls ReleaseByTag and returns the provided tag.
func (c *GitHubClient) LatestReleaseOrTag(ctx context.Context, owner, repo, version string) (string, []*github.ReleaseAsset, error) {
	if version == "" || version == "latest" {
		return c.LatestRelease(ctx, owner, repo)
	}
	assets, err := c.ReleaseByTag(ctx, owner, repo, version)
	if err != nil {
		return "", nil, err
	}
	return version, assets, nil
}

// FetchAsset ensures the named artifact is cached locally, downloading it if needed.
// It returns the local path to the cached file.
//
//   - component: cache subdirectory name (e.g., "koreader")
//   - version: release tag (e.g., "v2024.11")
//   - asset: the release asset to download
func (c *GitHubClient) FetchAsset(ctx context.Context, component, version string, asset *github.ReleaseAsset) (string, error) {
	filename := asset.GetName()
	cached, err := IsCached(component, version, filename)
	if err != nil {
		return "", err
	}

	destPath, err := CachedPath(component, version, filename)
	if err != nil {
		return "", err
	}

	if cached {
		return destPath, nil
	}

	if _, err := EnsureCacheDir(component, version); err != nil {
		return "", err
	}

	if err := c.DownloadAsset(ctx, asset.GetBrowserDownloadURL(), destPath); err != nil {
		return "", err
	}

	return destPath, nil
}
