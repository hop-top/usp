# Claude Code — Sessions Research Dossier

> Reference CLI for USP. Path-encoded project grouping + per-session
> JSONL transcripts. USP session taxonomy models Claude's layout as
> the canonical superset.

Last verified: 2026-04-11.
Source: <https://code.claude.com/docs/en/session-management>

## Summary

- **Store root:** `~/.claude/projects/` (JSONL transcripts) +
  `~/.claude/projects/<project>/memory/` (optional file-based
  memory)
- **Project key:** absolute cwd; `/` → `-`, `.` → `-`
- **Session file:** `<project-dir>/<session-uuid>.jsonl`
- **Session dir:** `<project-dir>/<session-uuid>/` (subagents +
  tool results)
- **Transcript format:** JSONL; one event per line; append-only
- **Resume CLI:** `claude --resume <uuid>` or `claude --continue`
- **Scope:** per-cwd (each worktree = distinct project key)
- **Platform:** macOS + Linux + Windows (via WSL)

## Layout

```pseudocode
~/.claude/
├── projects/
│   ├── -Users-jadb--w-ideacrafterslabs-uhp-hops-main/
│   │   ├── memory/                    # optional: auto-memory
│   │   │   ├── MEMORY.md             # index (~200 line cap)
│   │   │   └── <topic>.md            # per-topic memory
│   │   ├── <session-uuid>.jsonl       # main transcript
│   │   └── <session-uuid>/            # session artifacts
│   │       ├── subagents/
│   │       │   ├── agent-<hash>.jsonl      # subagent transcript
│   │       │   └── agent-<hash>.meta.json  # {agentType, description}
│   │       └── tool-results/
│   │           └── <id>.txt           # large tool output cache
│   ├── -Users-jadb--w-ideacrafterslabs-aps/
│   │   ├── memory/
│   │   │   ├── MEMORY.md
│   │   │   ├── feedback_branch_assumptions.md
│   │   │   └── hop-toolkit-architecture.md
│   │   └── ...
│   └── -Users-jadb--p-blog/
│       └── ...
├── settings.json
└── todos/                             # TodoWrite state
    └── <uuid>-agent-<uuid>.json       # compound key
```

## Project key encoding

- Rule: `absolute_cwd.replaceAll('/', '-').replaceAll('.', '-')`
- Dots are **replaced with dashes**, not stripped
- Example: `/Users/jadb/.w/ideacrafterslabs/uhp/hops/main`
  → `-Users-jadb--w-ideacrafterslabs-uhp-hops-main`
- Double dash `--` arises from `/` → `-` adjacent to `.` → `-`
  (`/.w` → `-` + `-` + `w` = `--w`)
- Worktrees are sibling keys — no nesting; each cwd is distinct
- No stable project ID — key changes if dir moves; no `.claude`
  marker file per-project

## Transcript schema (JSONL, per line)

```pseudocode
{
  "parentUuid": "<prev-event-uuid|null>",
  "isSidechain": false,
  "type": "user"|"assistant"|"system"|"tool_use"|"tool_result"
         |"file-history-snapshot",
  "subtype": "bridge_status" | ...,
  "message": { "role": "...", "content": ... },
  "content": "...",
  "timestamp": "2026-04-09T02:25:43.319Z",
  "uuid": "<event-uuid>",
  "userType": "external" | "internal",
  "entrypoint": "cli" | "web",
  "sessionId": "<session-uuid>",
  "cwd": "...",
  "version": "...",
  "gitBranch": "main",
  "permissionMode": "bypassPermissions" | ...,
  "promptId": "<uuid>",
  "requestId": "req_..."
}
```

- Linked-list via `parentUuid` → reconstructs conversation tree
- `isSidechain` marks sidechain branches
- `file-history-snapshot` events track file state for undo
- One file = one session; resume appends; compaction writes new
  file with `compacted` marker event

## Resume / continue semantics

- `claude --continue` — resumes most recent session in current cwd
- `claude --resume <uuid>` — resumes specific session
- `/resume` (slash command) — interactive session picker
- Resume re-reads the JSONL, rehydrates conversation, fires
  `SessionStart` hook with `source: resumed`
- Compaction: `PreCompact` → transcript rewritten with summary
  event → `PostCompact`; original preserved or rotated per config

## Memory subsystem (auto-memory skill)

- Location: `~/.claude/projects/<project-key>/memory/`
- `MEMORY.md` — index, always loaded; ~200 line cap
- `<topic>.md` — individual memories with YAML frontmatter
  (`name`, `description`, `type`, body)
- Types: `user`, `feedback`, `project`, `reference`
- Scoped per-project-key (worktrees don't share memory by default)
- Not the same as transcripts — memory is curated knowledge, JSONL is
  raw conversation history

## Subagent storage

- Subagents are NOT inline in main transcript
- Stored in `<session-uuid>/subagents/agent-<hash>.jsonl`
- Each has companion `.meta.json`:
  `{"agentType":"general-purpose","description":"..."}`
- `isSidechain` in main transcript marks dispatch points; actual
  subagent conversation lives in separate file
- Tool results cached in `<session-uuid>/tool-results/<id>.txt`
  (plain text, can be large — multi-MB observed)

## TodoWrite state

- Path: `~/.claude/todos/<uuid>-agent-<uuid>.json`
- Compound filename — session UUID repeated with `agent` infix
- Per-session; not cross-session
- Separate from projects dir; most files observed as 2 bytes (`[]`)

## Strengths

- Only CLI with path-encoded project grouping (human-decodable)
- Only CLI with per-session JSONL transcripts (append-only,
  rehydratable)
- Only CLI with first-class memory subsystem beside transcripts
- Subagent transcripts preserved as separate files alongside main
  session — full conversation graph reconstructable
- Compaction preserves history via rewrites, not destructive replace
- Session UUIDs globally unique; no per-project collision risk

## Known gotchas

- **Worktrees fragment sessions:** `uhp` and `uhp/hops/main` are
  distinct project keys — sessions don't travel between them
- Project key derived from cwd at session start; `cd` mid-session
  doesn't re-key
- Moving a project directory orphans sessions (no auto-relink)
- `parentUuid` chain can fork (branching / retries) — linearization
  is consumer's responsibility
- `memory/` subdir is skill-managed, not platform-managed; not
  every session writes memory
- No built-in search across sessions — must grep/parse JSONL
- JSONL files grow unbounded until compaction; no size-based
  rotation by default
- Subagent JSONL files are separate from main transcript — adapter
  must walk `<session>/subagents/` to get full picture
- Dot-to-dash in project key is lossy: `/foo/.bar` and `/foo/-bar`
  would produce same key (collision risk, unlikely in practice)

## Open questions

1. Does `--resume` support cross-worktree resumes (e.g., resuming
   a session from `uhp/hops/feature` while cwd is `uhp/hops/main`)?
2. Does compaction rewrite the file in place or create a new file +
   rename? What event marks the transition boundary?
3. Is `sessionId` stable across resume, or does each `--resume`
   mint a new child session ID linked to parent?
4. Memory subsystem: is `MEMORY.md` the only index, or are there
   hidden metadata files?
5. What triggers `file-history-snapshot` events — every tool call,
   or only file-modifying ones?
6. `tool-results/` cache: how long retained? Pruned on compaction?
7. Subagent hash in filename (`agent-<hash>`) — derived from what?
   Appears to be a truncated content hash, not a UUID.

## Integration notes for USP

Claude Code is the **reference adapter**. USP's session model is a
superset of Claude's layout. The adapter needs to:

1. Watch `~/.claude/projects/` for new `<project-key>/` dirs
2. Decode project-key → absolute cwd (reverse: ambiguous since both
   `/` and `.` map to `-`; use cwd field from first event)
3. Tail `*.jsonl` files as append-only event streams
4. Walk `<session-uuid>/subagents/` for subagent transcripts
5. Translate JSONL events to USP envelope (rename fields, add
   `cli.name=claude`, preserve `parentUuid` graph)
6. Emit session lifecycle events: `session.created`,
   `session.resumed`, `session.compacted`, `session.ended`

Note: reverse key decode is lossy (dots become dashes). Adapter should
read `cwd` field from first JSONL event for authoritative path.

Expected adapter size: ~300 LOC Go.
