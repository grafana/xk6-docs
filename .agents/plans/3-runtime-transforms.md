# Move transforms from prepare time to runtime

## Context

The transform pipeline (link stripping, shortcode cleanup, frontmatter removal, etc.) currently runs at release time in CI and the results are baked into the cached bundle. This means any transform bug fix requires rebuilding and re-releasing every bundle.

### Current flow

1. Release (CI): Clone k6-docs → run ALL 14 transform steps → package into .tar.zst → publish to GitHub releases
2. Runtime (user's machine): Download bundle → read markdown → print verbatim (zero transforms)

### After this change

1. Release (CI): Clone k6-docs → resolve shared shortcodes only → package into .tar.zst → publish
2. Runtime (user's machine): Download bundle → read markdown → run transforms (strip shortcodes, strip links, convert admonitions, etc.) → print

Shared shortcode expansion must stay at release time because it inlines content from files in the k6-docs shared/ directory that aren't shipped in the bundle (~20 occurrences across javascript-api: crypto tables, grpc methods, experimental module notices, etc.).

This means:
- Transform fixes ship with the extension binary — no bundle rebuilds needed
- Bundles store raw-ish content (shared shortcodes resolved, everything else intact)
- Future bundle embedding works naturally — same transform at display time

## Plan

### 1. Split Transform() into two functions (transform.go)

PrepareTransform(content string, sharedContent map[string]string) string
- Only step 1: resolve `{{< docs/shared >}}` shortcodes using the shared content map

Transform(content, version string) string (updated signature — drop sharedContent)
- Steps 2–14: all pure text transforms (shortcode stripping, admonition conversion, link stripping, frontmatter removal, whitespace normalization)

### 2. Update prepare tool (cmd/prepare/main.go)

Line 304: change `docs.Transform(content, k6Version, sharedContent)` → `docs.PrepareTransform(content, sharedContent)`

The bundle now stores markdown with shared shortcodes resolved but everything else raw.

### 3. Apply transform at runtime (docs.go)

Add a helper that reads + transforms:

```go
func readAndTransform(cacheDir, relPath, version string) string {
    raw := readMarkdown(cacheDir, relPath)
    if raw == "" {
        return ""
    }
    return Transform(raw, version)
}
```

Update all callers of readMarkdown:
- printSection() (line 102) — use readAndTransform(cacheDir, section.RelPath, version)
- printAll() (line 287) — use readAndTransform(cacheDir, sec.RelPath, version)
- printSearch() (line 186) — readContent callback uses readAndTransform; thread version through (available via idx.Version or add param)
- printBestPractices() (line 269) — apply Transform() after reading (needs version param)

### 4. Thread version to printSearch and printBestPractices

- printSearch(w, idx, term, cacheDir) → printSearch(w, idx, term, cacheDir, version)
- printBestPractices(w, cacheDir) → printBestPractices(w, cacheDir, version)
- Update call sites in cmd.go

### 5. Update tests

- transform_test.go: Split tests — PrepareTransform tests shared shortcode expansion only; Transform tests everything else (update signature, remove sharedContent param from non-shared tests)
- cmd/prepare/main_test.go: TestTransformedMarkdownContent should verify that output still has Hugo shortcodes/links (only shared shortcodes resolved)
- docs_test.go: Add test that readAndTransform applies transforms at display time
- cmd.go tests: Update any test that passes version/cacheDir to new signatures

### 6. Delete and rebuild local cache

After the change, the existing cached bundle (with pre-transformed content) won't cause errors — the runtime transform is idempotent on already-transformed text. But for clean testing, delete ~/.local/share/k6/docs/v1.6.x/ and rebuild with the updated prepare tool.

## Files to modify

- transform.go — split Transform into PrepareTransform + Transform
- transform_test.go — update tests for the split
- cmd/prepare/main.go — call PrepareTransform instead of Transform
- cmd/prepare/main_test.go — update assertions for raw output
- docs.go — add readAndTransform, update callers, thread version
- docs_test.go — add runtime transform tests
- cmd.go — update printSearch/printBestPractices call signatures

## Execution approach

Create a granular task list, then launch subagents (via Task tool) to tackle each change. Never do the work directly — all edits delegated to subagents. No agent teams.

## Verification

1. go test ./... — all tests pass
2. Rebuild local bundle: go run ./cmd/prepare --k6-version v1.6.1 --docs-path <path-to-k6-docs>
3. Verify raw bundle content: cat dist/markdown/javascript-api/k6/_index.md should still have markdown links
4. Run ./k6 x docs k6 — links should be stripped in output (transformed at runtime)
5. Run golangci-lint run — no lint issues
