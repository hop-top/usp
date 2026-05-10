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

---

# 2026-04-26 update — addenda since v0.0.1-alpha

> **Scope.** The body above documents v0.0.1-alpha (adapter
> pattern, kit delegation, project index). This addendum
> captures recent additions: lineage tracking + cross-CLI
> resume, the usp-ctxt bridge (Pipeline B), mentions-model
> integration (T-0068), and UUID-prefix matching (T-0061).

## Cross-references

- [kit](https://github.com/hop-top/kit/blob/main/docs/architecture.md) — `uxp.Adapter` base, kit/output, CLI consts
- [ctxt](https://github.com/hop-top/ctxt/blob/main/docs/architecture.md) — knowledge graph; consumer of usp-ctxt sync
- [aps](https://github.com/hop-top/aps/blob/main/docs/architecture.md) — agent profile; producer identity in ctxt mentions
- [xrr](https://github.com/hop-top/xrr/blob/main/docs/architecture.md) — e2e cassette record / replay

## Updated package layout

| Path | Role |
|---|---|
| `cmd/usp-ctxt/` | Bridge binary (Pipeline B) — separate from main `usp` binary |
| `lineage/store.go` | SQLite store for cross-CLI session chains (sessions + segments tables, WAL) |
| `internal/uspctxt/projection.go` | Session → Projection (mentions identity model) |
| `internal/uspctxt/bridge.go` | Run(SessionSource, CtxtClient, State); high-water-mark idempotency |
| `internal/uspctxt/state.go` | Per-CLI per-project high-water-marks |
| `internal/sessionutil/` | ResolveSessionID, FilterAdapters, CollectSessions, ParseSince, SortAndLimit |
| `cmd/usp/session_lineage.go` | Renders segment table for `usp session lineage <id>` |
| `cmd/usp/resume.go` | Cross-CLI resume via ResumeAdapter.InjectSession() |

## Updated session envelope

```go
Session {
  ID, CLI, ProjectCwd, StartedAt, EndedAt, TurnCount, Metadata,
  Segments []Segment   // lineage; empty if single-CLI
  ParentID string      // lineage root
}

Segment {
  CLI, NativeID, StartedAt, EndedAt, TurnCount
}
```

## Adapter contract — ResumeAdapter

Adapters that support cross-CLI resume implement an additional
interface beyond `SessionAdapter`:

```go
type ResumeAdapter interface {
  InjectSession(cwd string, turns []Turn) error
  ResumeCmd(nativeID string) []string
}
```

`usp resume --cli <cli>` calls `InjectSession()` to write
prior session turns into the target CLI's native format,
then `ResumeCmd()` returns the command to launch the CLI in
that session.

## Lineage and resume flow

```
usp session show <id>              → segment table per-CLI
usp session lineage <id>           → cross-CLI chain rendered
usp resume --cli codex            → InjectSession into codex format → exec ResumeCmd
```

The lineage store at
`~/.local/state/usp/sessions.db` tracks cross-CLI chains:

- `CreateSession(id, cwd)`
- `AddSegment(sessionID, cli, nativeID, turnCount)`
- `EndSegment()`
- `GetSession(id)` — returns full lineage with segment list

## USP-ctxt bridge (Pipeline B, T-0057)

`usp-ctxt sync` is a separate binary that walks each CLI's
session list since a high-water-mark, projects sessions to
ctxt-ready payloads, and upserts via `ctxt analyze
--mentions`.

Mentions identity model (post-T-0068, replaces tag-based
identity):

- `@usp.session.<id>` — canonical session identity
- `@agent.<id>` — producer (aps profile ID, sourced from `--agent` flag or `$USP_CTXT_AGENT`)
- `@cli.<name>` — originating CLI
- `@scope.<value>` — workspace / project scope

State at `~/.local/share/usp-ctxt/last_run.json` tracks
per-CLI per-project high-water-marks. Partial-failure safe:
re-runs retry skipped sessions.

Spec reference:
`<labspace>/hop/docs/ingestion-retrieval/spec.md` §4.

## UUID-prefix matching (T-0061)

`usp session show <id>` accepts partial UUID prefixes. The
ID-format pattern drives adapter priority:

| ID format | Adapter priority |
|---|---|
| UUIDv4 | Claude first |
| UUIDv7 | Codex first |
| `ses_*` prefix | OpenCode first |
| (other) | scan all |

This is purely a routing optimisation — all adapters are
still consulted on miss.

## Recent activity

- `docs: ctxt bridge (T-0061)` — `7348bb5`
- `test(uspctxt)` mentions assertions + slug normalisation (T-0068) — `3972bf5`
- `refactor(uspctxt)` bridge uses mention-based identity (T-0068) — `d8972a7`
- `refactor(uspctxt)` projection emits mentions instead of source-key (T-0068) — `2d849a2`
- `test(usp-ctxt)` projection + state + bridge unit tests (T-0057) — `fd9f93e`
- `feat(usp-ctxt)` batch bridge from usp sessions to ctxt (T-0057) — `b1b2b2a`
- `feat(session)` partial UUID prefix matching + project / since scoping — `6edaee4`
- `fix(list)` emit empty JSON array when no sessions — `d6d252b`
- `feat(claude)` inline tool_result output into preceding tool_call — `d0011ea`
- `fix(show)` route all formats through output.Render — `6ec99bd`

## Open questions

1. **Project index DB usage.** `index/project.go` is defined but not actively used in current list / show flow. When and where is `ProjectIndex` consulted?
2. **Copilot adapter.** README marks as planned; source not found. Roadmap or deferred?
3. **E2E cassette mode.** `e2e/cassettes/` exists. Is live recording active or tape-only?
4. **Partial UUID prefix spec.** T-0061 mentions partial UUID prefix matching. Full matching spec (8-char, 16-char) not documented.
5. **Slug normalisation rules.** Commit `3972bf5` mentions slug normalisation. Rules (lowercase, hyphens, max length) not documented.
6. **Resume fallback semantics.** If `usp resume --cli gemini` fails during `InjectSession()`, does the lineage store stay consistent?
7. **`docs/architecture/data-flow-v1.mmd`.** Diagram exists but not referenced from this doc. Up to date post-lineage / bridge?
