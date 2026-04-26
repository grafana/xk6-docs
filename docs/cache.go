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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
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
const defaultRefreshTimeout = 10 * time.Second

const (
	etagFile      = ".etag"
	lastCheckFile = ".last_check"
)

// defaultBaseCacheDir returns the default base directory for all doc version caches.
func defaultBaseCacheDir() (string, error) {
	home, err := os.UserHomeDir() //nolint:forbidigo // UserHomeDir queries a system property, not filesystem I/O
	if err != nil {
		return "", fmt.Errorf("cache dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "k6", "docs"), nil
}

// versionCacheDir returns the local cache directory for a specific version.
func versionCacheDir(baseDir, version string) string {
	return filepath.Join(baseDir, version)
}

// ensureDocs downloads and extracts the doc bundle for the given version if it
// is not already cached. When a cached copy exists, it periodically checks
// the remote ETag and re-downloads if a newer bundle is available.
func ensureDocs(
	ctx context.Context, baseDir, version string, httpClient *http.Client,
	fsys FS, refreshTimeout time.Duration, overrideURL string,
) (string, error) {
	if !isValidVersion(version) {
		return "", fmt.Errorf("invalid version %q: must contain only alphanumeric, dot, hyphen, underscore", version)
	}
	dir := versionCacheDir(baseDir, version)
	url := bundleURL(version, overrideURL)

	info, statErr := fsys.Stat(filepath.Clean(dir))
	if statErr == nil && info.IsDir() {
		// Staleness refresh is best-effort with a timeout.
		refreshCtx, cancel := context.WithTimeout(ctx, refreshTimeout)
		defer cancel()

		if checkStaleness(refreshCtx, dir, url, httpClient, fsys) {
			if err := refreshCache(refreshCtx, dir, version, url, httpClient, fsys); err != nil {
				return "", fmt.Errorf("refresh docs %s: %w", version, err)
			}
		}
		return dir, nil
	}

	body, etag, err := fetchBundle(ctx, httpClient, version, url)
	if err != nil {
		return "", err
	}
	return dir, installBundle(dir, version, body, etag, fsys)
}

// refreshCache replaces the cached docs with a freshly downloaded bundle.
// On fetch failure the old cache is preserved (returns nil).
// On install failure the broken dir is cleaned up and the error is returned
// so the caller can report it instead of serving a missing cache.
func refreshCache(ctx context.Context, dir, version, url string, httpClient *http.Client, fsys FS) error {
	body, etag, err := fetchBundle(ctx, httpClient, version, url)
	if err != nil {
		return nil //nolint:nilerr // intentional: fetch failure preserves stale cache
	}
	_ = fsys.RemoveAll(dir)
	return installBundle(dir, version, body, etag, fsys)
}

// fetchBundle downloads and buffers the entire doc bundle in memory.
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
func installBundle(dir, version string, body []byte, etag string, fsys FS) error {
	if err := fsys.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	if err := extract(bytes.NewReader(body), dir, fsys); err != nil {
		_ = fsys.RemoveAll(dir)
		return fmt.Errorf("extract docs %s: %w", version, err)
	}

	if err := writeMetaFile(fsys, filepath.Join(dir, etagFile), etag); err != nil {
		return fmt.Errorf("write etag %s: %w", version, err)
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	if err := writeMetaFile(fsys, filepath.Join(dir, lastCheckFile), now); err != nil {
		return fmt.Errorf("write last check %s: %w", version, err)
	}

	return nil
}

// checkStaleness reports whether the cache should be re-downloaded.
func checkStaleness(ctx context.Context, dir, url string, httpClient *http.Client, fsys FS) bool {
	if !isStale(dir, fsys) {
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
	storedETag, _ := readMetaFile(fsys, filepath.Join(dir, etagFile))

	if remoteETag == storedETag {
		_ = writeMetaFile(fsys, filepath.Join(dir, lastCheckFile),
			strconv.FormatInt(time.Now().Unix(), 10))
		return false
	}

	return true
}

// isStale reports whether the cache's last check is older than stalenessCheckInterval.
func isStale(dir string, fsys FS) bool {
	data, err := fsys.ReadFile(filepath.Join(dir, lastCheckFile))
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
func readMetaFile(fsys FS, path string) (string, error) {
	data, err := fsys.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read meta file %s: %w", filepath.Base(path), err)
	}
	return strings.TrimSpace(string(data)), nil
}

func writeMetaFile(fsys FS, path, content string) error {
	return fsys.WriteFile(path, []byte(content), 0o600)
}

// extract decompresses a zstd-compressed tar stream into destDir.
func extract(r io.Reader, destDir string, fsys FS) error {
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
			if err := fsys.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := fsys.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			data, err := io.ReadAll(io.LimitReader(tr, maxFileSize+1))
			if err != nil {
				return fmt.Errorf("read %s: %w", target, err)
			}
			if int64(len(data)) > maxFileSize {
				return fmt.Errorf("file %s exceeds maximum size (%d bytes)", target, maxFileSize)
			}
			if err := fsys.WriteFile(target, data, 0o600); err != nil {
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
func bundleURL(version, overrideURL string) string {
	if overrideURL != "" {
		return overrideURL
	}
	const base = "https://github.com/grafana/xk6-docs/releases/download"
	return base + "/doc-bundles/docs-" + version + ".tar.zst"
}

// isValidVersion reports whether version is safe to embed in a URL path.
func isValidVersion(version string) bool {
	if version == "" {
		return false
	}
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
