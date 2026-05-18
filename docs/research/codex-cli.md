# Codex CLI — Sessions Research Dossier

> Date-partitioned JSONL rollouts with embedded cwd. No project
> grouping dir — reconstruction requires indexing the cwd field out
> of session_meta headers.

Last verified: 2026-04-11.

## Summary

- **Store root:** `~/.codex/sessions/<YYYY>/<MM>/<DD>/`
- **Session file:** `rollout-<ISO-timestamp>-<uuid>.jsonl`
- **Project key:** none — cwd embedded in first-line `session_meta`
  event only
- **Transcript format:** JSONL; first line is `session_meta`;
  remaining lines are `response_item` events
- **Resume CLI:** `codex resume <session-id>` or
  `codex resume --last`
- **Scope:** global flat-ish store partitioned by date, not project
- **Archive:** `~/.codex/archived_sessions/` holds rotated/old
  sessions (flat dir, no date partitions)
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
│   │   └── 10/
│   │       └── 04/
│   │           └── rollout-2025-10-04T03-00-25-<uuid>.jsonl
│   └── 2026/
│       ├── 03/
│       │   ├── 03/
│       │   │   └── rollout-...jsonl
│       │   └── 04/
│       │       └── rollout-...jsonl
│       └── 04/
│           └── ...
├── archived_sessions/           # flat; no date subdirs
│   └── rollout-...jsonl
├── session_index.jsonl          # thread-name keyed session index
├── history.jsonl                # cross-session prompt history
├── config.toml
├── auth.json                    # auth credentials
├── version.json                 # tracks latest version + check time
├── logs_2.sqlite                # telemetry/event log (not sessions)
├── state_5.sqlite               # app state
├── models_cache.json            # cached model list
├── .codex-global-state.json     # global UI/app state
├── AGENTS.md                    # user agent instructions
├── automations/                 # scheduled automations
├── cache/
├── log/
├── memories/                    # persistent memory store
├── plugins/
├── rules/
├── shell_snapshots/
├── skills/
├── sqlite/
├── tmp/
├── vendor_imports/
└── worktrees/
```

## Session file naming

- Pattern:
  `rollout-<YYYY>-<MM>-<DD>T<HH>-<MM>-<SS>-<uuid>.jsonl`
- UUID is UUIDv7 (time-ordered) — stored in session_meta `id`
- Filename timestamp is session start; directory partition matches
  the Y/M/D at start, not at each write
- Long sessions that cross midnight stay in the start-day partition

## Transcript schema

### First line — `session_meta`

Schema evolved across CLI versions. Fields marked (v0.106+) were
absent in older releases (e.g. v0.44 used `instructions: null`
instead of `base_instructions`).

```pseudocode
{
  "timestamp": "2026-04-09T17:34:10.498Z",
  "type": "session_meta",
  "payload": {
    "id": "<session-uuid>",
    "timestamp": "2026-04-09T17:32:48.478Z",
    "cwd": "/Users/jadb/.w/ideacrafterslabs/xray/hops/main",
    "originator": "Codex Desktop",       # or "codex_cli_rs"
    "cli_version": "0.119.0-alpha.11",
    "source": "vscode",                  # or "cli"
    "model_provider": "openai",          # v0.106+; absent earlier
    "base_instructions": {               # v0.106+; was "instructions"
      "text": "..."
    },
    "dynamic_tools": [...],              # v0.119+; optional
    "git": {                             # enriched in v0.119+
      "commit_hash": "...",
      "branch": "main",
      "repository_url": "git@github.com:..."
    }
  }
}
```

- **`cwd` is the only project-identifying field.** No separate dir.
- `originator` values: `codex_cli_rs` (terminal), `Codex Desktop`
  (VS Code extension)
- `source` values: `cli` (terminal), `vscode` (extension)
- `git` field: empty `{}` in older versions; populated in v0.119+
- `dynamic_tools`: array of tool definitions injected by Desktop;
  absent in plain CLI sessions

### Subsequent lines

Actual type is `response_item` (not `user_message`/`tool_call`).
The `payload` carries the Responses-API item shape.

```pseudocode
{
  "timestamp": "...",
  "type": "response_item",
  "payload": {
    "type": "message",
    "role": "user",
    "content": [{"type": "input_text", "text": "..."}]
  }
}
```

Turn roles observed: `user`, `assistant`. Tool calls and results
are embedded inside `response_item` payloads, not as separate
top-level event types.

## session_index.jsonl

Codex maintains `~/.codex/session_index.jsonl` — one JSON object
per line mapping session IDs to thread names and update timestamps:

```pseudocode
{
  "id": "<session-uuid>",
  "thread_name": "Plan Go CLI refactor with charmbrace",
  "updated_at": "2026-03-06T04:30:43.674067Z"
}
```

This index is Codex-native (not USP-managed). USP can read it to
correlate session IDs with human-readable thread names without
parsing every JSONL file.

## history.jsonl

Cross-session prompt history. Each line contains a session_id,
unix timestamp, and the prompt text:

```pseudocode
{"session_id": "<uuid>", "ts": 1760018173, "text": "."}
```

Flat, not per-project. Useful for prompt search but not transcript
replay.

## Resume / continue semantics

- `codex resume --last` — resume most recent session (any cwd)
- `codex resume <session-id>` — resume by UUID
- `codex resume` (no args) — interactive picker (TUI)
- Resume reads the JSONL, rehydrates conversation, appends new
  turns to the same file
- **No cwd scoping on resume** — can resume a session from a
  different cwd; cwd at resume time is *not* rewritten into the
  session_meta

## Project grouping

- **None native.** Sessions are date-partitioned, not
  project-partitioned.
- Reconstruction requires:
  1. Walk `~/.codex/sessions/**/*.jsonl`
  2. Read first line of each -> extract `payload.cwd`
  3. Group by cwd (or by cwd prefix for worktree families)
- `session_index.jsonl` provides thread-name metadata but not
  cwd — still need to read session files for project grouping

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

Rebuild via `find + jq -n 'input | .payload.cwd'` over first
lines.

### Layer 2 — filesystem watcher

Watch `~/.codex/sessions/<YYYY>/<MM>/<DD>/` with fswatch; on new
`.jsonl`, read head, emit `session.created` with decoded cwd.

### Layer 3 — history.jsonl correlation

`history.jsonl` holds prompts cross-session. Correlate with
session files by timestamp + content hash to build prompt-level
project attribution when session files are archived.

## Known gotchas

- **No project dir** — walking dated partitions is mandatory to
  group by project; doing it on every event is expensive
- `cwd` is only in the first line — if file is truncated or first
  line is malformed, project attribution is lost
- `archived_sessions/` is flat (no date subdirs); rotation is
  opaque; no manifest of what moved
- Resume across different cwds silently works but pollutes
  semantic meaning (session says cwd=A but user is in B)
- UUIDv7 filenames are time-ordered lexicographically, but the
  day partition breaks global sort without recursion
- `history.jsonl` is per-user, not per-session — useful for
  prompt search but not transcript replay
- **Schema drift** — `session_meta` payload shape changes across
  CLI versions without a version field in the schema itself;
  adapters must handle missing fields gracefully
- **Subsequent line type is `response_item`** — not
  `user_message`/`tool_call` as might be expected; uses
  Responses API item envelope

## Open questions

1. Does resume from a different cwd append a second
   `session_meta` event or silently mutate subsequent events'
   implicit cwd?
2. What triggers archival to `archived_sessions/` — size, age,
   or manual? Is there a config knob?
3. Is the per-line schema (beyond `session_meta`) documented
   anywhere, or must adapters reverse-engineer it per version?
4. Does `codex resume` accept cwd scoping (`--cwd <path>`) or
   is it strictly session-id based?
5. What populates `dynamic_tools` in Desktop sessions — is it
   extension-managed or server-driven?

## Integration notes for USP

Codex is the hardest session adapter. Required layers:

- **Minimum viable:** Layer 1 (cwd index, rebuilt nightly)
  -> project grouping works, ~5 min staleness
- **Full:** Layer 1 + Layer 2 (fswatch) -> real-time grouping

Expected adapter size: ~400 LOC Go (date-partition walker +
head-line reader + cwd indexer + fswatch bridge).

USP must own the cwd index; Codex won't provide one. The index
doubles as the bridge between Codex's flat store and USP's
project-keyed session graph.

Adapter must handle schema drift: check for both `instructions`
(old) and `base_instructions` (new) fields. The
`session_index.jsonl` file is a bonus — can enrich USP session
metadata with thread names without extra file I/O.
