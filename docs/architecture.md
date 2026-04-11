# Architecture

Author: $USER

## Overview

USP normalizes session data from multiple AI coding CLIs into
a single envelope format. Three pillars:

1. **Adapter pattern** -- each CLI gets a `SessionAdapter`
   translating native storage into USP types
2. **Kit delegation** -- `hop.top/kit/uxp` handles CLI
   detection, registry, capability maps, project key
   derivation; adapters stay thin
3. **Project index** -- SQLite DB mapping working directories
   to per-CLI native keys for cross-project queries

## Package Layout

```
cmd/usp/
  main.go            CLI entry; Cobra root + subcommands
  session.go         session parent command
  session_list.go    list: fan-out, merge, render
  session_show.go    show: ID-priority resolve, stream turns
  doctor.go          environment health checks

session/
  envelope.go        Session, Turn, ToolCall types
  adapter.go         SessionAdapter interface

adapters/
  claude/            JSONL parser (~/.claude/projects/)
  codex/             date-partitioned JSONL (~/.codex/sessions/)
  gemini/            JSON + projects.json (~/.gemini/history/)
  opencode/          SQLite reader (~/.local/share/opencode/)

index/
  project.go         ProjectIndex (SQLite-backed cwd->key map)
```

## Data Flow

See diagram:
[docs/architecture/data-flow-v1.mmd](architecture/data-flow-v1.mmd)

**`usp session list`:**

1. Resolve project cwd (flag or `os.Getwd`)
2. Build adapter map (all 4, or filtered by `--cli`)
3. Fan-out `ListSessions(cwd)` to each adapter (best-effort;
   failures skipped)
4. Merge results, sort by `StartedAt` descending
5. Apply `--limit`, project into table rows
6. Render via `kit/output` (table / json / yaml)

**`usp session show <id>`:**

1. Parse session ID
2. ID-format priority: regex match determines adapter order
   - UUIDv4 -> Claude first
   - UUIDv7 -> Codex first
   - `ses_*` prefix -> OpenCode first
   - fallback -> default order
3. Try `GetSession(id)` on each adapter in priority order
4. First match wins; print metadata
5. `StreamTurns(id)` emits turns via channel; render inline

**`usp doctor`:**

1. Load `kit/uxp` default registry
2. For each CLI: check binary installed, store exists, store
   readable
3. USP-specific: check project index DB at
   `~/.local/share/usp/index.db`
4. Render aligned table with status symbols

## Adapter Contract

```
SessionAdapter interface:
  // From uxp.Adapter (kit)
  CLI()            -> CLIName
  Detect()         -> DetectResult, error
  Capabilities()   -> CapabilityMap

  // USP-specific
  ListSessions(cwd)  -> []Session, error
  GetSession(id)     -> *Session, error
  StreamTurns(id)    -> <-chan Turn, error
  ProjectKey(cwd)    -> string
```

Each adapter MUST:

- Return partial results on per-file errors (skip bad files)
- Return `nil, nil` when store is absent (not an error)
- Stream turns via buffered channel, close on completion
- Derive project keys using `kit/uxp` strategies where
  possible (`uxp.DeriveKey`)

## Session Envelope

```
Session:
  ID          string
  CLI         CLIName
  ProjectCwd  string
  StartedAt   Time
  EndedAt     *Time        (nil = active)
  TurnCount   int
  Metadata    map[string]any

Turn:
  Role        Role         (user | assistant | system)
  Content     string
  Timestamp   Time
  ToolCalls   []ToolCall

ToolCall:
  Name   string
  Input  string
  Output string
```

## Shared Infrastructure (kit/uxp)

USP delegates to `hop.top/kit/uxp` for:

| Concern | kit/uxp surface |
|---------|----------------|
| CLI detection | `uxp.Detect()`, `uxp.CLIRegistry` |
| Store path resolution | `uxp.ResolveStorePath()` |
| Doctor checks | `uxp.NewDoctor()`, `uxp.CheckCLIInstalled()`, `uxp.CheckStoreExists()` |
| Project key derivation | `uxp.DeriveKey()` with strategies (Embedded, SHA1, etc.) |
| Capability dimensions | `uxp.CapabilityMap`, `uxp.Support` (Native / Workaround / Missing) |
| CLI name constants | `uxp.CLIClaude`, `uxp.CLICodex`, `uxp.CLIGemini`, `uxp.CLIOpenCode` |

Adapters import kit/uxp but never import each other.

## Native Store Formats

| CLI | Path | Format | Project Grouping |
|-----|------|--------|-----------------|
| Claude Code | `~/.claude/projects/<key>/` | JSONL per session | dir-name = munged cwd |
| Codex CLI | `~/.codex/sessions/<YYYY>/<MM>/<DD>/` | JSONL with `session_meta` header | cwd embedded in meta |
| Gemini CLI | `~/.gemini/history/<alias>/` | JSON files; alias via `projects.json` | projects.json cwd->alias |
| OpenCode | `~/.local/share/opencode/opencode.db` | SQLite (session + message + part tables) | SHA1 of cwd as project_id |
