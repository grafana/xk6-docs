#!/usr/bin/env bash
set -euo pipefail

# Tests for bundle-sync.sh using a fake gh that serves JSON and processes jq.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# --- Fake gh command ---

cat > "$TMPDIR/gh" <<'FAKEGH'
#!/usr/bin/env bash
# Fake gh that serves canned JSON and processes -q/--jq like real gh.

# Find the data file for this call.
DATA=""
if [ "$1" = "release" ] && [ "$2" = "view" ]; then
  DATA="$FAKE_ASSETS_FILE"
elif [ "$1" = "api" ] && [[ "$2" == repos/grafana/k6-docs/contents/* ]]; then
  DATA="$FAKE_FOLDERS_FILE"
elif [ "$1" = "api" ] && [[ "$2" == repos/grafana/k6-docs/commits* ]]; then
  version=$(echo "$2" | grep -o 'path=docs/sources/k6/[^&]*' | sed 's|path=docs/sources/k6/||')
  DATA="$FAKE_DATES_DIR/$version"
  if [ ! -f "$DATA" ]; then
    echo "[]"
    exit 0
  fi
else
  echo "unexpected gh call: $*" >&2
  exit 1
fi

# Extract jq query from -q or --jq flag.
JQ_QUERY=""
ARGS=("$@")
for ((i=0; i<${#ARGS[@]}; i++)); do
  if [ "${ARGS[$i]}" = "-q" ] || [ "${ARGS[$i]}" = "--jq" ]; then
    JQ_QUERY="${ARGS[$((i+1))]}"
    break
  fi
done

if [ -n "$JQ_QUERY" ]; then
  jq -r "$JQ_QUERY" "$DATA"
else
  cat "$DATA"
fi
FAKEGH
chmod +x "$TMPDIR/gh"

# --- Helpers ---

setup_test() {
  rm -rf "$TMPDIR/dates"
  mkdir -p "$TMPDIR/dates"
  export FAKE_DATES_DIR="$TMPDIR/dates"
}

set_folders() {
  # Accepts pairs: name type name type ...
  local json="["
  local first=true
  while [ $# -gt 0 ]; do
    $first || json+=","
    json+="{\"name\":\"$1\",\"type\":\"$2\"}"
    first=false
    shift 2
  done
  json+="]"
  echo "$json" > "$TMPDIR/folders.json"
  export FAKE_FOLDERS_FILE="$TMPDIR/folders.json"
}

set_assets() {
  # Accepts pairs: name updatedAt name updatedAt ...
  local json="{\"assets\":["
  local first=true
  while [ $# -gt 0 ]; do
    $first || json+=","
    json+="{\"name\":\"$1\",\"updatedAt\":\"$2\"}"
    first=false
    shift 2
  done
  json+="]}"
  echo "$json" > "$TMPDIR/assets.json"
  export FAKE_ASSETS_FILE="$TMPDIR/assets.json"
}

set_docs_date() {
  local version="$1" date="$2"
  echo "[{\"commit\":{\"committer\":{\"date\":\"$date\"}}}]" > "$TMPDIR/dates/$version"
}

run_sync() {
  PATH="$TMPDIR:$PATH" "$SCRIPT_DIR/sync.sh" 2>/dev/null
}

PASS=0
FAIL=0

assert_eq() {
  local test_name="$1" expected="$2" actual="$3"
  if [ "$expected" = "$actual" ]; then
    echo "PASS: $test_name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $test_name"
    echo "  expected: '$expected'"
    echo "  actual:   '$actual'"
    FAIL=$((FAIL + 1))
  fi
}

# --- Test: missing bundle detected ---

setup_test
set_folders "v1.5.x" "dir" "v1.7.x" "dir"
set_assets "docs-v1.5.x.tar.zst" "2026-03-19T03:55:34Z"
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.7.x" "2026-03-25T09:00:00Z"

RESULT=$(run_sync)
assert_eq "missing bundle detected" "v1.7.x" "$RESULT"

# --- Test: stale bundle detected ---

setup_test
set_folders "v1.6.x" "dir"
set_assets "docs-v1.6.x.tar.zst" "2026-03-10T00:00:00Z"
set_docs_date "v1.6.x" "2026-03-18T09:00:00Z"

RESULT=$(run_sync)
assert_eq "stale bundle detected" "v1.6.x" "$RESULT"

# --- Test: up to date bundle skipped ---

setup_test
set_folders "v1.5.x" "dir"
set_assets "docs-v1.5.x.tar.zst" "2026-03-19T03:55:34Z"
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"

RESULT=$(run_sync)
assert_eq "up to date skipped" "" "$RESULT"

# --- Test: non-version folders skipped ---

setup_test
set_folders "next" "dir" "v1.7.x" "dir"
set_assets
set_docs_date "v1.7.x" "2026-03-25T09:00:00Z"

RESULT=$(run_sync)
assert_eq "non-version folders skipped" "v1.7.x" "$RESULT"

# --- Test: files in listing skipped ---

setup_test
set_folders "readme.md" "file" "v1.7.x" "dir"
set_assets
set_docs_date "v1.7.x" "2026-03-25T09:00:00Z"

RESULT=$(run_sync)
assert_eq "files in listing skipped" "v1.7.x" "$RESULT"

# --- Test: versions below v1.5.x skipped ---

setup_test
set_folders "v0.52.x" "dir" "v1.4.x" "dir" "v1.5.x" "dir"
set_assets
set_docs_date "v0.52.x" "2026-02-26T11:00:00Z"
set_docs_date "v1.4.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"

RESULT=$(run_sync)
assert_eq "old versions skipped" "v1.5.x" "$RESULT"

# --- Test: real-world scenario (the v1.7.0 bug) ---

setup_test
set_folders "next" "dir" "v0.52.x" "dir" "v1.4.x" "dir" \
            "v1.5.x" "dir" "v1.6.x" "dir" "v1.7.x" "dir"
set_assets "docs-v1.5.x.tar.zst" "2026-03-19T03:55:34Z" \
           "docs-v1.6.x.tar.zst" "2026-03-19T03:55:39Z"
set_docs_date "v0.52.x" "2026-02-26T11:00:00Z"
set_docs_date "v1.4.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.6.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.7.x" "2026-03-25T09:00:00Z"

RESULT=$(run_sync)
assert_eq "v1.7.0 bug scenario" "v1.7.x" "$RESULT"

# --- Test: multiple missing and stale ---

setup_test
set_folders "v1.5.x" "dir" "v1.6.x" "dir" "v1.7.x" "dir" "v1.8.x" "dir"
set_assets "docs-v1.5.x.tar.zst" "2026-03-19T00:00:00Z" \
           "docs-v1.6.x.tar.zst" "2026-03-10T00:00:00Z"
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.6.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.7.x" "2026-03-25T09:00:00Z"
set_docs_date "v1.8.x" "2026-03-25T10:00:00Z"

RESULT=$(run_sync)
assert_eq "multiple missing and stale" "v1.6.x v1.7.x v1.8.x" "$RESULT"

# --- Test: no assets at all (fresh start) ---

setup_test
set_folders "v1.5.x" "dir" "v1.6.x" "dir"
set_assets
set_docs_date "v1.5.x" "2026-03-18T09:00:00Z"
set_docs_date "v1.6.x" "2026-03-18T09:00:00Z"

RESULT=$(run_sync)
assert_eq "fresh start, no assets" "v1.5.x v1.6.x" "$RESULT"

# --- Summary ---

echo ""
echo "$PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] || exit 1
