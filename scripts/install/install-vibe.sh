#!/usr/bin/env bash
# install-vibe.sh — install Mistral Vibe CLI in a Debian/Ubuntu container.
# Upstream:   https://github.com/mistralai/mistral-vibe
# PyPI:       https://pypi.org/project/mistral-vibe/
# Installer:  https://mistral.ai/vibe/install.sh
# Verify:     vibe --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

export PATH="$HOME/.local/bin:$PATH"

if command -v vibe >/dev/null 2>&1; then
  log "already installed: $(vibe --version 2>/dev/null || echo vibe)"
  # Upgrade idempotently if uv-managed.
  if command -v uv >/dev/null 2>&1; then
    uv tool upgrade mistral-vibe --no-cache >/dev/null 2>&1 || true
  fi
  exit 0
fi

ensure_base_prereqs
ensure_uv

log "installing mistral-vibe via uv tool"
uv tool install mistral-vibe

log "verify: $(vibe --version)"
