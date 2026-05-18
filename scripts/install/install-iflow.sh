#!/usr/bin/env bash
# install-iflow.sh — install iFlow CLI in a Debian/Ubuntu container.
# Upstream:   https://docs.iflow.cn/en/cli/quickstart
# Package:    https://www.npmjs.com/package/@iflow-ai/iflow-cli
# Verify:     iflow --version
# Requires:   Node.js >= 22
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v iflow >/dev/null 2>&1; then
  log "already installed: $(iflow --version 2>/dev/null || echo iflow)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @iflow-ai/iflow-cli via npm"
$SUDO npm install -g @iflow-ai/iflow-cli

log "verify: $(iflow --version 2>&1 | head -1 || echo iflow installed)"
