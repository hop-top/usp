#!/usr/bin/env bash
# install-cursor-agent.sh — install Cursor Agent CLI in a Debian/Ubuntu container.
# Upstream:   https://cursor.com/
# Installer:  https://cursor.com/install (cursor-agent-installer.sh)
# Verify:     cursor-agent --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v cursor-agent >/dev/null 2>&1; then
  log "already installed: $(cursor-agent --version 2>/dev/null || echo cursor-agent)"
  exit 0
fi

ensure_base_prereqs

log "installing cursor-agent via official install script"
curl -fsS https://cursor.com/install | bash

# Installer drops binary into ~/.local/bin typically.
export PATH="$HOME/.local/bin:$PATH"

log "verify: $(cursor-agent --version)"
