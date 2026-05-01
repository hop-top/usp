# AGENTS.md

This file is for agents (and humans piloting agents) who need to use `usp`
without reading the Go source. If you're a human looking for the user-facing
overview, start with [README.md](README.md) instead.

## What usp does

`usp` is a cross-CLI session-management tool. It indexes, searches, and
resumes coding sessions across multiple AI assistant CLIs from a single
command surface. The killer feature is **cross-CLI resume**: start a
conversation in one CLI (e.g. Claude Code), continue it in another (e.g.
Codex) with full context preserved.

There are two binaries in this repo:

- `usp` (this doc covers it) — `cmd/usp/`. Index + search + resume.
- `usp-ctxt` — `cmd/usp-ctxt/`. Bridges sessions into
  [ctxt](https://github.com/jadb/ContextHelp). Out of scope here; see
  [README.md](README.md) for the bridge overview.

## Supported CLIs

The supported set is whatever ships under [`adapters/`](adapters/). As of
this writing:

| CLI | Adapter package | Storage shape |
|---|---|---|
| Claude Code | `adapters/claude` | JSONL per session under `~/.claude/projects/<key>/<sid>.jsonl` |
| Codex CLI | `adapters/codex` | Date-partitioned JSONL under `~/.codex/sessions/` |
| Gemini CLI | `adapters/gemini` | JSON snapshots under `~/.gemini/sessions/` |
| OpenCode | `adapters/opencode` | 12-table SQLite under `~/.local/share/opencode/` |

To confirm the live set in your checkout: `ls adapters/` or
`usp doctor`. The set grows over time; see the README "Supported CLIs"
table for planned additions (Copilot, Kimi, Amp, Cursor Agent, etc.).
No specific upstream-CLI version constraints are encoded today — adapters
are tolerant of schema additions and skip unknown event types.

## Core commands (agent crib sheet)

```sh
usp doctor                           # health-check all CLIs + DBs
usp install                          # detect CLIs and build the index
usp session list                     # list sessions, all CLIs, newest first
usp session list --format json       # machine-readable; preferred for agents
usp session show <full-uuid>         # turn-by-turn metadata + content
usp session search "<query>"         # FTS over indexed session content
usp session lineage <full-uuid>      # cross-CLI continuation chain
usp resume --tool <cli>              # continue most recent session in <cli>
usp resume --session <id> --tool <cli>  # resume a specific source session
```

Full per-command flag reference: [`docs/usp/api-cli.md`](docs/usp/api-cli.md).

## Environment variables

`usp` follows the [XDG Base Directory Spec][xdg]. Defaults in parentheses
apply when the variable is unset.

[xdg]: https://specifications.freedesktop.org/basedir-spec/latest/

| Variable | Used for | Default path |
|---|---|---|
| `XDG_STATE_HOME` | Lineage DB (`sessions.db`) | `~/.local/state/usp/sessions.db` |
| `XDG_DATA_HOME` | Index DB (`index.db`) | `~/.local/share/usp/index.db` |
| `XDG_CONFIG_HOME` | Config file (when supported) | `~/.config/usp/config.yaml` |

> Heads-up: today the paths are still hardcoded under `~/.local/state/usp/`
> and `~/.local/share/usp/` regardless of `XDG_*`. Migration to the kit
> `xdg` helper is tracked under `hop-top/usp#T-0084`. Configuration-file
> support is tracked under `hop-top/usp#T-0093`. Treat the variable
> mapping above as the **target** behaviour; document new code against
> it so the migration is a no-op for callers.

There are no other usp-specific env vars at the moment. See
[`docs/usp/configuration.md`](docs/usp/configuration.md) for the planned
`USP_*` env-var surface that will land alongside config-file support.

## JSONL parsing gotchas

If you're writing tooling that reads CLI session files directly (instead
of going through `usp`), here are the traps adapters already handle for
you:

### Claude Code

- Transcripts live at `~/.claude/projects/<key>/<sid>.jsonl`.
- `<key>` is the project cwd with **both `/` and `.` replaced by `-`**.
  E.g. `~/projects/my.app` → `-Users-jadb-projects-my-app`. Note the
  collision risk: `my.app` and `my-app` map to the same key. The claude
  adapter handles this, but raw consumers must not.
- Each line is a JSON event. Events chain via `parentUuid` — the parent
  pointer for a turn is on the event itself, not derived from order.
- `isSidechain: true` events are **sub-agent dispatches** (Task tool
  fan-outs). They have their own UUIDs, run in parallel with the main
  thread, and are reattached after their parent `Task` tool_use call by
  the claude adapter (see `adapters/claude/adapter.go` `resolveSidechain`).
- `tool_use` and `tool_result` are split across separate events; the
  adapter inlines the result into the preceding call so the timeline
  reads naturally.
- `message.usage` carries token counts (input/output/cache_read/
  cache_write) and is only present on assistant turns.

### Codex / Gemini / OpenCode

- Codex partitions JSONL by date; iterate every dated subdir.
- Gemini stores full session snapshots, not append-only streams.
- OpenCode is a 12-table SQLite DB; querying the wrong join order will
  silently lose tool calls. Use the adapter.

In all cases: prefer calling `usp` over reparsing yourself.

## Lineage DB

- Path: `<XDG_STATE_HOME>/usp/sessions.db` (default
  `~/.local/state/usp/sessions.db`).
- Engine: SQLite.
- Schema lives in [`lineage/`](lineage/). Two tables — `usp_sessions`
  (one row per cross-CLI conversation) and `usp_segments` (one row per
  CLI hop within a conversation, ordered).
- Written by `usp resume`; read by `usp session lineage`.

## Index DB

- Path: `<XDG_DATA_HOME>/usp/index.db` (default
  `~/.local/share/usp/index.db`).
- Engine: SQLite, with an FTS5 virtual table over session content for
  fast `usp session search`.
- Built by `usp install`. Re-run `usp install` after a long pause to
  pick up new sessions; the install command is idempotent (will be
  renamed to `usp setup`; tracked `hop-top/usp#T-0090`).

## Calling usp from an agent

A few rules of thumb:

1. **Use `--format json`.** Every list/search/show command supports JSON
   output. Tabular output is for humans and uses ANSI styling that pollutes
   pipes. JSON is stable across versions; table layout is not.
2. **Pass full UUIDs.** Partial-prefix lookups exist in `session show` and
   are landing for other commands (tracked `hop-top/usp#T-0061`), but for
   now the safe path is to pull the full ID from `usp session list
   --format json` first, then feed it to `show`/`lineage`/`resume`.
3. **`usp resume` replaces the running process.** Resume calls
   `syscall.Exec` on the target CLI's binary so the new CLI inherits the
   terminal directly. This is what you want from an interactive shell;
   it's surprising in scripts (the parent `usp` is gone, so any `&&`
   chain after `usp resume ...` will not run). If you need to wait, use
   `usp session list` after the spawned CLI exits, not a follow-on
   command in the same line.
4. **Stderr is for status, stdout is for data.** `usp resume` prints
   `Resuming in <cli> ...` to stderr before exec-ing; `session list/
   search/show` print results to stdout via `kit/output.Render`. Pipe
   stdout, not 2>&1.
5. **Exit codes are 0 (ok) or 1 (any error) today.** Richer codes
   (2 usage, 3 not-found, 4 exists, etc.) are tracked under
   `hop-top/usp#T-0091`; do not branch on specific non-zero codes yet.

## Pre-merge gate

Before merging or opening a PR, work through the four-step gate from
`~/.agents/AGENTS.md` § "Pre-merge / Pre-PR Gate":

1. **Unit tests** — every changed/new function covered.
2. **Story coverage** — link the change to a story under
   [`docs/stories/`](docs/stories/) (or add one).
3. **E2E coverage** — new user-facing behaviour gets at least one e2e
   under [`e2e/`](e2e/).
4. **Docs** — README, [`docs/usp/api-cli.md`](docs/usp/api-cli.md), and
   [`CHANGELOG.md`](CHANGELOG.md) updated for user-visible changes.

Apply all four steps in parallel where possible. Skip a step only with
≥0.9 confidence it doesn't apply (e.g. pure internal refactor → no story
needed; config-only → no test needed). When in doubt, ask.

## Where to look in source

| Area | Path |
|---|---|
| CLI command wiring | [`cmd/usp/`](cmd/usp/) |
| Adapter implementations | [`adapters/<cli>/`](adapters/) |
| Session envelope schema | [`session/envelope.go`](session/envelope.go) |
| Mention extractors | [`session/extractors.go`](session/extractors.go) |
| Lineage store | [`lineage/store.go`](lineage/store.go) |
| Index + FTS | [`index/`](index/) |
| End-to-end tests | [`e2e/`](e2e/) |
| Architecture overview | [`docs/architecture.md`](docs/architecture.md) |
