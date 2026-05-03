# Changelog

Notable changes to `usp`. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

`usp` is pre-release. There is no published version yet; everything
listed here lives on `main`. Future version bumps will be cut by
[release-please](https://github.com/googleapis/release-please) — keep
the structured `### Added` / `### Changed` / `### Fixed` headings so
the auto-generated release notes pick the entries up.

## Unreleased

### CLI parity re-check (post-2026-05-02)

Aligns `usp` with the latest `~/.ops/docs/cli-conventions-with-kit.md`
spec (audit at `~/.ops/reviews/usp-cli-review-2026-05-02.md`). 11/11
findings filed; 8 implemented, 1 doc-only wontfix, 2 already covered.

#### Added

- `--verbose` / `-V` global flag — count-style; raises slog level to
  Debug then Trace. (T-0097)
- `-C` / `--chdir` global flag — switch cwd before running any
  subcommand. (T-0099)
- `--help-all` and `--help-<group>` — kit-auto-wired; reveals the
  hidden MANAGEMENT group. (T-0101)
- `usp version` subcommand — under MANAGEMENT; mirrors `--version`.
  (T-0107)
- `usp upgrade` subcommand — wraps `kit/upgrade.RunCLI` for the
  human prompt + GitHub release flow. (T-0111)
- `usp config path` and `usp config paths` — kit-shared introspection
  via `kit/cli/config.RegisterPathSubcommands`. (T-0113)
- `usp alias {add,list,remove}` — backed by `kit/console/alias` YAML
  store at `$XDG_CONFIG_HOME/usp/aliases.yaml`. Stored aliases are
  registered as runtime command shims so `usp <alias>` dispatches
  to the target. (T-0105)
- `CHANGELOG.md` — release-please-compatible structure for upcoming
  tagged releases. (T-0116)

#### Changed

- `session skills` now uses `--tool` (matching every other
  session-tree command) instead of the one-off `--cli`. (T-0109)
- JSON / YAML output formats render timestamps as ISO 8601;
  relative time wording (`5m ago`) is reserved for `--format=table`.
  (T-0115)
- `go.mod` carries a `replace` for `hop.top/kit` → `kit/hops/main`
  so the local kit worktree drives builds. (T-0118)

#### Documented (wontfix / deferred)

- `--profile` / `--instance` globals (spec §5) intentionally absent —
  usp is single-tenant local-only. See `docs/usp/api-cli.md` §
  "Globals intentionally absent". (T-0103)

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

### CLI parity with kit conventions (initial)

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
