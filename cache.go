package docs

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"go.k6.io/k6/lib/fsext"
)

const (
	// maxFileSize is the maximum allowed size for a single file during extraction.
	// This prevents decompression bombs (gosec G110).
	maxFileSize = 50 << 20 // 50 MB

	// maxBundleSize caps the compressed bundle download.
	// Doc bundles are typically <2 MB; this prevents memory exhaustion
	// from a malicious or corrupted asset before extraction starts.
	maxBundleSize = 100 << 20 // 100 MB
)

// stalenessCheckInterval is how often we re-check the remote ETag
// to see if a newer doc bundle is available.
const stalenessCheckInterval = 24 * time.Hour

// defaultRefreshTimeout caps how long a staleness HEAD or refresh GET can take.
// Keeps the offline-first experience fast when the network is slow.
// Override via K6_DOCS_REFRESH_TIMEOUT env var (parsed as time.Duration).
const defaultRefreshTimeout = 10 * time.Second

const (
	etagFile      = ".etag"
	lastCheckFile = ".last_check"
)

func resolveRefreshTimeout(env map[string]string) time.Duration {
	if s := env["K6_DOCS_REFRESH_TIMEOUT"]; s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return defaultRefreshTimeout
}

// CacheDir returns the local cache directory for a given docs version.
// The layout is ~/.local/share/k6/docs/{version}/.
func CacheDir(env map[string]string, version string) (string, error) {
	home, err := homeDirFromEnv(env)
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "k6", "docs", version), nil
}

// IsCached reports whether the docs for the given version are already cached.
func IsCached(afs fsext.Fs, env map[string]string, version string) bool {
	dir, err := CacheDir(env, version)
	if err != nil {
		return false
	}
	info, err := afs.Stat(filepath.Clean(dir))
	return err == nil && info.IsDir()
}

// EnsureDocs downloads and extracts the doc bundle for the given version if it
// is not already cached. When a cached copy exists, it periodically checks
// the remote ETag and re-downloads if a newer bundle is available.
func EnsureDocs(
	ctx context.Context, afs fsext.Fs, env map[string]string, version string, httpClient *http.Client,
) (string, error) {
	if !isValidVersion(version) {
		return "", fmt.Errorf("invalid version %q: must contain only alphanumeric, dot, hyphen, underscore", version)
	}
	dir, err := CacheDir(env, version)
	if err != nil {
		return "", err
	}

	url := bundleURL(env, version)

	info, statErr := afs.Stat(filepath.Clean(dir))
	if statErr == nil && info.IsDir() {
		// Staleness refresh is best-effort with a timeout — if it fails
		// or takes too long, the stale cache is served silently.
		refreshCtx, cancel := context.WithTimeout(ctx, resolveRefreshTimeout(env))
		defer cancel()

		if checkStaleness(refreshCtx, afs, dir, url, httpClient) {
			if err := refreshCache(refreshCtx, afs, dir, version, url, httpClient); err != nil {
				return "", fmt.Errorf("refresh docs %s: %w", version, err)
			}
		}
		return dir, nil
	}

	body, etag, err := fetchBundle(ctx, httpClient, version, url)
	if err != nil {
		return "", err
	}
	return dir, installBundle(afs, dir, version, body, etag)
}

// refreshCache replaces the cached docs with a freshly downloaded bundle.
// On fetch failure the old cache is preserved (returns nil).
// On install failure the broken dir is cleaned up and the error is returned
// so the caller can report it instead of serving a missing cache.
func refreshCache(ctx context.Context, afs fsext.Fs, dir, version, url string, httpClient *http.Client) error {
	body, etag, err := fetchBundle(ctx, httpClient, version, url)
	if err != nil {
		return nil //nolint:nilerr // intentional: fetch failure preserves stale cache
	}
	_ = afs.RemoveAll(dir)
	return installBundle(afs, dir, version, body, etag)
}

// fetchBundle downloads and buffers the entire doc bundle in memory.
// Bundles are small (compressed docs), so the memory cost is acceptable.
// Buffering ensures the download is complete before the caller modifies
// any cache state.
func fetchBundle(ctx context.Context, httpClient *http.Client, version, url string) ([]byte, string, error) {
	resp, err := doRequest(ctx, httpClient, http.MethodGet, url)
	if err != nil {
		return nil, "", fmt.Errorf("download docs %s: %w", version, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download docs %s: HTTP %d", version, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBundleSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read docs %s: %w", version, err)
	}
	if int64(len(body)) > maxBundleSize {
		return nil, "", fmt.Errorf("download docs %s: bundle exceeds maximum size (%d bytes)", version, maxBundleSize)
	}

	return body, resp.Header.Get("ETag"), nil
}

// installBundle extracts a buffered bundle into dir and writes metadata.
func installBundle(afs fsext.Fs, dir, version string, body []byte, etag string) error {
	if err := afs.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	if err := extract(afs, bytes.NewReader(body), dir); err != nil {
		_ = afs.RemoveAll(dir)
		return fmt.Errorf("extract docs %s: %w", version, err)
	}

	if err := writeMetaFile(afs, filepath.Join(dir, etagFile), etag); err != nil {
		return fmt.Errorf("write etag %s: %w", version, err)
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := writeMetaFile(afs, filepath.Join(dir, lastCheckFile), now); err != nil {
		return fmt.Errorf("write last check %s: %w", version, err)
	}

	return nil
}

// checkStaleness reports whether the cache should be re-downloaded.
// Metadata parse errors are treated as stale so a corrupt .last_check
// or .etag self-heals on the next run instead of hard-failing.
// Network errors fall back to the cached copy silently.
func checkStaleness(ctx context.Context, afs fsext.Fs, dir, url string, httpClient *http.Client) bool {
	if !isStale(afs, dir) {
		return false
	}

	resp, err := doRequest(ctx, httpClient, http.MethodHead, url)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	remoteETag := resp.Header.Get("ETag")
	// Ignore read errors: a missing or corrupt .etag returns "" which won't
	// match any real ETag, so we re-download — the safe self-healing behaviour.
	storedETag, _ := readMetaFile(afs, filepath.Join(dir, etagFile))

	if remoteETag == storedETag {
		// Non-critical: failing to update the timestamp just means
		// we'll re-check on the next run.
		_ = writeMetaFile(afs, filepath.Join(dir, lastCheckFile),
			strconv.FormatInt(time.Now().Unix(), 10))
		return false
	}

	return true
}

// isStale reports whether the cache's last check is older than stalenessCheckInterval.
// Missing, unreadable, or malformed .last_check is treated as stale so the
// cache self-heals instead of hard-failing.
func isStale(afs fsext.Fs, dir string) bool {
	data, err := fsext.ReadFile(afs, filepath.Join(dir, lastCheckFile))
	if err != nil {
		return true
	}
	ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return true
	}
	return time.Since(time.Unix(ts, 0)) > stalenessCheckInterval
}

// readMetaFile reads a small metadata file. A missing file returns ("", nil).
func readMetaFile(afs fsext.Fs, path string) (string, error) {
	data, err := fsext.ReadFile(afs, path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read meta file %s: %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(data)), nil
}

func writeMetaFile(afs fsext.Fs, path, content string) error {
	return fsext.WriteFile(afs, path, []byte(content), 0o640)
}

// extract decompresses a zstd-compressed tar stream into destDir.
func extract(afs fsext.Fs, r io.Reader, destDir string) error {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return fmt.Errorf("zstd reader: %w", err)
	}
	defer zr.Close()

	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		clean := filepath.Clean(hdr.Name)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("illegal path traversal in tar entry: %q", hdr.Name)
		}

		target := filepath.Clean(filepath.Join(destDir, clean))

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := afs.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := afs.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			data, err := io.ReadAll(io.LimitReader(tr, maxFileSize+1))
			if err != nil {
				return fmt.Errorf("read %s: %w", target, err)
			}
			if int64(len(data)) > maxFileSize {
				return fmt.Errorf("file %s exceeds maximum size (%d bytes)", target, maxFileSize)
			}
			if err := fsext.WriteFile(afs, target, data, 0o640); err != nil {
				return fmt.Errorf("write %s: %w", target, err)
			}
		}
	}

	return nil
}

// doRequest performs an HTTP request with the given context.
func doRequest(ctx context.Context, client *http.Client, method, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req) //nolint:gosec // URL built by bundleURL with validated version.
}

// bundleURL returns the download URL for a docs bundle.
// K6_DOCS_BUNDLE_URL overrides the default GitHub release URL.
func bundleURL(env map[string]string, version string) string {
	if override := env["K6_DOCS_BUNDLE_URL"]; override != "" {
		return override
	}
	const base = "https://github.com/grafana/xk6-docs/releases/download"
	return base + "/doc-bundles/docs-" + version + ".tar.zst"
}

// isValidVersion reports whether version is safe to embed in a URL path.
// Valid versions contain only alphanumeric chars, dots, hyphens, and underscores.
func isValidVersion(version string) bool {
	if version == "" {
		return false
	}
	// Reject path traversal: ".", "..", or any dots-only string.
	if strings.Trim(version, ".") == "" {
		return false
	}
	for _, c := range version {
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '.' || c == '-' || c == '_':
		default:
			return false
		}
	}
	return true
}
