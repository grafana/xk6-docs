## Test-First Plan: `categories.go` + Shared Home Resolution (No Helper Sprawl)

### Summary
Create one source of truth for docs categories/path inclusion in `categories.go`, reuse it from `prepare`, `resolve`, and `transform`, and share home-dir fallback logic by placing it in `config.go` (not a new file).  
Design goal: no duplicate category definitions across files/tests and no one-use helper functions.

### API / Type Changes
1. Add exported function in [`categories.go`](/Users/inanc/grafana/xk6-docs/categories.go):
   - `IsIncludedDocsPath(path string) bool`
2. Keep category list itself unexported (single internal slice/map source).

### Phase 1: Tests First
1. Add [`categories_test.go`](/Users/inanc/grafana/xk6-docs/categories_test.go) in package `docs` and make it derive expectations from the same internal category slice used by production code.
2. Test `IsIncludedDocsPath` with generated cases:
   - For each canonical category except `reference`: `category/x` is included.
   - For `reference`: only `reference/glossary` and `reference/glossary/x` are included.
   - `reference/x` (non-glossary) is excluded.
   - Unknown category is excluded.
   - Leading/trailing slash normalization works.
3. Update [`resolve_test.go`](/Users/inanc/grafana/xk6-docs/resolve_test.go) to validate category-prefix resolution using shared category source (no hardcoded duplicate category lists in tests).
4. Remove duplicated category matrix from [`cmd/prepare/main_test.go`](/Users/inanc/grafana/xk6-docs/cmd/prepare/main_test.go) (`TestIsIncluded`) and rely on:
   - `categories_test.go` for category logic correctness.
   - Existing prepare integration tests for pipeline behavior.
5. Add/adjust config fallback tests in [`config_test.go`](/Users/inanc/grafana/xk6-docs/config_test.go):
   - `USERPROFILE` fallback when `XDG_CONFIG_HOME` and `HOME` are unset.
   - Error when neither `HOME` nor `USERPROFILE` is set and `XDG_CONFIG_HOME` is unset.

### Phase 2: Implement Shared Category Component (`categories.go`)
1. Add one canonical unexported slice for top-level categories.
2. Build one unexported lookup map from that slice.
3. Add one unexported reusable predicate for top-level category membership (used by multiple call sites).
4. Implement exported `IsIncludedDocsPath(path string) bool`:
   - Normalize path.
   - Validate top-level category from shared map.
   - Apply `reference/glossary` special-case.
5. Keep function count minimal; no extra wrappers.

### Phase 3: Wire Call Sites to Shared Category Logic
1. In [`cmd/prepare/main.go`](/Users/inanc/grafana/xk6-docs/cmd/prepare/main.go):
   - Remove local `includedCategories()` and `isIncluded()`.
   - Use `docs.IsIncludedDocsPath(relPath)` in filtering.
2. In [`resolve.go`](/Users/inanc/grafana/xk6-docs/resolve.go):
   - Replace local hardcoded categories with shared membership predicate.
3. In [`transform.go`](/Users/inanc/grafana/xk6-docs/transform.go):
   - Remove local included-categories map.
   - Use `IsIncludedDocsPath(cleanPath)` for internal-link category/path classification.
   - Preserve current output behavior.

### Phase 4: Shared Home Resolution in `config.go`
1. In [`config.go`](/Users/inanc/grafana/xk6-docs/config.go), add unexported:
   - `homeDirFromEnv(env map[string]string) (string, error)`
   - Rule: `HOME` first, then `USERPROFILE`, else error.
2. Update `configDir()` to use `homeDirFromEnv()` when `XDG_CONFIG_HOME` is not set.
3. Update [`cache.go`](/Users/inanc/grafana/xk6-docs/cache.go) `CacheDir()` to use the same `homeDirFromEnv()` from `config.go`.
4. Do not create `env_home.go`.

### Phase 5: Validate
1. Run `go test ./...`.
2. Run `golangci-lint run`.
3. Confirm no category definitions are duplicated across `prepare`, `resolve`, `transform`, and tests.

### Acceptance Criteria
1. Category taxonomy is defined once in `categories.go`.
2. `prepare`, `resolve`, and `transform` all consume shared category logic.
3. Cache/config home fallback logic is defined once in `config.go` and reused.
4. Tests no longer maintain separate hardcoded included/excluded category lists.
5. All tests and lint pass.

### Assumptions
1. `IsIncludedDocsPath` must be exported because it is used from `cmd/prepare` (different package).
2. Home fallback scope is exactly `HOME` then `USERPROFILE`.
3. No command UX changes are included in this refactor.
