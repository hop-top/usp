# OpenCode — Sessions Research Dossier

> SHA1-hashed project keys + SQLite (Drizzle ORM) primary store
> with JSON file mirror. Rich schema: sessions, messages, parts,
> todos, workspaces, permissions, events, accounts. Project
> reverse-lookup requires reading a JSON sidecar per hash.

Last verified: 2026-04-11.

## Summary

- **Primary store:** `~/.local/share/opencode/opencode.db`
  (SQLite via Drizzle ORM, ~272MB typical)
- **Mirror store:** `~/.local/share/opencode/storage/<kind>/`
- **Project key:** SHA1 hex of absolute cwd (40 chars)
- **Project metadata:** `storage/project/<hash>.json`
  (worktree, vcs, timestamps)
- **Session file:**
  `storage/session/<project-hash>/ses_<id>.json`
- **Message/part split:** `message/<ses-id>/msg_*` and
  `part/<msg-id>/prt_*` (parts nest under message, not session)
- **Resume:** TUI session picker; `opencode --session <id>`
- **Platform:** macOS + Linux + Windows

## Sources

- Sessions docs: <https://opencode.ai/docs/sessions/>
- Storage reference:
  <https://deepwiki.com/sst/opencode/storage>
- Plugins SDK: <https://opencode.ai/docs/plugins/>

## Layout

```pseudocode
~/.local/share/opencode/
├── opencode.db                  # SQLite primary (Drizzle ORM)
├── opencode.db-shm
├── opencode.db-wal
├── auth.json
├── bin/                         # bundled binaries
├── snapshot/                    # content snapshots (ROOT level)
│   └── <project-hash>/         # per-project snapshot dirs
├── storage/
│   ├── migration                # schema version marker (integer)
│   ├── project/
│   │   ├── <hash>.json          # one JSON per project hash
│   │   └── ...
│   ├── session/
│   │   └── <project-hash>/
│   │       ├── ses_<id>.json
│   │       └── ...
│   ├── message/
│   │   └── <session-id>/
│   │       └── msg_*.json
│   ├── part/
│   │   └── <message-id>/        # keyed by msg ID, not ses ID
│   │       └── prt_*.json
│   ├── session_diff/            # flat ses_*.json files
│   └── todo/                    # flat ses_*.json files
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

- Pattern: `ses_<26-char-mixed-case-alphanum>`
- Example: `ses_41a008b58ffeqNcmm2m48ptgWD`
- Time-ordered (ULID-family), globally unique
- Mixed-case alphanumeric (not strictly base32)
- Nested under project hash dir for filesystem locality

## Data model — SQLite schema (Drizzle ORM)

SQLite is canonical; filesystem mirrors for tooling/debug.
Schema managed by Drizzle ORM (`__drizzle_migrations` table).

### Core tables

| Table            | Key columns (beyond id/timestamps)                                                   |
|------------------|--------------------------------------------------------------------------------------|
| `project`        | `worktree`, `vcs`, `name`, `icon_url`, `icon_color`, `sandboxes`, `commands`         |
| `session`        | `project_id` FK, `parent_id`, `slug`, `directory`, `title`, `version`, `share_url`,  |
|                  | `summary_additions/deletions/files/diffs`, `revert`, `permission`, `workspace_id`    |
| `message`        | `session_id` FK (CASCADE), `data` JSON blob                                          |
| `part`           | `message_id` FK (CASCADE), `session_id`, `data` JSON blob                            |
| `todo`           | `session_id` FK (CASCADE), `content`, `status`, `priority`, `position` (composite PK)|
| `permission`     | `project_id` FK (CASCADE), `data` JSON blob                                          |
| `workspace`      | `project_id` FK, `branch`, `type`, `name`, `directory`, `extra`                      |
| `session_share`  | `session_id` FK, `secret`, `url`                                                     |
| `account`        | `email`, `url`, `access_token`, `refresh_token`, `token_expiry`                      |
| `account_state`  | singleton; `active_account_id` FK, `active_org_id`                                   |
| `control_account`| legacy/parallel auth; `email`+`url` composite PK                                     |
| `event_sequence` | `aggregate_id` PK, `seq`                                                             |
| `event`          | `aggregate_id` FK, `seq`, `type`, `data` — event-sourcing support                    |

### Filesystem mirror kinds

| Kind           | Path                              | Structure                          |
|----------------|-----------------------------------|------------------------------------|
| `project`      | `storage/project/<hash>.json`     | one JSON per project               |
| `session`      | `storage/session/<hash>/ses_*`    | session header per project dir     |
| `message`      | `storage/message/<ses-id>/msg_*`  | msgs nested under session          |
| `part`         | `storage/part/<msg-id>/prt_*`     | parts nested under MESSAGE id      |
| `todo`         | `storage/todo/ses_*.json`         | flat files, not nested dirs        |
| `session_diff` | `storage/session_diff/ses_*.json` | flat files, diffs per session      |
| `snapshot`     | `snapshot/<project-hash>/`        | ROOT level, not under `storage/`   |

- Rehydrating = session.json + msg_* + prt_* + todo joined by IDs
- `snapshot/` and `session_diff/` provide code-aware provenance

## Resume / continue semantics

- TUI session picker on startup (list of recent sessions for current cwd)
- `opencode --session <ses-id>` — direct resume
- Resume re-fetches session from SQLite (not file mirror)
- `session.created`, `session.idle`, `session.compacted`,
  `session.deleted` plugin events fire on lifecycle transitions

## Strengths

- Only CLI with SQLite (Drizzle ORM) primary store → fast queries
- Only CLI with normalized project metadata
  (`project/<hash>.json`) including vcs + timestamps
- Only CLI with message/part split → streaming-safe rehydration
- Only CLI with native session diffs + content snapshots
- Event-sourcing tables (`event`, `event_sequence`) suggest
  future replay/audit capability
- Workspace table enables multi-branch session isolation
- Worktrees get distinct hashes (different cwd → different hash)
- Per-session todo store is file-based + inspectable without SQL

## Known gotchas

- **SHA1 hash is opaque** — no way to identify a project without
  reading the sidecar JSON or grepping the `worktree` field
- **Two sources of truth:** SQLite is canonical, filesystem mirror
  can drift; USP must decide which to follow
- `opencode.db` gets large (272MB+) — full scans are expensive;
  prefer indexed queries (session_id, project_hash)
- No top-level project dir listing — must walk
  `storage/project/*.json` and map cwd → hash
- **Part nesting is by message, not session** —
  `storage/part/<msg-id>/prt_*.json`; grouping by session
  requires joining through message table
- **Todo and session_diff are flat** — `ses_*.json` files
  directly in their dirs, not nested by session/project
- **`snapshot/` lives at ROOT** — `~/.local/share/opencode/snapshot/`,
  NOT under `storage/`; easy to miss
- **`migration` lives under `storage/`** — not at root;
  currently contains integer "2"
- Schema managed by Drizzle ORM; can change between opencode
  versions; not officially documented

## Open questions

1. ~~Is SQLite schema documented?~~ No. Drizzle ORM manages it;
   reverse-engineer via `sqlite3 .schema`
2. Does filesystem mirror lag SQLite writes, or are they atomic?
3. What does `session_diff` contain — per-turn diffs or
   full-session diffs? Can USP reuse for change attribution?
4. Are `snapshot/` entries content-addressed (CAS-style) and
   deduplicated across sessions?
5. Does `time.initialized` differ from `time.created` because of
   lazy init, or does it track first prompt vs project creation?
6. What is the `event`/`event_sequence` table used for — is this
   an internal event bus or user-facing audit log?
7. How does `workspace` relate to `session` — is it for
   multi-branch or multi-directory workflows?

## Integration notes for USP

OpenCode is mid-to-high complexity. Strategy:

1. **Primary read path:** SQLite via read-only connection with
   Drizzle migration version check (integer in
   `storage/migration`)
2. **Fallback / indexing:** walk `storage/project/*.json` to build
   hash → cwd map; use for discovery + sanity checks
3. Watch `storage/session/<hash>/` and `storage/message/<id>/`
   with fswatch for live tailing; SQLite WAL alternatively via
   `PRAGMA wal_autocheckpoint`
4. Translate OpenCode multi-table model to USP envelope by joining
   session + message + part + todo into unified event stream
5. Account for workspace/permission/event tables — they may
   carry context needed for full session reconstruction
6. Respect SQLite as source of truth on conflict
7. Note `snapshot/` at root level when building file watchers

Expected adapter size: ~600 LOC Go (SQLite reader + project index
builder + multi-kind event joiner + workspace resolver +
fswatch bridge).
