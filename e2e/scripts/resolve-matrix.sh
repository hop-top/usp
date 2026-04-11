#!/usr/bin/env bash
# resolve-matrix.sh — resolve matrix.yaml minors to exact patch versions.
# Usage: resolve-matrix.sh [--combo latest|mixed|oldest]
set -euo pipefail
PATH="/opt/homebrew/bin:$HOME/.local/bin:$PATH"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MATRIX="$SCRIPT_DIR/../matrix.yaml"
COMBO="latest"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --combo) COMBO="$2"; shift 2 ;;
    *) echo "usage: $0 [--combo latest|mixed|oldest]" >&2; exit 1 ;;
  esac
done

# Parse clis block from matrix.yaml (name, package, minors array).
declare -A PKGS OLDEST MID NEWEST
while IFS= read -r line; do
  if [[ "$line" =~ ^[[:space:]]{2}([a-z]+):$ ]]; then
    cli="${BASH_REMATCH[1]}"
  elif [[ "$line" =~ package:\ *\"([^\"]+)\" ]]; then
    PKGS[$cli]="${BASH_REMATCH[1]}"
  elif [[ "$line" =~ minors:\ *\[(.+)\] ]]; then
    raw="${BASH_REMATCH[1]}"
    IFS=',' read -ra parts <<< "$raw"
    cleaned=()
    for p in "${parts[@]}"; do
      cleaned+=("$(echo "$p" | tr -d ' "')")
    done
    OLDEST[$cli]="${cleaned[0]}"
    MID[$cli]="${cleaned[$(( ${#cleaned[@]} / 2 ))]}"
    NEWEST[$cli]="${cleaned[${#cleaned[@]}-1]}"
  fi
done < "$MATRIX"

# Resolve a package@minor to its latest patch version.
resolve() {
  local pkg="$1" minor="$2"
  local raw ver
  raw=$(npm view "${pkg}@~${minor}.0" version 2>/dev/null | tail -1) || true
  if [[ -z "$raw" ]]; then
    raw=$(npm view "${pkg}@${minor}" version 2>/dev/null | tail -1) || true
  fi
  # npm may print "pkg@ver 'ver'" when multiple matches; extract semver.
  ver=$(echo "$raw" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | tail -1)
  echo "${ver:-unknown}"
}

# Select minor per CLI based on combo.
declare -A SELECTED
for cli in "${!PKGS[@]}"; do
  case "$COMBO" in
    latest)  SELECTED[$cli]="${NEWEST[$cli]}" ;;
    oldest)  SELECTED[$cli]="${OLDEST[$cli]}" ;;
    mixed)
      case "$cli" in
        claude)   SELECTED[$cli]="${OLDEST[$cli]}" ;;
        codex)    SELECTED[$cli]="${NEWEST[$cli]}" ;;
        gemini)   SELECTED[$cli]="${MID[$cli]}" ;;
        opencode) SELECTED[$cli]="${OLDEST[$cli]}" ;;
        *)        SELECTED[$cli]="${NEWEST[$cli]}" ;;
      esac ;;
    *) echo "unknown combo: $COMBO" >&2; exit 1 ;;
  esac
done

# Resolve and emit JSON.
echo "{"
first=true
for cli in claude codex gemini opencode; do
  [[ -z "${PKGS[$cli]:-}" ]] && continue
  ver=$(resolve "${PKGS[$cli]}" "${SELECTED[$cli]}")
  $first && first=false || echo ","
  printf '  "%s": "%s"' "$cli" "$ver"
done
echo ""
echo "}"
