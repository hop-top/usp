#!/usr/bin/env bash
# _common.sh — shared helpers for install-*.sh scripts.
# Sourced by per-agent scripts. Safe to source multiple times.

# Do not set -euo here; caller owns its shell options.

log() { printf '[%s] %s\n' "$(date +%H:%M:%S)" "$*"; }

# sudo wrapper — use sudo if not root and sudo exists; otherwise bare.
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
  fi
fi

ensure_apt_pkg() {
  # ensure_apt_pkg <pkg> [<pkg> ...]
  local missing=()
  local p
  for p in "$@"; do
    if ! dpkg -s "$p" >/dev/null 2>&1; then
      missing+=("$p")
    fi
  done
  if [ ${#missing[@]} -gt 0 ]; then
    log "apt: installing ${missing[*]}"
    $SUDO apt-get update -y
    DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y --no-install-recommends "${missing[@]}"
  fi
}

ensure_base_prereqs() {
  ensure_apt_pkg ca-certificates curl git
}

ensure_node() {
  if command -v node >/dev/null 2>&1 && command -v npm >/dev/null 2>&1; then
    return 0
  fi
  log "installing nodejs + npm via apt"
  ensure_apt_pkg nodejs npm
}

ensure_uv() {
  if command -v uv >/dev/null 2>&1; then
    return 0
  fi
  ensure_base_prereqs
  log "installing uv (astral) via official installer"
  curl -LsSf https://astral.sh/uv/install.sh | sh
  # uv installs to ~/.local/bin by default.
  export PATH="$HOME/.local/bin:$PATH"
}
