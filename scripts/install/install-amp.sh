#!/usr/bin/env bash
# install-amp.sh — install Amp (Sourcegraph) CLI in a Debian/Ubuntu container.
# Upstream:   https://ampcode.com/
# Package:    https://www.npmjs.com/package/@sourcegraph/amp
# Verify:     amp --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v amp >/dev/null 2>&1; then
  log "already installed: $(amp --version 2>/dev/null || echo amp)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @sourcegraph/amp via npm"
$SUDO npm install -g @sourcegraph/amp

log "verify: $(amp --version)"
