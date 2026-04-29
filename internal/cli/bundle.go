package cli

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/grafana/xk6-docs/docs"
)

// docsEnv bundles the context needed for reading and transforming docs.
type docsEnv struct {
	cat      *docs.Catalog
	idx      *docs.Index
	version  string
	depth    int
	cacheDir string // resolved version dir path for agent guide
}

func (env *docsEnv) readAndTransform(ctx context.Context, slug string) string {
	data, err := env.cat.Read(ctx, env.version, slug)
	if err != nil {
		return ""
	}
	return docs.Transform(string(data), env.version)
}

// setup resolves the version, ensures docs are cached, and loads the index.
func setup(
	ctx context.Context,
	env map[string]string,
	logf func(string, ...any),
	fs FS,
	versionFlag, cacheDirFlg string,
) (denv *docsEnv, err error) {
	version := versionFlag
	if version == "" {
		version = env["K6_DOCS_VERSION"]
	}
	if version == "" {
		version = env["K6_PROVISION_HOST_VERSION"]
	}
	if version == "" {
		version, err = detectK6Version(debug.ReadBuildInfo)
		if err != nil {
			return nil, fmt.Errorf("detect k6 version: %w", err)
		}
	}
	version = docs.VersionWildcard(version)

	explicitDir := cmp.Or(cacheDirFlg, env["K6_DOCS_CACHE_DIR"])
	base := cmp.Or(explicitDir, baseCacheDir(env))
	if base == "" {
		return nil, fmt.Errorf("neither HOME nor USERPROFILE is set")
	}

	opts := catalogOpts(env, base, explicitDir != "")
	cat := docs.NewCatalog(opts...)

	if explicitDir == "" {
		info, statErr := fs.Stat(filepath.Join(base, version))
		if statErr != nil || !info.IsDir() {
			logf("Downloading k6 %s docs...", version)
		}
	}

	idx, err := cat.Index(ctx, version)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	return &docsEnv{cat: cat, idx: idx, version: version, cacheDir: filepath.Join(base, version)}, nil
}

func catalogOpts(env map[string]string, base string, localOnly bool) []docs.Option {
	opts := []docs.Option{docs.WithCacheDir(base)}
	if localOnly {
		return append(opts, docs.WithLocalOnly())
	}
	if t := env["K6_DOCS_REFRESH_TIMEOUT"]; t != "" {
		if d, err := time.ParseDuration(t); err == nil && d > 0 {
			opts = append(opts, docs.WithRefreshTimeout(d))
		}
	}
	if u := env["K6_DOCS_BUNDLE_URL"]; u != "" {
		opts = append(opts, docs.WithBundleURL(u))
	}
	return opts
}

// baseCacheDir returns the doc cache base directory from HOME/USERPROFILE.
func baseCacheDir(env map[string]string) string {
	home := cmp.Or(env["HOME"], env["USERPROFILE"])
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
	content := docs.Transform(string(data), env.version)
	_, _ = fmt.Fprint(w, content)
	if !strings.HasSuffix(content, "\n") {
		_, _ = fmt.Fprintln(w)
	}
	return nil
}
