package docs

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	xdocs "github.com/grafana/xk6-docs/docs"
	"go.k6.io/k6/cmd/state"
)

// docsEnv bundles the context needed for reading and transforming docs.
type docsEnv struct {
	cat      *xdocs.Catalog
	idx      *xdocs.Index
	version  string
	depth    int
	cacheDir string // resolved version dir path for agent guide
}

func (env *docsEnv) readAndTransform(ctx context.Context, slug string) string {
	data, err := env.cat.Read(ctx, env.version, slug)
	if err != nil {
		return ""
	}
	return xdocs.Transform(string(data), env.version)
}

// setup resolves the version, ensures docs are cached, and loads the index.
func setup(
	ctx context.Context, gs *state.GlobalState, versionFlag, cacheDirFlg string,
) (env *docsEnv, err error) {
	version := versionFlag
	if version == "" {
		version = gs.Env["K6_DOCS_VERSION"]
	}
	if version == "" {
		version, err = detectK6Version(debug.ReadBuildInfo)
		if err != nil {
			return nil, fmt.Errorf("detect k6 version: %w", err)
		}
	}
	version = xdocs.VersionWildcard(version)

	explicitDir := cmp.Or(cacheDirFlg, gs.Env["K6_DOCS_CACHE_DIR"])
	base := cmp.Or(explicitDir, baseCacheDir(gs))
	if base == "" {
		return nil, fmt.Errorf("neither HOME nor USERPROFILE is set")
	}

	opts := catalogOpts(gs, base, explicitDir != "")
	cat := xdocs.NewCatalog(opts...)

	if explicitDir == "" {
		info, statErr := gs.FS.Stat(filepath.Join(base, version))
		if statErr != nil || !info.IsDir() {
			gs.Logger.Infof("Downloading k6 %s docs...", version)
		}
	}

	idx, err := cat.Index(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	return &docsEnv{cat: cat, idx: idx, version: version, cacheDir: filepath.Join(base, version)}, nil
}

func catalogOpts(gs *state.GlobalState, base string, localOnly bool) []xdocs.Option {
	opts := []xdocs.Option{xdocs.WithCacheDir(base)}
	if localOnly {
		return append(opts, xdocs.WithLocalOnly())
	}
	if t := gs.Env["K6_DOCS_REFRESH_TIMEOUT"]; t != "" {
		if d, err := time.ParseDuration(t); err == nil && d > 0 {
			opts = append(opts, xdocs.WithRefreshTimeout(d))
		}
	}
	if u := gs.Env["K6_DOCS_BUNDLE_URL"]; u != "" {
		opts = append(opts, xdocs.WithBundleURL(u))
	}
	return opts
}

// baseCacheDir returns the doc cache base directory from HOME/USERPROFILE.
func baseCacheDir(gs *state.GlobalState) string {
	home := cmp.Or(gs.Env["HOME"], gs.Env["USERPROFILE"])
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "k6", "docs")
}

// printBestPractices reads and prints the best_practices.md file from the cache.
func printBestPractices(ctx context.Context, env *docsEnv, w io.Writer) error {
	data, err := env.cat.ReadFile(ctx, env.version, "best_practices.md")
	if err != nil {
		return fmt.Errorf("read best practices: %w", err)
	}
	content := xdocs.Transform(string(data), env.version)
	_, _ = fmt.Fprint(w, content)
	if !strings.HasSuffix(content, "\n") {
		_, _ = fmt.Fprintln(w)
	}
	return nil
}
