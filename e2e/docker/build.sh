#!/usr/bin/env bash
# Build E2E Docker images for each version combo.
#
# Usage:
#   bash e2e/docker/build.sh                    # build all combos
#   bash e2e/docker/build.sh latest             # build one combo
#   bash e2e/docker/build.sh latest mixed       # build specific combos
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RESOLVE="${REPO_ROOT}/e2e/scripts/resolve-matrix.sh"
DOCKERFILE="${REPO_ROOT}/e2e/docker/Dockerfile"
ALL_COMBOS=("latest" "minimum" "mixed")

combos=("${@:-}")
if [[ ${#combos[@]} -eq 0 || -z "${combos[0]}" ]]; then
  combos=("${ALL_COMBOS[@]}")
fi

built=()

for combo in "${combos[@]}"; do
  printf '\n==> Building combo: %s\n' "$combo"

  json="$("$RESOLVE" --combo "$combo")"

  claude_ver="$(echo "$json"  | jq -r '.claude')"
  codex_ver="$(echo "$json"   | jq -r '.codex')"
  gemini_ver="$(echo "$json"  | jq -r '.gemini')"
  opencode_ver="$(echo "$json"| jq -r '.opencode')"

  tag="usp-e2e:${combo}"

  docker build \
    --build-arg CLAUDE_VERSION="$claude_ver" \
    --build-arg CODEX_VERSION="$codex_ver" \
    --build-arg GEMINI_VERSION="$gemini_ver" \
    --build-arg OPENCODE_VERSION="$opencode_ver" \
    -t "$tag" \
    -f "$DOCKERFILE" \
    "$REPO_ROOT"

  built+=("$tag")
done

printf '\n==> Built images:\n'
for img in "${built[@]}"; do
  printf '  %s\n' "$img"
done
