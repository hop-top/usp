#!/usr/bin/env bash
# install-agy.sh — attempt to install Antigravity (Google) in a Debian/Ubuntu container.
# Upstream:   https://antigravity.google/
#
# Antigravity is a VS Code fork distributed as a desktop application (.deb / .dmg /
# .exe). It is a GUI IDE and has no documented headless/server install mode. It
# cannot be meaningfully used inside a non-graphical Linux container.
#
# This script intentionally does NOT fail the overall install pipeline: it logs
# a clear warning and exits 0 so install-all.sh can continue.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v agy >/dev/null 2>&1; then
  log "already installed: $(agy --version 2>/dev/null || echo agy)"
  exit 0
fi

log "WARNING: Antigravity (agy) is a VS Code fork GUI application."
log "WARNING: no headless container install is supported by upstream."
log "WARNING: see https://antigravity.google/ to download the desktop build."
log "skipping agy install — exit 0"
exit 0
