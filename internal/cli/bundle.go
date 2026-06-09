package cli

import (
	"cmp"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/xk6-docs/docs"
	"github.com/grafana/xk6-docs/internal/bundle"
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
	versionFlag, sourceFlag, cacheDirFlg string,
) (denv *docsEnv, err error) {
	version := versionFlag
	if version == "" {
		version = env["K6_DOCS_VERSION"]
	}
	// --source targets in-development docs, which live under the "next"
	// directory. Default to it instead of the built k6 version so authors get
	// the version they're editing; an explicit --version still overrides.
	if version == "" && sourceFlag != "" {
		version = "next"
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

	if sourceFlag != "" {
		base, err = buildSourceBundle(logf, base, sourceFlag, version)
		if err != nil {
			return nil, err
		}
		explicitDir = base
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

// sourceCacheDir is the subdirectory under the doc cache base that holds
// source-preview builds. It is hidden from normal version discovery, which
// only matches "vX.Y.x" directory names (see docs/catalog.go versionDirRe).
const sourceCacheDir = ".sources"

// buildSourceBundle transforms a local k6-docs checkout into a bundle and
// returns its directory as a local-only cache base. Builds live under
// {cacheBase}/.sources/{hash-of-abs-source}/, so each source directory a user
// points at gets its own isolated build and they never affect each other or
// the downloaded version bundles. The build is skipped when the source's
// markdown files are unchanged since the last build (see [bundle.SourceStamp]).
func buildSourceBundle(logf func(string, ...any), cacheBase, source, version string) (string, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(abs))
	base := filepath.Join(cacheBase, sourceCacheDir, strconv.FormatUint(uint64(h.Sum32()), 16))
	out := filepath.Join(base, version)
	stampPath := filepath.Join(base, version+".stamp")

	// A stamp error (e.g. missing version dir) falls through to Build, which
	// surfaces the canonical "version root not found" error.
	stamp, stampErr := bundle.SourceStamp(abs, version)

	osfs := bundle.NewOSFS()
	if stampErr == nil && sourceUpToDate(osfs, out, stampPath, stamp) {
		return base, nil
	}

	logf("Building k6 %s docs from %s...", version, abs)
	if err := osfs.RemoveAll(out); err != nil {
		return "", fmt.Errorf("clear scratch dir: %w", err)
	}
	if err := bundle.Build(version, abs, out, osfs, io.Discard); err != nil {
		return "", fmt.Errorf("build docs from source: %w", err)
	}
	if stampErr == nil {
		if err := osfs.WriteFile(stampPath, []byte(stamp), 0o600); err != nil {
			return "", fmt.Errorf("write source stamp: %w", err)
		}
	}
	return base, nil
}

// sourceUpToDate reports whether a prior build for this source exists and its
// recorded stamp still matches, so the rebuild can be skipped.
func sourceUpToDate(osfs docs.FS, out, stampPath, stamp string) bool {
	if _, err := osfs.Stat(filepath.Join(out, "sections.json")); err != nil {
		return false
	}
	prev, err := osfs.ReadFile(stampPath)
	return err == nil && string(prev) == stamp
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
