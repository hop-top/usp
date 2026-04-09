#!/usr/bin/env bash
# install-opencode.sh — install opencode (sst) in a Debian/Ubuntu container.
# Upstream:   https://github.com/sst/opencode
# Installer:  https://opencode.ai/install
# Verify:     opencode --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v opencode >/dev/null 2>&1; then
  log "already installed: $(opencode --version 2>/dev/null || echo opencode)"
  exit 0
fi

ensure_base_prereqs

log "installing opencode via official install script"
curl -fsSL https://opencode.ai/install | bash

# Official installer puts opencode in ~/.opencode/bin by default; make it visible.
export PATH="$HOME/.opencode/bin:$HOME/.local/bin:$PATH"

if ! command -v opencode >/dev/null 2>&1; then
  log "official installer path not found — falling back to npm (opencode-ai)"
  ensure_node
  $SUDO npm install -g opencode-ai
fi

log "verify: $(opencode --version)"
