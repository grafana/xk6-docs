# History

Past incidents and lessons learned from xk6-docs releases.

## Burned Go module proxy versions

The Go module proxy caches versions permanently. A deleted tag stays in the proxy, so its version number can never be reused. These versions cannot be fixed.

| Version | Proxy commit | Git tag commit | Problem |
|---------|-------------|----------------|---------|
| v0.0.1 | `4b46c43` (Rename agent skill) | `4844c3d` (Fix tests after category changes) | Only 7 of 13 categories in hardcoded list. `set-up`, `release-notes`, `get-started`, `extensions`, `k6-studio`, `grafana-cloud-k6` all return "topic not found." |
| v0.0.2 | `90963ec` (Remove --all and --list) | deleted | Before categories fix. Same bug as v0.0.1. |
| v0.0.4 | `2a9139f` (Fix release workflow) | never tagged locally | Before categories fix. Same bug as v0.0.1. |
| v0.0.11 | `33dbecb` (Update k6-ci to v0.4.0) | deleted | Tagged and released on 2026-08-19, then cancelled. The code is fine, but the number is spent. |

v0.0.3 was never cached by the proxy.

`git tag` is not the authority on which numbers are free. After v0.0.11 was deleted, `git tag --sort=-v:refname | head -1` returns v0.0.10, so incrementing the patch points straight back at the burned v0.0.11. Ask the proxy instead, as the release skill pre-flight now does.

v0.0.5 is the first clean version. It was tagged after commit `12b54ac` which removed the hardcoded category list entirely — categories are now derived from the bundle's `sections.json` at runtime.

## Root cause: hardcoded categories

v0.0.1's `categories.go` had a compile-time list of category names used for slug resolution. The prepare tool (`cmd/prepare/`) also used this list to filter which docs went into bundles. When a category wasn't in the list, two things broke:

1. The resolver treated the arg as a JS API module shorthand (e.g. `set-up` became `javascript-api/set-up`), which didn't exist.
2. The prepare tool excluded the category's content from the bundle entirely.

The TOC still showed all categories because it came from `sections.json` (loaded from the bundle), not from the hardcoded list. So users could see topics they couldn't navigate to.

Fixed in `12b54ac`: categories are derived from the bundle at runtime. The prepare tool includes all top-level directories from k6-docs. No content knowledge lives in Go code.

## Timeline

- Mar 12: v0.0.1 tagged and cached by proxy at `4b46c43` (7 categories).
- Mar 13: v0.0.1 tag moved to `4844c3d` (13 categories). Proxy kept old code.
- Mar 1–3: v0.0.2 and v0.0.4 tagged at various points, cached by proxy, tags later deleted. All before the categories fix.
- Mar 24: `12b54ac` removed hardcoded categories entirely.
- Mar 25: v0.0.5 tagged at `b31d7f1` with the fix. Registry PR opened at grafana/k6-extension-registry#191.
