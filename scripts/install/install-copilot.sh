#!/usr/bin/env bash
# install-copilot.sh — install GitHub Copilot CLI (gh extension) in a Debian/Ubuntu container.
# Upstream:   https://github.com/github/gh-copilot
# Verify:     gh copilot --version
#
# NOTE: requires `gh auth login` at runtime. This script installs the extension
# but cannot authenticate non-interactively; callers must provide GH_TOKEN or
# run `gh auth login` inside the container for the extension to be usable.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

install_gh() {
  if command -v gh >/dev/null 2>&1; then
    return 0
  fi
  ensure_base_prereqs
  log "installing gh (GitHub CLI) via official apt repo"
  $SUDO install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | $SUDO tee /etc/apt/keyrings/githubcli-archive-keyring.gpg >/dev/null
  $SUDO chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | $SUDO tee /etc/apt/sources.list.d/github-cli.list >/dev/null
  $SUDO apt-get update -y
  DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y --no-install-recommends gh
}

install_gh

if gh extension list 2>/dev/null | grep -q 'github/gh-copilot'; then
  log "gh-copilot already installed; upgrading"
  gh extension upgrade github/gh-copilot || true
else
  if ! gh auth status >/dev/null 2>&1; then
    log "WARNING: gh is not authenticated — extension install may fail."
    log "         set GH_TOKEN env var or run 'gh auth login' before running this script."
  fi
  log "installing gh extension github/gh-copilot"
  gh extension install github/gh-copilot
fi

log "verify: $(gh copilot --version 2>&1 | head -1 || echo 'gh copilot installed')"
