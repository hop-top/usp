#!/usr/bin/env bash
# install-shai.sh — install SHAI (OVHcloud) CLI in a Debian/Ubuntu container.
# Upstream:   https://github.com/ovh/shai
# Installer:  https://raw.githubusercontent.com/ovh/shai/main/install.sh
# Verify:     shai --version
# Notes:      prebuilt Rust binary; installer drops into ~/.local/bin
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

export PATH="$HOME/.local/bin:$PATH"

if command -v shai >/dev/null 2>&1; then
  log "already installed: $(shai --version 2>/dev/null || echo shai)"
  exit 0
fi

ensure_base_prereqs

log "installing shai via official install script"
curl -fsSL https://raw.githubusercontent.com/ovh/shai/main/install.sh | sh

export PATH="$HOME/.local/bin:$PATH"

log "verify: $(shai --version 2>&1 | head -1 || echo shai installed)"
