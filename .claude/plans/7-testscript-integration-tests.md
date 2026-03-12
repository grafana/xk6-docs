# Simplify Tests with testscript

## Prerequisites

Before doing ANY work, read these files in order:
1. `/Users/inanc/grafana/xk6-docs/AGENTS.md` — project rules, architecture, feature docs
2. `/Users/inanc/.claude/CLAUDE.md` — user's global coding preferences (TDD, no comments, no refactoring)
3. This plan file

## Task List

Each task is a self-contained unit of work. Tasks within the same phase can be assigned to parallel subagents.

### Phase 1: Infrastructure (must complete before Phase 2)

| #   | Task                                                                                           | Files                          | Depends on |
| --- | ---------------------------------------------------------------------------------------------- | ------------------------------ | ---------- |
| 1   | Add testscript dependency                                                                      | `go.mod`, `go.sum`             | —          |
| 2   | Update testdata/cache fixture: change testing-guides description to > 80 bytes non-ASCII UTF-8 | `testdata/cache/sections.json` | —          |
| 3   | Write `docs_test.go`: TestMain + TestScripts + copyDir                                         | `docs_test.go`                 | 1          |
| 4   | Create `testdata/scripts/` directory                                                           | filesystem                     | —          |

### Phase 2: Write .txtar scripts (all parallel, after Phase 1)

| #   | Task                                              | File                                        | Agent |
| --- | ------------------------------------------------- | ------------------------------------------- | ----- |
| 5   | Write `toc.txtar` + generate golden               | `testdata/scripts/toc.txtar`                | A     |
| 6   | Write `view_topic.txtar` + generate golden        | `testdata/scripts/view_topic.txtar`         | A     |
| 7   | Write `list.txtar` + generate golden              | `testdata/scripts/list.txtar`               | A     |
| 8   | Write `all.txtar` + generate golden               | `testdata/scripts/all.txtar`                | A     |
| 9   | Write `best_practices.txtar` + generate golden    | `testdata/scripts/best_practices.txtar`     | A     |
| 10  | Write `slug_resolution.txtar` + generate golden   | `testdata/scripts/slug_resolution.txtar`    | B     |
| 11  | Write `slug_edge_cases.txtar` + generate golden   | `testdata/scripts/slug_edge_cases.txtar`    | B     |
| 12  | Write `search.txtar` + generate golden            | `testdata/scripts/search.txtar`             | B     |
| 13  | Write `search_edge_cases.txtar` + generate golden | `testdata/scripts/search_edge_cases.txtar`  | B     |
| 14  | Write `errors.txtar`                              | `testdata/scripts/errors.txtar`             | B     |
| 15  | Write `setup_errors.txtar`                        | `testdata/scripts/setup_errors.txtar`       | B     |
| 16  | Write `flags.txtar`                               | `testdata/scripts/flags.txtar`              | B     |
| 17  | Write `help.txtar`                                | `testdata/scripts/help.txtar`               | B     |
| 18  | Write `renderer.txtar`                            | `testdata/scripts/renderer.txtar`           | C     |
| 19  | Write `renderer_search.txtar`                     | `testdata/scripts/renderer_search.txtar`    | C     |
| 20  | Write `config.txtar`                              | `testdata/scripts/config.txtar`             | C     |
| 21  | Write `version_precedence.txtar`                  | `testdata/scripts/version_precedence.txtar` | C     |
| 22  | Write `auto_detect.txtar`                         | `testdata/scripts/auto_detect.txtar`        | C     |
| 23  | Write `debug_log.txtar`                           | `testdata/scripts/debug_log.txtar`          | C     |

### Phase 3: Cleanup (after Phase 2)

| #   | Task                                                                                  | Files                               | Depends on |
| --- | ------------------------------------------------------------------------------------- | ----------------------------------- | ---------- |
| 24  | Clean up `config_test.go`: remove all command-level tests, keep only unit tests       | `config_test.go`                    | 18-23      |
| 25  | Trim `resolve_test.go` to single Rule 1 slash-in-later-arg unit test                  | `resolve_test.go`                   | 10-11      |
| 26  | Delete `smoke_test.go`                                                                | `smoke_test.go`                     | 18-19      |
| 27  | Delete `testdata/golden/` directory                                                   | `testdata/golden/`                  | 5-23       |
| 28  | Run `go test ./... -v -count=1`                                                       | —                                   | 24-27      |
| 29  | Run `golangci-lint run`                                                               | —                                   | 28         |
| 30  | Add new Go unit tests: non-regular tar entries, IsCached failure, groupSlug condition | `cache_test.go`, `sections_test.go` | —          |

### Parallelization

- **Phase 1**: Tasks 1-4 run sequentially (fast, < 1 minute total)
- **Phase 2**: 3 parallel agents (A: tasks 5-9, B: tasks 10-17, C: tasks 18-23)
- **Phase 3**: Tasks 24-27 in parallel, then 28-29 sequentially, task 30 in parallel with anything

## Context

Tests are overcomplicated: MemMapFs, GlobalState construction, `newCmd(gs)`, inline string fixtures, Go-based golden file helpers. The `testscript` package (`github.com/rogpeppe/go-internal`) eliminates all this — register the command once in TestMain, write `.txtar` scripts that run it like a user would. Golden output lives inline in the scripts. No more test infrastructure code.

## How testscript works

Package: `github.com/rogpeppe/go-internal/testscript`
Docs: https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript

Note: `rogpeppe/go-internal/txtar` has a deprecation note pointing to `golang.org/x/tools/txtar` (golang/go#59264 is fixed). This only affects the txtar archive format library. The `testscript` package itself has no replacement and stays in `rogpeppe/go-internal`. We depend on `testscript`, not `txtar` directly.

### TestMain — register commands as binaries

```go
func TestMain(m *testing.M) {
    testscript.Main(m, map[string]func(){
        "myapp": myAppMain,  // registers "myapp" as a command available in scripts
    })
    // Main calls os.Exit — never returns
}
```

Commands registered here can be called with `exec myapp args...` in `.txtar` scripts. The function runs in-process (no subprocess), so coverage is collected.

### Run — execute script files

```go
func TestScripts(t *testing.T) {
    testscript.Run(t, testscript.Params{
        Dir:           "testdata/scripts",     // directory of .txtar files
        Setup:         func(env *testscript.Env) error { ... }, // pre-script setup
        UpdateScripts: os.Getenv("UPDATE") != "",  // auto-update golden on failure
    })
}
```

Each `.txtar` file in `Dir` becomes a subtest. Run one with: `go test -run TestScripts/^toc$`

### Setup — shared fixture initialization

```go
Setup: func(env *testscript.Env) error {
    // env.WorkDir is the temp directory for this script
    // Copy shared fixtures there
    return copyDir("testdata/cache", filepath.Join(env.WorkDir, "cache"))
}
```

Files defined in the `-- filename --` section of `.txtar` are also extracted to `env.WorkDir`.

### .txtar script syntax

```
# Comment (phase marker — shown on failure)
exec myapp arg1 arg2           # run command, must succeed
! exec myapp bad-arg           # run command, must FAIL
stdout 'pattern'               # last stdout must match regex
! stdout 'pattern'             # last stdout must NOT match regex
stderr 'pattern'               # last stderr must match regex
cmp stdout golden.txt          # exact comparison of stdout vs file
cmpenv stdout golden.txt       # same but expands $VARS in golden file

env KEY=value                  # set environment variable
env KEY=$WORK/path             # $WORK = script's temp working dir
mkdir $WORK/subdir             # create directory
cp source.txt $WORK/dest.txt   # copy file

-- filename.txt --             # define a file in the archive
file contents here
-- another/file.txt --
more contents
```

### TTY simulation (pseudo-terminal)

By default, `exec` captures stdout/stderr via pipes — the command sees `IsTTY=false`.

To simulate a terminal (make `IsTTY=true`):

```
# Attach a pseudo-terminal to the NEXT exec command.
# The file contents become raw terminal input (can be empty).
ttyin empty.txt
exec myapp arg1 arg2

# Check terminal output (NOT stdout — terminal output is separate).
ttyout 'expected pattern'
```

- `ttyin file` — attaches PTY to next `exec`. Use `ttyin -stdin file` to also connect stdin.
- `ttyout pattern` — grep against the raw terminal output (not `stdout`).
- Without `ttyin`, use `stdout` / `stderr` for pipe-captured output.
- With `ttyin`, the command's stdout goes to the terminal, so use `ttyout` (not `stdout`).

This is critical for testing renderer behavior:
- **Non-TTY** (default pipe): `exec` + `stdout` — renderer path is skipped
- **TTY** (`ttyin` + `exec`): `ttyout` — renderer path is active, fallback testable

### Key variables in scripts

- `$WORK` — script's temporary working directory (files from archive + Setup)
- `$HOME` — set to `/no-home` by default (isolates from real home)
- `$TMPDIR` — `$WORK/.tmp`

### UpdateScripts (golden file auto-update)

When `UpdateScripts: true` and `cmp stdout golden.txt` fails:

1. testscript finds `golden.txt` defined inside the **same `.txtar` file** (the `-- golden.txt --` archive section)
2. It **rewrites the `.txtar` file on disk**, replacing the `-- golden.txt --` content with actual stdout
3. The `cmp` succeeds instead of failing

So golden content lives **inside** each `.txtar` file — not in separate files. The whole test (script + expected output) is one self-contained file.

**Workflow:**
```bash
# 1. Write scripts with placeholder golden content
# -- golden.txt --
# (placeholder)

# 2. Generate real golden content
UPDATE_GOLDEN=1 go test -run TestScripts

# 3. testscript rewrites each .txtar, replacing placeholders with actual output

# 4. Commit the updated .txtar files

# 5. From now on, go test compares output against committed golden content
go test -run TestScripts
```

**Quoting caveat:** If output contains lines that look like txtar markers (`-- something --`), testscript auto-quotes with `txtar.Quote` (prefixes lines with `>`). If that happens, add `unquote golden.txt` before `cmp` in the script.

## Files to modify

| File                       | Action                                                                                                                                                                                                              |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `go.mod` / `go.sum`        | `go get github.com/rogpeppe/go-internal`                                                                                                                                                                            |
| `docs_test.go`             | Rewrite: `TestMain` + `TestScripts` + `copyDir` helper only                                                                                                                                                         |
| `resolve_test.go`          | Keep ONE unit test: Rule 1 slash-in-later-arg (can't prove through command output — error message uses original args, not resolved slug). Delete everything else.                                                   |
| `config_test.go`           | Remove ALL command-level/integration tests (renderer, debug log, invalid config). Keep ONLY pure unit tests: `TestConfigDir`, `TestCacheDirUSERPROFILE`, `TestLoadConfig`. No integration tests in unit test files. |
| `smoke_test.go`            | Delete (testscript has `ttyin`/`ttyout` for TTY simulation)                                                                                                                                                         |
| `testdata/scripts/*.txtar` | Create all script files (config YAML inline in each script's archive)                                                                                                                                               |
| `testdata/golden/`         | Delete (golden output moves inline into `.txtar` files)                                                                                                                                                             |

## Files unchanged

| File                       | Why                                                              |
| -------------------------- | ---------------------------------------------------------------- |
| `cache_test.go`            | Tests EnsureDocs/extract with mock HTTP — not command-level      |
| `sections_test.go`         | Tests Index data structure with its own `testdata/sections.json` |
| `transform_test.go`        | Tests 13-step transform pipeline (60+ edge cases)                |
| `categories_test.go`       | Tests IsIncludedDocsPath                                         |
| `version_test.go`          | Tests MapToWildcard / DetectK6Version                            |
| `cmd/prepare/main_test.go` | Tests prepare pipeline                                           |

## Implementation

### Step 1: Add testscript dependency

```bash
go get github.com/rogpeppe/go-internal
```

### Step 2: Rewrite docs_test.go

The entire file becomes ~30 lines:

```go
package docs

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "testing"

    "github.com/rogpeppe/go-internal/testscript"
    "github.com/sirupsen/logrus"
    "go.k6.io/k6/cmd/state"
)

func TestMain(m *testing.M) {
    testscript.Main(m, map[string]func(){
        "k6-docs": func() {
            gs := state.NewGlobalState(context.Background())
            // Enable debug logging so debug_log.txtar can assert on log output.
            gs.Logger.SetLevel(logrus.DebugLevel)
            cmd := newCmd(gs)
            cmd.SetArgs(os.Args[1:])
            if err := cmd.Execute(); err != nil {
                fmt.Fprintf(os.Stderr, "Error: %v\n", err)
                os.Exit(1)
            }
        },
    })
}

func TestScripts(t *testing.T) {
    testscript.Run(t, testscript.Params{
        Dir: "testdata/scripts",
        Setup: func(env *testscript.Env) error {
            return copyDir("testdata/cache", filepath.Join(env.WorkDir, "cache"))
        },
        UpdateScripts: os.Getenv("UPDATE_GOLDEN") != "",
    })
}
```

Keep `copyDir` helper (already exists in `smoke_test.go`). Delete everything else.

**Required testdata/cache fixture sections** (all scripts depend on these via Setup):
- `javascript-api` (category, children: k6-http, jslib, k6-jslib)
- `javascript-api/k6-http` (children: get, post, cookiejar, k6-http-get)
- `javascript-api/k6-http/get`, `javascript-api/k6-http/post` (leaf)
- `javascript-api/k6-http/k6-http-get` (dedup trigger — childName = "get")
- `javascript-api/k6-http/cookiejar` (children: cookiejar-clear)
- `javascript-api/k6-http/cookiejar/cookiejar-clear` (parent-prefix stripping)
- `javascript-api/jslib` (no k6- prefix, for bare-name resolution)
- `javascript-api/k6-jslib` (k6-prefixed variant, proves priority)
- `using-k6` (category, children: scenarios)
- `using-k6/scenarios` (leaf)
- `examples` (category, children: websockets)
- `examples/websockets` (description > 80 chars for truncation)
- `testing-guides` (category, NO children — zero-children branch. Description must contain multi-byte UTF-8 characters AND be > 80 bytes, to test byte-based truncation. Example: `"テスト実行のガイドとベストプラクティスについての詳細なドキュメンテーション情報を提供します"` (108 bytes, will truncate mid-character with `...`).)
- `best_practices.md` at cache root

These already exist in `testdata/cache/` from the current codebase BUT the testing-guides description must be updated to include non-ASCII characters > 80 bytes. The `toc.txtar` golden file will capture the exact truncation behavior (including mid-character `...`). Verify with `cat testdata/cache/sections.json` before starting.

**Note on TTY:** testscript has `ttyin`/`ttyout` commands that attach a pseudo-terminal to `exec`. This enables testing both non-TTY (default pipe) and TTY (with `ttyin`) modes in the same `.txtar` script. All renderer tests (skip, use, fallback) are covered in `renderer.txtar`.

**Note on first-run download:** When no `--cache-dir` and no `K6_DOCS_CACHE_DIR` are set, the command calls `EnsureDocs` which downloads from GitHub. This can't be tested in testscript without network access. The download + extract pipeline is tested by `cache_test.go:TestEnsureDocs` with a mock HTTP client. At command level, we always provide `--cache-dir` or `K6_DOCS_CACHE_DIR` to bypass the download.

### Step 3: Create .txtar scripts

Each script runs the command like a user and compares against inline golden output.

**testdata/scripts/toc.txtar** — TOC, truncation (ASCII + non-ASCII), zero-children category, alignment
```
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
cmp stdout golden.txt

-- golden.txt --
(generated by UPDATE_GOLDEN=1 — will contain non-ASCII truncation for testing-guides)
```

**testdata/scripts/view_topic.txtar** — content, subtopics footer, frontmatter stripped, parent prefix, usage hints
```
# Section with children shows subtopics footer
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http
cmp stdout golden-http.txt

# Leaf section: no subtopics footer, frontmatter stripped
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
cmp stdout golden-http-get.txt
! stdout '---'
! stdout 'title:'

# Child names strip parent prefix (cookiejar-clear → clear)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http cookiejar
stdout 'Subtopics: clear'

# Non-JS-API usage hint
exec k6-docs --cache-dir $WORK/cache --version v0.55.x using-k6
stdout 'Use: k6 x docs using-k6 <subtopic>'

# Missing markdown file → section appears blank (no error shown to user).
# Delete the file from copied cache. Debug log goes to stderr but that's expected.
# The key assertion: command succeeds (no ! exec) and no user-visible error on stderr.
rm $WORK/cache/markdown/testing-guides/_index.md
exec k6-docs --cache-dir $WORK/cache --version v0.55.x testing-guides
! stderr 'error'
! stderr 'not found'

-- golden-http.txt --
(generated by UPDATE_GOLDEN=1)
-- golden-http-get.txt --
(generated by UPDATE_GOLDEN=1)
```

**testdata/scripts/slug_resolution.txtar** — all 3 rules + edge cases
```
# Full slug with /
exec k6-docs --cache-dir $WORK/cache --version v0.55.x javascript-api/k6-http/get
cmp stdout golden-get.txt

# JS API shortcut (same output proves correct resolution)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
cmp stdout golden-get.txt

# Case insensitive
exec k6-docs --cache-dir $WORK/cache --version v0.55.x HTTP GET
cmp stdout golden-get.txt

# k6- prefix dedup
exec k6-docs --cache-dir $WORK/cache --version v0.55.x k6-http get
cmp stdout golden-get.txt

# Category prefix
exec k6-docs --cache-dir $WORK/cache --version v0.55.x using-k6 scenarios
cmp stdout golden-scenarios.txt

# Parent prefix fallback (cookiejar/clear → cookiejar/cookiejar-clear)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http cookiejar clear
cmp stdout golden-clear.txt

# Bare name prefers unprefixed (both jslib and k6-jslib exist)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x jslib
cmp stdout golden-jslib.txt

-- golden-get.txt --
(generated)
-- golden-scenarios.txt --
(generated)
-- golden-clear.txt --
(generated)
-- golden-jslib.txt --
(generated)
```

**testdata/scripts/list.txtar** — categories, children, dedup, leaf
```
# No args shows categories (no TOC header)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --list
cmp stdout golden-no-args.txt
! stdout 'k6 Documentation'

# Topic with children (dedup: no "Alternate GET")
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --list http
cmp stdout golden-http.txt
! stdout 'Alternate GET'

# Leaf topic — exact format with golden comparison
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --list http get
cmp stdout golden-leaf.txt

-- golden-leaf.txt --
(generated)
-- golden-no-args.txt --
(generated)
-- golden-http.txt --
(generated)
```

**testdata/scripts/all.txtar**
```
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --all
cmp stdout golden.txt
! stdout 'Subtopics:'

-- golden.txt --
(generated)
```

**testdata/scripts/search.txtar**
```
# Title/description match
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search Scenarios
cmp stdout golden-scenarios.txt

# Description match grouped by JS API module
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search 'GET request'
cmp stdout golden-get-request.txt

# Multi-group sorted alphabetically
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search k6
cmp stdout golden-k6.txt

# Body content match
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search 'WebSocket example content'
stdout 'websockets'

# No results
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search zzzznotfound
stdout '\(no results\)'

# Missing arg errors
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x search
stderr 'requires at least 1 arg'

-- golden-scenarios.txt --
(generated)
-- golden-get-request.txt --
(generated)
-- golden-k6.txt --
(generated)
```

**testdata/scripts/best_practices.txtar**
```
exec k6-docs --cache-dir $WORK/cache --version v0.55.x best-practices
cmp stdout golden.txt
! stdout '---'

-- golden.txt --
(generated)
```

**testdata/scripts/errors.txtar**
```
# Unknown topic
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x nonexistent-topic-xyz
stderr 'topic not found'

# Missing best_practices.md
! exec k6-docs --cache-dir $WORK/empty-cache --version v0.55.x best-practices
stderr 'best practices'

-- empty-cache/sections.json --
{"version":"v0.55.x","sections":[]}
```

**testdata/scripts/help.txtar** — help text for main command and search subcommand
```
# Main command help text
exec k6-docs --help
stdout 'Print k6 documentation'
stdout 'Access k6 documentation from the command line'
stdout 'docs \[topic\] \[subtopic\.\.\.\]'

# Search subcommand help text
exec k6-docs search --help
stdout 'Search documentation'
```

**testdata/scripts/flags.txtar** — flag precedence, silently-ignored combos
```
# --all ignores --list (--all wins, checked first in code)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --all --list
stdout 'k6 Documentation'
! stdout 'Subtopics:'

# --all ignores args
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --all http get
stdout 'k6 Documentation'

# best-practices ignores extra args
exec k6-docs --cache-dir $WORK/cache --version v0.55.x best-practices extra-arg
! stdout 'extra-arg'
stdout 'best practices'

# best-practices is case-sensitive (Best-Practices is not recognized → topic not found)
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x Best-Practices
stderr 'topic not found'

# --list has no effect on best-practices
exec k6-docs --cache-dir $WORK/cache --version v0.55.x --list best-practices
stdout 'best practices'

# --version and --cache-dir inherited by search subcommand
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search k6
stdout 'Results for'
```

**testdata/scripts/version_precedence.txtar**
```
# --version flag overrides K6_DOCS_VERSION env
env K6_DOCS_VERSION=v9.9.x
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation \(v0\.55\.x\)'

# Clear stale env before next test
env K6_DOCS_VERSION=v0.55.x
env K6_DOCS_CACHE_DIR=
exec k6-docs --cache-dir $WORK/cache
stdout 'k6 Documentation \(v0\.55\.x\)'

# K6_DOCS_CACHE_DIR env used when no --cache-dir flag
env K6_DOCS_VERSION=
env K6_DOCS_CACHE_DIR=$WORK/cache
exec k6-docs --version v0.55.x
stdout 'k6 Documentation \(v0\.55\.x\)'

# --cache-dir flag overrides K6_DOCS_CACHE_DIR env
env K6_DOCS_VERSION=
env K6_DOCS_CACHE_DIR=/nonexistent/path
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation \(v0\.55\.x\)'

# --version is used VERBATIM — no wildcard mapping applied.
# v0.55.2 (not a wildcard) stays as v0.55.2 in output.
env K6_DOCS_VERSION=
env K6_DOCS_CACHE_DIR=
exec k6-docs --cache-dir $WORK/cache-exact --version v0.55.2
stdout 'k6 Documentation \(v0\.55\.2\)'

# K6_DOCS_VERSION env also used verbatim
env K6_DOCS_VERSION=v0.55.2
env K6_DOCS_CACHE_DIR=
exec k6-docs --cache-dir $WORK/cache-exact
stdout 'k6 Documentation \(v0\.55\.2\)'

-- cache-exact/sections.json --
{"version":"v0.55.2","sections":[]}
```

**testdata/scripts/auto_detect.txtar** — version auto-detection from Go build info
```
# No --version flag, no K6_DOCS_VERSION env.
# DetectK6Version reads go.k6.io/k6 from build info.
# We provide the cache via env to avoid a real download.
env K6_DOCS_CACHE_DIR=$WORK/cache
exec k6-docs
stdout 'k6 Documentation \('
```

**testdata/scripts/config.txtar**
```
# Invalid config warns but continues
env XDG_CONFIG_HOME=$WORK/invalid-cfg
mkdir $WORK/invalid-cfg/k6
cp invalid.yaml $WORK/invalid-cfg/k6/docs.yaml
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation'
stderr 'ignoring invalid config'

# Config loaded via HOME fallback (no XDG_CONFIG_HOME).
# Must clear XDG_CONFIG_HOME so configDir falls through to HOME.
# Use TTY + renderer cat -n to prove the config was actually loaded
# (cat -n adds tabs; without TTY, renderer is skipped regardless of config).
env XDG_CONFIG_HOME=
env HOME=$WORK/home
mkdir $WORK/home/.config/k6
cp cat-n.yaml $WORK/home/.config/k6/docs.yaml
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
ttyout '\t'

# HOME and USERPROFILE both unset → config fails gracefully with warning
env HOME=
env USERPROFILE=
env XDG_CONFIG_HOME=
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation'
stderr 'neither HOME nor USERPROFILE'

# File-unreadable config (permission denied) — warning logged, continues
env XDG_CONFIG_HOME=$WORK/unreadable-cfg
mkdir $WORK/unreadable-cfg/k6
cp valid.yaml $WORK/unreadable-cfg/k6/docs.yaml
chmod 000 $WORK/unreadable-cfg/k6/docs.yaml
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation'
stderr 'ignoring invalid config'

-- empty.txt --
-- invalid.yaml --
:
  :
    : [invalid
-- cat-n.yaml --
renderer: cat -n
-- valid.yaml --
renderer: cat
```

**testdata/scripts/renderer.txtar** — renderer behavior, both TTY and non-TTY

testscript has `ttyin` which attaches a pseudo-terminal to the next `exec`, making `IsTTY=true`. And `ttyout` to check terminal output.

```
# --- Non-TTY (default: stdout is a pipe) ---

# Renderer configured but SKIPPED in non-TTY (cat -n adds tabs if invoked)
env XDG_CONFIG_HOME=$WORK/cat-cfg
mkdir $WORK/cat-cfg/k6
cp cat-n.yaml $WORK/cat-cfg/k6/docs.yaml
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stdout 'k6 Documentation'
! stdout '\t'

# No renderer configured — raw output (clear XDG so previous config doesn't leak)
env XDG_CONFIG_HOME=
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
stdout 'http\.get'

# --- TTY (ttyin attaches pseudo-terminal) ---

# Renderer IS used when TTY + configured (cat -n adds tabs)
env XDG_CONFIG_HOME=$WORK/cat-cfg
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
ttyout 'http\.get'

# Failing renderer falls back to raw output in TTY mode
env XDG_CONFIG_HOME=$WORK/fail-cfg
mkdir $WORK/fail-cfg/k6
cp false.yaml $WORK/fail-cfg/k6/docs.yaml
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
ttyout 'http\.get'

# Missing renderer binary falls back to raw output in TTY mode
env XDG_CONFIG_HOME=$WORK/missing-cfg
mkdir $WORK/missing-cfg/k6
cp missing.yaml $WORK/missing-cfg/k6/docs.yaml
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x http get
ttyout 'http\.get'

-- empty.txt --
-- cat-n.yaml --
renderer: cat -n
-- false.yaml --
renderer: "false"
-- missing.yaml --
renderer: nonexistent-renderer-binary-xyz
```

**testdata/scripts/debug_log.txtar**
```
# Non-TTY logs agent mode to stderr (main command only)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stderr 'agent mode'

# TTY logs interactive mode to stderr
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x
stderr 'interactive mode'

# Search does NOT emit debug log
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search k6
! stderr 'agent mode'
! stderr 'interactive mode'

-- empty.txt --
```

**testdata/scripts/search_edge_cases.txtar** — search contract details from report §2
```
# No results exact format: header + blank line + two-space indent "(no results)"
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search zzzznotfound
cmp stdout golden-no-results.txt

# Multiple words joined by spaces
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search WebSocket example content
stdout 'Results for "WebSocket example content":'

# Normalized match: spaces/dashes ignored (e.g., "cookiejar" matches "cookie-jar" or slug)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search cookiejar
stdout 'k6-http:'

# Trailing blank line after every group; bare javascript-api excluded from grouping
# (golden verifies exact format — no "javascript-api:" group header should appear)
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search k6
cmp stdout golden-k6.txt
! stdout 'javascript-api:'

# --list and --all flags not available on search subcommand
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x search --list k6
stderr 'unknown flag'
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x search --all k6
stderr 'unknown flag'

-- golden-no-results.txt --
(generated)
-- golden-k6.txt --
(generated)
```

**testdata/scripts/renderer_search.txtar** — renderer also used for search output in TTY mode
```
# Renderer IS used for search output when TTY
env XDG_CONFIG_HOME=$WORK/cat-cfg
mkdir $WORK/cat-cfg/k6
cp cat-n.yaml $WORK/cat-cfg/k6/docs.yaml
ttyin empty.txt
exec k6-docs --cache-dir $WORK/cache --version v0.55.x search k6
ttyout 'Results for'

-- empty.txt --
-- cat-n.yaml --
renderer: cat -n
```

**testdata/scripts/slug_edge_cases.txtar** — resolution edge cases from report §3
```
# Rule 1: slash in args[0] with multiple args → join all with /
# Args: "javascript-api/k6-http" + "get" → joins to "javascript-api/k6-http/get" → exists.
# This proves Rule 1 joins all args (not just first) when slash present.
exec k6-docs --cache-dir $WORK/cache --version v0.55.x javascript-api/k6-http get
cmp stdout golden-get.txt

# Rule 1: slash in args[1] (not first arg).
# Args: "http" + "k6-http/get" → Rule 1 joins: "http/k6-http/get" (not found).
# Error message uses original args joined by space, so we can't distinguish from Rule 3 by error alone.
# But we CAN verify the error happens (proving something was tried and failed).
# The UNIT test for slash-in-later-arg is in resolve_test.go (kept for this specific case).
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x http k6-http/get
stderr 'topic not found'

# Category matching is case-sensitive: "Using-k6" is NOT a category
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x Using-k6 scenarios
stderr 'topic not found'

# Nonexistent slug after fallback still returns error
! exec k6-docs --cache-dir $WORK/cache --version v0.55.x totally-fake-module nonexistent
stderr 'topic not found'

-- golden-get.txt --
(generated)
```

**testdata/scripts/setup_errors.txtar** — setup() error paths from report §1
```
# Bad sections.json → load index error
! exec k6-docs --cache-dir $WORK/bad-cache --version v0.55.x
stderr 'load index'

-- bad-cache/sections.json --
{bad json
```

### Step 4: Generate golden output

```bash
UPDATE_GOLDEN=1 go test -run TestScripts -count=1
```

This auto-populates every `(generated)` placeholder with actual command output.

### Step 5: Delete old files

- `resolve_test.go` — trim to single Rule 1 slash-in-later-arg unit test (see task 25)
- `testdata/golden/` directory — golden output now inline in `.txtar`
- All Go-based command test infrastructure from `docs_test.go`

- `smoke_test.go` — TTY tests now in `renderer.txtar` using `ttyin`/`ttyout`

### Step 6: Clean up config_test.go

Remove all command-level tests (renderer, debug log, invalid config). Keep only:
- `TestConfigDir` — unit test for configDir()
- `TestCacheDirUSERPROFILE` — unit test for CacheDir()
- `TestLoadConfig` — unit test for loadConfig()

## Coverage Matrix

Every user-facing behavior and its test:

### Main Command (area 1)
| Behavior                                        | Script                                   |
| ----------------------------------------------- | ---------------------------------------- |
| Help text (Short + Long)                        | `help.txtar`                             |
| Search help text                                | `help.txtar`                             |
| No args → TOC                                   | `toc.txtar`                              |
| --list no args → categories (slugs, not titles) | `list.txtar`                             |
| --list with topic → subtopics                   | `list.txtar`                             |
| --all → everything                              | `all.txtar`                              |
| --all ignores --list and args                   | `flags.txtar`                            |
| best-practices → guide                          | `best_practices.txtar`                   |
| best-practices ignores extra args               | `flags.txtar`                            |
| best-practices case-sensitive                   | `flags.txtar` (Best-Practices → error)   |
| --list has no effect on best-practices          | `flags.txtar`                            |
| best-practices missing file → error             | `errors.txtar`                           |
| topic → section content                         | `view_topic.txtar`                       |
| unknown topic → error                           | `errors.txtar`                           |
| bad sections.json → load index error            | `setup_errors.txtar`                     |
| missing markdown → section blank silently       | `view_topic.txtar` (add missing-md test) |
| --version flag overrides env                    | `version_precedence.txtar`               |
| K6_DOCS_VERSION env used                        | `version_precedence.txtar`               |
| K6_DOCS_CACHE_DIR env used                      | `version_precedence.txtar`               |
| --cache-dir flag overrides env                  | `version_precedence.txtar`               |
| --version used VERBATIM (not wildcard-mapped)   | `version_precedence.txtar` (v0.55.2)     |
| K6_DOCS_VERSION used VERBATIM                   | `version_precedence.txtar` (v0.55.2)     |
| --version/--cache-dir inherited by search       | `flags.txtar`                            |
| Auto-detect version (no flag, no env)           | `auto_detect.txtar`                      |
| First-run download                              | `cache_test.go` (Go, requires HTTP mock) |

### Search (area 2)
| Behavior                                   | Script                                                   |
| ------------------------------------------ | -------------------------------------------------------- |
| Title/description match                    | `search.txtar` (Scenarios)                               |
| Description match + JS API module grouping | `search.txtar` (GET request)                             |
| Multi-group sorted alphabetically          | `search.txtar` (k6)                                      |
| Body content match                         | `search.txtar` (WebSocket)                               |
| No results with header                     | `search.txtar` + `search_edge_cases.txtar`               |
| Missing arg → error                        | `search.txtar`                                           |
| Multiple words joined                      | `search_edge_cases.txtar`                                |
| Normalized match (dashes/spaces stripped)  | `search_edge_cases.txtar` (cookiejar)                    |
| No results exact format (golden)           | `search_edge_cases.txtar` (golden)                       |
| Trailing blank line after groups           | `search_edge_cases.txtar` (golden)                       |
| Debug log NOT emitted for search           | `debug_log.txtar`                                        |
| --list/--all not available on search       | `search_edge_cases.txtar`                                |
| bare javascript-api excluded from grouping | `search_edge_cases.txtar` (`! stdout 'javascript-api:'`) |
| Renderer used for search in TTY            | `renderer_search.txtar` (ttyin)                          |

### Topic Resolution (area 3)
| Behavior                                    | Script                                                           |
| ------------------------------------------- | ---------------------------------------------------------------- |
| Full slug with /                            | `slug_resolution.txtar`                                          |
| Rule 1: slash in args[0] with multiple args | `slug_edge_cases.txtar`                                          |
| Rule 1: slash in later arg (args[1]+)       | `resolve_test.go` (unit — error message can't distinguish rules) |
| Category prefix (case-sensitive)            | `slug_resolution.txtar`                                          |
| Category case-sensitive: wrong case → error | `slug_edge_cases.txtar`                                          |
| JS API shortcut                             | `slug_resolution.txtar`                                          |
| Case insensitive lookup                     | `slug_resolution.txtar`                                          |
| k6- prefix dedup                            | `slug_resolution.txtar`                                          |
| Parent prefix fallback                      | `slug_resolution.txtar`                                          |
| Bare name prefers unprefixed                | `slug_resolution.txtar`                                          |
| Nonexistent slug after fallback → error     | `slug_edge_cases.txtar`                                          |

### Rendering (area 7)
| Behavior                                 | Test                        |
| ---------------------------------------- | --------------------------- |
| Non-TTY: renderer configured but SKIPPED | `renderer.txtar` (no ttyin) |
| Non-TTY: no renderer → raw output        | `renderer.txtar`            |
| TTY: renderer IS used                    | `renderer.txtar` (ttyin)    |
| TTY: failing renderer → fallback         | `renderer.txtar` (ttyin)    |
| TTY: missing renderer → fallback         | `renderer.txtar` (ttyin)    |
| TTY: debug log "interactive mode"        | `debug_log.txtar` (ttyin)   |
| Non-TTY: debug log "agent mode"          | `debug_log.txtar`           |

### Configuration (area 8)
| Behavior                                   | Test                                 |
| ------------------------------------------ | ------------------------------------ |
| Valid config via XDG_CONFIG_HOME           | `renderer.txtar`                     |
| Valid config via HOME fallback (TTY proof) | `config.txtar` (ttyin + cat -n)      |
| Invalid config → warn + continue           | `config.txtar`                       |
| Missing config → silent continue           | `renderer.txtar` (no-config section) |
| File unreadable → warn + continue          | `config.txtar` (chmod 000)           |
| HOME+USERPROFILE both unset → warning      | `config.txtar` (stderr check)        |
| configDir(), CacheDir(), loadConfig()      | `config_test.go` (Go unit tests)     |

### Display Formatting (area 10)
| Behavior                                   | How verified                                                                                                                                                                                                                              |
| ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Aligned columns                            | golden file exact match in `toc.txtar`, `list.txtar`, `search.txtar`                                                                                                                                                                      |
| Truncation at 80 chars                     | `toc.txtar` golden (websockets `...`)                                                                                                                                                                                                     |
| Deduplication                              | `list.txtar` golden + `! stdout 'Alternate GET'`                                                                                                                                                                                          |
| TOC header with version                    | `toc.txtar` golden                                                                                                                                                                                                                        |
| Subtopics footer (---, names, usage hint)  | `view_topic.txtar` golden                                                                                                                                                                                                                 |
| Child prefix stripping                     | `view_topic.txtar` `stdout 'Subtopics: clear'`                                                                                                                                                                                            |
| slugToArgs (JS API)                        | `view_topic.txtar` golden (`Use: k6 x docs http <subtopic>`)                                                                                                                                                                              |
| slugToArgs (non-JS API)                    | `view_topic.txtar` (`Use: k6 x docs using-k6 <subtopic>`)                                                                                                                                                                                 |
| Frontmatter stripped                       | `view_topic.txtar` `! stdout '---'`, `! stdout 'title:'`                                                                                                                                                                                  |
| Zero-children category                     | `toc.txtar` golden (testing-guides single line)                                                                                                                                                                                           |
| printAll: sections.json order, blank lines | `all.txtar` golden                                                                                                                                                                                                                        |
| printList: title — description header      | `list.txtar` golden                                                                                                                                                                                                                       |
| printList: (no subtopics) exact format     | `list.txtar` golden                                                                                                                                                                                                                       |
| Missing markdown → blank silently          | `view_topic.txtar`                                                                                                                                                                                                                        |
| Byte-based truncation (non-ASCII)          | Requires non-ASCII fixture data in testdata/cache/sections.json. Add a section with a description containing multi-byte characters (e.g., Japanese) > 80 bytes. The golden file in `toc.txtar` will capture the mid-character truncation. |

### Other areas (kept as Go unit tests)
| Area                        | Test file                                                                   |
| --------------------------- | --------------------------------------------------------------------------- |
| Caching (area 6)            | `cache_test.go` — EnsureDocs, extract, security, multi-version, permissions |
| Version mapping (area 5)    | `version_test.go` — MapToWildcard, DetectK6Version                          |
| Categories (area 4)         | `categories_test.go` — IsIncludedDocsPath, isCategory                       |
| Transform pipeline (area 9) | `transform_test.go` — all 13 steps                                          |
| Sections index (area 10)    | `sections_test.go` — LoadIndex, Lookup, Search, Children                    |
| cmd/prepare (area 11)       | `cmd/prepare/main_test.go` — run(), ensureDocsRepo, pipeline                |

### Behaviors tested only at Go unit level (not command-level testable)

These need dedicated Go unit tests in their respective files. Do NOT mix unit and integration tests in the same file — testscript handles all integration tests.

| Behavior                                   | Test file          | Status                                       |
| ------------------------------------------ | ------------------ | -------------------------------------------- |
| "detect k6 version" error wrapping         | `version_test.go`  | Already covered by `TestDetectK6Version`     |
| "ensure docs" HTTP error wrapping          | `cache_test.go`    | Already covered by `TestEnsureDocsHTTPError` |
| Non-regular tar entries silently skipped   | `cache_test.go`    | **NEW** — add test with symlink in tar       |
| IsCached returns false on CacheDir failure | `cache_test.go`    | **NEW** — add test with empty env            |
| Search groupSlug double condition          | `sections_test.go` | **NEW** — add test with specific fixture     |

## Verification

```bash
go test ./... -v -count=1          # all tests pass
go test -run TestScripts -v         # see all script names
go test -run TestScripts/^toc$ -v   # run single script
golangci-lint run                   # clean
UPDATE_GOLDEN=1 go test -run TestScripts  # regenerate golden output
```
