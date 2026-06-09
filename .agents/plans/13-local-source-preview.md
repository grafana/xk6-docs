# 13 — Preview local k6-docs with `--source`

## Why this matters

Today a documentation author editing the `k6-docs` repository cannot see how
their in-progress page looks through `k6 x docs` without a multi-step dance:
run the standalone `cmd/prepare` tool to transform the repo into a bundle, copy
the output into a version-named cache directory by hand, build a `k6` binary,
and finally run `k6 x docs --cache-dir ... --version ...`. That is too much
friction for someone who just wants to preview an edit.

After this change, an author runs a single command:

    k6 x docs --source ~/grafana/k6-docs --version next using-k6 feature-flags

`k6 x docs` reads the markdown straight from that local working tree, applies
the exact same transform the published bundles use (Hugo shortcodes stripped,
admonitions converted, links cleaned), and renders the page. Editing the file
and re-running reflects the change immediately. No download, no manual cache
setup, no separate build of a docs bundle.

"Working tree" means the author's checked-out `k6-docs` repository directory,
including any uncommitted edits — the tool reads files as they are on disk.

## Background a newcomer needs

This repository builds `k6 x docs`, a terminal command that prints offline k6
documentation. Docs are NOT embedded in the binary. Normally, on first run the
binary detects the k6 version, downloads a compressed bundle from GitHub, and
caches it under `~/.local/share/k6/docs/{version}/`. A bundle directory holds
`sections.json` (the index of every page) and a `markdown/` subtree (the
transformed page text). The code that reads a bundle and serves pages is the
`docs.Catalog` type in `docs/catalog.go`.

A `Catalog` has a "local-only" mode: when constructed with
`docs.WithCacheDir(base)` and `docs.WithLocalOnly()`, it never touches the
network and reads bundles directly from `{base}/{version}/sections.json` and
`{base}/{version}/markdown/`. The CLI already exposes this via the `--cache-dir`
flag (see `internal/cli/bundle.go`, function `setup`). This plan reuses that
exact mechanism.

The transform-and-index pipeline that turns a raw `k6-docs` checkout into a
bundle currently lives in the standalone tool `cmd/prepare/main.go`, inside
package `main`. Because it is `package main`, no other package can import it.
The `run` function there does six steps: ensure the repo is present (clone if a
path was not given), locate the version directory under
`docs/sources/k6/{version}/`, build a map of shared content, walk the markdown
files transforming each one, populate parent/child relationships, and write
`sections.json` plus `markdown/` (including an embedded `best_practices.md`).

To reuse that pipeline inside `k6 x docs`, we lift everything except the clone
step into a new importable package, `internal/bundle`. The standalone tool
keeps the clone step and calls the new package. The CLI calls the new package
to build a bundle from the author's `--source` directory into a scratch
directory, then serves it local-only.

## What the user can do after this change

Run, from anywhere, against a local k6-docs checkout:

    k6 x docs --source /path/to/k6-docs --version next using-k6 feature-flags
    k6 x docs --source /path/to/k6-docs --version next search "feature flag"

The first prints the rendered `using-k6/feature-flags` page built from the
local tree. The second searches the local tree. Omitting `--version` uses the
k6 version detected from the binary's build info (useful for previewing edits
to the current version's docs rather than `next`).

The flag also works without a value-less form: `--source` always takes the
path to the k6-docs repository root (the directory that contains
`docs/sources/k6/`).

## Design decisions (resolved here, not left to the reader)

Invocation is a flag, `--source`, not an environment variable. Decided with the
user: a flag is discoverable in `--help` and in shell completions.

The feature ships in the released extension, not as a dev-only convenience.

Reuse the published transform pipeline rather than reading raw markdown. If the
tool simply printed raw files, shortcodes and admonitions would render as
literal `{{< ... >}}` noise and would not match what readers see online. Reuse
guarantees the preview equals production.

Build into a scratch directory on disk and serve it local-only, rather than
building an in-memory filesystem. The pipeline already writes through the
`docs.FS` filesystem abstraction and walks the source with the real OS, and the
local-only Catalog path is already battle-tested. Writing to a temp directory
reuses both with the least new code.

The scratch directory is deterministic per source path, not a fresh random temp
dir each run: `{os.TempDir()}/xk6-docs-source/{hash-of-abs-source-path}`, with
the version as a subdirectory inside it. Before each build the version
subdirectory is removed and rebuilt, so edits (including deletions) are always
reflected and stale files never linger. There is no cleanup-on-exit: the
scratch directory is small, bounded to one per source-path-and-version, and a
human previewing docs expects scratch output to persist between runs much like
the manual flow it replaces. This avoids entangling temp-dir lifetime with
cobra's command lifecycle (`setup` is called more than once per invocation: in
`PersistentPreRunE` for the agent guide, in the run handler, and in the help
function). A deterministic rebuilt-in-place directory is idempotent across all
those calls.

Shell completions do NOT honor `--source`. Completion runs as a separate
process on every TAB; rebuilding the whole `k6-docs` tree on each keystroke
would be unacceptably slow. Completions continue to read the normal version
cache. A brand-new local-only page therefore will not appear in TAB
suggestions, but the page still renders when typed and run. This tradeoff is
acceptable for a preview feature and is the minimal, safe behavior.

## Repository orientation

Files you will touch or create, with full paths:

New package `internal/bundle`:

  - `internal/bundle/bundle.go` — the lifted pipeline. Exports `Build` and
    `NewOSFS`. Contains the previously-unexported helpers.
  - `internal/bundle/best_practices.md` — moved from `cmd/prepare/`. The
    `//go:embed` directive must sit next to the file it embeds.

Modified:

  - `cmd/prepare/main.go` — keep flag parsing, `ensureDocsRepo`, `mkTempDir`.
    Replace the inline pipeline with a call to `bundle.Build`. Drop now-unused
    imports (`encoding/json`, `sort`, `embed`, `gopkg.in/yaml.v3`) and the
    moved functions, the `frontmatter` type, and the `bestPracticesContent`
    embed.
  - `cmd/prepare/fs.go` — delete. Its `osFS` is replaced by `bundle.NewOSFS`.
  - `internal/cli/cmd.go` — add `source` to `docsOpts`; register the `--source`
    persistent flag with directory completion; pass `opts.source` to `setup`
    at all three call sites (the `PersistentPreRunE`, `newWithDocs`, and
    `setHelpFunc`).
  - `internal/cli/bundle.go` — `setup` gains a `sourceFlag string` parameter.
    When non-empty, build a bundle from the source tree into the deterministic
    scratch directory and use that as the local-only cache base.

Test fixtures and scripts:

  - `internal/clitest/testdata/scripts/source.txtar` — new testscript that
    embeds a tiny k6-docs source tree and asserts `--source` renders a page
    from it.

Unchanged but relevant: `docs/catalog.go` (local-only Catalog), `docs/fs.go`
(its own internal `osFS`, separate from the new `bundle.NewOSFS`),
`internal/cli/completion.go` (`setupForCompletion` is deliberately left alone).

## The extraction (no behavior change)

Move these from `cmd/prepare/main.go` into `internal/bundle/bundle.go`, keeping
their logic byte-for-byte: the `frontmatter` type, `buildSharedContentMap`,
`parseFrontmatter`, `deduplicateYAMLKeys`, `slugFromRelPath`, `categoryFromSlug`,
`walkAndProcess`, `processEntry`, `resolveAliases`, `populateChildren`,
`writeSectionsJSON`, `writeBestPractices`, and the `bestPracticesContent`
embed. Lower-case (unexported) helpers stay unexported inside the new package.

Add the package's public entry point. Its body is exactly steps 2 through 6 of
the current `run` (everything after the repo is located):

    // Build transforms the k6-docs working tree at k6DocsPath into a bundle
    // (sections.json + markdown/) under outputDir, for the given version.
    func Build(k6Version, k6DocsPath, outputDir string, afs docs.FS, stderr io.Writer) error {
        docsVersion := docs.VersionWildcard(k6Version)
        versionRoot := filepath.Join(k6DocsPath, "docs", "sources", "k6", docsVersion)
        if _, err := afs.Stat(filepath.Clean(versionRoot)); err != nil {
            return fmt.Errorf("version root not found: %w", err)
        }
        sharedDir := filepath.Join(versionRoot, "shared")
        sharedContent, err := buildSharedContentMap(afs, sharedDir)
        if err != nil {
            return fmt.Errorf("build shared content: %w", err)
        }
        markdownDir := filepath.Join(outputDir, "markdown")
        sharedRel, _ := filepath.Rel(versionRoot, sharedDir)
        sections, err := walkAndProcess(afs, versionRoot, markdownDir, sharedContent, filepath.ToSlash(sharedRel))
        if err != nil {
            return fmt.Errorf("walk docs: %w", err)
        }
        populateChildren(sections)
        idx := &docs.Index{Version: k6Version, Sections: sections}
        if err := writeSectionsJSON(afs, outputDir, idx); err != nil {
            return err
        }
        return writeBestPractices(afs, outputDir)
    }

Add an exported OS filesystem so callers outside the package can supply the
real filesystem. This mirrors the existing `cmd/prepare/fs.go` `osFS` exactly,
just exported via a constructor:

    func NewOSFS() docs.FS { return osFS{} }

(with the same `osFS` method set as `cmd/prepare/fs.go` had).

Note that `writeSectionsJSON` and `writeBestPractices` currently call
`log.Printf("Wrote %s", ...)`. Those log lines are cosmetic progress from the
standalone tool. Keep them as-is for the standalone tool. They write to the
default logger (stderr). When the CLI calls `Build`, this would leak "Wrote ..."
lines to the user's stderr. To avoid that, the CLI build wrapper redirects the
standard logger is overkill; instead these two functions should log through the
passed `stderr` writer rather than the global logger. Change both to take no
new parameter but write via `fmt.Fprintf(... )` only when a logger is wired —
the simplest correct approach: replace the `log.Printf` calls with writes to a
package-level no-op by default. Concretely, give `Build` ownership: have
`writeSectionsJSON` and `writeBestPractices` stop logging entirely (remove the
two `log.Printf` lines) and instead have the standalone `cmd/prepare` `run`
print its own "Done" line as it already does. This removes the only reason the
pipeline imported `log`, keeps stderr clean for the CLI, and loses nothing the
user relies on.

After extraction, update `cmd/prepare/main.go` `run` to:

    func run(k6Version, k6DocsPath, outputDir string, afs docs.FS, stderr io.Writer) error {
        docsPath, cleanup, err := ensureDocsRepo(k6DocsPath, defaultRepoURL, afs, stderr)
        if err != nil {
            return err
        }
        if cleanup != nil {
            defer cleanup()
        }
        if err := bundle.Build(k6Version, docsPath, outputDir, afs, stderr); err != nil {
            return err
        }
        _, _ = fmt.Fprintln(stderr, "Done: sections written")
        return nil
    }

and `main` to construct the filesystem via `bundle.NewOSFS()` (deleting
`cmd/prepare/fs.go`).

Acceptance for the extraction milestone: `make build` succeeds, and the
existing prepare testscripts pass unchanged:

    go test ./cmd/prepare/...

These scripts (`cmd/prepare/testdata/scripts/*.txtar`) exercise the binary
end-to-end, so a green run proves the pipeline still produces identical output.

## Wiring `--source` into the CLI

In `internal/cli/cmd.go`, add the field and flag:

    type docsOpts struct {
        version  string
        source   string
        cacheDir string
        depth    int
        pager    bool
        width    int
    }

In `registerFlags`:

    cmd.PersistentFlags().StringVar(&opts.source, "source", "",
        "Build docs from a local k6-docs checkout instead of downloading")
    _ = cmd.RegisterFlagCompletionFunc("source", completionDirs)

Pass `opts.source` to `setup` at the three call sites (search for `setup(`):
the `PersistentPreRunE` in `buildDocsCmd`, the closure in `newWithDocs`, and
`setHelpFunc`.

In `internal/cli/bundle.go`, change `setup`'s signature to accept the source
and build when present. The new parameter slots in after `versionFlag`:

    func setup(
        ctx context.Context,
        env map[string]string,
        logf func(string, ...any),
        fs FS,
        versionFlag, sourceFlag, cacheDirFlg string,
    ) (denv *docsEnv, err error) {

After the version is resolved and wildcarded (the existing block ending with
`version = docs.VersionWildcard(version)`), insert source handling so it
overrides the cache base:

    explicitDir := cmp.Or(cacheDirFlg, env["K6_DOCS_CACHE_DIR"])
    if sourceFlag != "" {
        explicitDir, err = buildSourceBundle(fs, sourceFlag, version)
        if err != nil {
            return nil, err
        }
    }
    base := cmp.Or(explicitDir, baseCacheDir(env))

Because `explicitDir` is now non-empty, the existing `catalogOpts(env, base,
explicitDir != "")` call already selects `WithLocalOnly`, and the existing
"Downloading..." branch (guarded by `explicitDir == ""`) is correctly skipped.

Add the helper in the same file:

    // buildSourceBundle transforms a local k6-docs checkout into a bundle under
    // a deterministic scratch directory and returns that directory as a
    // local-only cache base. The version subdirectory is rebuilt every call so
    // edits are always reflected.
    func buildSourceBundle(fs FS, source, version string) (string, error) {
        abs, err := filepath.Abs(source)
        if err != nil {
            return "", fmt.Errorf("resolve source path: %w", err)
        }
        h := fnv.New32a()
        _, _ = h.Write([]byte(abs))
        base := filepath.Join(os.TempDir(), "xk6-docs-source", strconv.FormatUint(uint64(h.Sum32()), 16))
        out := filepath.Join(base, version)
        if err := fs.RemoveAll(out); err != nil {
            return "", fmt.Errorf("clear scratch dir: %w", err)
        }
        if err := bundle.Build(version, abs, out, bundle.NewOSFS(), io.Discard); err != nil {
            return "", fmt.Errorf("build docs from source: %w", err)
        }
        return base, nil
    }

New imports for `internal/cli/bundle.go`: `hash/fnv`, `io`, `os`, `strconv`,
and `github.com/grafana/xk6-docs/internal/bundle`. (`filepath`, `cmp`, `fmt`
are already imported.) Using `os.TempDir` directly here is acceptable: building
from source inherently touches the real OS (the pipeline walks the source tree
with `filepath.WalkDir`), so this code path is not subject to the `fs FS`
abstraction the way cache reads are. The `fs.RemoveAll` call still goes through
the injected `FS` so the clear step stays consistent with the rest of `setup`.

## Tests

Test-drive this with a new testscript. The clitest harness
(`internal/clitest/docs_test.go`) builds a real `k6` binary with the extension
and runs every `.txtar` under `internal/clitest/testdata/scripts/`. It provides
a `ptyexec` command that runs the binary with a pseudo-terminal attached so the
human (TTY) rendering path is exercised and the page content is printed (a plain
`exec` is non-TTY and would print the agent guide instead of content).

Create `internal/clitest/testdata/scripts/source.txtar`. It embeds a minimal
k6-docs tree and asserts the rendered page contains a unique marker. The txtar
format places file contents under `-- path --` markers, which testscript
materializes into `$WORK`. Use indentation here to avoid nested code fences;
in the actual file these are normal txtar lines (no leading indentation):

    # Preview a page built from a local k6-docs source tree.
    ptyexec ./k6 x docs --source $WORK/src --version next using-k6 my-feature
    stdout 'UNIQUE_MARKER_TOKEN'

    # Search also works against the local source.
    ptyexec ./k6 x docs --source $WORK/src --version next search 'my-feature'
    stdout 'my-feature'

    -- src/docs/sources/k6/next/_index.md --
    ---
    title: k6
    ---
    Root.

    -- src/docs/sources/k6/next/using-k6/_index.md --
    ---
    title: Using k6
    weight: 10
    ---
    Using k6 overview.

    -- src/docs/sources/k6/next/using-k6/my-feature.md --
    ---
    title: My Feature
    weight: 20
    ---
    UNIQUE_MARKER_TOKEN describes the feature.

Run it:

    go test ./internal/clitest/ -run TestScripts

Before wiring `--source`, this test fails because the flag is unknown (cobra
errors "unknown flag: --source"). After wiring, it passes. That is the failing-
then-passing TDD checkpoint.

Also confirm nothing else regressed:

    make test
    make lint

## End-to-end manual proof

After building (`make build`), against the user's real checkout on the
feature-flags branch:

    ./k6 x docs --source /Users/inanc/grafana/k6-docs --version next using-k6 feature-flags

Expected: the rendered "Feature flags" page (a styled header reading
"Feature flags" followed by the page body). Editing
`/Users/inanc/grafana/k6-docs/docs/sources/k6/next/using-k6/feature-flags.md`
and re-running reflects the edit with no other steps.

Note for the runner: piping the output (for example through `head`) trips the
non-TTY agent-guide path, which prints navigation instructions instead of the
page. Run it bare in a terminal, or via the test harness's `ptyexec`, to see the
rendered page.

## Documentation updates

Per the project rules, after the feature works:

  - `CLAUDE.md` — document the `--source` flag under the TTY/browsing section
    and note completions do not honor it.
  - `.agents/features.md` — add the user-facing `--source` preview feature.
  - `README.md` — add a short "Preview local docs" note for contributors.
  - `internal/cli/skills/xk6-docs/SKILL.md` — only if agent guidance changes;
    `--source` is a human preview feature, so likely no skill change. Confirm
    by reading the skill before deciding.

## Progress

- [x] Milestone 1: extract pipeline into `internal/bundle`; `cmd/prepare`
      delegates; `go test ./cmd/prepare/...` green; `make build` succeeds.
- [x] Milestone 2: add failing `source.txtar`; confirmed it failed on unknown flag.
- [x] Milestone 3: wire `--source` through `setup`; `source.txtar` passes
      (render from source, cross-category search, edit reflection).
- [x] Milestone 4: `make test` green; `golangci-lint` (project gate) clean;
      manual proof against the real k6-docs feature-flags branch confirmed.
      Note: `xk6 lint`'s gosec (G304 on the OS filesystem boundary) and
      govulncheck findings pre-exist on `main` and are unchanged by this work
      (the G304 pair simply moved from the deleted `cmd/prepare/fs.go` into
      `internal/bundle/bundle.go`).
- [x] Milestone 5: documentation updated — `AGENTS.md` (symlinked as CLAUDE.md),
      `.agents/features.md`, `README.md`. SKILL.md intentionally unchanged:
      `--source` is an author/contributor feature; agents read docs straight
      from the filesystem and do not use CLI flags.

## Decision log

- Flag over env var, and ship in released extension: chosen by the user.
- Completions ignore `--source`: rebuilding the tree per TAB keystroke is too
  slow; renders-when-typed is enough for a preview feature.
- Scratch dir moved from the temp dir to the doc cache base, under a hidden
  `.sources/{hash-of-abs-source}/` subdir (user request: keep builds in the
  standard `~/.local/share/k6/docs` location without affecting normal usage).
  Hidden from version discovery because `versionDirRe` matches only `vX.Y.x`.
  Hashing the absolute source path isolates the multiple k6-docs checkouts a
  user may point at.
- Skip rebuild when unchanged (user request): `bundle.SourceStamp` digests the
  source `.md` files (path, size, mtime); the build runs only when the stamp
  differs from `{version}.stamp` or the prior build is missing. Replaces the
  earlier rebuild-every-run approach. A stamp error (e.g. missing version dir)
  falls through to `bundle.Build` so its "version root not found" error wins.
- With `--source`, `--version` defaults to `next` (user request): authors edit
  in-development docs, so defaulting to the built k6 version was the wrong
  target. Explicit `--version`/`K6_DOCS_VERSION` still overrides. Documented in
  the command's `--help` Long text, README, and AGENTS.md.
- Skip behavior is validated by a scripted test, not a unit test (user
  preference for behavioral testscripts over internal unit tests). To make skip
  observable through the binary, `buildSourceBundle` logs `Building k6 <version>
  docs from <path>...` (mirrors the existing "Downloading..." message) only when
  it actually builds; `source.txtar` asserts the message appears on first build
  and after an edit, and is absent on an unchanged rerun.
- Plan stored in `.agents/plans/` to match the existing twelve plans; the
  `.agents/plans.md` guide text says `.claude/plans/`, but the repository's
  actual plans live in `.agents/plans/`. Followed reality.
