# Plan: `k6 x docs skill` command + staleness check + nightly CI

## Problem
After `npx skills add grafana/xk6-docs`, agents have no binary to run `./k6 x docs`.
Users may not have xk6-docs compiled into their k6. Doc bundles can go stale.

## Solution
1. `k6 x docs skill` — installs the agent skill from embedded files
2. ETag staleness check — keeps cached docs fresh
3. Nightly CI — rebuilds bundles when k6-docs changes

## Steps (TDD — each step: write failing test, then implement, then lint)

### Step 1: Embed skill files
- Add `//go:embed skills/xk6-docs` to a new `skill.go`
- Test: embedded FS contains SKILL.md and references

### Step 2: SKILL.md templating
- SKILL.md uses `<binary>` placeholder instead of `./k6 x docs`
- `installSkill(destDir, binaryPath)` copies files, replaces `<binary>` with absolute path
- Test: installed SKILL.md contains the binary path, no placeholder

### Step 3: `skill` subcommand (no args → help table)
- `k6 x docs skill` shows glamour-rendered agent table
- Test: command runs, output contains known agents

### Step 4: `skill` subcommand (with dir arg → install)
- `k6 x docs skill ~/.claude/skills` installs to that dir
- Uses `os.Executable()` for binary path
- Test: files installed to correct dir

### Step 5: ETag staleness check in cache.go
- Store `.etag` and `.last_check` in cache dir after download
- On cache hit: if >24h, HEAD request, compare ETag, re-download if changed
- Add `Head` to HTTPClient interface
- Test: stale cache triggers re-download, fresh cache doesn't

### Step 6: Nightly CI workflow
- `.github/workflows/nightly-bundle.yml`
- Cron 3AM UTC, checks k6-docs commits vs bundle timestamps
- No Go test needed — just the workflow file

### Step 7: Update AGENTS.md, README
