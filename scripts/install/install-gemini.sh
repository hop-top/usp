#!/usr/bin/env bash
# install-gemini.sh — install Gemini CLI (Google) in a Debian/Ubuntu container.
# Upstream:   https://github.com/google-gemini/gemini-cli
# Package:    https://www.npmjs.com/package/@google/gemini-cli
# Verify:     gemini --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v gemini >/dev/null 2>&1; then
  log "already installed: $(gemini --version 2>/dev/null || echo gemini)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @google/gemini-cli via npm"
$SUDO npm install -g @google/gemini-cli

log "verify: $(gemini --version)"
