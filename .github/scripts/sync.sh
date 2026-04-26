#!/usr/bin/env bash
set -euo pipefail

# Finds doc bundle versions that need building.
# Compares k6-docs version folders against existing bundle assets.
# Outputs space-separated list of versions to build.
#
# Required env: GH_TOKEN
# Usage: .github/scripts/bundle-sync.sh

MIN_VERSION="v0.47.x"

# Get existing bundle assets with their upload dates.
declare -A ASSET_DATES
ASSETS=$(gh release view doc-bundles --json assets \
  -q '.assets[] | select(.name | endswith(".tar.zst")) | "\(.name) \(.updatedAt)"' 2>/dev/null || true)

if [ -n "$ASSETS" ]; then
  while IFS=' ' read -r name date; do
    version="${name#docs-}"
    version="${version%.tar.zst}"
    ASSET_DATES["$version"]="$date"
  done <<< "$ASSETS"
fi

# List version folders in k6-docs and compare.
VERSIONS=""
FOLDERS=$(gh api repos/grafana/k6-docs/contents/docs/sources/k6 \
  --jq '.[] | select(.type == "dir") | .name' 2>/dev/null)

for FOLDER in $FOLDERS; do
  # Skip non-version folders (e.g. "next") and versions below minimum.
  [[ "$FOLDER" =~ ^v[0-9]+\.[0-9]+\.x$ ]] || continue
  [[ "$FOLDER" < "$MIN_VERSION" ]] && continue

  DOCS_DATE=$(gh api "repos/grafana/k6-docs/commits?per_page=1&path=docs/sources/k6/${FOLDER}" \
    --jq '.[0].commit.committer.date' 2>/dev/null || echo "")

  ASSET_DATE="${ASSET_DATES[$FOLDER]:-}"

  if [ -z "$ASSET_DATE" ]; then
    echo "$FOLDER: missing bundle" >&2
    VERSIONS="${VERSIONS}${FOLDER} "
  elif [ -n "$DOCS_DATE" ] && [[ "$DOCS_DATE" > "$ASSET_DATE" ]]; then
    echo "$FOLDER: stale (docs=$DOCS_DATE asset=$ASSET_DATE)" >&2
    VERSIONS="${VERSIONS}${FOLDER} "
  else
    echo "$FOLDER: up to date" >&2
  fi
done

echo "$VERSIONS" | xargs
