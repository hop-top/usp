# Codex CLI — Sessions Research Dossier

> Date-partitioned JSONL rollouts with embedded cwd. No project
> grouping dir — reconstruction requires indexing the cwd field out
> of session_meta headers.

Last verified: 2026-04-08.

## Summary

- **Store root:** `~/.codex/sessions/<YYYY>/<MM>/<DD>/`
- **Session file:** `rollout-<ISO-timestamp>-<uuid>.jsonl`
- **Project key:** ❌ none — cwd embedded in first-line `session_meta`
  event only
- **Transcript format:** JSONL; first line is `session_meta`; remaining
  lines are turn events
- **Resume CLI:** `codex resume <session-id>` or `codex resume --last`
- **Scope:** global flat-ish store partitioned by date, not project
- **Archive:** `~/.codex/archived_sessions/` holds rotated/old sessions
- **Platform:** macOS + Linux + Windows

## Sources

- Sessions docs: <https://developers.openai.com/codex/sessions>
- Config: <https://developers.openai.com/codex/config-advanced>
- Resume reference: `codex resume --help`

## Layout

```pseudocode
~/.codex/
├── sessions/
│   ├── 2025/
│   │   └── 12/
│   │       └── 29/
│   │           └── rollout-2025-12-29T14-22-10-<uuid>.jsonl
│   ├── 2026/
│   │   ├── 03/
│   │   │   ├── 03/
│   │   │   │   └── rollout-2026-03-03T23-52-37-019cb730-d80b-...jsonl
│   │   │   └── 04/
│   │   │       ├── rollout-2026-03-04T06-08-34-...jsonl
│   │   │       └── rollout-2026-03-04T08-29-46-...jsonl
│   │   └── 04/
│   │       └── ...
├── archived_sessions/
│   └── <rotated files>
├── history.jsonl                    # cross-session prompt history
├── config.toml
└── logs_1.sqlite                    # telemetry/event log (not sessions)
```

## Session file naming

- Pattern: `rollout-<YYYY>-<MM>-<DD>T<HH>-<MM>-<SS>-<uuid>.jsonl`
- UUID is UUIDv7 (time-ordered) — stored in session_meta `id` field
- Filename timestamp is session start; directory partition matches
  the Y/M/D at start, not at each write
- Long sessions that cross midnight stay in the start-day partition

## Transcript schema

### First line — `session_meta`

```pseudocode
{
  "timestamp": "2026-03-04T04:55:04.183Z",
  "type": "session_meta",
  "payload": {
    "id": "<session-uuid>",
    "timestamp": "2026-03-04T04:52:37.517Z",
    "cwd": "/Users/jadb/.w/ideacrafterslabs/clear",
    "originator": "codex_cli_rs",
    "cli_version": "0.106.0",
    "source": "cli",
    "model_provider": "openai",
    "base_instructions": { "text": "..." }
  }
}
```

- **`cwd` is the only project-identifying field.** No separate dir.
- `originator` distinguishes CLI sessions from web/IDE
- Subsequent lines are turn events (user, assistant, tool_call,
  tool_result, reasoning) — not documented as stable schema

### Subsequent lines

```pseudocode
{"timestamp": "...", "type": "user_message", "payload": {...}}
{"timestamp": "...", "type": "tool_call", "payload": {...}}
{"timestamp": "...", "type": "tool_result", "payload": {...}}
```

## Resume / continue semantics

- `codex resume --last` — resume most recent session (any cwd)
- `codex resume <session-id>` — resume by UUID
- `codex resume` (no args) — interactive picker (TUI)
- Resume reads the JSONL, rehydrates conversation, appends new turns
  to the same file
- **No cwd scoping on resume** — can resume a session from a different
  cwd; cwd at resume time is *not* rewritten into the session_meta

## Project grouping

- **None native.** Sessions are date-partitioned, not project-partitioned.
- Reconstruction requires:
  1. Walk `~/.codex/sessions/**/*.jsonl`
  2. Read first line of each → extract `payload.cwd`
  3. Group by cwd (or by cwd prefix for worktree families)
- `~/.codex/history.jsonl` (cross-session prompt history) is flat and
  not per-project either

## Workaround strategy — 3 layers

### Layer 1 — cwd index

Maintain an external sidecar index keyed by cwd:

```pseudocode
# ~/.codex/sessions/.cwd-index.json (USP-managed)
{
  "/Users/jadb/.w/ideacrafterslabs/uhp": [
    "rollout-2026-03-03T23-52-37-019cb730-....jsonl",
    "rollout-2026-03-04T08-29-46-019cb90a-....jsonl"
  ],
  "/Users/jadb/.w/ideacrafterslabs/clear": [...]
}
```

Rebuild via `find + jq -n 'input | .payload.cwd'` over first lines.

### Layer 2 — filesystem watcher

Watch `~/.codex/sessions/<YYYY>/<MM>/<DD>/` with fswatch; on new
`.jsonl`, read head, emit `session.created` with decoded cwd.

### Layer 3 — history.jsonl correlation

`~/.codex/history.jsonl` holds prompts cross-session. Correlate with
session files by timestamp + content hash to build prompt-level
project attribution when session files are archived.

## Known gotchas

- **No project dir** — walking dated partitions is mandatory to group
  by project; doing it on every event is expensive
- `cwd` is only in the first line — if file is truncated or first
  line is malformed, project attribution is lost
- `archived_sessions/` rotation is opaque; no manifest of what moved
- Resume across different cwds silently works but pollutes semantic
  meaning (session says cwd=A but user is in B)
- UUIDv7 filenames are time-ordered lexicographically, but the day
  partition breaks global sort without recursion
- `history.jsonl` is per-user, not per-session — useful for prompt
  search but not transcript replay

## Open questions

1. Does resume from a different cwd append a second `session_meta`
   event or silently mutate subsequent events' implicit cwd?
2. What triggers archival to `archived_sessions/` — size, age, or
   manual? Is there a config knob?
3. Is the per-line schema (beyond `session_meta`) documented
   anywhere, or must adapters reverse-engineer it per version?
4. Does `codex resume` accept cwd scoping (`--cwd <path>`) or is it
   strictly session-id based?

## Integration notes for USP

Codex is the hardest session adapter. Required layers:

- **Minimum viable:** Layer 1 (cwd index, rebuilt nightly)
  → project grouping works, ~5 min staleness
- **Full:** Layer 1 + Layer 2 (fswatch) → real-time grouping

Expected adapter size: ~400 LOC Go (date-partition walker + head-line
reader + cwd indexer + fswatch bridge).

USP must own the cwd index; Codex won't provide one. The index
doubles as the bridge between Codex's flat store and USP's
project-keyed session graph.
