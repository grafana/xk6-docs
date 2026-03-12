#!/usr/bin/env bash
# Validates all lookup paths in skill references against the actual docs.
# Run after a k6 version bump to find broken/missing paths.
#
# Usage: ./skills/xk6-docs/scripts/validate-paths.sh [path-to-k6-binary]

set -euo pipefail

K6="${1:-./k6}"
REFS_DIR="$(dirname "$0")/../references"

if [[ ! -x "$K6" ]]; then
  echo "error: k6 binary not found at $K6" >&2
  echo "usage: $0 [path-to-k6-binary]" >&2
  exit 1
fi

# Extract lookup paths from the last column of markdown tables.
# Matches: "| ... | `javascript-api k6-http get` |" or "| ... | `using-k6 thresholds` |"
# Only picks up paths starting with known top-level categories.
paths=$(grep -hoE '`(javascript-api|using-k6|using-k6-browser|testing-guides|results-output|examples)[- a-z0-9/]+`' "$REFS_DIR"/*.md \
  | tr -d '\`' \
  | sort -u)

total=0
failed=0
broken=()

while IFS= read -r path; do
  [[ -z "$path" ]] && continue
  total=$((total + 1))
  # shellcheck disable=SC2086
  if ! "$K6" x docs $path >/dev/null 2>&1; then
    failed=$((failed + 1))
    broken+=("$path")
  fi
done <<< "$paths"

echo "Tested $total paths"

if [[ $failed -gt 0 ]]; then
  echo ""
  echo "BROKEN ($failed):"
  for p in "${broken[@]}"; do
    echo "  $p"
  done
fi

# Check for new modules not covered by any reference.
echo ""
echo "Checking for uncovered modules..."
# Get all children under javascript-api from the depth-2 TOC.
doc_modules=$("$K6" x docs --depth 2 2>/dev/null \
  | sed -n '/^- javascript-api$/,/^- [^ ]/{ /^  - /p; }' \
  | sed 's/^  - //' | sort)
SKILL_DIR="$(dirname "$0")/.."
skill_modules=$(grep -rhoE 'javascript-api [a-z0-9.-]+' "$SKILL_DIR"/SKILL.md "$REFS_DIR"/*.md \
  | sed 's/javascript-api //' | sort -u)

uncovered=()
while IFS= read -r mod; do
  [[ -z "$mod" ]] && continue
  if ! echo "$skill_modules" | grep -qxF "$mod"; then
    uncovered+=("$mod")
  fi
done <<< "$doc_modules"

if [[ ${#uncovered[@]} -gt 0 ]]; then
  echo "UNCOVERED modules (${#uncovered[@]}):"
  for m in "${uncovered[@]}"; do
    echo "  javascript-api $m"
  done
  exit_code=1
else
  echo "All modules covered."
fi

if [[ $failed -gt 0 || ${#uncovered[@]} -gt 0 ]]; then
  exit 1
else
  echo ""
  echo "Skill is up to date."
fi
