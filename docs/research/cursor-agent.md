# Cursor Agent CLI — Sessions Research Dossier

> SQLite-per-session with CAS blob storage. Sessions nested under an
> opaque user hash, NOT per-project. Per-project data in `projects/`
> only holds MCP cache + trust markers, not transcripts.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Store root:** `~/.cursor/chats/<user-hash>/<session-uuid>/store.db`
- **Session file:** SQLite with `blobs(id,data)` + `meta(key,value)`
- **Blob format:** content-addressed SHA256 → BLOB (binary payload)
- **Project key:** ❌ not used for session storage; session is global
- **Project dir:** `~/.cursor/projects/<slashed-cwd>/` (MCP cache only,
  no transcripts)
- **Resume CLI:** interactive TUI / `cursor-agent` (details undocumented)
- **Prompt history:** `~/.cursor/prompt_history.json` (flat array of
  prior user prompts, cross-session)
- **Platform:** macOS + Linux + Windows
- **Binary:** `/Users/jadb/.local/bin/cursor-agent`

## Sources

- Product: <https://cursor.com/>
- Verified filesystem: `~/.cursor/` (chats, projects, prompt_history)
- SQLite schema: `sqlite3 store.db ".schema"` on verified local db

## Layout

```pseudocode
~/.cursor/
├── chats/
│   └── 4c6e0abceabe6e778df03464c59514a3/        # user/auth hash (1 seen)
│       ├── 5d8b1dfa-6f35-475c-906b-385ab50f9138/
│       │   └── store.db                          # SQLite (CAS blobs)
│       ├── 92019078-a66e-4311-8593-7815af17dba4/
│       │   └── store.db
│       ├── c117ecb0-1da3-45ab-839e-572bf81cbf02/
│       │   ├── store.db
│       │   ├── store.db-shm
│       │   └── store.db-wal
│       ├── c8e5d114-6081-45f2-9408-2eac11175d46/
│       │   └── store.db
│       └── f77665cf-d0c1-435f-833b-9541785c5a8e/
│           └── store.db
├── projects/
│   ├── Users-jadb/
│   │   └── mcp-cache.json                        # MCP tool cache
│   ├── Users-jadb-claude/
│   │   └── mcp-cache.json
│   ├── Users-jadb-Projects-IdeaCraftersHQ-castor-guide/
│   │   ├── mcp-approvals.json
│   │   ├── mcp-cache.json
│   │   └── .workspace-trusted                    # trust marker
│   └── Users-jadb-Library-CloudStorage-Dropbox-.../
│       ├── mcp-approvals.json
│       ├── mcp-cache.json
│       ├── worker.log
│       └── .workspace-trusted
├── extensions/                                    # VSCode-style
├── skills/
├── argv.json
├── cli-config.json                                # model + permissions
├── ide_state.json                                 # recentlyViewedFiles
├── mcp.json                                       # user MCP config
└── prompt_history.json                            # flat array, ~41KB
```

## Project key encoding (projects/ dir only)

- Rule: `absolute_cwd.lstrip('/').replace('/', '-')`
- Example: `/Users/jadb/Projects/IdeaCraftersHQ/castor-guide`
  → `Users-jadb-Projects-IdeaCraftersHQ-castor-guide`
- Similar to Claude Code's scheme but **without leading dash**
- **Projects dir holds MCP cache + trust, NOT transcripts** — easy
  to mistake for session storage (it isn't)

## Session storage — SQLite CAS

```pseudocode
# ~/.cursor/chats/<user-hash>/<session-uuid>/store.db
CREATE TABLE blobs (
  id TEXT PRIMARY KEY,   # sha256 hex of blob content
  data BLOB              # binary payload (opaque)
);
CREATE TABLE meta (
  key TEXT PRIMARY KEY,
  value TEXT
);
```

- `blobs.id` is SHA256 hex → content-addressed deduplication
- Blob content is opaque — likely protobuf or custom binary format
- `meta` table is empty on inspected db (no visible keys)
- `store.db-wal` + `store.db-shm` present → WAL mode enabled

## Session layout

- First level: `chats/<user-hash>/` — opaque hash, likely tied to
  auth identity; only one exists locally despite multiple machines
- Second level: `<session-uuid>/` — UUIDv4 per session
- Third level: `store.db` — SQLite with blobs + meta

## Resume / continue semantics

- **Undocumented in `--help`.** Binary was not run for `--help` dump
  (requires interactive auth flow)
- Interactive TUI session picker presumed based on store structure
- Prompt completion likely uses `~/.cursor/prompt_history.json`
  directly for cross-session history navigation

## Project grouping

- **Not in `chats/`.** Sessions are flat under user hash.
- **Partial in `projects/`.** But `projects/<slashed-cwd>/` holds
  MCP state, not transcripts. Cannot be used to group sessions.
- Reconstruction requires reading blob payloads to extract cwd
  (if stored) — format is binary-opaque without reverse engineering

## cli-config.json — verified content

```pseudocode
{
  "permissions": {
    "allow": ["Shell(ls)", "Shell(npm init)", ...],
    "deny": []
  },
  "version": 1,
  "editor": { "vimMode": false },
  "model": {
    "modelId": "grok-code-fast-1",
    "displayName": "Grok",
    ...
  },
  "hasChangedDefaultModel": true,
  "privacyCache": { "ghostMode": true, "privacyMode": 1 },
  "network": { "useHttp1ForAgent": false }
}
```

- Permission allowlist is global, not per-project
- Model selection is global
- Privacy mode ("ghost") affects whether sessions sync server-side

## Strengths

- CAS blob storage deduplicates large repeated payloads
  (system prompts, tool outputs)
- WAL-mode SQLite → concurrent reads safe
- One db per session → isolation; no cross-session locking
- User-hash partition isolates multi-account scenarios cleanly

## Known gotchas

- **Blob format is opaque.** USP cannot decode without Cursor-side
  reverse engineering — no JSON escape hatch
- **No project grouping in session store.** Per-cwd filtering
  requires walking every blob and hoping cwd is embedded
- `projects/<slashed-cwd>/` is misleading — it's MCP config, not
  transcripts; do not treat as session root
- Only one user-hash dir exists locally; multi-machine or
  multi-account scenarios would create siblings — code must
  handle n-way
- No stable schema documentation
- Ghost mode ("privacyMode": 1) may mean sessions are local-only
  even on disk, but verification requires live session capture
- `prompt_history.json` is flat + cross-session; useful for prompt
  search but not replay
- `ide_state.json` tracks `recentlyViewedFiles` globally, not
  per-session

## Open questions

1. What is the blob binary format — protobuf, msgpack, custom?
   Is there a schema definition anywhere in the binary?
2. What does the `<user-hash>` encode — authenticated identity hash,
   installation ID, or machine ID?
3. Does `meta` table ever hold data (e.g., session title, cwd,
   timestamps), or is it always empty?
4. Is there a session index file at `chats/` level for fast
   enumeration + metadata preview?
5. Does Ghost mode disable `store.db` entirely, or does it just
   disable cloud sync?
6. How does Cursor resume — is there a "most-recent" pointer, or
   is it mtime-based scan of `chats/<user-hash>/*/`?

## Integration notes for USP

Cursor is **blocked on binary format reverse engineering**. Options:

1. **Metadata-only adapter:** expose session UUIDs + mtimes from
   `chats/<user-hash>/<uuid>/store.db` without decoding blobs;
   emit `session.discovered` events only
2. **Full adapter:** reverse-engineer blob format, likely via
   strings + protobuf fingerprinting — high effort, high risk of
   breaking on version bumps
3. **Hybrid:** metadata + `prompt_history.json` for prompt search

Recommendation: Option 1 for MVP. Treat Cursor as "presence only"
in USP until blob format is documented.

Expected adapter size (metadata only): ~150 LOC Go (sqlite3
reader + dir walker + mtime-based ordering).
