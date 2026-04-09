#!/usr/bin/env bash
# install-claude.sh — install Claude Code CLI (Anthropic) in a Debian/Ubuntu container.
# Upstream:   https://www.npmjs.com/package/@anthropic-ai/claude-code
# Docs:       https://docs.claude.com/en/docs/claude-code
# Verify:     claude --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v claude >/dev/null 2>&1; then
  log "already installed: $(claude --version 2>/dev/null || echo claude)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @anthropic-ai/claude-code via npm"
$SUDO npm install -g @anthropic-ai/claude-code

log "verify: $(claude --version)"
