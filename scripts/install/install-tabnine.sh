#!/usr/bin/env bash
# install-tabnine.sh — install Tabnine CLI in a Debian/Ubuntu container.
# Upstream:   https://docs.tabnine.com/main/getting-started/tabnine-cli
# Install:    curl $TABNINE_HOST/update/cli/installer.mjs | node --input-type=module - $TABNINE_HOST
# Verify:     tabnine --version
#
# Tabnine CLI install requires TABNINE_HOST (e.g. https://console.tabnine.com
# or an on-prem console URL). There is no public generic installer URL — the
# installer is served by each customer console. Export TABNINE_HOST before
# running, or this script will exit 1.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

export PATH="$HOME/.local/bin:$PATH"

if command -v tabnine >/dev/null 2>&1; then
  log "already installed: $(tabnine --version 2>/dev/null || echo tabnine)"
  exit 0
fi

if [ -z "${TABNINE_HOST:-}" ]; then
  log "ERROR: TABNINE_HOST is not set."
  log "ERROR: Tabnine CLI installer is served by your Tabnine console."
  log "ERROR: Example: export TABNINE_HOST=https://console.tabnine.com"
  log "ERROR: See https://docs.tabnine.com/main/getting-started/tabnine-cli"
  exit 1
fi

ensure_base_prereqs
ensure_node

log "installing tabnine from $TABNINE_HOST"
curl -fsSL "$TABNINE_HOST/update/cli/installer.mjs" \
  | node --input-type=module - "$TABNINE_HOST"

log "verify: $(tabnine --version 2>&1 | head -1 || echo tabnine installed)"
