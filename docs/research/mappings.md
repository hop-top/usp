# Sessions Mappings — Claude Code ↔ Gemini ↔ Codex ↔ OpenCode

> Living document. Source of truth for USP session taxonomy +
> per-CLI layout coverage. Update whenever a CLI ships session
> format changes. Feeds USP adapter capability maps and
> `usp doctor` output.

Last verified: 2026-04-11.

## Reference model: Claude Code

Claude Code has the cleanest session layout: path-encoded project
key + per-session append-only JSONL transcripts + optional memory
subsystem. USP's session model is a superset of Claude's layout
plus OpenCode's message/part/diff splits (12-table Drizzle
schema, not the earlier 5-way estimate).

Source: <https://code.claude.com/docs/en/session-management>

## Core dimensions

| Dimension            | Description                                                      |
|----------------------|------------------------------------------------------------------|
| Store root           | Filesystem path where session state lives                       |
| Project grouping     | How sessions are associated with a project / cwd                |
| Project key          | The identifier used to group sessions by project                |
| Session ID           | The per-session identifier                                       |
| Transcript format    | File format of the conversation history                        |
| Append model         | Append-only stream vs snapshot vs hybrid                        |
| Resume surface       | CLI/TUI affordances for resuming a session                     |
| Cross-session index  | Whether a central index maps keys → sessions                   |
| Memory layer         | Whether there's a knowledge-store beside transcripts           |
| Worktree isolation   | Whether sibling worktrees get distinct keys                    |

## Complete mapping table

| Dimension           | Claude Code                          | Gemini CLI                         | Codex CLI                         | OpenCode                           |
|---------------------|--------------------------------------|-------------------------------------|------------------------------------|------------------------------------|
| Store root          | `~/.claude/projects/`                | `~/.gemini/history/`                | `~/.codex/sessions/<Y>/<M>/<D>/`  | `~/.local/share/opencode/`         |
| Project grouping    | ✅ per-cwd dir                        | ✅ per-alias dir                    | ❌ date-partitioned (no grouping)  | ✅ per-hash dir                     |
| Project key         | slash-to-dash cwd (dots stripped)    | basename + suffix (via map)        | none (cwd in session_meta only)   | SHA1 hex of cwd                    |
| Key → cwd decode    | ✅ string replace                    | 🔧 `projects.json` lookup           | 🔧 read first line of each file    | 🔧 read `project/<hash>.json`      |
| Central index       | ❌ none needed (filesystem IS index) | ✅ `projects.json`                  | ≈ `session_index.jsonl` (no cwd)  | ✅ SQLite + `project/*.json`        |
| Session ID          | UUIDv4                                | user-chosen tag (+ implicit "last")| UUIDv7                             | ULID-ish `ses_<26ch>`              |
| Transcript format   | JSONL append-only                    | per-chat JSON snapshot             | JSONL append-only                 | SQLite + JSON mirror (12-table)    |
| Append model        | ✅ append stream                      | ❌ snapshot per save                | ✅ append stream                   | ✅ append via SQLite                |
| First-line header   | event in stream                      | n/a (full JSON)                    | ✅ `session_meta`                  | session.json sidecar               |
| Resume by ID        | ✅ `--resume <uuid>`                  | ✅ `chat resume <tag>`              | ✅ `resume <id>` / `--last`        | ✅ `--session <id>` / TUI           |
| Resume most recent  | ✅ `--continue`                       | ✅ untagged implicit                | ✅ `resume --last`                 | ✅ TUI top                          |
| Cross-cwd resume    | 🔧 UUID only                         | 🔧 via `--history-dir`              | ✅ unscoped (silent cwd drift)     | ✅ via session ID                   |
| Worktree isolation  | ✅ distinct cwd → distinct dir        | ✅ distinct alias per cwd           | ❌ only via cwd field               | ✅ distinct hash per cwd            |
| Memory subsystem    | ✅ `projects/<key>/memory/`           | ❌ GEMINI.md is instructions only   | ❌ none                            | 🔧 `plans/`, `todo/` (different)    |
| Code diffs          | ❌ none                               | ❌ none                             | ❌ none                            | ✅ `session_diff/`                  |
| Content snapshots   | ❌ none                               | ❌ none                             | ❌ none                            | ✅ `snapshot/`                      |
| Todo state          | ✅ `~/.claude/todos/<uuid>.json`      | ❌ none                             | ❌ none                            | ✅ `storage/todo/<ses>/`            |
| Prompt history      | within JSONL                         | within chat JSON                   | ✅ separate `history.jsonl`        | within SQLite                      |
| Archive mechanism   | ❌ unbounded (compaction only)        | ❌ unbounded                        | ✅ `archived_sessions/`            | ❌ SQLite grows                     |

### Legend

- ✅ **native** — first-class, no plumbing
- 🔧 **workaround** — requires lookup, index, or decode
- ❌ **missing** — no direct equivalent; either impossible or
  structurally absent
- ≈ **close** — semantic overlap with caveats

## Coverage summary

19 dimensions mapped (see table above). Every cell classified.
Counts updated 2026-04-11 after dossier cross-check.

| CLI         | ✅ | 🔧 | ≈ | ❌ | Adapter estimate  |
|-------------|---|---|---|---|-------------------|
| Claude Code | 14 | 1  | — | 4  | **Ref** (~300 LOC)|
| OpenCode    | 16 | 2  | — | 1  | Mid (~600 LOC)    |
| Gemini CLI  | 10 | 2  | — | 7  | Easy-mid (~300)   |
| Codex CLI   | 10 | 1  | 1 | 7  | Hard (~400 LOC)   |

Adapter LOC updated per dossier findings (OpenCode 12-table
schema raised estimate from ~500 to ~600).

## Workaround patterns (cross-CLI)

### Pattern A — reverse project-key decode

Compute the CLI's native project key from an absolute cwd and look
up sessions by that key.

- Claude: `cwd.replace('/', '-').replace('.', '')` → dir lookup
  (dot-stripping is lossy; read `cwd` from first JSONL event)
- Gemini: read `projects.json` → alias → `history/<alias>/`
- OpenCode: `sha1(cwd)` → `storage/session/<hash>/`
- Codex: ❌ no key; must walk + read first-line cwd

### Pattern B — central index build

Walk the native store, extract (cwd, session-id) pairs, build a
USP-managed reverse index so subsequent queries are O(1).

Required for Codex project grouping (native `session_index.jsonl`
has thread names but no cwd). Useful as cache for Gemini (if
`projects.json` is large) and OpenCode (if SQLite is slow).

### Pattern C — filesystem watcher

Watch the native store with fswatch / inotify to emit session
lifecycle events as files appear/change. Used for all four CLIs
when USP is running as a daemon.

- Claude: watch `~/.claude/projects/*/` for new `*.jsonl`
- Gemini: watch `~/.gemini/history/*/` for new `*.json`
- Codex: watch `~/.codex/sessions/<Y>/<M>/<D>/` (rolling)
- OpenCode: watch SQLite WAL or `storage/session/*/`

### Pattern D — schema join

Rehydrate a full session by joining multiple stores.

- OpenCode: 12-table Drizzle schema; core join path is
  session → message → part (part keyed by msg_id, not ses_id);
  also todo, permission, workspace, event tables
- Claude: JSONL + `~/.claude/todos/<uuid>.json`
- Gemini/Codex: single-stream (no join needed)

### Pattern E — subagent / child artifact walk

Claude stores subagent transcripts in separate files under
`<session>/subagents/agent-<hash>.jsonl` — not inline in the main
JSONL. Adapter must discover + walk this dir per session. Other
CLIs embed tool/agent output inline.

### Pattern F — header extraction

Extract metadata by reading only the first line / first bytes of
a transcript file to avoid parsing the whole history.

- Codex: mandatory — `cwd` only in first-line `session_meta`
- Claude: optional — first event usually has session context
- OpenCode: skip — metadata lives in sidecar JSON, not transcript

## Project grouping strategies

The fundamental split between CLIs is **how a session is attributed
to a project**. Five strategies observed:

1. **Path encoding in directory name** — Claude Code
   - Pro: human-decodable, filesystem-native
   - Con: breaks on rename, no stable ID
2. **Basename + collision suffix via central map** — Gemini CLI
   - Pro: short, central `projects.json`
   - Con: aliases are semantically meaningless; collisions everywhere
3. **Content-hash of cwd** — OpenCode
   - Pro: deterministic, stable, collision-free in practice
   - Con: opaque; requires sidecar for reverse lookup
4. **Embedded field in session header** — Codex CLI
   - Pro: self-describing files
   - Con: no grouping without walking every file
5. **Hybrid (path + memory store)** — Claude Code auto-memory
   - Pro: transcripts and curated knowledge co-located
   - Con: skill-managed, not platform-managed

## USP's strategy

USP adopts **cwd as canonical project key** (like Claude) and
provides:

- A USP-managed index `~/.config/usp/projects.db` mapping cwd →
  (claude_key, gemini_alias, codex_sessions, opencode_hash)
- Adapters translate USP cwd-based queries into native keys per
  CLI
- Sessions are queryable uniformly: `usp session list --project <cwd>`
- Cross-CLI session join: `usp session show <session-id>` walks
  all adapters

## Verified findings (2026-04-11)

Key corrections after cross-checking dossiers:

- **Claude:** project key strips dots (lossy); subagent transcripts
  live in `<session>/subagents/agent-<hash>.jsonl` (separate files,
  not inline); adapter must walk subagent dir for full picture
- **Gemini:** no chat transcript files observed on disk — only
  `.project_root` markers; suggests `/chat save` never used or
  transcripts ephemeral until explicit save
- **Codex:** `session_index.jsonl` exists as native index (maps
  session ID → thread name + timestamp) but lacks cwd; schema
  drifts across CLI versions (field renames, additions) without
  a schema version field; `response_item` is the line type (not
  `user_message`/`tool_call`)
- **OpenCode:** 12-table Drizzle schema (not 5); part table keyed
  by message ID not session ID; `snapshot/` at root level (not
  under `storage/`); `migration` marker under `storage/`

## Open questions

1. Should USP normalize session IDs across CLIs (prefix with
   `cli:` e.g. `claude:uuid`, `codex:uuid7`) or keep native IDs
   with CLI-scoped namespaces?
2. ~~How should USP handle Codex's cross-cwd resume drift?~~
   **Resolved:** Codex resume is unscoped — no cwd rewrite on
   resume. USP must attribute by original `session_meta.cwd` and
   flag any resumed-from-different-cwd sessions.
3. ~~Should USP mirror OpenCode's 5-way split?~~ **Updated:**
   OpenCode has 12 tables. Core join path: session → message →
   part. USP should flatten to event stream but preserve join
   keys as metadata for lossless round-trip.
4. Is there a portable format USP should export sessions to for
   cross-tool archival (e.g., one canonical JSONL schema)?

## Per-CLI research dossiers

See separate files in this directory:

- [claude-code.md](claude-code.md) — reference layout + memory subsystem
- [gemini-cli.md](gemini-cli.md) — basename+suffix alias via projects.json
- [codex-cli.md](codex-cli.md) — date-partitioned rollouts, cwd in header
- [opencode.md](opencode.md) — SHA1 hash + SQLite + 12-table schema
- [kit-integration.md](kit-integration.md) — hop-top/kit delegation map
