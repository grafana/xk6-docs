# k6 docs

k6 documentation is a directory of markdown files at: <dir>

Browse and read them directly. Combine commands with `&&` and `|` to minimize calls.

## Structure

- `_index.md` in each directory has the overview and lists children.
- Directories map to topic areas: `using-k6/`, `javascript-api/`, `using-k6-browser/`, `examples/`.
- API modules are under `javascript-api/` with a `k6-` prefix (e.g. `k6-browser/`, `k6-net-grpc/`).

## Recipes

```sh
# Start with _index.md — it has the overview
cat <dir>/javascript-api/_index.md

# Find topics by keyword
grep -rl "websocket" <dir>/ --include="*.md"

# Scan titles in a directory
head -5 <dir>/using-k6/*.md

# Read multiple files in one call
cat <dir>/using-k6/thresholds.md <dir>/using-k6/metrics.md

# Find by path
find <dir> -name "*.md" -path "*browser*"

# Combine: discover and read in one call
ls <dir>/javascript-api/k6-browser/ && cat <dir>/javascript-api/k6-browser/framelocator/_index.md
grep -rl "grpc" <dir>/ --include="*.md" | head -5 | xargs cat
```
