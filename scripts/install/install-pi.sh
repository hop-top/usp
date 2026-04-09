#!/usr/bin/env bash
# install-pi.sh — install Pi coding agent CLI in a Debian/Ubuntu container.
# Upstream:   https://github.com/badlogic/pi-mono
# Package:    https://www.npmjs.com/package/@mariozechner/pi-coding-agent
# Verify:     pi --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v pi >/dev/null 2>&1; then
  log "already installed: $(pi --version 2>/dev/null || echo pi)"
  exit 0
fi

ensure_base_prereqs
ensure_node

log "installing @mariozechner/pi-coding-agent via npm"
$SUDO npm install -g @mariozechner/pi-coding-agent

log "verify: $(pi --version 2>&1 | head -1 || echo pi installed)"
