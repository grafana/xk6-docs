# Plan: k6 x docs Subcommand Extension

## Context

AI agents working with k6 need documentation accessible via CLI — not a browser, not an MCP server. The k6 x docs subcommand serves version-specific k6 documentation as clean markdown via progressive disclosure: k6 x docs shows the index, k6 x docs http shows the HTTP module docs, k6 x docs http get drills into a specific function.

The k6-docs repo uses Hugo with shortcodes (`{{< code >}}`, `{{< admonition >}}`, `{{< docs/shared >}}`, etc.) that render as broken syntax outside Hugo. A CI pipeline transforms these into clean markdown and publishes compressed doc bundles. The extension downloads the right bundle at runtime based on the detected k6 version.

## Architecture: Runtime Fetch + Cache

Docs are NOT embedded in the binary (unlike mcp-k6). Instead:

1. CI pipeline (triggered on k6 release): clones k6-docs → runs prepare for that version → transforms markdown → compresses with zstd (max compression) → publishes as GitHub release artifact
2. Runtime (first k6 x docs): detects k6 version via runtime/debug.ReadBuildInfo() → maps v1.5.0 → v1.5.x → downloads matching .tar.zst bundle from GitHub releases → extracts to ~/.local/share/k6/docs/v1.5.x/
3. Subsequent runs: reads from cache

Why not embed? The extension is compiled into k6 via xk6. Users often combine multiple extensions (xk6 build --with docs --with dashboard --with ...). There's no xk6 build hook or compile-time version detection — the extension can't know which k6 version it's being built for until runtime. Embedding all versions (like mcp-k6) would bloat every k6 binary.

k6 version detection — k6 itself uses this pattern (internal/cmd/version.go):
```go
buildInfo, _ := debug.ReadBuildInfo()
for _, dep := range buildInfo.Deps {
    if dep.Path == "go.k6.io/k6" {
        return dep.Version // e.g., "v1.5.0"
    }
}
```

## Repository

New repo: xk6-docs (based on xk6-subcommand-example template).

## File Structure

```
xk6-docs/
├── register.go              # init() + subcommand.RegisterExtension("docs", ...)
├── cmd.go                   # Cobra command definition + dispatch
├── docs.go                  # Section lookup, output formatting
├── docs_test.go             # Command + output tests
├── resolve.go               # ALL slug resolution logic lives here (single source of truth)
├── resolve_test.go          # Slug resolution tests
├── sections.go              # Section types, index loading, search
├── sections_test.go         # Section index + search tests
├── cache.go                 # Download, decompress (zstd), cache to disk
├── cache_test.go            # Cache tests
├── version.go               # k6 version detection via ReadBuildInfo
├── version_test.go          # Version detection tests
├── transform.go             # Hugo shortcode resolution + markdown cleanup (used by prepare)
├── transform_test.go        # Transform tests
├── cmd/
│   └── prepare/
│       └── main.go          # CI tool: clone k6-docs, transform, index, compress bundle
├── .github/
│   └── workflows/
│       ├── ci.yml           # Lint + test on PR/push
│       ├── release-bundle.yml  # Build + publish doc bundle (primary: repository_dispatch)
│       └── release-bundle-poll.yml  # Fallback: scheduled poll for new k6 releases (disabled)
├── Makefile
├── go.mod                   # deps: go.k6.io/k6, spf13/cobra, klauspost/compress, yaml.v3
├── go.sum
└── README.md
```

## CI: Doc Bundle Pipeline

### Primary: repository_dispatch (enabled)

A step in k6's release workflow fires an event:
```yaml
# In k6's release workflow
- uses: peter-evans/repository-dispatch@v3
  with:
    repository: grafana/xk6-docs
    event-type: k6-release
    client-payload: '{"version": "${{ github.ref_name }}"}'
```

The release-bundle.yml workflow listens and builds:
```yaml
on:
  repository_dispatch:
    types: [k6-release]

jobs:
  build-bundle:
    steps:
      - uses: actions/checkout@v4
      - run: go run ./cmd/prepare --k6-version=${{ github.event.client_payload.version }}
      - run: tar -cf - -C dist . | zstd --ultra -22 -o docs-$VERSION.tar.zst
      - uses: softprops/action-gh-release@v2
        with:
          tag_name: docs-$VERSION
          files: docs-$VERSION.tar.zst
```

### Fallback: Scheduled poll (disabled by default)

release-bundle-poll.yml — a cron workflow that checks `gh release list -R grafana/k6` for new versions, compares against existing bundles, builds any missing ones. Disabled by default. Enable if repository_dispatch doesn't work due to cross-repo auth issues.

### Compression

Use zstd with maximum compression (--ultra -22) via github.com/klauspost/compress/zstd. Zstd achieves excellent compression ratios on text while decompressing extremely fast. The Go library (klauspost/compress) is the standard choice — battle-tested, pure Go, no CGO.

Bundle format: .tar.zst containing:
```
sections.json
best_practices.md
markdown/
  javascript-api/
    k6-http/
      _index.md
      get.md
      ...
  using-k6/
    scenarios.md
    ...
```

## CLI Interface

Two modes: read mode (default) shows documentation content, list mode (--list) shows what's inside a topic.

```
# Read mode (default) — shows content
k6 x docs                                  # Top-level overview / table of contents
k6 x docs http                             # JS API shortcut → javascript-api/k6-http
k6 x docs http get                         # JS API shortcut → javascript-api/k6-http/get
k6 x docs browser page click              # JS API shortcut → javascript-api/k6-browser/page/click
k6 x docs using-k6 scenarios              # Category prefix → using-k6/scenarios
k6 x docs javascript-api/k6-http/get      # Full slug (with /) always works

# List mode (--list) — shows children/index at any level
k6 x docs --list                           # List top-level categories
k6 x docs http --list                      # List subtopics under k6/http
k6 x docs using-k6 --list                 # List using-k6 sections

# Search — searches titles, descriptions, AND markdown body text
k6 x docs search <term>                   # Returns usable topic/subtopic paths

# Other
k6 x docs best-practices                  # Print the best practices guide
k6 x docs --all                            # Concatenate all docs (pipe to file or LLM context)
```

## Slug Resolution (resolve.go — single source of truth)

All slug resolution logic lives in one file. No resolution logic anywhere else. The command layer calls Resolve(args []string) (slug string, err error) and gets back a canonical slug.

Rules (deterministic, no ambiguity):

1. If input contains / → treat as a full slug, use directly.
   - javascript-api/k6-http/get → javascript-api/k6-http/get
2. If first word matches a known category prefix → join all words with /.
   Known categories: javascript-api, using-k6, using-k6-browser, testing-guides, examples, results-output, reference
   - using-k6 scenarios → using-k6/scenarios
   - using-k6 k6-options reference → using-k6/k6-options/reference
   - examples websockets → examples/websockets
   - testing-guides test-types → testing-guides/test-types
3. If first word does NOT match any category → it's a JS API module shortcut. Prepend javascript-api/k6- to the first word, join the rest with /.
   - http get → javascript-api/k6-http/get
   - browser page click → javascript-api/k6-browser/page/click
   - metrics → javascript-api/k6-metrics
   - ws → javascript-api/k6-ws
   - data → javascript-api/k6-data

The k6- prefix is stripped from user input to avoid repetition (the docs use k6-http, k6-browser, etc. as directory names but users just type http, browser).

## --list output format (compact, machine-friendly):

```
$ k6 x docs http --list
http — HTTP requests module

  get           Issue an HTTP GET request
  post          Issue an HTTP POST request
  put           Issue an HTTP PUT request
  del           Issue an HTTP DELETE request
  batch         Issue multiple HTTP requests in parallel
  request       Issue any type of HTTP request
  ...
```

## Search (k6 x docs search <term>)

Searches across frontmatter title, description, AND markdown body text (case-insensitive).

Returns matching topics/subtopics as usable paths that users can feed directly back to k6 x docs:
```
$ k6 x docs search threshold
Results for "threshold":

  using-k6 thresholds          Pass/fail criteria for metrics
  http expected-statuses       Create a callback for setResponseCallback
```

The output uses the short form (not full slugs) so users can copy-paste directly: k6 x docs using-k6 thresholds or k6 x docs http expected-statuses.

## Content Scope

Include everything needed for writing and running k6 tests:

| Category | Include | Notes |
|---|---|---|
| javascript-api/ | All | Core API reference — most queried content |
| using-k6/ | All | Options, scenarios, lifecycle, checks, thresholds, metrics |
| using-k6-browser/ | All | Browser testing |
| testing-guides/ | All | Test types, API load testing |
| examples/ | All | Practical code examples |
| results-output/ | All | Output handling |
| get-started/ | No | Too introductory |
| set-up/ | No | Installation — not useful at test-writing time |
| extensions/ | No | Meta-docs about extension ecosystem |
| grafana-cloud-k6/ | No | Cloud-specific |
| reference/ | Glossary only | |
| release-notes/ | No | |
| Best practices | Yes | Standalone guide. Covers test structure, performance, error handling, data management, auth, monitoring, design patterns, code quality, modern k6 features, browser testing. Accessible via k6 x docs best-practices. |

## Implementation Steps

### Step 1: Scaffold the repo

Create the repo from the xk6-subcommand-example template:
- go.mod with module github.com/grafana/xk6-docs
- register.go calling subcommand.RegisterExtension("docs", newCmd)
- Makefile with lint, test, build, and prepare targets
- README.md covering: what this is, CLI usage, how to build with xk6, how doc bundles work, how CI triggers
- .gitignore

### Step 2: Section index (sections.go) + slug resolution (resolve.go)

sections.go — types, index loading, search:

```go
type Section struct {
    Slug        string   // e.g., "javascript-api/k6-http/get"
    RelPath     string   // e.g., "javascript-api/k6-http/get.md"
    Title       string   // from frontmatter
    Description string   // from frontmatter
    Weight      int      // sort order
    Category    string   // top-level dir (e.g., "javascript-api")
    Children    []string // child slugs (populated from directory structure)
    IsIndex     bool     // true for _index.md files
}

type Index struct {
    Version  string    // e.g., "v1.5.x"
    Sections []Section
}
```

- bySlug lookup map for O(1) access
- Search function: case-insensitive substring match across frontmatter title, description, AND full markdown body text. Returns matching sections with their short-form paths (usable as k6 x docs arguments).

resolve.go — ALL slug resolution logic (single source of truth):

```go
// Resolve converts user-provided args into a canonical slug.
// This is the ONLY place slug resolution happens.
func Resolve(args []string) string
```

Known category prefixes (const list):
```go
var categories = []string{
    "javascript-api", "using-k6", "using-k6-browser",
    "testing-guides", "examples", "results-output", "reference",
}
```

### Step 3: Build the transform pipeline (transform.go)

Process raw Hugo markdown into clean standalone markdown. Order matters:

1. Resolve `{{< docs/shared >}}` — inline the referenced shared file content (strip its frontmatter first).
2. Strip `{{< code >}}` / `{{< /code >}}` — just remove the shortcode tags, keep the content (code blocks inside are already valid markdown)
3. Convert `{{< admonition type="X" >}}...{{< /admonition >}}` — to `> **Note/Caution:** ...` blockquotes
4. Strip `{{< section >}}` — remove entirely (auto-generated child lists, not useful in CLI output)
5. Strip decorative shortcodes — `{{< youtube >}}`, `{{< card-grid >}}`, `{{< docs/hero-simple >}}`, `{{< docs/k6/* >}}`
6. Replace `<K6_VERSION>` in URLs with actual version string
7. Strip Hugo comments — `<!-- md-k6:skip -->`, `<!-- md-k6:skipall -->`
8. Strip YAML frontmatter — the title is already an H1 in the body
9. Normalize whitespace — collapse 3+ consecutive newlines to 2

### Step 4: Build the cache + version detection (cache.go, version.go)

version.go — detect k6 version at runtime:
```go
func detectK6Version() (string, error) {
    buildInfo, ok := debug.ReadBuildInfo()
    // find "go.k6.io/k6" in buildInfo.Deps
    // return version, map v1.5.0 → v1.5.x
}
```

cache.go — download, decompress, and cache doc bundles:
- Cache directory: ~/.local/share/k6/docs/{version}/
- Download URL pattern: https://github.com/grafana/xk6-docs/releases/download/docs-{version}/docs-{version}.tar.zst
- Decompress with github.com/klauspost/compress/zstd
- Extract tar to cache directory
- Return cache path for section index + markdown files
- Handle errors gracefully: network failures, corrupt downloads, disk permission issues

### Step 5: Build the prepare command (cmd/prepare/main.go)

CI tool that processes k6-docs into a doc bundle:

1. Accept --k6-version flag (e.g., v1.5.x) and --k6-docs-path flag (local path, skips git clone)
2. If no local path: clone k6-docs with --depth 1
3. Walk docs/sources/k6/{version}/ filtering to included categories
4. Build a shared content map from the shared/ directory
5. For each markdown file:
   - Parse YAML frontmatter → extract title, description, weight
   - Run full transform pipeline
   - Write clean markdown to dist/markdown/{relative-path}
6. Build section index from collected metadata
7. Populate Children fields based on directory structure
8. Write dist/sections.json
9. Copy/generate dist/best_practices.md

Output: dist/ directory ready to be compressed into a .tar.zst bundle.

### Step 6: Implement the Cobra command (cmd.go, docs.go)

```go
func newCmd(gs *state.GlobalState) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "docs [topic] [subtopic]",
        Short: "Print k6 documentation",
    }
    // flags: --list, --all
    // subcommands: search
    // special slugs: best-practices
    // dispatch to: printTOC, printSection, printList, printSearch, printBestPractices, printAll
}
```

On first invocation:
1. Call detectK6Version() to get the k6 version
2. Call cache to ensure docs are available (download if needed)
3. Load sections.json from cache
4. Dispatch to the appropriate output function

### Step 7: GitHub workflows

- ci.yml: Run on PR/push — make lint && make test && make build
- release-bundle.yml (enabled): Triggered by repository_dispatch from k6 release → run prepare → compress → publish release artifact
- release-bundle-poll.yml (disabled): Fallback cron that polls for new k6 releases. Enable if repository_dispatch fails due to cross-repo auth.

### Step 8: README.md

Cover:
- What this extension does
- CLI usage examples
- How to build with xk6
- How doc bundles work (runtime fetch + cache)
- How CI auto-publishes bundles on k6 releases
- Development guide (running prepare locally, running tests)

## Execution: Subagents

The main agent spawns subagents via the Task tool (with isolation: "worktree" for parallel work). No formal team — just direct subagent spawning and merging.

All subagents MUST:
1. Invoke the golang-patterns skill before writing any Go code.
2. Follow strict TDD (Red/Green/Refactor): write a failing test first, then write the minimal code to make it pass, then refactor. No production code without a test driving it.
3. Before committing: run make lint && make test — every commit must build, be linted, and have passing tests.
4. Create a commit with their changes when done.

Parallelism:

| Wave | Subagents | What |
|---|---|---|
| 1 | scaffold | go.mod, register.go, Makefile, .gitignore |
| 2 (parallel) | resolve, transform, cache | resolve.go, sections.go / transform.go / cache.go, version.go — all with tests |
| 3 (parallel) | prepare, command | cmd/prepare/main.go / cmd.go, docs.go — all with tests |
| 4 | readme-ci | .github/workflows/ + README.md |

Main agent merges worktree branches after each wave.

## Verification

1. make prepare K6_VERSION=v1.5.x K6_DOCS_PATH=~/grafana/k6-docs — prepare docs from local checkout
2. make test — run all unit and integration tests
3. make lint — run golangci-lint
4. make build — build with xk6: xk6 build --with github.com/grafana/xk6-docs=.
5. Test CLI commands:
   - ./k6 x docs — should print table of contents
   - ./k6 x docs --list — should list top-level categories
   - ./k6 x docs http — should print k6/http module docs (JS API shortcut)
   - ./k6 x docs http --list — should list subtopics under http (get, post, put, ...)
   - ./k6 x docs http get — should print full get() docs (→ javascript-api/k6-http/get)
   - ./k6 x docs browser page click — should resolve → javascript-api/k6-browser/page/click
   - ./k6 x docs using-k6 scenarios — should print scenarios docs (category prefix)
   - ./k6 x docs using-k6 scenarios --list — should list scenario subtopics
   - ./k6 x docs javascript-api/k6-http/get — full slug should work directly
   - ./k6 x docs search threshold — should search titles, descriptions, and body text
   - ./k6 x docs best-practices — should print the best practices guide
   - ./k6 x docs --all | wc -l — should dump all docs
6. Verify no Hugo shortcodes remain: ./k6 x docs --all | grep '{{<' should return nothing
