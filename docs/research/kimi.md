# Kimi Code — Sessions Research Dossier

> MD5-hashed work directories under `~/.kimi/sessions/<md5>/<uuid>/`
> containing `context.jsonl` + `wire.jsonl` + session state. Central
> `~/.kimi/kimi.json` metadata file tracks work_dir → last_session_id
> reverse map.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Package:** `kimi-cli` 1.30.0 (Homebrew; Python package `kimi_cli`)
- **Binary:** `/opt/homebrew/bin/kimi` → Python script launcher
- **Store root:** `~/.kimi/sessions/<md5(work_dir)>/<session-uuid>/`
- **Session files:**
  - `context.jsonl` — conversation history (append-only)
  - `wire.jsonl` — wire-protocol message log
  - `state.json` / session state (via `session_state.py`)
  - `subagents/` — nested sub-agent session dirs
- **Project key:** MD5 hex of `work_dir` path (utf-8 encoded)
- **Metadata index:** `~/.kimi/kimi.json` — work_dirs array with
  `path`, `kaos`, `last_session_id`
- **Session ID:** UUID v4 (minted via `uuid.uuid4()`)
- **Resume CLI:** `kimi --session [ID]` / `-S` / `-r` |
  `kimi --continue` / `-C`
- **Env override:** `KIMI_SHARE_DIR` (defaults to `~/.kimi`)
- **Platform:** macOS + Linux + Windows

## Sources

- Local install: `/opt/homebrew/Cellar/kimi-cli/1.30.0/libexec/lib/
  python3.14/site-packages/kimi_cli/`
- Verified source files:
  - `kimi_cli/share.py` — `get_share_dir() = KIMI_SHARE_DIR or ~/.kimi`
  - `kimi_cli/metadata.py` — `WorkDirMeta`, `get_metadata_file()`,
    `sessions_dir` property
  - `kimi_cli/session.py` — `Session` class, create/find/list/continue
  - `kimi_cli/session_state.py` — `SessionState` + persistence
- Docs: <https://code.kimi.com/>
- CLI help: `kimi --help` (flags `-S`/`-r`/`-C`/`-w`/`--add-dir`)
- Verified filesystem: `~/.kimi/skills/` exists; no `sessions/`
  or `kimi.json` yet (kimi has not been run interactively)

## Layout

```pseudocode
~/.kimi/                                       # or $KIMI_SHARE_DIR
├── kimi.json                                  # metadata index (all work_dirs)
├── sessions/
│   ├── <md5-of-work-dir>/                     # e.g. kimi runs in /project/foo
│   │   ├── <session-uuid>/
│   │   │   ├── context.jsonl                  # message history
│   │   │   ├── wire.jsonl                     # wire protocol log
│   │   │   ├── <session state files>          # via SessionState
│   │   │   └── subagents/
│   │   │       └── <sub-session-uuid>/
│   │   │           └── ...
│   │   └── <session-uuid-2>/
│   │       └── ...
│   └── <other-md5>/                           # different work_dir
│       └── ...
└── skills/                                    # user skills
```

- If `kaos != local_kaos.name` (multi-machine/remote), dir becomes
  `<kaos>_<md5>` (see `WorkDirMeta.sessions_dir`)
- Session dir is created lazily (`mkdir parents exist_ok`)
- On this machine: `sessions/` + `kimi.json` absent (only `skills/`
  populated) — kimi binary present but never run

## kimi.json — metadata index

```pseudocode
# ~/.kimi/kimi.json
{
  "work_dirs": [
    {
      "path": "/Users/jadb/.w/ideacrafterslabs/uhp/hops/main",
      "kaos": "local",
      "last_session_id": "6644b7d1-8d7d-45e5-9d25-04bd254b1bff"
    },
    {
      "path": "/Users/jadb/.w/ideacrafterslabs/clear",
      "kaos": "local",
      "last_session_id": null
    }
  ]
}
```

- **Central index** — unlike Codex/Vibe, Kimi has a persistent
  cwd → (md5, last_session_id) map
- Atomic writes via `utils/io.atomic_json_write`
- `kaos` field distinguishes environments (local vs. remote KAOS)
- Lookup: `metadata.get_work_dir_meta(path)` iterates `work_dirs`
  (O(n) linear scan; no hash map)

## Project key encoding

```pseudocode
# From WorkDirMeta.sessions_dir (metadata.py)
def sessions_dir(self):
  path_md5 = md5(self.path.encode('utf-8')).hexdigest()
  basename = path_md5 if self.kaos == local_kaos.name
             else f"{self.kaos}_{path_md5}"
  return get_share_dir() / "sessions" / basename
```

- **MD5**, not SHA256 (unlike OpenCode, Qwen, Gemini) — weakest
  hash of all researched CLIs for project keying; collision
  probability remains negligible at human scale but
  cryptographically broken
- Path is canonicalized via `KaosPath(...).canonical()` before
  hashing (see `Session.create/find/list/continue_`)
- **KAOS prefix** for non-local environments — kimi supports
  multiple machine/environment namespaces

## Transcript schema

### context.jsonl

- Append-only JSONL
- One message per line
- Role field: `user`, `assistant`, `system`, or `_internal`
  (underscore-prefix = non-visible / scaffolding)
- Schema is `kosong.message.Message` (external package);
  fields include at least `role` + `content` + tool-use support
- `Session.is_empty()` walks this file to detect non-scaffold
  messages

### wire.jsonl

- Wrapped by `WireFile` class (`kimi_cli/wire/file.py`)
- Records include `TurnBegin` (from `wire/types.py`); used for
  title derivation in `Session.refresh`
- Distinct from `context.jsonl`: wire log is turn-level protocol
  events (begin/end, tool calls), context is message stream

### session_state (persistent)

Stored separately via `save_session_state(state, session_dir)`.
Fields in `SessionState` (from `session_state.py`, summary):

```pseudocode
SessionState:
  custom_title: str | None
  title_generated: bool
  title_generate_attempts: int
  archived: bool
  archived_at: timestamp | None
  auto_archive_exempt: bool
  approval_settings: ...
  plan_mode: ...
  workspace_scope: ...
```

- State reload is concurrency-aware: `save_state()` reloads
  externally-mutable fields from disk to avoid overwriting
  concurrent web-API mutations
- **State lives at session level, not work_dir level** — each
  session has its own state file

## Resume / continue semantics

### `--continue` (`-C`)

```pseudocode
# Session.continue_(work_dir)
1. Load metadata
2. Get work_dir_meta for work_dir (None if never seen)
3. If last_session_id is None → None (no prior session)
4. Find session by id → rehydrate
```

- **Per-cwd scope.** Uses `work_dir_meta.last_session_id` — each
  work_dir has its own "most recent" pointer; no global last

### `--session <ID>` / `-S` / `-r`

- Without arg → **interactive picker** for current work_dir
- With arg → direct resume by UUID
- `Session.find(work_dir, session_id)` — scoped to work_dir only;
  cross-cwd resume not supported

### `--session` without ID

- Triggers `Session.list(work_dir)` → sorted by `updated_at` desc,
  filtered through `is_empty()` check

### Migration

`_migrate_session_context_file(work_dir_meta, session_id)` handles
older layout where context was at `sessions/<md5>/<session-id>.jsonl`
(flat) → new layout `sessions/<md5>/<session-id>/context.jsonl`
(nested). Migration happens on `find`/`list` access.

## Project grouping

- **Per-cwd via MD5 hash.** Clean separation at filesystem level.
- Forward lookup (cwd → hash): `metadata.get_work_dir_meta(path)`
  then compute `md5(path)`
- Backward lookup (hash → cwd): scan `kimi.json.work_dirs` and
  compare hashes; no native reverse index
- Worktrees land in distinct hashes cleanly

## Strengths

- **Central metadata file** (`kimi.json`) — best-of-both-worlds:
  Gemini-style central index + opaque hash sharding
- Per-cwd `last_session_id` → fast `--continue` without scanning
- Clean sub-agent nesting at filesystem level (`subagents/` dir)
- **Context vs. wire split**: message stream and protocol events
  are separate files — simpler parsing per concern
- Migration-aware: tolerates older flat layout
- Web API mutation handling → session state reloads on save
- KAOS namespacing supports multi-environment sessions
- Canonical path handling via `KaosPath` eliminates `..`/symlink
  variance

## Known gotchas

- **MD5 hash** — cryptographically broken but fine for
  filesystem keying; adapters must not assume SHA*
- Metadata index (`kimi.json`) is authoritative but held in memory
  by kimi process; concurrent CLI invocations race on write
  (`save_metadata` writes atomically but `load`-mutate-`save`
  cycle has a gap)
- `last_session_id` can point at a deleted session if `Session.delete()`
  ran without updating metadata → `--continue` silently creates
  new session in that case
- `context.jsonl` roles prefixed with `_` (underscore) are
  invisible in `is_empty()` check — adapters must not replay
  those to users
- Sub-agent sessions live at `sessions/<md5>/<session-id>/subagents/
  <sub-id>/` — NOT under their own work_dir md5; flatten carefully
- `wire.jsonl` schema is coupled to `kimi_cli.wire.types`; any
  version bump can break field names
- `~/.kimi/skills/` on this machine is populated via the agents
  skills system but no sessions — purely presence, not activity
- `KIMI_SHARE_DIR` env var can redirect everything; adapters must
  honor it
- Title is derived lazily via `Session.refresh()` walking
  `wire.jsonl`; expensive for long sessions

## Open questions

1. Exact `SessionState` persisted JSON shape (not dumped here)
2. Does sub-agent share metadata index or maintain its own?
3. What's in `kimi_cli.background/` (worker, store, manager) —
   is there a separate background task store the adapter should
   follow?
4. Does `Session.list` include archived sessions by default?
5. Is there a compaction mechanism, or does `context.jsonl` grow
   unbounded?
6. Does the web API (mentioned in `save_state` comment) write to
   the same filesystem location, or a parallel store?
7. What triggers `title_generate_attempts` — is there a backoff?

## Integration notes for USP

Kimi is **mid-low complexity** — central metadata is a gift.

Strategy:

1. **Bootstrap:** probe `~/.kimi/kimi.json`; create-on-demand
2. **Load metadata:** parse `kimi.json` → build cwd → md5 map
   (forward) + md5 → cwd map (reverse)
3. **Walk sessions:** for each known md5, walk
   `sessions/<md5>/<session-id>/` and load
   `context.jsonl` + `wire.jsonl` + state
4. **Subagent recursion:** descend into `subagents/` for each
   parent session; emit `session.spawned_by=<parent-id>`
5. **Watch:** fswatch `~/.kimi/sessions/` + `~/.kimi/kimi.json`;
   on metadata change, re-index; on jsonl append, emit turn events
6. **USP envelope translation:**
   - `context.jsonl` line → user/assistant/tool event
   - `wire.jsonl` `TurnBegin` → `turn.started`
   - Session state change → `session.state_changed`
7. **Respect `_`-prefixed roles:** exclude from user-facing
   transcripts (internal scaffolding)
8. **Honor `KIMI_SHARE_DIR`** env var for sandbox configs
9. **Handle MD5 collisions defensively** (not expected but cheap)

Expected adapter size: ~400 LOC Go (metadata reader + dual jsonl
tail + subagent recursion + migration-aware path resolution).
Simpler than OpenCode's 5-way split; more structured than Codex.
