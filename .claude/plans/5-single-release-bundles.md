# Single release for doc bundles

## Context

Doc bundles currently create a separate GitHub release per k6 version (e.g., `docs-v1.5.x`, `docs-v1.6.x`). This pollutes the repo's release page — these aren't releases of this application, just doc artifacts. Instead, use a single release (`doc-bundles`) and add each version's bundle as an asset.

`softprops/action-gh-release@v2` supports this: reusing the same `tag_name` updates the existing release and adds/overwrites assets. `make_latest: false` prevents it from hijacking the "Latest" badge.

## Changes

### 1. `cache.go` — `downloadURL()`

URL pattern changed from per-version tag to fixed tag:
```
BEFORE: /releases/download/docs-{version}/docs-{version}.tar.zst
AFTER:  /releases/download/doc-bundles/docs-{version}.tar.zst
```

### 2. `.github/workflows/release-bundle.yml`

- `tag_name` → `doc-bundles`
- Added `make_latest: false`

### 3. `.github/workflows/release-bundle-poll.yml`

- Same `tag_name` and `make_latest` changes
- "Already exists" check now looks for the asset in the `doc-bundles` release instead of looking for a release by tag name

### 4. `go.mod` — lowered `go.k6.io/k6` from v1.6.0 to v1.5.0

## The go.mod version problem

The extension auto-detects which k6 version it's bundled with by reading `go.k6.io/k6` module version from Go build info. Users build with xk6:

```
xk6 build v1.5.0 --with github.com/grafana/xk6-docs
```

Go uses Minimum Version Selection — it always picks the highest version any module requires. The extension's `go.mod` required `go.k6.io/k6 v1.6.0`, so even when a user asked xk6 for v1.5.0, Go silently resolved to v1.6.0. The extension then detected v1.6.x and showed the wrong docs.

The fix: set the floor to v1.5.0 (the first k6 version with the subcommand API). Now Go respects whatever version the user asks for.

## Current limitation

The extension must support all k6 versions from v1.5.0 onwards. This means:

- The `go.mod` floor stays at v1.5.0 permanently
- The extension can only use k6 APIs that exist in v1.5.0 (currently: `subcommand.RegisterExtension` and `cmd/state.GlobalState`)
- If the extension ever needs a newer k6 API, use Go build tags to conditionally compile against newer versions while keeping the v1.5.0 fallback
