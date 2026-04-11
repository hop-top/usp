# GitHub Copilot CLI — Sessions Research Dossier

> Standalone `copilot` binary (NOT `gh copilot`, NOT VS Code
> extension). SQLite session store at `~/.copilot/state/session-store.db`
> with normalized `sessions/turns/checkpoints/session_files/session_refs`
> schema. Discovered via bundled JS source in `~/.copilot/pkg/`.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Binary:** `/opt/homebrew/bin/copilot` → Mach-O arm64
- **Cask:** `copilot-cli` (Homebrew), version 0.0.422
- **Store root:** `~/.copilot/state/session-store.db` (SQLite, WAL mode)
- **Schema:** normalized — `sessions`, `turns`, `checkpoints`,
  `session_files`, `session_refs`, `schema_version`
- **Project key:** ❌ no directory sharding; `sessions.cwd` is a TEXT
  column — group by SQL `GROUP BY cwd`
- **Resume CLI:** `copilot --continue` | `copilot --resume[=<id>]`
- **Log dir:** `~/.copilot/logs/` (default; configurable via `--log-dir`)
- **Session-state dir:** `~/.copilot/session-state/` (session artifacts
  like `plan.md`)
- **Platform:** macOS + Linux + Windows

## Sources

- Binary source (bundled JS): `~/.copilot/pkg/darwin-arm64/0.0.422/index.js`
- CLI help: `copilot --help` (mentions `--continue`, `--resume`,
  `--log-dir ~/.copilot/logs/`)
- Verified filesystem: `~/.copilot/` (pkg, skills; state/ absent
  until first run)
- Docs: <https://docs.github.com/en/copilot/how-tos/use-copilot-for-common-tasks/use-copilot-in-the-cli>
- Distinct from: `gh copilot` extension (suggest/explain only, no
  persistent sessions); VS Code `GitHub.copilot-chat` extension
  (stores in VSCode workspaceStorage, different product)

## Layout

```pseudocode
~/.copilot/
├── state/
│   ├── session-store.db                  # SQLite primary store
│   ├── session-store.db-wal
│   └── session-store.db-shm
├── logs/                                 # CLI + agent logs
│   └── <session-id>.log                  # inferred naming
├── session-state/                        # per-session artifacts
│   └── <session-id>/
│       └── plan.md                       # agent's working plan file
├── pkg/
│   ├── darwin-arm64/
│   │   └── 0.0.422/
│   │       ├── index.js                  # 16M bundled Node source
│   │       ├── ripgrep/bin/rg
│   │       ├── definitions/
│   │       └── schemas/
│   ├── universal/1.0.15/
│   └── tmp/
├── mcp-config.json                       # MCP servers config
├── copilot-instructions.md               # user-level instructions
├── instructions/                         # per-topic instructions dir
└── skills/                               # user skills dir
```

- `state/` directory is **created on first interactive run** — may
  not exist until the user runs `copilot` at least once
- On this machine, `~/.copilot/state/` is absent (only `skills/` +
  `pkg/` populated); paths derived from static analysis of
  `pkg/darwin-arm64/0.0.422/index.js`

## SQLite schema (verified from binary source)

```pseudocode
CREATE TABLE schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  cwd TEXT,                               # working dir at session start
  repository TEXT,                        # git remote / repo identifier
  branch TEXT,                            # git branch
  summary TEXT,                           # LLM-generated session summary
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE turns (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  session_id TEXT NOT NULL REFERENCES sessions(id),
  turn_index INTEGER NOT NULL,
  user_message TEXT,
  assistant_response TEXT,
  timestamp TEXT DEFAULT (datetime('now'))
);

CREATE TABLE checkpoints (...);           # schema not dumped; present
CREATE TABLE session_files (...);         # per-session file tracking
CREATE TABLE session_refs (...);          # likely @-refs / context refs
```

- PRAGMAs: `journal_mode = WAL`, `busy_timeout = 3000`,
  `foreign_keys = ON`
- `sessions.cwd` is the only project-grouping field — **no dir
  sharding**; all sessions in one db, query via SQL
- `turns` is flat (one row per turn); no tool_call/tool_result split
  at schema level (may be embedded in `assistant_response` JSON)

## Resume / continue semantics

From `copilot --help`:

- `copilot --continue` — resume most recent session in cwd
- `copilot --resume` — interactive session picker (TUI)
- `copilot --resume=<session-id>` — direct resume by ID
- `copilot --share[=<path>]` — export session to markdown after
  completion (default: `./copilot-session-<id>.md`)
- `copilot --share-gist` — upload session to GitHub gist
- `--max-autopilot-continues <count>` — throttle for non-interactive

## Project grouping

- **SQL-native.** `SELECT DISTINCT cwd FROM sessions` → project list
- `repository` + `branch` columns enable git-aware grouping
  (cross-worktree sessions land under same repo)
- **No filesystem sharding** — single db file holds all sessions
- `--continue` scoping appears to use cwd match (not verified
  against source; inferred from command semantics)

## Strengths

- Only CLI using SQLite with normalized sessions/turns/checkpoints
  split (vs. OpenCode's 5-way filesystem split)
- `repository` + `branch` columns = native git-awareness; worktrees
  can potentially share sessions via repo match
- Foreign keys enforce referential integrity (turns → sessions)
- WAL mode → safe concurrent reads
- `--share` / `--share-gist` built-in export paths (USP can reuse)
- `schema_version` table → migration-safe

## Known gotchas

- **`state/` absent until first run** — USP must handle bootstrap
  gracefully (empty store is normal)
- Bundled binary makes schema fragile across versions; no stable
  schema doc upstream
- `session_state/<id>/plan.md` is filesystem-side, not in SQLite —
  dual storage requires joining by session_id
- `cwd` is captured at session start; subsequent `cd` operations
  don't update it (same as Codex)
- `turns.assistant_response` likely holds tool calls inline as JSON;
  adapters must parse strings to extract tool events
- `checkpoints` / `session_files` / `session_refs` schemas not
  dumped here — require live inspection post-bootstrap
- Confusion risk: three "Copilot" entry points coexist:
  - `copilot` (this, standalone CLI, SQLite sessions) ✅
  - `gh copilot` (gh extension, suggest/explain only, no sessions)
  - VS Code `GitHub.copilot-chat` (IDE chats in workspaceStorage
    + `$HOME/Library/Application Support/Code/User/globalStorage/github.copilot-chat/copilotCli/`)

## Open questions

1. Full schemas for `checkpoints`, `session_files`, `session_refs` —
   require running `copilot` once then dumping `.schema`
2. Is `sessions.repository` derived from git remote URL, git repo
   root path, or something else?
3. Does `--continue` match on cwd exactly, on repo, or on most-recent
   regardless of cwd?
4. Does `session-state/<id>/plan.md` get cleaned up on session
   delete, or is it orphaned?
5. Is there cross-machine sync (via GitHub account) for sessions,
   like Copilot chat on the web?
6. Are turn messages stored as plain text or structured JSON inside
   `user_message`/`assistant_response`?

## Integration notes for USP

Copilot CLI is **mid-complexity** — clean SQLite but undocumented.

Strategy:

1. **Bootstrap check:** probe for `~/.copilot/state/session-store.db`;
   if absent, emit `adapter.waiting` status (user hasn't run CLI yet)
2. **Read path:** open db read-only with WAL mode; snapshot
   `schema_version` and refuse unknown versions loudly
3. **Project index:** `SELECT DISTINCT cwd, repository, branch FROM
   sessions` → build cwd → session[] map in-memory
4. **Event stream:** tail `turns` table via polling (no native
   change feed) or watch SQLite WAL file via fswatch
5. **Sidecar join:** for each session, check `~/.copilot/session-state/<id>/`
   for artifact files (plan.md, etc.); emit as attached resources
6. **Translate to USP envelope:** map `turns` rows to user/assistant
   events; parse `assistant_response` JSON for embedded tool calls
7. **Git awareness:** use `sessions.repository + branch` as secondary
   grouping key alongside cwd (worktree-aware)

Expected adapter size: ~400 LOC Go (SQLite reader + schema guard +
artifact joiner + poll loop).
