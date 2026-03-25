# Add shell completions for `k6 x docs`

This Plan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.claude/PLANS.md`.


## Purpose / Big Picture

After this change, users can press Tab to auto-complete topic names, subcommands, and flags when using `k6 x docs`. Instead of guessing or remembering slug paths like `http get` or `using-k6/scenarios`, the shell suggests them. This works in bash, zsh, fish, and PowerShell via k6's built-in `k6 completion` command — no separate subcommand needed.

To see it working: type `k6 x docs <TAB>` and see a grid of categories and JS API module shortcuts. Type `k6 x docs http <TAB>` and see `get`, `post`, `cookiejar`, etc.


## Progress

- [x] Milestone 1: Core completion logic (`completion.go` + `completion_test.go`)
- [x] Milestone 2: Wire completions into cobra commands (`cmd.go` changes)
- [x] ~~Milestone 3: `completions` subcommand~~ — Removed (see Decision Log)
- [x] ~~Milestone 4: Integration tests for completions subcommand~~ — Removed with M3
- [x] Milestone 5: Update AGENTS.md


## Surprises & Discoveries

- Parallel subtests sharing a single cobra command cause races via `SetOut()` — each subtest needs its own command instance.
- `cobra.Completion` is a type alias for `string` in cobra v1.10.2, so `cobra.Completion(value)` and `string(c)` are unnecessary conversions flagged by `unconvert`.
- k6 1.7.0 auto-provisioning (PR #5664) does NOT intercept `__complete` — completions silently return nothing when the extension isn't compiled into the binary. This is acceptable: no error, just no completions until the user builds with xk6 or the provisioned binary is used directly.
- k6 already provides `k6 completion [bash|zsh|fish|powershell]` via cobra's built-in completion command. Extensions that register `ValidArgsFunction` piggyback on this automatically — no separate subcommand needed. Milestones 3 and 4 were built, tested, then removed after discovering this.
- Completions with descriptions cause zsh to dump a long vertical list instead of a compact grid. When a topic has 50+ children (e.g. browser/frame), the list overflows the terminal and the prompt redraws below. Removing descriptions lets zsh display completions in a grid.
- `gocritic` flags `strings.HasPrefix("best-practices", prefix)` as reversed args (constant as first arg looks suspicious). Fix: assign the constant to a local variable first.
- Naming consistency: all internal function names use "completion" (noun), never "complete" (verb). E.g. `completionTopicArgs`, `completionFirstArg`, `completionDeeper`, `completionDirs`, `newTopicCompletion`.


## Decision Log

- Decision: Use cobra's built-in `ValidArgsFunction` instead of generating custom shell scripts.
  Rationale: Cobra already handles the shell-specific plumbing. When `k6 completion zsh` is sourced, typing `k6 x docs <TAB>` runs `k6 __completeNoDesc x docs ""` under the hood, which invokes our `ValidArgsFunction`. One Go function serves all four shells. No custom shell scripting needed.
  Date/Author: 2026-03-24

- Decision: The completion function must not trigger network downloads. If docs are not cached yet, it returns no completions silently.
  Rationale: Tab completion must be fast (under 100ms). The `setup()` function in `cmd.go` calls `EnsureDocs()` which can download a multi-MB bundle on first run. The completion path needs a lighter code path that only reads the local cache. If the cache doesn't exist, the user gets no completions until they run `k6 x docs` once (which triggers the download).
  Date/Author: 2026-03-24

- Decision: First-arg completions include both categories and JS API module shortcuts.
  Rationale: Users type `k6 x docs http get`, not `k6 x docs javascript-api k6-http get`. The completion list at position 0 should mirror what the CLI actually accepts: category names (`using-k6`, `examples`, ...) plus short JS API module names (`http`, `browser`, `crypto`, ...) and the special keyword `best-practices`. This matches `ResolveWithLookup` behavior.
  Date/Author: 2026-03-24

- Decision: Remove the `completions` subcommand. Rely on k6's built-in `k6 completion` command.
  Rationale: k6 already provides `k6 completion [bash|zsh|fish|powershell]` via cobra. Extensions that register `ValidArgsFunction` automatically get completions through this mechanism. A separate `k6 x docs completions` subcommand was redundant — it generated the same scripts as `k6 completion`. The subcommand was built (Milestones 3 & 4), tested, then removed after manual testing revealed it wasn't needed.
  Date/Author: 2026-03-24

- Decision: Completions return plain names without descriptions.
  Rationale: Descriptions cause zsh to display completions as a long vertical list (one per line). For topics with many children (e.g. browser/frame with 50+ methods), this overflows the terminal and creates a poor UX — the prompt redraws below the list. Without descriptions, zsh displays completions in a compact grid. The descriptions are available via `k6 x docs <topic>` anyway.
  Date/Author: 2026-03-24


## Known Limitations

**k6 1.7.0 auto-provisioning does not support shell completions.**

k6 1.7.0 (PR #5664) can auto-provision extensions without xk6. When a user runs `k6 x docs`, k6 detects the missing extension, calls a remote build service, downloads a provisioned binary with xk6-docs compiled in, and spawns it as a child process. This works for normal commands.

However, shell completions do not trigger provisioning. When the user presses Tab, the shell runs `k6 __completeNoDesc x docs ""`. Cobra's hidden `__completeNoDesc` command resolves the command tree without calling `RunE` — and it's `RunE` on the `x` command that detects missing extensions and triggers provisioning. Since `__completeNoDesc` bypasses `RunE`, the stock k6 binary (without xk6-docs) handles the completion request and returns nothing for `docs` topics.

Impact: Users who rely on auto-provisioning get no tab completions. No error — just silence on Tab press.

Fix required in k6 core: k6 needs to intercept `__complete`/`__completeNoDesc` the same way it intercepts regular commands — detect the unregistered extension, provision if needed, and forward the `__complete` call to the provisioned binary. The provisioned binary is already cached from the first `k6 x docs` run, so the Tab-press latency would just be the subprocess spawn.

Workaround for users: build with xk6 (`xk6 build --with xk6-docs`) so the extension is compiled directly into the binary.


## Outcomes & Retrospective

**What was delivered:**
- `completion.go` — `setupForCompletion`, `completionTopicArgs`, `completionFirstArg`, `completionDeeper`, `newTopicCompletion`, `completionDirs`
- `completion_test.go` — 10 test cases covering all arg positions, filtering, nil index, directives
- `cmd.go` changes — `ValidArgsFunction` on docs/search/skill commands, flag completions for version/cache-dir/depth
- `AGENTS.md` — documented the completions feature

**What was removed after initial implementation:**
- `completions.go` — subcommand was redundant with `k6 completion`
- `completions_test.go` — tests for the removed subcommand
- `testdata/scripts/completions.txtar` — integration tests for the removed subcommand

**Key learnings:**
- Check what the host CLI already provides before building convenience wrappers. k6's built-in `k6 completion` command made our `completions` subcommand unnecessary.
- Test with real shell interaction early. The description overflow issue and the discovery that completions already worked (via `k6 completion zsh`) only surfaced during manual testing with `xk6 build`.
- Cobra completions without descriptions display as a grid in zsh, which is the better UX for large result sets.


## Context and Orientation

This is an extension for k6 (the load testing tool). It registers as `k6 x docs` via cobra, providing offline documentation browsing. The extension lives in the `docs` Go package at the repository root.

Key files and their roles:

- `register.go` — Calls `subcommand.RegisterExtension("docs", newCmd)` in `init()`. This is the only file allowed to have `init()`.
- `cmd.go` — Builds the cobra command tree: root `docs` command + `search` and `skill` subcommands. Contains `newDocsCmd()`, `prepareRun()`, `setup()`, and the `docsOpts` struct. The `setup()` function resolves version, ensures docs are cached (potentially downloading), and loads the index.
- `completion.go` — Completion logic. `setupForCompletion` is a lightweight `setup()` that skips network I/O. `completionTopicArgs` computes completions based on arg position. `newTopicCompletion` returns a closure for `ValidArgsFunction`.
- `sections.go` — Defines `Section` and `Index` types. `LoadIndex()` reads `sections.json` from a cache directory into memory. `Index` has `Lookup(slug)`, `Children(slug)`, `TopLevel()`, and `Search()` methods.
- `resolve.go` — `ResolveWithLookup(args, exists)` converts CLI args into canonical slugs. `normalizeArgs()` splits slash-separated segments. Fallbacks: `withK6Prefix` inserts `k6-` on JS API slugs, `withParentFallback` prepends parent name to last segment.
- `docs.go` — `showDocs()`, `printSearch()`, display logic. `slugToArgs()` converts slugs back to CLI args (strips `javascript-api/` prefix and `k6-` prefix). `childName()` extracts short child names.
- `categories.go` — `docCategories()` returns all 13 category names. `isCategory()` checks membership.
- `cache.go` — `CacheDir()` returns the cache path for a version. `IsCached()` checks if a version is cached. `EnsureDocs()` downloads if needed.
- `version.go` — `DetectK6Version()` reads from Go build info. `MapToWildcard()` converts `v1.5.0` to `v1.5.x`.
- `config.go` — `HomeDir()` returns the user's home directory.

The cobra command tree looks like:

    k6 (root, owned by k6 core — provides `k6 completion` for shell completions)
      x (subcommand group, owned by k6 core)
        docs [topic] [subtopic...]     ← our extension (ValidArgsFunction for tab completion)
          search <term>                ← same ValidArgsFunction
          skill [directory]            ← completes directories

Testing uses two approaches: (1) `testscript` `.txtar` files in `testdata/scripts/` for CLI integration tests, driven by `TestScripts` in `docs_test.go`, which registers a `k6-docs` command that runs the extension in-process; (2) standard Go unit tests for logic functions. The TDD workflow requires a failing test before any implementation code.

Cobra's completion system works as follows. When a user presses Tab, the shell runs `k6 __completeNoDesc x docs <typed-args>`. Cobra has a hidden `__completeNoDesc` command that walks the command tree, finds the target command, and calls its `ValidArgsFunction`. The function signature is:

    func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective)

Where `args` are the already-completed positional arguments and `toComplete` is the partial word being typed. The function returns a list of completions and a directive bitmask. `cobra.ShellCompDirectiveNoFileComp` suppresses filename fallback.


## Plan of Work

The work is split into two milestones (Milestones 3-4 were removed after discovering the `completions` subcommand was redundant).


### Milestone 1: Core completion logic

Create `completion.go` with pure functions that compute completions given an index and the current arg state. No side effects, no network access — only reads from the in-memory `Index`.

Functions:

`setupForCompletion` — a lightweight version of `setup()` that skips network I/O. Resolves version (flag > env > auto-detect) and cache dir (flag > env > default), checks if cache exists, loads the index. Returns `nil` on any failure — completions silently degrade.

`completionTopicArgs` — the core logic:
- Zero args: returns categories + JS API module shortcuts + `best-practices`, filtered by `toComplete` prefix.
- One or more args: resolves via `ResolveWithLookup`, returns children's short names (via `childName`), filtered by `toComplete` prefix, deduplicated.
- All completions are plain names without descriptions (for grid display in shells).
- Always returns `cobra.ShellCompDirectiveNoFileComp`.

`completion_test.go` — 10 test cases covering all arg positions, filtering, nil index, directives.


### Milestone 2: Wire completions into cobra commands

Modify `cmd.go`:
- Set `ValidArgsFunction` on `docs` and `search` commands (shared via `newTopicCompletion` closure).
- Set `ValidArgsFunction` on `skill` command to complete directories.
- Register flag completions: `--version` (no file), `--cache-dir` (directories), `--depth` (no file).


### Milestone 5: Update AGENTS.md

Document the completions feature: `ValidArgsFunction` provides dynamic topic completion, piggybacks on k6's built-in `k6 completion` command, requires cached docs.


## Validation and Acceptance

**Unit tests:**

Run `go test -v ./...` from the repo root. All tests pass, including:
- `TestCompleteTopicArgs` — verifies completion logic for all arg positions, filtering, nil index, deduplication, directives

**Linter:**

Run `golangci-lint run`. No errors.

**Manual verification:**

Build:

    xk6 build --with xk6-docs=.

Test dynamic completions (no descriptions):

    ./k6 __completeNoDesc x docs ""

Expected: one topic per line (categories + JS API shortcuts + `best-practices`), no tab-separated descriptions, followed by `:4` directive.

    ./k6 __completeNoDesc x docs http ""

Expected: children of `javascript-api/k6-http` (get, post, cookiejar, etc.), no descriptions.

    ./k6 __completeNoDesc x docs browser frame ""

Expected: children of `javascript-api/k6-browser/frame` (check, click, content, ...), no descriptions, displayed as grid in zsh.


## Idempotence and Recovery

All steps are safe to repeat. Tests are deterministic and use in-memory filesystems or testscript sandboxes. No network calls, no mutations to the real filesystem.


## Interfaces and Dependencies

**New files:**

- `completion.go` — completion logic, no external dependencies beyond cobra types
- `completion_test.go` — unit tests for completion logic

**Modified files:**

- `cmd.go` — wire `ValidArgsFunction` on commands, register flag completions
- `AGENTS.md` — document the new feature

**Key function signatures:**

In `completion.go`:

    func setupForCompletion(gs *state.GlobalState, opts *docsOpts) *Index
    func completionTopicArgs(idx *Index, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective)
    func newTopicCompletion(gs *state.GlobalState, opts *docsOpts) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective)
    func completionDirs(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective)

**Dependencies:** No new dependencies. Uses `github.com/spf13/cobra` (already imported) and existing package functions (`docCategories`, `isCategory`, `childName`, `ResolveWithLookup`, `LoadIndex`, `IsCached`, `CacheDir`, `DetectK6Version`, `MapToWildcard`).
