## RULES
1. Update this file concisely whenever features are added, removed, or changed.
2. Update the `.agents/features.md` file when user-facing features are added, removed, or changed.
3. Read `.agents/history.md` for past incidents and lessons learned.
4. Never take easy shortcuts or chase after easy wins. Execute what's asked for.
5. Make the minimal change that delivers the new feature or fixes the bug.
3. Use TDD: write a minimal test, fail it, write a minimal code to pass it. Repeat.
6. User facing features should be tested with test scripts in `testdata/scripts`.
7. Never skip linters (`//nolint` without proof), trick the linter, change `go.mod` k6 floor below v1.5.0, add global vars or `init()` (except `register.go`).
8. Plans: When writing complex features or significant refactors, use a Plan (as described in `.agents/PLANS.md`) from design to implementation. Store plans in `.claude/plans/` with incrementing numbers.

---

`k6 x docs` — offline k6 documentation in the terminal. For humans and AI agents. Docs are not embedded in the binary. On first run, the extension detects the k6 version from build info, downloads a matching compressed doc bundle (`.tar.zst`) from GitHub releases, and caches it locally (`~/.local/share/k6/docs/{version}/`). Subsequent runs serve from cache with no network. Cached bundles are checked for staleness every 24 hours via ETag comparison; stale bundles are re-downloaded automatically. A separate standalone prepare tool (`cmd/prepare/`) builds these bundles by cloning the k6-docs Hugo repository, transforming markdown into CLI-friendly format, building a searchable index (`sections.json`), and compressing everything. CI auto-publishes bundles as assets under a single `doc-bundles` GitHub release.

## Browsing
- **Non-TTY** (agents, pipes): serves the same content as TTY, as plain markdown with no ANSI styling and no pager. Args resolve to topics and search works identically, so agents run `k6 x docs`, `k6 x docs <topic>`, and `k6 x docs search <query>` and read the printed output directly. No markdown-directory path or filesystem recipes are printed.
- **TTY** (human mode):
  - `k6 x docs` with no args prints `# k6 {version}` header followed by a depth-controlled bullet tree of categories and their children (default depth: 1), and a blockquote example hint.
  - `k6 x docs http get` resolves args to a slug (case-insensitive) and prints the cached markdown content (trimmed). If the topic has children, a `---` separator and `**{path} subtopics:**` section is appended with a depth-controlled bullet tree of children, and a blockquote example hint using the slug path form (`k6 x docs {path}/<subtopic>`).
  - `k6 x docs best-practices` prints a curated guide (embedded in the prepare tool via `//go:embed`).
  - `k6 x docs search <query>` fuzzy searches (case-insensitive, ignores punctuation, spaces, slashes) and prints an indented tree: `- {group}` with `  - {child}` underneath, no descriptions. Footer shows `Example:` with a sample navigation command. Search uses the same arg normalization and resolve rules as docs navigation (shared `normalizeArgs` and `ResolveWithLookup`), so `search browser page`, `search browser/page`, and `search javascript-api browser page` all produce the same results. `--depth` flag controls tree depth (same as TOC/section footers).

## Shell completions
- `ValidArgsFunction` on the `docs` and `search` commands provides dynamic topic completion via cobra's `__complete` mechanism. Users set up completions once via k6's built-in `k6 completion zsh` (or bash/fish/powershell); extensions like xk6-docs piggyback on that.
- First-arg completions include categories, JS API module shortcuts, `best-practices`, and the `search`/`skill` subcommands. Deeper args complete children of the resolved slug.
- The `search` subcommand is marked `Hidden` in cobra so cobra's completion engine doesn't add it automatically. `completionFirstArg` iterates `cmd.Commands()` to include hidden subcommands when the cache is ready. This gives the completion function full control over when they appear — they are suppressed when the cache is missing.
- Completions require cached docs. When the cache doesn't exist and `AutoExtensionResolution` is enabled, an active help message tells the user to press ENTER to load docs. When it's disabled, completions are silently empty. Other errors (bad index, version detection failure) are silent.

## Slug resolution
- Categories are derived from the bundle's `sections.json` at runtime — no hardcoded category list in the binary. If the first arg (or its first segment) exists as a slug in the index, it's used directly. Otherwise, it's treated as a JS API module shorthand.
- Args are normalized first: slashes are split so `mod/child` is treated identically to `mod child`.
- JS API shorthand: `k6 x docs mod child` → `javascript-api/k6-mod/child`
- Full slug: `k6 x docs javascript-api/k6-mod/child` → `javascript-api/k6-mod/child`
- Category: `k6 x docs some-category topic` → `some-category/topic` (when `some-category` exists in the index)
- k6-prefix fallback: `withK6Prefix` in `resolve.go` inserts `k6-` on the second segment of any `javascript-api/` slug when the original doesn't exist. Existing docs are prioritized.
- Parent-prefix fallback: `withParentFallback` in `resolve.go` retries `parent/child` as `parent/parent-child` when the original doesn't exist.

### Rendering
- Built-in markdown rendering via `glamour` library (`render.go`). Automatically renders with ANSI styling when stdout is a TTY and color is enabled. Non-TTY output prints plain markdown (the same content, no ANSI styling).
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

### Local source preview (`--source`)
- `--source <k6-docs-path>` (in `cmd.go`/`bundle.go`) makes `k6 x docs` build docs from a local k6-docs working tree instead of downloading. For docs authors previewing in-progress edits.
- `setup` runs the shared `internal/bundle` pipeline (same transform as published bundles) into a scratch dir under the doc cache base: `{cacheBase}/.sources/{hash-of-abs-source}/{version}`. The `.sources` subdir is hidden from version discovery (`versionDirRe` matches only `vX.Y.x`), so it never affects normal usage or downloaded bundles. Each source path a user points at hashes to its own subdir, so different checkouts don't collide. Served local-only.
- Rebuild is skipped when unchanged: `bundle.SourceStamp` digests the source `.md` files (path, size, mtime) into `{version}.stamp`; `buildSourceBundle` rebuilds only when the stamp differs or the prior build is missing. So edits/additions/deletions are reflected, unchanged reruns are fast, and there is no cleanup-on-exit. A rebuild logs `Building k6 <version> docs from <path>...` via `logf` (stderr); a skip is silent — `source.txtar` asserts on this to test skip-vs-rebuild end-to-end.
- With `--source`, `--version` defaults to `next` (in-development docs), not the detected k6 version — authors want the version they're editing. An explicit `--version` (or `K6_DOCS_VERSION`) overrides it to another dir under `docs/sources/k6/`. A missing version dir yields "version root not found".
- Completions ignore `--source` (they read the normal cache) — rebuilding the tree per TAB keystroke would be too slow. A brand-new local page renders when typed but won't appear in completions.
- The generated `cloud-rest-api` section (below) is not produced under `--source`; only `cmd/prepare` injects it.

### Bundle preparation (`internal/bundle`, standalone `cmd/prepare/`)
- The transform-and-index pipeline lives in `internal/bundle` (`Build`), shared by the standalone `cmd/prepare/` tool and the `--source` preview. `cmd/prepare/` adds only flag parsing and the optional k6-docs clone.
- Clones k6-docs if not present, checks out matching tag.
- Builds shared content map from `docs/sources/shared/`.
- Walks markdown files, parses YAML frontmatter (deduplicates duplicate keys by keeping first occurrence), derives slugs. All top-level directories are included (only the shared content directory is skipped).
- Handles slug collisions: prefers `_index.md` over leaf `.md` (it has children).
- Populates parent→child relationships.
- Outputs: `dist/sections.json`, `dist/markdown/**/*.md` (including `best_practices.md`).
- `Build` accepts `WithExtraSections`, a hook run after the walk and before parent/child wiring; `cmd/prepare` uses it to inject the Cloud REST API section. `internal/bundle` does not import the generator.

### Cloud REST API reference (`cloud-rest-api/`, `internal/bundle/restdoc/`)
- `internal/bundle/restdoc` parses the Grafana Cloud k6 OpenAPI specs and renders one markdown page per endpoint under `cloud-rest-api/<v5|v6>/<operationId>`, plus `_index.md` pages, returning `docs.Section`s. Ported from xk6-rest (parser + renderer only — no runtime fetch/cache, no `k6 x rest` command).
- `cmd/prepare` fetches the live v6 spec from `api.k6.io` at build time (10s timeout, injectable `*http.Client`) and passes it in; on any error it falls back to the v6 spec embedded in `restdoc`. The v5 spec is always the embedded, hand-authored reference (the v5 API publishes no OpenAPI doc and is deprecated).
- Wired only through `cmd/prepare` via `WithExtraSections`, so `restdoc`'s embedded specs never enter the extension binary, and `--source` previews omit the section.
- Served by the normal docs machinery (TOC, `search`, completion, rendering). Freshness is opportunistic: the baked v6 snapshot refreshes when a bundle is rebuilt (driven by k6-docs commits in `sync.sh`) or via a manual sync `workflow_dispatch`; `sync.sh` does not inspect the OpenAPI spec.

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
- k6 provisioning: k6's auto-extension resolution discovers xk6-docs via the [extension registry](https://github.com/grafana/k6-extension-registry) (`registry.yaml`). After tagging a new version, open a PR to add it to the `versions` list under the `github.com/grafana/xk6-docs` entry. The provisioning service resolves the latest listed version from the registry, fetches it from the Go module proxy, and builds a binary. Go module proxy caches are immutable — a faulty tag cannot be replaced, only superseded by a new version.
