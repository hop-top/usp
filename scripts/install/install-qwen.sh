#!/usr/bin/env bash
# install-qwen.sh — install Qwen Code CLI in a Debian/Ubuntu container.
# Upstream:   https://github.com/QwenLM/qwen-code
# Package:    https://www.npmjs.com/package/@qwen-code/qwen-code
# Verify:     qwen --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v qwen >/dev/null 2>&1; then
  log "already installed: $(qwen --version 2>/dev/null || echo qwen)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @qwen-code/qwen-code via npm"
$SUDO npm install -g @qwen-code/qwen-code

log "verify: $(qwen --version)"
