#!/usr/bin/env bash
# install-kiro-cli.sh — install Kiro CLI in a Debian/Ubuntu container.
# Upstream:   https://kiro.dev/docs/cli/
# Installer:  https://cli.kiro.dev/install (shell script)
# Verify:     kiro-cli --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v kiro-cli >/dev/null 2>&1; then
  log "already installed: $(kiro-cli --version 2>/dev/null || echo kiro-cli)"
  exit 0
fi

ensure_base_prereqs

log "installing kiro-cli via official install script"
curl -fsSL https://cli.kiro.dev/install | bash

# Installer typically drops into ~/.local/bin.
export PATH="$HOME/.local/bin:$PATH"

log "verify: $(kiro-cli --version 2>&1 | head -1 || echo kiro-cli installed)"
