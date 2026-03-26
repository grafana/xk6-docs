## RULES
1. Update this file concisely whenever features are added, removed, or changed.
2. Update the `features.md` file when user-facing features are added, removed, or changed.
4. Never take easy shortcuts or chase after easy wins. Execute what's asked for.
5. Make the minimal change that delivers the new feature or fixes the bug.
3. Use TDD: write a minimal test, fail it, write a minimal code to pass it. Repeat.
6. User facing features should be tested with test scripts in `testdata/scripts`.
7. Never skip linters (`//nolint` without proof), trick the linter, change `go.mod` k6 floor below v1.5.0, add global vars or `init()` (except `register.go`).
8. Plans: When writing complex features or significant refactors, use a Plan (as described in `.claude/PLANS.md`) from design to implementation. Store plans in `.claude/plans/` with incrementing numbers.

---

`k6 x docs` — offline k6 documentation in the terminal. For humans and AI agents. Docs are not embedded in the binary. On first run, the extension detects the k6 version from build info, downloads a matching compressed doc bundle (`.tar.zst`) from GitHub releases, and caches it locally (`~/.local/share/k6/docs/{version}/`). Subsequent runs serve from cache with no network. Cached bundles are checked for staleness every 24 hours via ETag comparison; stale bundles are re-downloaded automatically. A separate standalone prepare tool (`cmd/prepare/`) builds these bundles by cloning the k6-docs Hugo repository, transforming markdown into CLI-friendly format, building a searchable index (`sections.json`), and compressing everything. CI auto-publishes bundles as assets under a single `doc-bundles` GitHub release.

## Browsing
- `k6 x docs` prints `# k6 {version}` header followed by a depth-controlled bullet tree of categories and their children (default depth: 1), and a blockquote example hint.
- `k6 x docs http get` resolves args to a slug (case-insensitive) and prints the cached markdown content (trimmed). If the topic has children, a `---` separator and `**{path} subtopics:**` section is appended with a depth-controlled bullet tree of children, and a blockquote example hint using the slug path form (`k6 x docs {path}/<subtopic>`).
- `k6 x docs best-practices` prints a curated guide (embedded in the prepare tool via `//go:embed`).
- `k6 x docs search <query>` fuzzy searches (case-insensitive, ignores punctuation, spaces, slashes) and prints an indented tree: `- {group}` with `  - {child}` underneath, no descriptions. Footer shows `Example:` with a sample navigation command. Search uses the same arg normalization and resolve rules as docs navigation (shared `normalizeArgs` and `ResolveWithLookup`), so `search browser page`, `search browser/page`, and `search javascript-api browser page` all produce the same results. `--depth` flag controls tree depth (same as TOC/section footers).

## Shell completions
- `ValidArgsFunction` on the `docs` and `search` commands provides dynamic topic completion via cobra's `__complete` mechanism. Users set up completions once via k6's built-in `k6 completion zsh` (or bash/fish/powershell); extensions like xk6-docs piggyback on that.
- First-arg completions include categories, JS API module shortcuts, and `best-practices`. Deeper args complete children of the resolved slug.
- Completions require cached docs — if the cache doesn't exist, no completions are returned (no network I/O). Users must run `k6 x docs` once to trigger the initial download.

## Slug resolution
- Categories are derived from the bundle's `sections.json` at runtime — no hardcoded category list in the binary. If the first arg (or its first segment) exists as a slug in the index, it's used directly. Otherwise, it's treated as a JS API module shorthand.
- Args are normalized first: slashes are split so `mod/child` is treated identically to `mod child`.
- JS API shorthand: `k6 x docs mod child` → `javascript-api/k6-mod/child`
- Full slug: `k6 x docs javascript-api/k6-mod/child` → `javascript-api/k6-mod/child`
- Category: `k6 x docs some-category topic` → `some-category/topic` (when `some-category` exists in the index)
- k6-prefix fallback: `withK6Prefix` in `resolve.go` inserts `k6-` on the second segment of any `javascript-api/` slug when the original doesn't exist. Existing docs are prioritized.
- Parent-prefix fallback: `withParentFallback` in `resolve.go` retries `parent/child` as `parent/parent-child` when the original doesn't exist.

### Rendering
- Built-in markdown rendering via `glamour` library (`render.go`). Automatically renders with ANSI styling when stdout is a TTY and color is enabled. Non-TTY output (e.g. piped to an agent) is raw markdown.
- `--depth` flag (int, default 1) controls how many levels of subtopics are shown in TOC and section footers. `printTree` is the single recursive function used everywhere.
- Links to the current version's online docs are stripped: `[text](https://grafana.com/docs/k6/v1.6.1/foo)` → `text`.
- Stripped: Shared shortcodes (`{{< docs/shared >}}`), code tags (`{{< code >}}`), section tags (`{{< section >}}`), React/MDX component tags (`<Glossary>`), `<br/>`, internal doc links, image links, remaining markdown links, HTML comments, YAML frontmatter.
- Converted: Admonitions (`{{< admonition type="warning" >}}`) → `> **Warning:** ...` blockquotes.
- Placeholders replaced: `<K6_VERSION>` → actual version.
- All internal doc links and remaining markdown links are stripped to plain text (URL removed).
- Duplicate child names are deduplicated in search results.

### Documentation version handling
- Auto-detects k6 version from Go build info.
- Maps to wildcard: `v1.5.0` → `v1.5.x`, `v1.6.0-rc.1` → `v1.6.x`.
- Override via `--version` flag or `K6_DOCS_VERSION` env var (wildcard mapping is always applied, e.g. `--version 1.6.0` → `v1.6.x`).
- Cache dir override via `--cache-dir` flag or `K6_DOCS_CACHE_DIR` env var.
- `go.mod` floor for `go.k6.io/k6` must stay at v1.5.0 so Go's MVS doesn't override the k6 version users build with via xk6. Extension code can only use k6 APIs from v1.5.0; use build tags if newer APIs are needed.

### Bundle preparation (standalone `cmd/prepare/`)
- Clones k6-docs if not present, checks out matching tag.
- Builds shared content map from `docs/sources/shared/`.
- Walks markdown files, parses YAML frontmatter (deduplicates duplicate keys by keeping first occurrence), derives slugs. All top-level directories are included (only the shared content directory is skipped).
- Handles slug collisions: prefers `_index.md` over leaf `.md` (it has children).
- Populates parent→child relationships.
- Outputs: `dist/sections.json`, `dist/markdown/**/*.md`, `dist/best_practices.md`.

### Agent skill (`skills/xk6-docs/`)
- Installable via `k6 x docs skill <dir>` (embeds SKILL.md + references via `//go:embed`, templates `<binary>` placeholder with the running binary's absolute path via `os.Args[0]`).
- `k6 x docs skill` (no args) shows a glamour-rendered table of supported agents and their skill directories.
- SKILL.md tells agents: if the binary path fails, tell the user to re-run `k6 x docs skill <dir>` and stop.
- Never duplicate docs content (code examples, API descriptions). Only provide navigation paths and gotchas that save agents from trial-and-error.
- Each reference is a single module/area workflow.
- Before updating the skill, use `./k6 x docs` yourself to verify paths and discover gotchas.
- Run `skills/xk6-docs/scripts/validate-paths.sh ./k6` to find broken paths and uncovered modules.

### Cache staleness (`cache.go`)
- On download, stores ETag in `.etag` and current timestamp in `.last_check` inside the cache dir.
- On cache hit, if `.last_check` is >24h old (or missing), does a HEAD request to compare ETags.
- If ETag changed: re-downloads the bundle. If same: updates `.last_check`. On network error: silently serves from cache.

### CI/CD
- CI: lint + test + build on push/PR to main.
- Release: triggered by `vx.y.z` tag push. Builds k6 binaries (with this extension via xk6) for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64. Publishes binaries + `checksums.txt` to a GitHub release.
- Bundle sync: runs every 3 hours (or manual). Compares k6-docs version folders (v1.5.x+) against existing bundle assets; builds missing bundles and rebuilds stale ones. Detection logic is in `.github/scripts/sync.sh`, tested by `sync_test.sh` (run via `make test-gh`). To verify sync is working: compare `gh release view doc-bundles` asset dates against `gh api repos/grafana/k6-docs/commits?per_page=1&path=docs/sources/k6/<version>` dates — no version folder should be newer than its bundle.
