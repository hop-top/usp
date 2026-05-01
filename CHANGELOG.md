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
- `usp setup` command (T-0090) — replaces `usp install` (kept as a
  hidden deprecated alias for one release).
- Help-output groups (KNOWLEDGE / LIFECYCLE / ORGANIZE) so `usp --help`
  scans cleanly as the surface grows (T-0089).
- Richer exit codes per cli-conventions §8.1: `2` usage, `3` not found
  (T-0091). `4` exists / `5` unauthorized reserved.
- Next-step hints printed to stderr after successful runs; suppressed by
  `--no-hints` or non-tty stdout (T-0092).
- `--config <path>` and `--offline` global flags; layered config loader
  via `kit/config` honoring `/etc/usp/`, `$XDG_CONFIG_HOME/usp/`,
  `./.usp.yaml`, `--config`, `USP_*` env, then CLI flags (T-0093).
- Initial config keys: `default_tool`, `default_limit`.

### Changed

- `usp-ctxt` bridge identity is now mention-based (T-0068): the bridge
  identifies entities via their mentions instead of opaque source keys.
- `usp-ctxt` projection emits mentions instead of source-key strings
  (T-0068).
- `usp resume` takes the source ID positionally: `usp resume <id>
  --tool <cli>` (T-0087). `--session <id>` is a hidden deprecated alias.
- `--format` now inherits from the root global; per-subcommand
  redeclarations removed (T-0088).
- Lineage and index DB paths honor `XDG_STATE_HOME` / `XDG_DATA_HOME`
  via `kit/xdg`; defaults unchanged (T-0084).
- `usp doctor` and `usp setup` render via `kit/output.Render`, so
  `--format json|yaml` works end-to-end (T-0085).
- Default `slog` handler routes to stderr via `kit/log`; `-V` / `--quiet`
  control level (T-0086).

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

- Partial-UUID prefix lookup not yet supported in `usp session show`
  / `lineage` / `lineage <id>`; full UUID required (T-0061).
- `--help-all` not auto-wired (kit v0.3.2-patch.3 lacks
  `cli.HelpConfig`); groups still render under `--help`.

[Unreleased]: https://github.com/hop-top/usp/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/hop-top/usp/releases/tag/v0.1.0
