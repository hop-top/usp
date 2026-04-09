# OpenCode — Sessions Research Dossier

> SHA1-hashed project keys + SQLite primary store with JSON file
> mirror. Cleanest separation of session/message/part/todo concerns
> of any CLI, but project reverse-lookup requires reading a JSON
> sidecar per hash.

Last verified: 2026-04-08.

## Summary

- **Primary store:** `~/.local/share/opencode/opencode.db` (SQLite, ~270MB typical)
- **Mirror store:** `~/.local/share/opencode/storage/<kind>/<project-hash>/`
- **Project key:** SHA1 hex of absolute cwd (40 chars)
- **Project metadata:** `storage/project/<hash>.json` (worktree, vcs, timestamps)
- **Session file:** `storage/session/<project-hash>/ses_<id>.json`
- **Message/part split:** separate `message/` and `part/` dirs per session
- **Resume:** TUI session picker; `opencode --session <id>`
- **Platform:** macOS + Linux + Windows

## Sources

- Sessions docs: <https://opencode.ai/docs/sessions/>
- Storage reference: <https://deepwiki.com/sst/opencode/storage>
- Plugins SDK: <https://opencode.ai/docs/plugins/>

## Layout

```pseudocode
~/.local/share/opencode/
├── opencode.db                            # SQLite primary (sessions+messages+parts)
├── opencode.db-shm
├── opencode.db-wal
├── auth.json
├── migration                              # schema version marker
├── storage/
│   ├── project/
│   │   ├── 4e2f4754d19ed904f11553caadcd97fca696dcaa.json
│   │   ├── 5b788e42594b6300c1df079e07652c19da2cade6.json
│   │   └── ...                            # one JSON per project hash
│   ├── session/
│   │   ├── 4e2f4754d19ed904f11553caadcd97fca696dcaa/
│   │   │   ├── ses_3a8bb9976ffe9A5l1tEw0rtiBJ.json
│   │   │   ├── ses_40cd6ce5dffeCVqGe9YTN0WT28.json
│   │   │   └── ...
│   │   └── <other-hash>/
│   ├── message/
│   │   └── <session-id>/
│   │       └── msg_*.json
│   ├── part/
│   │   └── <session-id>/
│   │       └── prt_*.json
│   ├── session_diff/                      # computed diffs per session
│   ├── todo/
│   │   └── <session-id>/
│   │       └── todo_*.json
│   └── snapshot/                          # git-like content snapshots
├── log/
├── plans/
└── tool-output/
```

## Project key — SHA1 of cwd

- Rule: `sha1(absolute_cwd).hexdigest()`
- Example: `/Users/jadb/Repositories/oss-xray-codes`
  → `4e2f4754d19ed904f11553caadcd97fca696dcaa`
- **Opaque.** Reverse lookup requires reading the sidecar JSON:

```pseudocode
# storage/project/4e2f4754d19ed904f11553caadcd97fca696dcaa.json
{
  "id": "4e2f4754d19ed904f11553caadcd97fca696dcaa",
  "worktree": "/Users/jadb/Repositories/oss-xray-codes",
  "vcs": "git",
  "sandboxes": [],
  "time": {
    "created": 1768622779374,
    "updated": 1769080596165,
    "initialized": 1769080626876
  }
}
```

- `worktree` field is the ground-truth cwd
- `vcs` indicates git/hg/none
- `sandboxes` array for sandbox-isolated runs
- `time` is epoch-ms (not ISO)

## Session ID format

- Pattern: `ses_<26-char-base32-ulid-ish>`
- Example: `ses_3a8bb9976ffe9A5l1tEw0rtiBJ`
- Time-ordered (ULID-family), globally unique
- Nested under project hash dir for filesystem locality

## Data model — 5-way split

OpenCode is the only CLI that splits session state across 5 distinct
kinds:

| Kind          | Path                              | Purpose                          |
|---------------|-----------------------------------|----------------------------------|
| `project`     | `storage/project/<hash>.json`     | cwd + vcs metadata               |
| `session`     | `storage/session/<hash>/ses_*`    | session header + title + state   |
| `message`     | `storage/message/<ses-id>/msg_*`  | user/assistant messages          |
| `part`        | `storage/part/<ses-id>/prt_*`     | message parts (streaming chunks) |
| `todo`        | `storage/todo/<ses-id>/todo_*`    | todo list state                  |
| `session_diff`| `storage/session_diff/`           | computed code diffs per session  |
| `snapshot`    | `storage/snapshot/`               | content-addressed file snapshots |

- SQLite holds canonical data; filesystem mirror for tooling/debug
- Rehydrating a session = load session.json + all msg_* + all prt_*
  + all todo_* joined by session_id
- `session_diff/` and `snapshot/` are unique: code-aware provenance

## Resume / continue semantics

- TUI session picker on startup (list of recent sessions for current cwd)
- `opencode --session <ses-id>` — direct resume
- Resume re-fetches session from SQLite (not file mirror)
- `session.created`, `session.idle`, `session.compacted`,
  `session.deleted` plugin events fire on lifecycle transitions

## Strengths

- Only CLI with SQLite primary store → fast queries across sessions
- Only CLI with normalized project metadata (`project/<hash>.json`)
  including vcs + timestamps
- Only CLI with message/part split → streaming-safe rehydration
- Only CLI with native session diffs + content snapshots
- Worktrees get distinct hashes cleanly (different cwd → different hash)
- Per-session todo store is file-based and inspectable without SQL

## Known gotchas

- **SHA1 hash is opaque** — no way to identify a project without
  reading the sidecar JSON or grepping the `worktree` field
- **Two sources of truth:** SQLite is canonical, filesystem mirror
  can drift; USP must decide which to follow
- `opencode.db` gets large (270MB+) — full scans are expensive;
  prefer indexed queries (session_id, project_hash)
- No top-level project dir listing — must walk `storage/project/*.json`
  and map cwd → hash
- `storage/message/<session-id>/` is *not* nested under project hash —
  flat per-session, so two projects can't share msg dirs by accident
  but grouping requires joining on session_id
- Schema can change between opencode versions; `migration` file
  tracks current version but the SQLite schema isn't officially
  documented

## Open questions

1. Is the SQLite schema documented anywhere, or must adapters
   reverse-engineer via `sqlite3 .schema`?
2. Does filesystem mirror lag SQLite writes, or are they atomic?
3. What does `session_diff` contain — per-turn diffs or
   full-session diffs? Can USP reuse it for change attribution?
4. Are `snapshot/` entries content-addressed (CAS-style) and
   deduplicated across sessions?
5. Does `time.initialized` differ from `time.created` because of
   lazy init, or does it track first prompt vs project creation?

## Integration notes for USP

OpenCode is mid-complexity. Strategy:

1. **Primary read path:** SQLite via read-only connection with
   explicit schema version check against `migration` file
2. **Fallback / indexing:** walk `storage/project/*.json` to build
   hash → cwd map; use for discovery + sanity checks
3. Watch `storage/session/<hash>/` and `storage/message/<id>/` with
   fswatch for live tailing; SQLite WAL alternatively via
   `PRAGMA wal_autocheckpoint`
4. Translate OpenCode 5-way split to USP envelope by joining
   session + message + part + todo into a unified event stream
5. Respect SQLite as source of truth on conflict

Expected adapter size: ~500 LOC Go (SQLite reader + project index
builder + multi-kind event joiner + fswatch bridge).
