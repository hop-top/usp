# Sessions Mappings — Claude Code ↔ Gemini ↔ Codex ↔ OpenCode

> Living document. Source of truth for USP session taxonomy +
> per-CLI layout coverage. Update whenever a CLI ships session
> format changes. Feeds USP adapter capability maps and
> `usp doctor` output.

Last verified: 2026-04-08.

## Reference model: Claude Code

Claude Code has the cleanest session layout: path-encoded project
key + per-session append-only JSONL transcripts + optional memory
subsystem. USP's session model is a superset of Claude's layout
plus OpenCode's message/part/diff splits.

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
| Project key         | slash-to-dash cwd                    | basename + suffix (via map)        | none (cwd in session_meta only)   | SHA1 hex of cwd                    |
| Key → cwd decode    | ✅ string replace                    | 🔧 `projects.json` lookup           | 🔧 read first line of each file    | 🔧 read `project/<hash>.json`      |
| Central index       | ❌ none needed (filesystem IS index) | ✅ `projects.json`                  | ❌ none                            | ✅ SQLite + `project/*.json`        |
| Session ID          | UUIDv4                                | user-chosen tag (+ implicit "last")| UUIDv7                             | ULID-ish `ses_<26ch>`              |
| Transcript format   | JSONL append-only                    | per-chat JSON snapshot             | JSONL append-only                 | SQLite + JSON mirror (5-way split) |
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

| CLI         | Native ✅ | Workaround 🔧 | Missing ❌ | Fitness for USP adapter |
|-------------|----------|---------------|------------|-------------------------|
| Claude Code | 14       | 2             | 4          | **Reference** (~250 LOC)|
| OpenCode    | 12       | 4             | 4          | Mid (~500 LOC)          |
| Gemini CLI  | 9        | 4             | 7          | Easy-mid (~300 LOC)     |
| Codex CLI   | 5        | 5             | 10         | Hard (~400 LOC)         |

## Workaround patterns (cross-CLI)

### Pattern A — reverse project-key decode

Compute the CLI's native project key from an absolute cwd and look
up sessions by that key.

- Claude: `cwd.replace('/', '-')` → direct dir lookup
- Gemini: read `projects.json` → alias → `history/<alias>/`
- OpenCode: `sha1(cwd)` → `storage/session/<hash>/`
- Codex: ❌ no key; must walk + read first-line cwd

### Pattern B — central index build

Walk the native store, extract (cwd, session-id) pairs, build a
USP-managed reverse index so subsequent queries are O(1).

Required for Codex. Useful as cache for Gemini (if `projects.json`
is large) and OpenCode (if SQLite is slow).

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

- OpenCode: session + message + part + todo by session_id
- Claude: JSONL + `~/.claude/todos/<uuid>.json`
- Gemini/Codex: single-stream (no join needed)

### Pattern E — header extraction

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

## Open questions

1. Should USP normalize session IDs across CLIs (prefix with
   `cli:` e.g. `claude:uuid`, `codex:uuid7`) or keep native IDs
   with CLI-scoped namespaces?
2. How should USP handle Codex's cross-cwd resume drift when
   attributing turns to a project?
3. Should USP mirror OpenCode's 5-way split (session/message/part/
   todo/diff) or flatten to a single event stream?
4. Is there a portable format USP should export sessions to for
   cross-tool archival (e.g., one canonical JSONL schema)?

## Per-CLI research dossiers

See separate files in this directory:

- [claude-code.md](claude-code.md) — reference layout + memory subsystem
- [gemini-cli.md](gemini-cli.md) — basename+suffix alias via projects.json
- [codex-cli.md](codex-cli.md) — date-partitioned rollouts, cwd in header
- [opencode.md](opencode.md) — SHA1 hash + SQLite + 5-way split
- [kit-integration.md](kit-integration.md) — hop-top/kit delegation map
