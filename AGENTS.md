## RULES
1. Update this file concisely whenever features are added, removed, or changed.
2. Always use TDD (red/green/refactor). Tests must compile and fail on assertions before writing implementation.
3. Never skip linters (`//nolint` without proof), trick the linter, change `go.mod` k6 floor below v1.5.0, add global vars or `init()` (except `register.go`).
4. Plans: Store plans in `.claude/plans/` with incrementing numbers.

---

`k6 x docs` — offline k6 documentation in the terminal. For humans and AI agents. Docs are not embedded in the binary. On first run, the extension detects the k6 version from build info, downloads a matching compressed doc bundle (`.tar.zst`) from GitHub releases, and caches it locally (`~/.local/share/k6/docs/{version}/`). Subsequent runs serve from cache with no network. A separate standalone prepare tool (`cmd/prepare/`) builds these bundles by cloning the k6-docs Hugo repository, transforming markdown into CLI-friendly format, building a searchable index (`sections.json`), and compressing everything. CI auto-publishes bundles as assets under a single `doc-bundles` GitHub release.

## Browsing
- `k6 x docs` prints `# k6 {version}` header followed by a depth-controlled bullet tree of categories and their children (default depth: 2), and a blockquote example hint. Depth is configurable via `depth` in `~/.config/k6/docs.yaml`.
- `k6 x docs http get` resolves args to a slug (case-insensitive) and prints the cached markdown content (trimmed). If the topic has children, a `---` separator and `**{path} subtopics:**` section is appended with a depth-controlled bullet tree of children (same `depth` config), and a blockquote example hint using the slug path form (`k6 x docs {path}/<subtopic>`).
- `k6 x docs best-practices` prints a curated guide (embedded in the prepare tool via `//go:embed`).
- `k6 x docs search <query>` fuzzy searches (case-insensitive, ignores punctuation, spaces, slashes) and prints an indented tree: `- {group}` with `  - {child}` underneath, no descriptions. Footer shows `Example:` with a sample navigation command. Search uses the same arg normalization and resolve rules as docs navigation (shared `normalizeArgs` and `ResolveWithLookup`), so `search browser page`, `search browser/page`, and `search javascript-api browser page` all produce the same results. Configurable `depth` controls tree depth (same as TOC/section footers).

## Slug resolution
- Args are normalized first: slashes are split so `browser/elementhandle` is treated identically to `browser elementhandle`.
- `k6 x docs http get` → `javascript-api/k6-http/get`
- `k6 x docs javascript-api/k6-http/get` → `javascript-api/k6-http/get`
- `k6 x docs javascript-api/browser/elementhandle` → `javascript-api/k6-browser/elementhandle`
- `k6 x docs using-k6 scenarios` → `using-k6/scenarios`
- k6-prefix fallback: `withK6Prefix` in `resolve.go` inserts `k6-` on the second segment of any `javascript-api/` slug when the original doesn't exist. Existing docs are prioritized (e.g. `jslib` stays as-is since `javascript-api/jslib` exists).
- Parent-prefix fallback: `k6 x docs http cookiejar clear` → tries `.../cookiejar/clear` (miss) → `.../cookiejar/cookiejar-clear` (hit). Handled by `withParentFallback` in `resolve.go`.

### Rendering
- Optional configurable renderer (e.g. `glow`) for pretty terminal output in `~/.config/k6/docs.yaml`.
- Configurable `depth` (int, default 2) in `~/.config/k6/docs.yaml` controls how many levels of subtopics are shown in TOC and section footers. Override via `--depth` flag (always wins over config). `printTree` is the single recursive function used everywhere.
- Links to the current version's online docs are stripped: `[text](https://grafana.com/docs/k6/v1.6.1/foo)` → `text`.
- Stripped: Shared shortcodes (`{{< docs/shared >}}`), code tags (`{{< code >}}`), section tags (`{{< section >}}`), React/MDX component tags (`<Glossary>`), `<br/>`, internal doc links, image links, remaining markdown links, HTML comments, YAML frontmatter.
- Converted: Admonitions (`{{< admonition type="warning" >}}`) → `> **Warning:** ...` blockquotes.
- Placeholders replaced: `<K6_VERSION>` → actual version.
- Internal doc links to included categories are converted to plain text (URL stripped). Links to excluded categories keep the URL.
- Duplicate child names are deduplicated in search results (e.g. `javascript-api/k6-http/get` and `k6-http-get` both resolve to child name `get`, but only one is shown).

### Documentation version handling
- Auto-detects k6 version from Go build info.
- Maps to wildcard: `v1.5.0` → `v1.5.x`, `v1.6.0-rc.1` → `v1.6.x`.
- Override via `--version` flag or `K6_DOCS_VERSION` env var (wildcard mapping is always applied, e.g. `--version 1.6.0` → `v1.6.x`).
- Cache dir override via `--cache-dir` flag or `K6_DOCS_CACHE_DIR` env var.
- `go.mod` floor for `go.k6.io/k6` must stay at v1.5.0 so Go's MVS doesn't override the k6 version users build with via xk6. Extension code can only use k6 APIs from v1.5.0; use build tags if newer APIs are needed.

### Bundle preparation (standalone `cmd/prepare/`)
- Clones k6-docs if not present, checks out matching tag.
- Builds shared content map from `docs/sources/shared/`.
- Walks markdown files, parses YAML frontmatter (deduplicates duplicate keys by keeping first occurrence), derives slugs, filters to included categories.
- Handles slug collisions: prefers `_index.md` over leaf `.md` (it has children).
- Populates parent→child relationships.
- Outputs: `dist/sections.json`, `dist/markdown/**/*.md`, `dist/best_practices.md`.

### CI/CD
- CI: lint + test + build on push/PR to main.
- Release: triggered by `vx.y.z` tag push. Builds k6 binaries (with this extension via xk6) for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. Publishes binaries + `checksums.txt` to a GitHub release.
- Release bundle: triggered by k6 release dispatch or manual. Clones k6-docs, runs prepare, compresses with `zstd --ultra -22`, publishes asset to the single `doc-bundles` GitHub release.
- Release poll: manual fallback (schedule disabled). Polls k6 releases, builds if asset missing from the `doc-bundles` release.