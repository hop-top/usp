#!/usr/bin/env bash
# install-windsurf.sh — attempt to install Windsurf in a Debian/Ubuntu container.
# Upstream:   https://windsurf.com/
#
# Windsurf is a VS Code fork desktop IDE (Codeium). Its only CLI surface is an
# optional `windsurf` shim added to PATH to launch the GUI — not a headless
# agent. There is no documented headless/server install mode usable inside a
# non-graphical Linux container.
#
# Does NOT fail the overall pipeline: logs a warning and exits 0.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

if command -v windsurf >/dev/null 2>&1; then
  log "already installed: $(windsurf --version 2>/dev/null || echo windsurf)"
  exit 0
fi

log "WARNING: Windsurf is a VS Code fork GUI application (Codeium)."
log "WARNING: no headless container install is supported by upstream."
log "WARNING: see https://windsurf.com/ to download the desktop build."
log "skipping windsurf install — exit 0"
exit 0
