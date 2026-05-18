#!/usr/bin/env bash
# install-forge.sh — install Forge Code CLI in a Debian/Ubuntu container.
# Upstream:   https://forgecode.dev/
# Installer:  https://forgecode.dev/cli (shell script)
# Package:    https://www.npmjs.com/package/@antinomyhq/forge (alt install)
# Verify:     forge --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v forge >/dev/null 2>&1; then
  log "already installed: $(forge --version 2>/dev/null || echo forge)"
  exit 0
fi

ensure_base_prereqs

log "installing forge via official install script"
curl -fsSL https://forgecode.dev/cli | sh

# Installer may drop into ~/.local/bin.
export PATH="$HOME/.local/bin:$PATH"

log "verify: $(forge --version 2>&1 | head -1 || echo forge installed)"
