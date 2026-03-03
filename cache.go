package docs

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/klauspost/compress/zstd"
	"go.k6.io/k6/lib/fsext"
)

// maxFileSize is the maximum allowed size for a single file during extraction.
// This prevents decompression bombs (gosec G110).
const maxFileSize = 50 << 20 // 50 MB

// HTTPClient is the interface used to download doc bundles.
type HTTPClient interface {
	Get(url string) (*http.Response, error)
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
// is not already cached. It returns the path to the cache directory.
func EnsureDocs(afs fsext.Fs, env map[string]string, version string, httpClient HTTPClient) (string, error) {
	dir, err := CacheDir(env, version)
	if err != nil {
		return "", err
	}

	info, statErr := afs.Stat(filepath.Clean(dir))
	if statErr == nil && info.IsDir() {
		return dir, nil
	}

	resp, err := httpClient.Get(downloadURL(version))
	if err != nil {
		return "", fmt.Errorf("download docs %s: %w", version, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download docs %s: HTTP %d", version, resp.StatusCode)
	}

	if err := afs.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}

	if err := extract(afs, resp.Body, dir); err != nil {
		// Clean up partial extraction.
		_ = afs.RemoveAll(dir)
		return "", fmt.Errorf("extract docs %s: %w", version, err)
	}

	return dir, nil
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
			f, err := afs.OpenFile(target, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC, 0o640)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			n, copyErr := io.Copy(f, io.LimitReader(tr, maxFileSize+1))
			if copyErr != nil {
				_ = f.Close()
				return fmt.Errorf("write %s: %w", target, copyErr)
			}
			if n > maxFileSize {
				_ = f.Close()
				return fmt.Errorf("file %s exceeds maximum size (%d bytes)", target, maxFileSize)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("close %s: %w", target, err)
			}
		}
	}

	return nil
}

// downloadURL returns the release URL for a given docs version.
func downloadURL(version string) string {
	const base = "https://github.com/grafana/xk6-subcommand-docs/releases/download"
	return base + "/doc-bundles/docs-" + version + ".tar.zst"
}
