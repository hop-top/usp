#!/usr/bin/env bash
# install-kimi.sh — install Kimi Code CLI (Moonshot AI) in a Debian/Ubuntu container.
# Upstream:   https://github.com/MoonshotAI/kimi-cli
# Docs:       https://moonshotai.github.io/kimi-cli/en/guides/getting-started.html
# PyPI:       https://pypi.org/project/kimi-cli/
# Verify:     kimi --version
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

export PATH="$HOME/.local/bin:$PATH"

if command -v kimi >/dev/null 2>&1; then
  log "already installed: $(kimi --version 2>/dev/null || echo kimi)"
  if command -v uv >/dev/null 2>&1; then
    uv tool upgrade kimi-cli --no-cache >/dev/null 2>&1 || true
  fi
  exit 0
fi

ensure_base_prereqs
ensure_uv

log "installing kimi-cli via uv tool (python 3.13)"
uv tool install --python 3.13 kimi-cli

log "verify: $(kimi --version)"
