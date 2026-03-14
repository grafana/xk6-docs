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

---

## What changed

### Problems

**AI agents couldn't look up docs.** After installing the skill, agents would immediately fail
trying to run `k6 x docs` because users' k6 (installed via Homebrew, download, or custom build)
doesn't have the docs extension. There was no way for an agent to find the right binary to use.

**Docs silently went out of date.** Once downloaded, doc bundles were served from cache forever.
Fixes published to k6-docs between k6 releases were invisible to users — they'd get wrong or
outdated information with no indication anything was stale.

### Installing the skill now takes one command

`k6 x docs skill ~/.claude/skills` installs the agent skill directly from the user's own k6
binary, referencing the exact binary they already run their tests with. Running it without
arguments shows a table of supported agents (Claude Code, Cursor, Codex, etc.) with the directory
to use for each. The previous npx install had no way to connect the skill to the user's binary,
so most setups failed silently the first time an agent tried to look something up.

### Docs stay fresh automatically

Cached doc bundles are re-checked once per day — silently re-downloaded if updated, or served
from cache without error if the network is unavailable. Doc fixes in k6-docs used to be invisible
to users until they manually cleared the cache or waited for the next k6 release.

### Bundle updates are targeted per k6 version

A nightly CI job checks whether k6-docs has new commits for each version's specific content
directory (e.g. `docs/sources/k6/v1.6.x`) and only rebuilds the bundles that actually changed.
Previously, bundles were only rebuilt on k6 releases, so doc fixes between releases never reached
users at all.
