#!/usr/bin/env bash
# install-auggie.sh — install Auggie CLI (Augment Code) in a Debian/Ubuntu container.
# Upstream:   https://docs.augmentcode.com/cli/overview
# Package:    https://www.npmjs.com/package/@augmentcode/auggie
# Verify:     auggie --version
# Requires:   Node.js >= 22
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v auggie >/dev/null 2>&1; then
  log "already installed: $(auggie --version 2>/dev/null || echo auggie)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @augmentcode/auggie via npm"
$SUDO npm install -g @augmentcode/auggie

log "verify: $(auggie --version 2>&1 | head -1 || echo auggie installed)"
