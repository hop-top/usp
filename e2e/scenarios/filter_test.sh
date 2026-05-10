#!/usr/bin/env bash
# E2E: session list filter tests (--cli, --project, --limit, --since, compound)
set -euo pipefail

PASS=0; FAIL=0

assert_eq() {
  local got="$1" want="$2" msg="$3"
  if [[ "$got" == "$want" ]]; then
    echo "PASS: $msg (got $got)"
    PASS=$((PASS+1))
  else
    echo "FAIL: $msg — want $want, got $got"
    FAIL=$((FAIL+1))
  fi
}

assert_le() {
  local got="$1" max="$2" msg="$3"
  if (( got <= max )); then
    echo "PASS: $msg (got $got)"
    PASS=$((PASS+1))
  else
    echo "FAIL: $msg — want <= $max, got $got"
    FAIL=$((FAIL+1))
  fi
}

# ── Seed sessions across CLIs and projects ──────────────────────────

PROJECT_A="/tmp/usp-e2e/project-a"
PROJECT_B="/tmp/usp-e2e/project-b"
mkdir -p "$PROJECT_A" "$PROJECT_B"

echo "--- Seeding sessions ---"

# Claude in project-a
(cd "$PROJECT_A" && claude --dangerously-skip-permissions -p "say hello" --output-format json) \
  || true

# Codex in project-b
(cd "$PROJECT_B" && codex exec "say hello") \
  || true

# Gemini in project-a
(cd "$PROJECT_A" && gemini -p "say hello") \
  || true

# OpenCode in project-b
(cd "$PROJECT_B" && opencode -p "say hello") \
  || true

# Verify at least some sessions seeded before testing filters.
all_json=$(usp session list --format json 2>/dev/null || echo "[]")
total=$(echo "$all_json" | jq 'length' 2>/dev/null || echo "0")
if [[ "$total" -eq 0 ]]; then
  echo "SKIP: No sessions seeded (API keys likely absent)."
  exit 0
fi
echo "Seeded $total sessions."

echo ""
echo "--- Running filter tests ---"

# Helper: safe json length (treats empty/missing stdout as 0).
json_len() {
  local raw="${1:-[]}"
  if [[ -z "$raw" ]]; then raw="[]"; fi
  echo "$raw" | jq 'length' 2>/dev/null || echo "0"
}

# ── --cli filter ───────────────────────────────────────────────────

result=$(usp session list --cli claude --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "1" "--cli claude returns 1 session"

result=$(usp session list --cli codex --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "1" "--cli codex returns 1 session"

# ── --project filter ────────────────────────────────────────────────

result=$(usp session list --project "$PROJECT_A" --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "2" "--project project-a returns 2 sessions"

result=$(usp session list --project "$PROJECT_B" --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "2" "--project project-b returns 2 sessions"

# ── --limit filter ──────────────────────────────────────────────────

result=$(usp session list --limit 2 --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_le "$count" "2" "--limit 2 returns at most 2"

result=$(usp session list --limit 1 --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_le "$count" "1" "--limit 1 returns at most 1"

# ── --since filter ──────────────────────────────────────────────────

result=$(usp session list --since 1h --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "4" "--since 1h returns all 4 recent sessions"

result=$(usp session list --since 1m --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "4" "--since 1m returns all 4 recent sessions"

# ── Compound: --cli + --project ────────────────────────────────────

result=$(usp session list --cli claude --project "$PROJECT_A" --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "1" "--cli claude + --project project-a = 1"

result=$(usp session list --cli claude --project "$PROJECT_B" --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "0" "--cli claude + --project project-b = 0"

result=$(usp session list --cli codex --project "$PROJECT_B" --format json 2>/dev/null || echo "[]")
count=$(json_len "$result")
assert_eq "$count" "1" "--cli codex + --project project-b = 1"

# ── Summary ─────────────────────────────────────────────────────────

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ "$FAIL" -eq 0 ]] || exit 1
