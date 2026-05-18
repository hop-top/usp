#!/usr/bin/env bash
# install-codebuddy.sh — install CodeBuddy CLI (Tencent) in a Debian/Ubuntu container.
# Upstream:   https://www.codebuddy.ai/docs/cli/installation
# Package:    https://www.npmjs.com/package/@tencent-ai/codebuddy-code
# Verify:     codebuddy --version
# Requires:   Node.js >= 18.20
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v codebuddy >/dev/null 2>&1; then
  log "already installed: $(codebuddy --version 2>/dev/null || echo codebuddy)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @tencent-ai/codebuddy-code via npm"
$SUDO npm install -g @tencent-ai/codebuddy-code

log "verify: $(codebuddy --version 2>&1 | head -1 || echo codebuddy installed)"
