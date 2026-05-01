# Changelog

All notable changes to `usp` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Per-CLI mention extractors (T-0078): adapters can plug language- or
  tool-specific extractors that lift `@file`, `@model`, etc. mentions out
  of raw transcripts into the normalized envelope.
- Per-turn signal fields on the `Turn` schema (T-0077) so adapters can
  attach intent, sub-agent, and tool-call metadata to individual turns.
- Usage telemetry in `Session.Metadata` (T-0080): input/output/cache_read/
  cache_write token counts, durations, and CLI versions where the source
  CLI exposes them.
- Claude sub-agent (sidechain) resolution in the timeline (T-0082):
  `isSidechain: true` events are now reattached after their parent
  `Task` tool_use, with `Turn.Subtype = "sidechain"`.
- `usp-ctxt` projection emits `@file` mentions and `## Intents` sections
  (T-0079), so ctxt search can find sessions by file touched or
  slash-command intent.
- `usp-ctxt` cost-band and model mentions (T-0081): hint tags
  `#cost:low|med|high` plus `@model.<slug>` mentions derived from
  Claude pricing × usage telemetry.
- Signals reference table in README and architecture docs (T-0083).
- `AGENTS.md`, `docs/usp/api-cli.md`, `docs/usp/configuration.md`,
  this `CHANGELOG.md` (T-0094).

### Changed

- `usp-ctxt` bridge identity is now mention-based (T-0068): the bridge
  identifies entities via their mentions instead of opaque source keys.
- `usp-ctxt` projection emits mentions instead of source-key strings
  (T-0068).

## [0.1.0] — 2026-04-30

Initial public release.

### Added

- Cross-CLI session indexing across Claude Code, Codex CLI, Gemini CLI,
  and OpenCode.
- `usp doctor` — environment health check per CLI.
- `usp install` — detect CLIs and build the local index.
- `usp session list` — list sessions across all CLIs with `--tool`,
  `--project`, `--since`, `--limit`, `--format` filters.
- `usp session show <id>` — display session metadata + turn summary,
  with partial-UUID prefix matching scoped by `--project`/`--since`/
  `--tool`.
- `usp session search <query>` — FTS5 search over indexed session
  content.
- `usp session lineage <id>` — show the cross-CLI continuation chain
  for a usp session.
- `usp resume --tool <cli>` — continue a conversation from one CLI in
  another via `syscall.Exec` handoff. Lineage rows recorded in the
  state DB.
- `usp-ctxt` companion binary — batch-bridge usp sessions into ctxt
  knowledge objects, idempotent re-runs via per-CLI high-water-mark
  file.
- Adapter plug points: `SessionAdapter`, `ResumeAdapter`,
  `MentionExtractor` interfaces in [`session/`](session/).
- Lineage SQLite store at `~/.local/state/usp/sessions.db`
  (`lineage/`).
- Session index SQLite store at `~/.local/share/usp/index.db`
  (`index/`).
- End-to-end replay tests covering multi-CLI shared sessions
  (`e2e/`).

### Known issues

- Paths are hardcoded under `~/.local/state/usp/` and
  `~/.local/share/usp/`; XDG migration is tracked under T-0084.
- No config-file support yet; layered loader tracked under T-0093.
- Exit codes collapse every error to `1`; richer codes tracked under
  T-0091.
- `usp resume` takes the source ID via `--session` flag; positional
  form tracked under T-0092.

[Unreleased]: https://github.com/hop-top/usp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hop-top/usp/releases/tag/v0.1.0
