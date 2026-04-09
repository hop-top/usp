#!/usr/bin/env bash
# install-all.sh — install all AI coding CLI agents inside a Debian/Ubuntu container.
#
# Usage:
#   sudo bash install-all.sh              # install everything
#   bash install-all.sh                    # if already root, or sudo is available
#
# Order (grouped):
#   base prereqs
#   → npm-based:    claude codex gemini opencode amp qwen kimi
#                   qoder auggie iflow pi codebuddy
#   → python/uv:    vibe
#   → curl/other:   cursor-agent forge kiro-cli shai tabnine copilot
#   → GUI stubs:    agy windsurf trae
#
# On failure of any single agent, logs an error and continues to the next.
# Prints a status summary at the end.
set -uo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_common.sh
. "$SCRIPT_DIR/_common.sh"

AGENTS=(
  # npm-based
  claude
  codex
  gemini
  opencode
  amp
  qwen
  kimi
  qoder
  auggie
  iflow
  pi
  codebuddy
  # python / uv-based
  vibe
  # curl installers / other
  cursor-agent
  forge
  kiro-cli
  shai
  tabnine
  copilot
  # GUI stubs (no-op inside containers)
  agy
  windsurf
  trae
)

declare -A STATUS=()

banner() {
  printf '\n==> installing %s\n' "$1"
}

log "installing base prereqs (curl, ca-certificates, git)"
if ensure_base_prereqs; then
  log "base prereqs OK"
else
  log "FAILED: base prereqs"
fi

for agent in "${AGENTS[@]}"; do
  script="$SCRIPT_DIR/install-${agent}.sh"
  if [ ! -x "$script" ] && [ ! -r "$script" ]; then
    log "FAILED: $agent (script not found: $script)"
    STATUS[$agent]="FAIL"
    continue
  fi
  banner "$agent"

  # Pre-check: was it already installed before we ran?
  pre_installed="no"
  case "$agent" in
    cursor-agent) cmd="cursor-agent" ;;
    copilot)      cmd="gh" ;;
    agy)          cmd="agy" ;;
    windsurf)     cmd="windsurf" ;;
    trae)         cmd="trae" ;;
    qoder)        cmd="qodercli" ;;
    kiro-cli)     cmd="kiro-cli" ;;
    *)            cmd="$agent" ;;
  esac
  if command -v "$cmd" >/dev/null 2>&1; then
    pre_installed="yes"
  fi

  if bash "$script"; then
    case "$agent" in
      # GUI stubs intentionally skip in containers
      agy|windsurf|trae)
        STATUS[$agent]="SKIP"
        ;;
      *)
        if [ "$pre_installed" = "yes" ]; then
          STATUS[$agent]="OK (already)"
        else
          STATUS[$agent]="OK"
        fi
        ;;
    esac
  else
    rc=$?
    log "FAILED: $agent (exit $rc)"
    STATUS[$agent]="FAIL"
  fi
done

printf '\n==================== summary ====================\n'
printf '%-14s %s\n' "AGENT" "STATUS"
printf '%-14s %s\n' "-----" "------"
for agent in "${AGENTS[@]}"; do
  printf '%-14s %s\n' "$agent" "${STATUS[$agent]:-UNKNOWN}"
done
printf '=================================================\n'

# Exit non-zero if anything fully failed (ignore SKIP).
for agent in "${AGENTS[@]}"; do
  if [ "${STATUS[$agent]:-}" = "FAIL" ]; then
    exit 1
  fi
done
exit 0
