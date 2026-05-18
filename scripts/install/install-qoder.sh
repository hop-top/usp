#!/usr/bin/env bash
# install-qoder.sh — install Qoder CLI in a Debian/Ubuntu container.
# Upstream:   https://qoder.com/cli
# Package:    https://www.npmjs.com/package/@qoder-ai/qodercli
# Verify:     qodercli --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v qodercli >/dev/null 2>&1; then
  log "already installed: $(qodercli --version 2>/dev/null || echo qodercli)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @qoder-ai/qodercli via npm"
$SUDO npm install -g @qoder-ai/qodercli

log "verify: $(qodercli --version 2>&1 | head -1 || echo qodercli installed)"
