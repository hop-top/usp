#!/usr/bin/env bash
# install-trae.sh — attempt to install Trae in a Debian/Ubuntu container.
# Upstream:   https://trae.ai/
#
# Trae is an AI-powered desktop IDE (adaptive AI IDE). Upstream docs reference
# VS Code / JetBrains plugins and state "Trae is coming to Linux soon" — no
# Linux build, no headless mode, no documented CLI. Cannot run inside a
# non-graphical container.
#
# Does NOT fail the overall pipeline: logs a warning and exits 0.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v trae >/dev/null 2>&1; then
  log "already installed: $(trae --version 2>/dev/null || echo trae)"
  exit 0
fi

log "WARNING: Trae is a desktop AI IDE; no Linux or headless build."
log "WARNING: upstream states Linux support 'coming soon'; no CLI documented."
log "WARNING: see https://trae.ai/ for desktop downloads."
log "skipping trae install — exit 0"
exit 0
