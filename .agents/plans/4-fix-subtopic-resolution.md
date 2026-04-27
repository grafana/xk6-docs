# Fix subtopic display and resolution for nested sections

## Context

Two related problems with nested sections (117 children across k6-html, k6-http, k6-net-grpc, etc.):

1. **Redundant display**: `k6 x docs http cookiejar` shows subtopic `cookiejar-clear` — the `cookiejar-` prefix is noise since you're already viewing cookiejar.
2. **Broken footer**: Footer says `Use: k6 x docs cookiejar <subtopic>` but that resolves to `javascript-api/k6-cookiejar/...` (wrong). It should say `Use: k6 x docs http cookiejar <subtopic>`.

After the fix, the user sees:

```
Subtopics: clear, cookiesforurl, delete, set
Use: k6 x docs http cookiejar <subtopic>
```

And `k6 x docs http cookiejar clear` resolves correctly.

## Data analysis

- 117 children have parent-name prefix (e.g., `cookiejar/cookiejar-clear`)
- 778 children don't (e.g., `k6-http/get`)
- Zero naming collisions after stripping — safe to strip unconditionally

## Changes

### 1. Strip parent prefix from displayed child names (`docs.go`)

Update `childName` — after stripping the parent slug prefix, also strip the parent's last segment + `-` from the child name:

`childName("javascript-api/k6-http/cookiejar/cookiejar-clear", "javascript-api/k6-http/cookiejar")` currently returns `cookiejar-clear`, should return `clear`.

### 2. Fix footer usage hint (`docs.go`)

Add `slugToArgs(slug string) string` — converts slug to CLI args:
- `javascript-api/k6-http/cookiejar` → `http cookiejar`
- `javascript-api/crypto/subtlecrypto` → `crypto subtlecrypto`
- `using-k6/scenarios` → `using-k6 scenarios`

Rules:
1. `javascript-api/` prefix: strip it, strip `k6-` from first segment, join with spaces
2. Otherwise: join segments with spaces

Replace `childName(section.Slug, "")` in footer with `slugToArgs(section.Slug)`.

### 3. Resolver fallback for stripped names (`resolve.go`)

When `exists` callback returns false, try prepending the parent segment name. In `ResolveWithLookup`, after building the slug and checking `exists()`, if it fails, try: take last two segments `parent/child`, retry as `parent/parent-child`.

This handles `http cookiejar clear` → tries `javascript-api/k6-http/cookiejar/clear` (miss) → tries `javascript-api/k6-http/cookiejar/cookiejar-clear` (hit).

### Files

- **`docs.go`** — update `childName` to strip parent prefix, add `slugToArgs`, update footer
- **`resolve.go`** — add parent-prefix fallback in `ResolveWithLookup`
- **`docs_test.go`** — add `TestSlugToArgs`, update footer assertions, update `childName` tests
- **`resolve_test.go`** — add test for parent-prefix fallback resolution
- **`AGENTS.md`** — update

## Out of scope: slug collision (cookiejar)

Only 1 collision exists in 908 docs files: `cookiejar.md` (the `http.cookieJar()` function) and `cookiejar/_index.md` (the `CookieJar` class) produce the same slug. The class wins at prepare time, dropping the function page. The leaf file has `slug: cookiejar-method` in its frontmatter but the prepare tool ignores it. Separate fix needed — doesn't affect this plan since only one slug survives in the index.

## Execution

Use TDD (red/green/refactor) and the golang-patterns skill. Delegate work to subagents. Update AGENTS.md concisely.

## Verification

1. `go test ./...` passes
2. `golangci-lint run --no-config` — no new issues
3. `k6 x docs http cookiejar` shows `clear, cookiesforurl, delete, set` and footer `Use: k6 x docs http cookiejar <subtopic>`
4. `k6 x docs http cookiejar clear` resolves to cookiejar-clear page
