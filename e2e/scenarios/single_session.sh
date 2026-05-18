#!/usr/bin/env bash
# single_session.sh — drive a complete session lifecycle for each core CLI.
# Runs inside Docker with XRR_MODE=record, XRR_CASSETTE_DIR=/cassettes.
# API keys may be absent; script prints WARN and continues.
set -euo pipefail

PROJECT_BASE="/tmp/usp-e2e"
PASS=0; WARN=0; FAIL=0

status() {
  local level="$1" cli="$2" msg="$3"
  echo "${level}: [${cli}] ${msg}"
  case "$level" in
    PASS) PASS=$((PASS+1)) ;; WARN) WARN=$((WARN+1)) ;; FAIL) FAIL=$((FAIL+1)) ;;
  esac
}

# ---------- Claude Code ----------
run_claude() {
  local dir="$PROJECT_BASE/claude-test"
  mkdir -p "$dir" && cd "$dir"
  git init -q . 2>/dev/null || true

  # Non-interactive: --print sends prompt, prints response, exits.
  echo "List files in /tmp" \
    | claude --print 2>/dev/null || true

  echo "What is 2+2?" \
    | claude --print --continue 2>/dev/null || true

  # Store: ~/.claude/projects/<project-key>/<session>.jsonl
  local store="$HOME/.claude/projects"
  if compgen -G "${store}"/*/*.jsonl >/dev/null 2>&1; then
    status PASS claude "session JSONL found in ${store}"
  else
    status WARN claude "no session JSONL (API key missing or store path changed)"
  fi
}

# ---------- Codex CLI ----------
run_codex() {
  local dir="$PROJECT_BASE/codex-test"
  mkdir -p "$dir" && cd "$dir"
  git init -q . 2>/dev/null || true

  # Non-interactive: codex exec runs a single prompt then exits.
  codex exec "List files in /tmp" 2>/dev/null || true
  codex exec "What is 2+2?" 2>/dev/null || true

  # Store: ~/.codex/sessions/<YYYY>/<MM>/<DD>/rollout-*.jsonl
  local store="$HOME/.codex/sessions"
  if compgen -G "${store}"/*/*/*/rollout-*.jsonl >/dev/null 2>&1; then
    status PASS codex "session rollout found in ${store}"
  else
    status WARN codex "no session rollout (API key missing or store path changed)"
  fi
}

# ---------- Gemini CLI ----------
run_gemini() {
  local dir="$PROJECT_BASE/gemini-test"
  mkdir -p "$dir" && cd "$dir"
  git init -q . 2>/dev/null || true

  # Non-interactive: pipe prompt via stdin; gemini reads stdin when not a tty.
  echo "List files in /tmp" | gemini 2>/dev/null || true
  echo "What is 2+2?" | gemini 2>/dev/null || true

  # Store: ~/.gemini/history/<alias>/<chat-tag>.json + projects.json
  local store="$HOME/.gemini/history"
  if [ -d "$store" ] && [ "$(ls -A "$store" 2>/dev/null)" ]; then
    status PASS gemini "history dir populated in ${store}"
  else
    status WARN gemini "no history entries (API key missing or store path changed)"
  fi
}

# ---------- OpenCode ----------
run_opencode() {
  local dir="$PROJECT_BASE/opencode-test"
  mkdir -p "$dir" && cd "$dir"
  git init -q . 2>/dev/null || true

  # Non-interactive: opencode --non-interactive or pipe mode.
  echo "List files in /tmp" | opencode 2>/dev/null || true
  echo "What is 2+2?" | opencode 2>/dev/null || true

  # Store: ~/.local/share/opencode/storage/session/<hash>/ses_*.json
  local store="$HOME/.local/share/opencode/storage/session"
  if compgen -G "${store}"/*/ses_*.json >/dev/null 2>&1; then
    status PASS opencode "session JSON found in ${store}"
  else
    status WARN opencode "no session JSON (API key missing or store path changed)"
  fi
}

# ---------- Main ----------
echo "=== USP E2E: single_session scenario ==="
echo "XRR_MODE=${XRR_MODE:-unset}  XRR_CASSETTE_DIR=${XRR_CASSETTE_DIR:-unset}"
echo ""

for cli in claude codex gemini opencode; do
  echo "--- ${cli} ---"
  "run_${cli}"
  echo ""
done

echo "=== Results: PASS=${PASS} WARN=${WARN} FAIL=${FAIL} ==="
[ "$FAIL" -eq 0 ] || exit 1
