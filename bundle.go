package docs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib/fsext"
)

// docsEnv bundles the context needed for reading and transforming docs.
type docsEnv struct {
	FS       fsext.Fs
	CacheDir string
	Version  string
	Depth    int
}

func (env *docsEnv) readAndTransform(relPath string) string {
	raw := readMarkdown(env.FS, env.CacheDir, relPath)
	if raw == "" {
		return ""
	}
	return Transform(raw, env.Version)
}

// setup resolves the version, ensures docs are cached, and loads the index.
// It checks flags, then env vars, then auto-detection for both version and
// cache directory.
func setup(
	ctx context.Context, gs *state.GlobalState, versionFlag, cacheDirFlg string,
) (version, cacheDir string, idx *Index, err error) {
	version = versionFlag
	if version == "" {
		version = gs.Env["K6_DOCS_VERSION"]
	}
	if version == "" {
		version, err = DetectK6Version()
		if err != nil {
			return "", "", nil, fmt.Errorf("detect k6 version: %w", err)
		}
	}

	version = MapToWildcard(version)

	cacheDir = cacheDirFlg
	if cacheDir == "" {
		cacheDir = gs.Env["K6_DOCS_CACHE_DIR"]
	}

	if cacheDir == "" {
		if !IsCached(gs.FS, gs.Env, version) {
			gs.Logger.Infof("Downloading k6 %s docs...", version)
		}
		cacheDir, err = EnsureDocs(ctx, gs.FS, gs.Env, version, http.DefaultClient)
		if err != nil {
			return "", "", nil, fmt.Errorf("ensure docs: %w", err)
		}
	}

	idx, err = LoadIndex(gs.FS, cacheDir)
	if err != nil {
		return "", "", nil, fmt.Errorf("load index: %w", err)
	}

	return version, cacheDir, idx, nil
}

// readMarkdown reads a markdown file from the cache directory.
func readMarkdown(afs fsext.Fs, cacheDir, relPath string) string {
	path := filepath.Join(cacheDir, "markdown", relPath)
	data, err := fsext.ReadFile(afs, path)
	if err != nil {
		return ""
	}
	return string(data)
}

// printBestPractices reads and prints the best_practices.md file from the cache.
func printBestPractices(env *docsEnv, w io.Writer) error {
	path := filepath.Join(env.CacheDir, "markdown", "best_practices.md")
	data, err := fsext.ReadFile(env.FS, path)
	if errors.Is(err, fs.ErrNotExist) {
		// Fall back to old bundle layout for backward compatibility.
		path = filepath.Join(env.CacheDir, "best_practices.md")
		data, err = fsext.ReadFile(env.FS, path)
	}
	if err != nil {
		return fmt.Errorf("read best practices: %w", err)
	}
	content := Transform(string(data), env.Version)
	_, _ = fmt.Fprint(w, content)
	if !strings.HasSuffix(content, "\n") {
		_, _ = fmt.Fprintln(w)
	}
	return nil
}
