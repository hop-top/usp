#!/usr/bin/env bash
# install-codex.sh — install OpenAI Codex CLI in a Debian/Ubuntu container.
# Upstream:   https://github.com/openai/codex
# Package:    https://www.npmjs.com/package/@openai/codex
# Verify:     codex --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v codex >/dev/null 2>&1; then
  log "already installed: $(codex --version 2>/dev/null || echo codex)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @openai/codex via npm"
$SUDO npm install -g @openai/codex

log "verify: $(codex --version)"
