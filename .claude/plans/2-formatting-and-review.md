# Plan: Fix `k6 x docs` output formatting, search, and review findings

## Context

The initial `xk6-docs` implementation is complete. This plan fixes formatting problems across the CLI output, adds fuzzy search, adds agent detection, and addresses code review findings. All work done by subagents using TDD (red/green/refactor). Every commit must pass `go test -race -count=1 ./...`.

**Branch:** Create `fix/formatting-and-review` from `main` before any changes.

## Review findings coverage (~/review.md)

| # | Severity | Issue | Action |
|---|----------|-------|--------|
| F1 | P2 | Duplicate `k6-` prefix in resolve.go | **Fix** — Wave 1 resolve-fix |
| F2 | P1 | Release tag mismatch | **Already fixed** — workflow maps exact→wildcard |
| F3 | P2 | Root `--list` unreachable | **Fix** — Wave 2 output-formatting |
| F4 | P2 | Shared content map keys not normalized | **Fix** — Wave 1 prepare-fixes |
| F5 | P3 | Dead `bySlug` map | **Fix** — Wave 1 prepare-fixes |
| F6 | P3 | `MapToWildcard` comment mismatch | **Fix** — Wave 1 prepare-fixes |
| F7 | P2 | Go 1.24 vs 1.26 | **Skip** — matches k6 v1.6.0 baseline |
| F8 | P3 | Double transform at runtime | **Fix** — Wave 2 output-formatting |
| F9 | P3 | yaml.v3 in root module | **Skip** — restructuring not worth it now |
| F10 | P3 | Hardcoded path in test | **Fix** — Wave 1 prepare-fixes |
| F11 | P2 | Duplicate YAML keys drop metadata | **Fix** — Same as Bug 1 (Wave 1 prepare-fixes) |
| F12 | P2 | Slug collisions not handled | **Fix** — Same as Bug 3 (Wave 1 prepare-fixes) |
| F13 | P2 | Search paths not runnable as CLI input | **Fix** — Wave 2 output-formatting (hierarchical search) |
| F14 | P2 | `--all` duplicates headings | **Fix** — Same as Bug 2 (Wave 2 output-formatting) |
| F15 | P2 | `slugToShortArgs` creates ambiguous names | **Fix** — Wave 2: replace `slugToShortArgs` with `childName()` which preserves `k6-` prefix |
| F16 | P3 | Transform leaves `<Glossary>`, `<DescriptionList>`, `<br/>` | **Fix** — Wave 1 transform-fix |
| F17 | P3 | Unbounded line length in listings | **Fix** — Same as Fix 4 (truncation, Wave 2) |

---

## Bug 1: 64 sections have empty titles due to duplicate YAML keys

**Root cause:** k6-docs has ~60 files with duplicate `description:` keys in frontmatter. Go's `yaml.v3` is strict and errors on duplicate keys. `parseFrontmatter` catches the error and returns an empty struct, losing the title/description/weight.

**File:** `cmd/prepare/main.go:197-212`

**Fix:** Before passing to `yaml.Unmarshal`, deduplicate YAML keys by keeping only the first occurrence of each key:

```go
func deduplicateYAMLKeys(yamlBlock string) string {
    seen := make(map[string]bool)
    var lines []string
    for _, line := range strings.Split(yamlBlock, "\n") {
        if idx := strings.Index(line, ":"); idx > 0 && line[0] != ' ' && line[0] != '#' {
            key := strings.TrimSpace(line[:idx])
            if seen[key] {
                continue
            }
            seen[key] = true
        }
        lines = append(lines, line)
    }
    return strings.Join(lines, "\n")
}
```

This fixes: `http url` (title was empty, shows as blank), `httpx/get`, `httpx/post`, etc. — all 64 missing titles.

After this fix, `make prepare` must be re-run to regenerate `dist/`.

---

## Bug 2: `--all` produces duplicate headings

**Root cause:** `printAll` (`docs.go:167`) writes `# sec.Title` AND the cached markdown already starts with `# Title` (frontmatter stripped but H1 kept by prepare).

**File:** `docs.go:156-174`

**Fix:** Don't prepend `# Title` in `printAll`. Just print the content directly (it already has its own heading).

---

## Bug 3: Duplicate `cookiejar` entry

**Root cause:** k6-docs has both `cookiejar.md` (the function) and `cookiejar/_index.md` (the class), both mapping to slug `javascript-api/k6-http/cookiejar`. The second overwrites the first in sections.json but both appear as children.

**File:** `cmd/prepare/main.go` — `walkAndProcess` and `populateChildren`

**Fix:** When building sections, detect slug collisions. If a slug already exists, either:
- Prefer the `_index.md` version (it has children), or
- Append a disambiguator like `-fn` for the function version

Simplest: skip the non-index version when a collision occurs (the _index version is more useful since it has children).

---

## Fix 1: Relative child names everywhere

Child names repeat the parent: `http get`, `browser closecontext`, `using-k6 http-debugging`. Should be `get`, `closecontext`, `http-debugging`.

### Single helper in `docs.go`:

```go
func childName(childSlug, parentSlug string) string {
    if strings.HasPrefix(childSlug, parentSlug+"/") {
        return childSlug[len(parentSlug)+1:]
    }
    if i := strings.LastIndex(childSlug, "/"); i >= 0 {
        return childSlug[i+1:]
    }
    return childSlug
}
```

### All places to apply:

| Location | File:Line | Current call | After |
|----------|-----------|--------------|-------|
| `printTOC` children | `docs.go:54-56` | `slugToShortArgs(child.Slug)` | `childName(child.Slug, cat.Slug)` — keep `k6-` prefix (F15) |
| `printSection` subtopics | `docs.go:76-77` | `slugToShortArgs(c.Slug)` | `childName(c.Slug, section.Slug)` |
| `printList` children | `docs.go:108-110` | `slugToShortArgs(child.Slug)` | `childName(child.Slug, slug)` |
| `printSearch` results | `docs.go:134-136` | `slugToShortArgs(sec.Slug)` | restructure to hierarchical (see Fix 4) |

**Important (F15):** Do NOT strip `k6-` from child names. Both `javascript-api/crypto` and `javascript-api/k6-crypto` exist. Stripping `k6-` would collapse them to the same display name `crypto`. `childName()` already handles this correctly since it only strips the parent slug prefix.

### Subtopics footer: add blank line before separator

```go
fmt.Fprintln(w)  // blank line before separator
fmt.Fprintln(w, "---")
```

---

## Fix 2: Fuzzy search — ignore spaces and dashes

"close context" / "close-context" should match "closecontext".

**File:** `sections.go`

```go
func normalize(s string) string {
    return strings.ToLower(strings.NewReplacer("-", "", " ", "").Replace(s))
}
```

In `Search`, add normalized matching after the existing case-insensitive check:
```go
normTerm := normalize(term)
if strings.Contains(normalize(sec.Title), normTerm) ||
    strings.Contains(normalize(sec.Description), normTerm) { ... }
// Same for body text
```

---

## Fix 3: Remove blank lines between headers and content

| Location | Current | Fix |
|----------|---------|-----|
| `printSearch` line 133 | `fmt.Fprintln(w)` after "Results for" header | Remove |
| `printList` line 107 | `fmt.Fprintln(w)` after title line | Remove |
| `printTOC` line 39-40 | `\n\n` after header, then "Use:" | Change to `\n` |

---

## Fix 4: Hierarchical grouping for ALL listings

Currently all listing views (TOC, list, search) dump flat lists. All should use consistent hierarchical formatting with description truncation.

### Description truncation (all listings):

```go
func truncate(s string, max int) string {
    if len(s) <= max { return s }
    return s[:max-3] + "..."
}
```

Max 80 chars for descriptions.

### `printTOC` redesign (`docs.go:37-59`)

**Before:**
```
## Using k6
  using-k6 http-debugging Things don't always work as expected. For those cases...
```

**After:**
```
## Using k6
  http-debugging       Things don't always work as expected. For those cases th...
  checks               Checks are like asserts but differ in that they do not h...
```

Use `childName()` + `truncate()`. Do NOT strip `k6-` from child names (F15).

### `printList` redesign (`docs.go:86-112`)

**Before:**
```
k6/http — The k6/http module contains functionality...

  http get
  http options
```

**After:**
```
k6/http — The k6/http module contains functionality...
  get
  options
  url
```

Use `childName()` + `truncate()`. Remove blank line between header and list.

### `printSearch` redesign (`docs.go:114-138`)

**Before (flat):**
```
Results for "newcontext":

  javascript-api
  browser              An overview of the browser-level APIs...
  browser browsercontext addcookies Clears context cookies.
```

**After (hierarchical):**
```
Results for "newcontext":
browser: An overview of the browser-level APIs from browser module.
  browsercontext
    addcookies         Clears context cookies.
    addinitscript      Adds an init script.
  context              Browser module: context method
  newcontext           Browser module: newContext method

using-k6-browser:
  migrate-from-playwright-to-k6  A migration guide to ease the process of tran...
```

Implementation:
1. Group results by topic. JS API → group by module (strip `javascript-api/k6-`). Others → group by category.
2. Sort groups alphabetically. Within each group, sort items alphabetically.
3. Build hierarchy: if a child's parent is also in results, nest it.
4. Omit bare `javascript-api` as a result (adds no info, it's the default).
5. Truncate descriptions at 80 chars.
6. Group header: `{topic}: {description}` if the topic itself matched, else just `{topic}:`.

---

## Fix 5: Strip remaining React/MDX component tags from transform (F16)

**File:** `transform.go`

The transform pipeline misses React/MDX component tags (`<Glossary>`, `<DescriptionList>`) and `<br/>` tags. These appear in the glossary (33 `<br/>` instances) and a few other files (181 total occurrences across 53 files).

**Fix:** Add two new regex patterns to the pipeline:

```go
reComponentTag = regexp.MustCompile(`</?[A-Z][a-zA-Z]*[^>]*>`)  // <Glossary>, </DescriptionList>, etc.
reBrTag        = regexp.MustCompile(`<br\s*/?>`)                  // <br/>, <br />
```

Insert after step 5 (strip remaining shortcodes), before step 6 (version replacement):
```go
// 5b. Strip React/MDX component tags.
s = reComponentTag.ReplaceAllString(s, "")
// 5c. Strip <br/> tags.
s = reBrTag.ReplaceAllString(s, "")
```

**Caveat:** `reComponentTag` uses `[A-Z]` start to distinguish from lowercase HTML tags like `<code>`, `<pre>`. This won't accidentally match `<API_TOKEN>` placeholders because those contain `_`.

---

## Fix 6: Move `bestPracticesContent` to embedded file

**File:** `cmd/prepare/main.go`, `cmd/prepare/best_practices.md`

The `bestPracticesContent` const is ~640 lines of Go string with backtick-escaping gymnastics. Move it to a proper `.md` file and use `//go:embed`:

1. Extract the content from the const to `cmd/prepare/best_practices.md` (clean markdown, no Go escaping)
2. Replace the const with:
```go
//go:embed best_practices.md
var bestPracticesContent string
```
3. The `writeBestPractices` function stays the same — it writes `bestPracticesContent` to the output dir

---

## Fix 7: Use k6's logger, not `log` package

**Runtime code** (cmd.go, docs.go, cache.go, etc.) must use `gs.Logger` from k6's GlobalState. Currently no runtime code uses `log`, but all new code (agent detection, etc.) must follow this rule.

**Prepare tool** (`cmd/prepare/main.go`) is a standalone CLI — `log` is acceptable there since it has no k6 GlobalState.

---

## Fix 8: Create Claude Code skill for `k6 x docs`

Create a skill file at `xk6-docs/.claude/skills/k6-x-docs.md` using the skill-creator skill. The skill should teach Claude Code how to use `k6 x docs` effectively:
- Available commands and flags
- Search patterns
- How to read specific module docs
- Best practices lookup

Also update `README.md` to show how to install the skill using `npx`.

---

## Fix 9: Agent detection (log only, verbose mode)

**File:** `cmd.go`

Detect if stdout is a TTY. Log result only when verbose. No behavior change yet.

```go
import "golang.org/x/term"

// In RunE, after setup:
if term.IsTerminal(int(os.Stdout.Fd())) {
    gs.Logger.Debug("docs: interactive mode (stdout is TTY)")
} else {
    gs.Logger.Debug("docs: agent mode (stdout is not a TTY)")
}
```

---

## Review findings — implementation notes

| Finding | Fix detail |
|---------|-----------|
| F1 | `strings.TrimPrefix(args[0], "k6-")` before prepending `k6-` in resolve.go:31 |
| F3 | Check `listFlag` before `len(args)==0` branch. Add `printTopLevelList` to docs.go |
| F4 | `m[filepath.ToSlash(rel)] = string(data)` in cmd/prepare/main.go:167 |
| F5 | Delete unused `bySlug` map in cmd/prepare/main.go:314-317 |
| F6 | Update comment in version.go:26-32 to match actual behavior |
| F8 | Remove `Transform()` calls in docs.go:66 and docs.go:165 |
| F10 | Use `K6_DOCS_PATH` env var, fall back to `$HOME/grafana/k6-docs` |
| F13 | Hierarchical search groups results by topic. Display paths that match valid CLI input. Group header is the module/category name users can type |
| F15 | Replace `slugToShortArgs` with `childName()` which preserves `k6-` prefix. `k6-crypto` and `crypto` remain distinguishable |
| F16 | Add `reComponentTag` and `reBrTag` regexes to transform.go pipeline after step 5 |
| F17 | `truncate()` helper caps descriptions at 80 chars in all listing views |

---

## Files to modify

| File | Changes |
|------|---------|
| `docs.go` | `childName()`, `truncate()`, rewrite all 4 listing fns, `printTopLevelList`, remove `Transform()` calls (F8), remove blank lines, fix `--all` duplicate headings (F14), fix ambiguous names (F15), fix search CLI paths (F13), truncate descriptions (F17) |
| `docs_test.go` | Update all output assertions |
| `cmd.go` | Agent detection log (Fix 6), fix `--list` root dispatch (F3) |
| `resolve.go` | Strip `k6-` prefix (F1) |
| `resolve_test.go` | Add `k6-` test cases |
| `sections.go` | `normalize()`, fuzzy search |
| `sections_test.go` | Fuzzy search tests |
| `transform.go` | Strip `<Glossary>`, `<DescriptionList>`, `<br/>` tags (F16) |
| `transform_test.go` | Add tests for React/MDX tag stripping |
| `version.go` | Fix comment (F6) |
| `cmd/prepare/main.go` | Deduplicate YAML keys (Bug 1/F11), fix slug collisions (Bug 3/F12), normalize shared content keys (F4), remove dead code (F5), embed best practices (Fix 6) |
| `cmd/prepare/main_test.go` | Use env var for k6-docs path (F10), add duplicate-key YAML test |
| `cmd/prepare/best_practices.md` | **New file** — extracted from const in main.go (Fix 6) |
| `.claude/skills/k6-x-docs.md` | **New file** — Claude Code skill for k6 x docs (Fix 8) |
| `README.md` | Add npx skill installation instructions (Fix 8) |

---

## Execution: granular task list + subagents

Create a new branch `fix/formatting-and-review` from `main` first.

Then create all tasks via TaskCreate and launch subagents (no teams). Each subagent follows TDD: write failing test → implement → verify green → commit. Every commit must pass `go test -race -count=1 ./...`.

### Task list

| ID | Task | Files | Depends on | Wave |
|----|------|-------|------------|------|
| 1 | Create branch `fix/formatting-and-review` from `main` | — | — | 0 |
| 2 | F1: Strip `k6-` prefix in resolver | `resolve.go`, `resolve_test.go` | 1 | 1 |
| 3 | Fix 2: Add `normalize()` + fuzzy search matching | `sections.go`, `sections_test.go` | 1 | 1 |
| 4 | Bug 1/F11: Deduplicate YAML keys in frontmatter parser | `cmd/prepare/main.go`, `cmd/prepare/main_test.go` | 1 | 1 |
| 5 | Bug 3/F12: Handle slug collisions (prefer _index.md) | `cmd/prepare/main.go`, `cmd/prepare/main_test.go` | 4 | 1 |
| 6 | F4: Normalize shared content map keys with `filepath.ToSlash` | `cmd/prepare/main.go` | 1 | 1 |
| 7 | F5: Remove dead `bySlug` map in `populateChildren` | `cmd/prepare/main.go` | 1 | 1 |
| 8 | F6: Fix `MapToWildcard` comment to match behavior | `version.go` | 1 | 1 |
| 9 | F10: Use `K6_DOCS_PATH` env var in test | `cmd/prepare/main_test.go` | 1 | 1 |
| 10 | F16/Fix 5: Strip React/MDX component tags + `<br/>` | `transform.go`, `transform_test.go` | 1 | 1 |
| 11 | Fix 1/F15: Add `childName()` helper, replace `slugToShortArgs` in all listing fns | `docs.go`, `docs_test.go` | 1 | 2 |
| 12 | Fix 3: Remove blank lines between headers and content | `docs.go`, `docs_test.go` | 11 | 2 |
| 13 | Fix 4/F17: Add `truncate()` helper, apply to all listing fns | `docs.go`, `docs_test.go` | 12 | 2 |
| 14 | Fix 4/F13: Rewrite `printSearch` with hierarchical grouping | `docs.go`, `docs_test.go` | 13 | 2 |
| 15 | Bug 2/F14: Fix `--all` duplicate headings | `docs.go`, `docs_test.go` | 11 | 2 |
| 16 | F8: Remove double `Transform()` calls at runtime | `docs.go`, `docs_test.go` | 11 | 2 |
| 17 | F3: Fix root `--list` dispatch + `printTopLevelList` | `docs.go`, `docs_test.go`, `cmd.go` | 11 | 2 |
| 18 | Fix 9: Add TTY agent detection (verbose log only) | `cmd.go` | 1 | 2 |
| 19 | Fix 6: Move `bestPracticesContent` to embedded .md file | `cmd/prepare/main.go`, `cmd/prepare/best_practices.md` | 1 | 1 |
| 20 | Fix 8: Create Claude Code skill for `k6 x docs` + update README | `.claude/skills/k6-x-docs.md`, `README.md` | 1 | 1 |
| 21 | Re-run `make prepare` with YAML dedup fix | `dist/` | 4, 5, 6, 7, 10, 19 | 3 |
| 22 | Rebuild with xk6 + run full CLI smoke tests | — | all | 3 |

### Subagent waves

**Wave 0** (me): Create branch.

**Wave 1** (8 parallel subagents — all independent, touching different files):
- Tasks 2, 3, 4+5 (sequential pair), 6, 7, 8, 9, 10
- Tasks 4 and 5 go to the same agent since they both modify `cmd/prepare/main.go`
- Tasks 6 and 7 can go to the same agent (both small changes in `cmd/prepare/main.go`)
- So effectively: **5 subagents** in parallel

| Subagent | Tasks |
|----------|-------|
| resolve-fix | 2 |
| search-fuzzy | 3 |
| prepare-yaml-and-slugs | 4, 5 |
| prepare-small-fixes | 6, 7, 8, 9, 19 |
| transform-fix | 10 |
| skill-and-readme | 20 |

**Wave 2** (subagents for docs.go/cmd.go — must run after wave 1 merges):
- Tasks 11-18 modify `docs.go` and `docs_test.go` so they must be sequential
- Launch **1 large subagent** that does tasks 11→12→13→14→15→16→17→18 in order
- Each task = 1 commit following TDD

| Subagent | Tasks |
|----------|-------|
| output-formatting | 11, 12, 13, 14, 15, 16, 17, 18 (sequential) |

**Wave 3** (after all code changes):
- Task 21: re-prepare subagent
- Task 22: smoke test subagent (after 21)

| Subagent | Tasks |
|----------|-------|
| re-prepare | 21 |
| smoke-test | 22 (after 21) |

---

## Rules for all subagents

1. **k6 logger**: All runtime extension code must use `gs.Logger` (from k6's GlobalState), never the stdlib `log` package. Only `cmd/prepare/main.go` (standalone CLI) may use `log`.
2. **TDD**: Write failing test → implement → verify green → commit.
3. **Buildable**: Every commit must pass `go test -race -count=1 ./...`.
4. **No AI attribution**: Never add Co-Authored-By or Generated-By to commits.

---

## Verification

1. `go test -race -count=1 ./...`
2. `make prepare K6_VERSION=v1.5.x K6_DOCS_PATH=~/grafana/k6-docs` (re-generate with YAML fix)
3. `xk6 build --with github.com/grafana/xk6-docs=.`
4. Smoke tests:
   - `./k6 x docs` — relative child names, truncated descriptions, no prefix repetition
   - `./k6 x docs --list` — works (was unreachable)
   - `./k6 x docs http` — subtopics: `get, post, url, ...` (not `http get, ...`)
   - `./k6 x docs http --list` — children: `get`, `post`, `url` (not `http get`)
   - `./k6 x docs http url` — shows title (was empty before YAML fix)
   - `./k6 x docs browser --list` — no `browser` prefix, no duplicate cookiejar
   - `./k6 x docs search newcontext` — hierarchical, grouped by topic
   - `./k6 x docs search "close context"` — fuzzy matches "closecontext"
   - `./k6 x docs k6-http get` — resolves correctly
   - `./k6 x docs --all | head -20` — no duplicate headings
   - `./k6 x docs -v 2>&1 | grep "docs:"` — agent/interactive detection logged
   - `./k6 x docs encoding --list` — clean subtopic names (not `b64decode--input...`)
   - `./k6 x docs best-practices` — still works after embed refactor
5. Verify `.claude/skills/k6-x-docs.md` exists and is well-formed
6. Verify `README.md` contains npx installation instructions
