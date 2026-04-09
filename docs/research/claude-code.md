# Claude Code — Sessions Research Dossier

> Reference CLI for USP. Path-encoded project grouping + per-session
> JSONL transcripts. USP session taxonomy models Claude's layout as
> the canonical superset.

Last verified: 2026-04-08. Source: <https://code.claude.com/docs/en/session-management>

## Summary

- **Store root:** `~/.claude/projects/` (JSONL transcripts) +
  `~/.claude/projects/<project>/memory/` (optional file-based memory)
- **Project key:** absolute cwd with `/` → `-` (slash-to-dash encoding)
- **Session file:** `<project-dir>/<session-uuid>.jsonl` (one per session)
- **Transcript format:** JSONL; one event per line; append-only
- **Resume CLI:** `claude --resume <session-uuid>` or `claude --continue`
- **Scope:** per-cwd (each worktree = distinct project key)
- **Platform:** macOS + Linux + Windows (via WSL)

## Layout

```pseudocode
~/.claude/
├── projects/
│   ├── -Users-jadb--w-ideacrafterslabs-uhp/
│   │   ├── memory/                        # optional: auto-memory skill
│   │   │   ├── MEMORY.md                  # index
│   │   │   └── <topic>.md                 # per-topic memory files
│   │   └── <session-uuid>.jsonl           # transcript(s)
│   ├── -Users-jadb--w-ideacrafterslabs-uhp-hops-main/
│   │   └── 6644b7d1-8d7d-45e5-9d25-04bd254b1bff.jsonl
│   └── -Users-jadb--p-blog/
│       └── ...
├── settings.json
└── todos/                                 # TodoWrite state per session
```

## Project key encoding

- Rule: `absolute_cwd.replace('/', '-')`
- Example: `/Users/jadb/.w/ideacrafterslabs/uhp/hops/main`
  → `-Users-jadb--w-ideacrafterslabs-uhp-hops-main`
- Double dash `--` preserves leading dot in `.w` (the `/.` becomes `-.` → `--`)
- Worktrees are sibling keys — no nesting; each absolute cwd is distinct
- No stable project ID — key changes if directory moves; no `.claude`
  marker file per-project

## Transcript schema (JSONL, per line)

```pseudocode
{
  "parentUuid": "<prev-event-uuid|null>",
  "isSidechain": false,
  "type": "user" | "assistant" | "system" | "tool_use" | "tool_result",
  "subtype": "bridge_status" | ...,
  "content": "...",
  "timestamp": "2026-04-09T02:25:43.319Z",
  "uuid": "<event-uuid>",
  "userType": "external" | "internal",
  "entrypoint": "cli" | "web",
  "sessionId": "<session-uuid>",
  "cwd": "...",
  "version": "..."
}
```

- Linked-list via `parentUuid` → reconstructs conversation tree
- `isSidechain` marks subagent branches (Agent tool dispatches)
- One file = one session; resume appends; compaction writes a new
  file with a `compacted` marker event

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

## TodoWrite state

- Path: `~/.claude/todos/<session-uuid>.json`
- Per-session; not cross-session
- Separate from projects dir; keyed by session UUID only

## Strengths

- Only CLI with path-encoded project grouping that's human-decodable
- Only CLI with per-session JSONL transcripts (append-only, rehydratable)
- Only CLI with first-class memory subsystem beside transcripts
- Subagent transcripts are inline (`isSidechain: true`) — no separate
  store, full conversation graph preserved
- Compaction preserves history via rewrites, not destructive replace
- Session UUIDs are globally unique; no per-project collision risk

## Known gotchas

- **Worktrees fragment sessions:** `uhp` and `uhp/hops/main` are
  distinct project keys — sessions don't travel between them
- Project key is derived from cwd at session start; `cd` mid-session
  doesn't re-key
- Moving a project directory orphans its sessions (no auto-relink)
- `parentUuid` chain can fork (branching / retries) — linearization
  is consumer's responsibility
- `memory/` subdir is skill-managed, not platform-managed; not every
  session writes memory
- No built-in search across sessions — must grep/parse JSONL manually
- JSONL files grow unbounded until compaction; no size-based rotation
  by default

## Open questions

1. Does `--resume` support cross-worktree resumes (e.g., resuming a
   session from `uhp/hops/feature` while cwd is `uhp/hops/main`)?
2. Does compaction rewrite the file in place or create a new file +
   rename, and what event shows the transition boundary?
3. Is the `sessionId` field stable across resume, or does each
   `--resume` mint a new child session ID linked to the parent?
4. Does the memory subsystem have any canonical index beyond
   `MEMORY.md`, or is it purely file-based with skill-level conventions?

## Integration notes for USP

Claude Code is the **reference adapter**. USP's session model is a
superset of Claude's layout. The adapter only needs to:

1. Watch `~/.claude/projects/` for new `<project-key>/` dirs
2. Decode project-key → absolute cwd (reverse slash-to-dash)
3. Tail `*.jsonl` files as append-only event streams
4. Translate JSONL events to USP envelope (rename fields, add
   `cli.name=claude`, preserve `parentUuid` graph)
5. Emit session lifecycle events: `session.created`, `session.resumed`,
   `session.compacted`, `session.ended`

No synthesis required. Expected adapter size: ~250 LOC Go.
