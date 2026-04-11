#!/usr/bin/env bash
# Run an E2E scenario in a Docker container.
#
# Usage:
#   bash e2e/docker/run-scenario.sh <combo> <scenario>
#   bash e2e/docker/run-scenario.sh latest single_session
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

combo="${1:?Usage: run-scenario.sh <combo> <scenario>}"
scenario="${2:?Usage: run-scenario.sh <combo> <scenario>}"

image="usp-e2e:${combo}"
cassette_dir="${REPO_ROOT}/e2e/cassettes/${combo}/${scenario}"

mkdir -p "$cassette_dir"

docker run --rm \
  -v "${cassette_dir}:/cassettes" \
  -e XRR_MODE=record \
  -e XRR_CASSETTE_DIR=/cassettes \
  -w /src/usp \
  "$image" \
  bash "/src/usp/e2e/scenarios/${scenario}.sh"
