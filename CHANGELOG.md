# Changelog

Notable changes to `usp`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

`usp` is pre-release. There is no published version yet; everything
listed here lives on `main`.

## Unreleased

### Core

- Cross-CLI session indexing across Claude Code, Codex CLI, Gemini
  CLI, OpenCode.
- `usp doctor` — environment health check per CLI.
- `usp setup` — detect CLIs and build the local index.
- `usp session list / show / search / lineage` — read-side surface,
  full-text search, cross-CLI continuation chain.
- `usp resume <id> --tool <cli>` — stream turns from one CLI's
  session and `syscall.Exec` into another, recording lineage.
- `usp-ctxt` companion binary — bridge usp sessions into ctxt as
  knowledge objects with stable mentions; idempotent re-runs via
  per-CLI high-water-mark file.

### Adapter signal extraction

- Per-turn signal fields on `session.Turn` (`Subtype`, `Metadata`).
- Per-CLI mention extractors lift `@file` mentions out of tool calls
  and into the normalized envelope.
- Usage telemetry on `Session.Metadata`: input/output/cache_read/
  cache_write tokens, cost (claude pricing table), duration, model,
  CLI version — populated when the source CLI exposes them.
- Claude sub-agent (sidechain) resolution: `isSidechain:true` events
  reattach after their parent `Task` tool_use; `Turn.Subtype="sidechain"`.

### uspctxt projection

- Mention vocabulary: `@usp.session`, `@agent`, `@cli`, `@project`,
  `@usp.lineage`, `@scope`, `@file`, `@model`.
- Hint vocabulary: `#hash`, `#cost:low|med|high`, `#tokens:small|med|large`.
- Body sections: `## Intents` (slash commands invoked) and
  `## Telemetry` (model/tokens/duration/cost when present).
- Bridge identity is mention-based, not source-key (T-0068).

### CLI parity with kit conventions

- Help-output groups: KNOWLEDGE / LIFECYCLE / ORGANIZE.
- Exit codes per cli-conventions §8.1: `0` ok, `1` generic, `2`
  usage, `3` not found. `4` exists / `5` unauthorized reserved.
- Next-step hints (kit/output.HintSet) on `setup` / `doctor` /
  `resume` / `session list`. Suppressed on non-tty or `--no-hints`.
- Globals: `--config <path>`, `--offline` (placeholder).
- Layered config (kit/config): defaults → `/etc/usp/config.yaml` →
  `$XDG_CONFIG_HOME/usp/config.yaml` → `./.usp.yaml` → `--config` →
  `USP_*` env → CLI flags. Initial keys: `default_tool`, `default_limit`.
- Lineage and index DBs honor `XDG_STATE_HOME` / `XDG_DATA_HOME` via
  kit/xdg. Defaults: `~/.local/state/usp/sessions.db` and
  `~/.local/share/usp/index.db`.
- `usp doctor` and `usp setup` render via `kit/output.Render`;
  `--format json|yaml` works end-to-end.
- Default `slog` handler routes to stderr via kit/log;
  `-V`/`-VV` lower level, `--quiet` raises it.
- `--format` inherited from the root persistent flag; per-subcommand
  redeclarations removed.

### Known limitations

- Partial-UUID prefix lookup not yet supported (T-0061); full UUID
  required for `session show / lineage / resume <id>`.
- `--help-all` not auto-wired (kit v0.3.2-patch.3 lacks
  `cli.HelpConfig`); `--help` still renders groups correctly.
