package docs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// releasesURL is the GitHub API endpoint for the doc-bundles release.
// Used to discover available documentation versions.
const releasesURL = "https://api.github.com/repos/grafana/xk6-docs/releases/tags/doc-bundles"

// versionDiscoveryTimeout caps how long the GitHub API call can take.
const versionDiscoveryTimeout = 15 * time.Second

// Catalog provides access to per-version documentation bundles.
//
// By default it uses HTTP+cache: downloads bundles from GitHub and caches
// them locally. Use [WithFS] for an embedded or test filesystem backend.
type Catalog struct {
	fsys           fs.FS        // non-nil = FS mode
	baseCacheDir   string       // override for default cache base
	httpClient     *http.Client // override for http.DefaultClient
	cacheFS        FS
	refreshTimeout time.Duration
	bundleURL      string
	localOnly      bool

	mu       sync.Mutex
	versions []string
	indexes  map[string]*Index

	versionsOnce sync.Once
}

// Option configures a [Catalog].
type Option func(*Catalog)

// WithFS sets the filesystem backend. The FS must contain per-version
// bundle directories (for example "v1.7.x/sections.json").
func WithFS(fsys fs.FS) Option {
	return func(c *Catalog) { c.fsys = fsys }
}

// WithCacheDir overrides the default cache base directory
// (~/.local/share/k6/docs). Only used in HTTP+cache mode.
func WithCacheDir(dir string) Option {
	return func(c *Catalog) { c.baseCacheDir = dir }
}

// WithHTTPClient overrides the default http.Client for downloads and
// version discovery. Only used in HTTP+cache mode.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Catalog) { c.httpClient = client }
}

// WithRefreshTimeout overrides the default staleness-check timeout.
func WithRefreshTimeout(d time.Duration) Option {
	return func(c *Catalog) { c.refreshTimeout = d }
}

// WithBundleURL overrides the default GitHub bundle download URL.
func WithBundleURL(url string) Option {
	return func(c *Catalog) { c.bundleURL = url }
}

// WithLocalOnly restricts the catalog to locally cached bundles only.
func WithLocalOnly() Option {
	return func(c *Catalog) { c.localOnly = true }
}

// NewCatalog returns a [Catalog]. Without options it uses HTTP+cache
// (on-demand bundle download). Pass [WithFS] for an embedded FS backend.
// Versions are discovered lazily on first access.
func NewCatalog(opts ...Option) *Catalog {
	c := &Catalog{
		indexes: make(map[string]*Index),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.cacheFS == nil {
		c.cacheFS = osFS{}
	}

	if c.fsys != nil {
		// FS mode: discover versions eagerly from directory listing.
		c.versions = discoverVersionsFS(c.fsys)
	}

	return c
}

// Versions returns the discovered version directory names sorted latest-first.
// In HTTP+cache mode, versions are discovered lazily from GitHub + cache scan.
// The returned slice is a copy and safe to mutate.
func (c *Catalog) Versions() []string {
	c.discoverVersions(context.Background())
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.versions))
	copy(out, c.versions)
	return out
}

// Latest returns the newest discovered version, or an empty string if none
// were found.
func (c *Catalog) Latest() string {
	c.discoverVersions(context.Background())
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.versions) == 0 {
		return ""
	}
	return c.versions[0]
}

// Index returns the loaded index for version. An empty version resolves to
// [Catalog.Latest]. Indexes are cached per resolved version.
func (c *Catalog) Index(ctx context.Context, version string) (*Index, error) {
	c.discoverVersions(ctx)

	resolved := version
	if resolved == "" {
		c.mu.Lock()
		if len(c.versions) > 0 {
			resolved = c.versions[0]
		}
		c.mu.Unlock()
	}
	if resolved == "" {
		return nil, fmt.Errorf("no documentation versions available")
	}

	c.mu.Lock()
	if idx, ok := c.indexes[resolved]; ok {
		c.mu.Unlock()
		return idx, nil
	}
	c.mu.Unlock()

	if c.fsys != nil {
		return c.loadIndexFS(resolved)
	}
	return c.loadIndexHTTP(ctx, resolved)
}

// Read returns the markdown content for slug under version. Aliases declared
// in the section index are resolved transparently.
func (c *Catalog) Read(ctx context.Context, version, slug string) ([]byte, error) {
	idx, err := c.Index(ctx, version)
	if err != nil {
		return nil, err
	}

	sec, ok := idx.Lookup(slug)
	if !ok {
		return nil, fmt.Errorf("unknown documentation slug %q", slug)
	}

	return c.readFile(ctx, idx.Version, sec.RelPath)
}

// ReadFile returns the raw content of a file at relPath within the version's
// markdown directory. Unlike [Catalog.Read], it does not go through the
// section index. Use it for files not listed in sections.json
// (e.g. best_practices.md).
func (c *Catalog) ReadFile(ctx context.Context, version, relPath string) ([]byte, error) {
	// Index ensures the version is downloaded (HTTP mode) or valid (FS mode)
	// and gives us the resolved version string.
	idx, err := c.Index(ctx, version)
	if err != nil {
		return nil, err
	}
	return c.readFile(ctx, idx.Version, relPath)
}

// readFile reads a file from the markdown directory for the given version.
func (c *Catalog) readFile(_ context.Context, version, relPath string) ([]byte, error) {
	if c.fsys != nil {
		p := path.Join(version, "markdown", relPath)
		data, err := fs.ReadFile(c.fsys, p)
		if errors.Is(err, fs.ErrNotExist) {
			return fs.ReadFile(c.fsys, path.Join(version, relPath))
		}
		return data, err
	}

	baseDir := c.resolveBaseCacheDir()
	p := filepath.Join(versionCacheDir(baseDir, version), "markdown", relPath)
	data, err := c.cacheFS.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return c.cacheFS.ReadFile(filepath.Join(versionCacheDir(baseDir, version), relPath))
	}
	return data, err
}

func (c *Catalog) loadIndexFromDir(dir string) (*Index, error) {
	data, err := c.cacheFS.ReadFile(filepath.Join(dir, "sections.json"))
	if err != nil {
		return nil, fmt.Errorf("load index %s: %w", dir, err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse index %s: %w", dir, err)
	}
	return &idx, nil
}

// loadIndexFS loads a version's index from the FS backend.
func (c *Catalog) loadIndexFS(version string) (*Index, error) {
	if !c.hasVersion(version) {
		return nil, fmt.Errorf("unknown documentation version %q", version)
	}

	p := path.Join(version, "sections.json")
	data, err := fs.ReadFile(c.fsys, p)
	if err != nil {
		return nil, fmt.Errorf("read sections index for %s: %w", version, err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("decode sections index for %s: %w", version, err)
	}

	c.mu.Lock()
	c.indexes[version] = &idx
	c.mu.Unlock()
	return &idx, nil
}

// loadIndexHTTP downloads the version's bundle if not cached and loads it.
func (c *Catalog) loadIndexHTTP(ctx context.Context, version string) (*Index, error) {
	baseDir := c.resolveBaseCacheDir()

	var dir string
	if c.localOnly {
		dir = versionCacheDir(baseDir, version)
	} else {
		var err error
		dir, err = ensureDocs(ctx, baseDir, version, c.resolveHTTPClient(), c.cacheFS, c.resolveRefreshTimeout(), c.bundleURL)
		if err != nil {
			return nil, err
		}
	}

	idx, err := c.loadIndexFromDir(dir)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.indexes[version] = idx
	if !slices.Contains(c.versions, version) {
		c.versions = append(c.versions, version)
		sortVersions(c.versions)
	}
	c.mu.Unlock()

	return idx, nil
}

func (c *Catalog) hasVersion(version string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Contains(c.versions, version)
}

func (c *Catalog) resolveBaseCacheDir() string {
	if c.baseCacheDir != "" {
		return c.baseCacheDir
	}
	dir, err := defaultBaseCacheDir()
	if err != nil {
		return ""
	}
	return dir
}

func (c *Catalog) resolveHTTPClient() *http.Client {
	if c.httpClient != nil {
		return c.httpClient
	}
	return http.DefaultClient
}

func (c *Catalog) resolveRefreshTimeout() time.Duration {
	if c.refreshTimeout > 0 {
		return c.refreshTimeout
	}
	return defaultRefreshTimeout
}

// discoverVersions populates c.versions lazily (once).
func (c *Catalog) discoverVersions(ctx context.Context) {
	c.versionsOnce.Do(func() {
		if c.fsys != nil {
			return // already set in NewCatalog
		}
		ctx, cancel := context.WithTimeout(ctx, versionDiscoveryTimeout)
		defer cancel()
		c.mu.Lock()
		c.versions = c.discoverHTTPVersions(ctx)
		c.mu.Unlock()
	})
}

// discoverHTTPVersions merges locally cached versions with those advertised
// by the GitHub releases API.
func (c *Catalog) discoverHTTPVersions(ctx context.Context) []string {
	baseDir := c.resolveBaseCacheDir()
	cached := scanCachedVersions(c.cacheFS, baseDir)
	if c.localOnly {
		sortVersions(cached)
		return cached
	}
	remote := fetchRemoteVersions(ctx, c.resolveHTTPClient())
	return mergeAndSortVersions(cached, remote)
}

// scanCachedVersions returns version directory names found in baseDir.
func scanCachedVersions(fsys FS, baseDir string) []string {
	if baseDir == "" {
		return nil
	}
	entries, err := fsys.ReadDir(baseDir)
	if err != nil {
		return nil
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && versionDirRe.MatchString(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	return versions
}

// fetchRemoteVersions queries the GitHub releases API for doc-bundles
// assets and extracts version names. Returns nil on any failure.
func fetchRemoteVersions(ctx context.Context, client *http.Client) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil
	}

	resp, err := client.Do(req) //nolint:gosec // releasesURL is a constant.
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	const maxBody = 1 << 20 // 1 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil
	}

	var release struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return nil
	}

	var versions []string
	for _, a := range release.Assets {
		if v, ok := versionFromAssetName(a.Name); ok {
			versions = append(versions, v)
		}
	}
	return versions
}

// versionFromAssetName extracts the version from an asset name like
// "docs-v1.7.x.tar.zst".
func versionFromAssetName(name string) (string, bool) {
	if !strings.HasPrefix(name, "docs-") || !strings.HasSuffix(name, ".tar.zst") {
		return "", false
	}
	v := strings.TrimPrefix(name, "docs-")
	v = strings.TrimSuffix(v, ".tar.zst")
	if !versionDirRe.MatchString(v) {
		return "", false
	}
	return v, true
}

var versionDirRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.x$`)

// discoverVersionsFS returns version directory names found in fsys,
// sorted latest-first.
func discoverVersionsFS(fsys fs.FS) []string {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() && versionDirRe.MatchString(e.Name()) {
			versions = append(versions, e.Name())
		}
	}
	sortVersions(versions)
	return versions
}

func mergeAndSortVersions(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var all []string
	for _, v := range a {
		if !seen[v] {
			seen[v] = true
			all = append(all, v)
		}
	}
	for _, v := range b {
		if !seen[v] {
			seen[v] = true
			all = append(all, v)
		}
	}
	sortVersions(all)
	return all
}

// sortVersions sorts version strings latest-first (descending major, minor).
func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		mi, ni := parseVersion(versions[i])
		mj, nj := parseVersion(versions[j])
		if mi != mj {
			return mi > mj
		}
		return ni > nj
	})
}

func parseVersion(v string) (major, minor int) {
	m := versionDirRe.FindStringSubmatch(v)
	if m == nil {
		return 0, 0
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	return major, minor
}
