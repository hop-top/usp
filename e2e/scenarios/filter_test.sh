#!/usr/bin/env bash
# E2E: session list filter tests (--tool, --project, --limit, --since, compound)
set -euo pipefail

PASS=0; FAIL=0

assert_eq() {
  local got="$1" want="$2" msg="$3"
  if [[ "$got" == "$want" ]]; then
    echo "PASS: $msg (got $got)"
    ((PASS++))
  else
    echo "FAIL: $msg — want $want, got $got"
    ((FAIL++))
  fi
}

assert_le() {
  local got="$1" max="$2" msg="$3"
  if (( got <= max )); then
    echo "PASS: $msg (got $got)"
    ((PASS++))
  else
    echo "FAIL: $msg — want <= $max, got $got"
    ((FAIL++))
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

echo ""
echo "--- Running filter tests ---"

# ── --tool filter ───────────────────────────────────────────────────

result=$(usp session list --tool claude --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "1" "--tool claude returns 1 session"

result=$(usp session list --tool codex --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "1" "--tool codex returns 1 session"

# ── --project filter ────────────────────────────────────────────────

result=$(usp session list --project "$PROJECT_A" --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "2" "--project project-a returns 2 sessions"

result=$(usp session list --project "$PROJECT_B" --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "2" "--project project-b returns 2 sessions"

# ── --limit filter ──────────────────────────────────────────────────

result=$(usp session list --limit 2 --format json)
count=$(echo "$result" | jq 'length')
assert_le "$count" "2" "--limit 2 returns at most 2"

result=$(usp session list --limit 1 --format json)
count=$(echo "$result" | jq 'length')
assert_le "$count" "1" "--limit 1 returns at most 1"

# ── --since filter ──────────────────────────────────────────────────

result=$(usp session list --since 1h --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "4" "--since 1h returns all 4 recent sessions"

result=$(usp session list --since 1m --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "4" "--since 1m returns all 4 recent sessions"

# ── Compound: --tool + --project ────────────────────────────────────

result=$(usp session list --tool claude --project "$PROJECT_A" --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "1" "--tool claude + --project project-a = 1"

result=$(usp session list --tool claude --project "$PROJECT_B" --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "0" "--tool claude + --project project-b = 0"

result=$(usp session list --tool codex --project "$PROJECT_B" --format json)
count=$(echo "$result" | jq 'length')
assert_eq "$count" "1" "--tool codex + --project project-b = 1"

# ── Summary ─────────────────────────────────────────────────────────

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ "$FAIL" -eq 0 ]] || exit 1
