# Changelog

## [0.1.0-alpha.2](https://github.com/hop-top/usp/compare/usp/v0.1.0-alpha.1...usp/v0.1.0-alpha.2) (2026-05-18)


### Features

* **ci:** publish to Scoop bucket on tag ([a38ff25](https://github.com/hop-top/usp/commit/a38ff25ede1f8439381f63351db982486121a7ca))
* **ci:** publish to Scoop bucket on tag (Windows install) ([c38ad07](https://github.com/hop-top/usp/commit/c38ad0763eef18e850a10dcd20787facaa7ddb3b))

## [0.1.0-alpha.1](https://github.com/hop-top/usp/compare/usp/v0.1.0-alpha.0...usp/v0.1.0-alpha.1) (2026-05-18)


### Features

* **adapters:** pluggable mention extractors per CLI (T-0078) ([afb1cad](https://github.com/hop-top/usp/commit/afb1cad56b6f8bed3ed748226af8c6a999b55a43))
* **adapters:** populate usage telemetry in Metadata (T-0080) ([d28a324](https://github.com/hop-top/usp/commit/d28a324c9012955f4f8d4991dec631018d817f72))
* all-projects listing, --since filter, session search, --tool flag ([173d160](https://github.com/hop-top/usp/commit/173d16081ba78e9b7508fac06bc52c30f8bf8e95))
* centralize session API and improve session UX ([97d7257](https://github.com/hop-top/usp/commit/97d72570a09c5bbbf762b8dc3874bd40a0576e04))
* **ci:** publish cross-platform binaries + Homebrew formula on tag ([7077e9b](https://github.com/hop-top/usp/commit/7077e9bb56676bcccd0b3574210dc9900b3ee14d))
* **ci:** publish cross-platform binaries + Homebrew formula on tag ([cc5e534](https://github.com/hop-top/usp/commit/cc5e534db6bb7daeb0a8cd61b0ed33b3e9879068))
* **claude:** inline tool_result output into preceding tool_call ([d0011ea](https://github.com/hop-top/usp/commit/d0011eaaf76e077d55c3357c2362db79c464b51c))
* **claude:** resolve sidechain sub-agents into timeline (T-0082) ([eca3d8d](https://github.com/hop-top/usp/commit/eca3d8d7d06dd26d4f15dfd4494acdae97accff5))
* **cli:** --help-all reveals MANAGEMENT (T-0101) ([0ec0e19](https://github.com/hop-top/usp/commit/0ec0e191706846b65e1dd7e2c2d7cccd357c3d24))
* **cli:** --verbose/-V global flag (T-0097) ([976153a](https://github.com/hop-top/usp/commit/976153ac386a114515bbdc2ceb0f47481c1faa22))
* **cli:** -C/--chdir global flag (T-0099) ([c39d9d4](https://github.com/hop-top/usp/commit/c39d9d4e500664d201b7e67520e684332620eb70))
* **cli:** alias subcommand via kit/console/alias (T-0105) ([a8518de](https://github.com/hop-top/usp/commit/a8518dec12ffa76243d0baa26e44dd77da86174a))
* **cli:** command groups + help-all (T-0089) ([aa78047](https://github.com/hop-top/usp/commit/aa7804753f77587911c046bedcd2331576f32418))
* **cli:** config file + globals (T-0093) ([3f5bacd](https://github.com/hop-top/usp/commit/3f5bacd3a059d79eb2c4a5b0820dd15698fd90a6))
* **cli:** config path/paths under MANAGEMENT (T-0113) ([e990958](https://github.com/hop-top/usp/commit/e990958a057cd3e0b6d1215255ff4ab86fc64141))
* **cli:** exit codes per spec section 8.1 (T-0091) ([d4c2c77](https://github.com/hop-top/usp/commit/d4c2c77469eb09ede34e3d163c8a5ba235efda24))
* **cli:** next-step hints (T-0092) ([8457aa4](https://github.com/hop-top/usp/commit/8457aa4ba4c969606c7658ec6facb79a66834a3d))
* **cli:** self-upgrade subcommand (T-0111) ([1e7c8f5](https://github.com/hop-top/usp/commit/1e7c8f5076815d5a3aec7a424a693289b44219df))
* **cli:** version subcommand under MANAGEMENT (T-0107 partial) ([17c5325](https://github.com/hop-top/usp/commit/17c5325747e0a2f514747ed830dbcae48b4fd670))
* cross-CLI session resume + lineage tracking ([6298c01](https://github.com/hop-top/usp/commit/6298c01fb7d94ee70d4adeb31d37f1f07ba5e93a))
* **e2e:** Go Docker test harness with xrr recording ([d852e20](https://github.com/hop-top/usp/commit/d852e20a4e936e66374118300396d664b9075f2c))
* **e2e:** scenario scripts for single-CLI, cross-resume, filters ([#2](https://github.com/hop-top/usp/issues/2)) ([e4150cc](https://github.com/hop-top/usp/commit/e4150cce2693fa0f1bb7ec26fa1f39584af0088d))
* **e2e:** xrr env wiring + cassette directory structure ([#3](https://github.com/hop-top/usp/issues/3)) ([b1f10de](https://github.com/hop-top/usp/commit/b1f10dec075759475d61228edda376e907be8ba9))
* **session:** add Segment lineage + ResumeAdapter interface ([082d748](https://github.com/hop-top/usp/commit/082d748c4e3e4f32f986a3c261302e8bdab72d21))
* **session:** add tools view ([c1fc03f](https://github.com/hop-top/usp/commit/c1fc03f3235118c8923782e9415b955f72a8626a))
* **session:** extend Turn schema for per-turn signals (T-0077) ([5bbfc3b](https://github.com/hop-top/usp/commit/5bbfc3b88eb2f3270cd00880b635881ce624177f))
* **session:** list skills invoked in session(s) by filter ([eb9b119](https://github.com/hop-top/usp/commit/eb9b1197784ef4de91c75cbaad39619f2cde35be))
* **session:** migrate session IDs to TypeID ([4e10f4a](https://github.com/hop-top/usp/commit/4e10f4add8aaefd25b5cd8f3d5fd20f7830ce460))
* **session:** partial UUID prefix matching + project/since scoping ([6edaee4](https://github.com/hop-top/usp/commit/6edaee43c157a69a0221a09ace7cf1e5517d7bd7))
* **session:** render transcripts via kit's ToolCapability taxonomy ([1a7ab68](https://github.com/hop-top/usp/commit/1a7ab684603e10aad97c89d9386887f3a011cbc0))
* usp v0.0.1-alpha.1 — cross-CLI session management ([e543b29](https://github.com/hop-top/usp/commit/e543b29b5bef6d268453657b4844b9539fa2679f))
* **usp-ctxt:** batch bridge from usp sessions to ctxt (T-0057) ([b1b2b2a](https://github.com/hop-top/usp/commit/b1b2b2a0ab0edef2abc57a8d1b9564b0ba200e30))
* **uspctxt:** emit [@file](https://github.com/file) mentions + intent summary (T-0079) ([334c915](https://github.com/hop-top/usp/commit/334c915102e525e3d9bff1a21197f56478bb2fce))
* **uspctxt:** hint cost band + model mention from telemetry (T-0081) ([1c3ea14](https://github.com/hop-top/usp/commit/1c3ea14334d6b426dc600241dc47216b2c7ebf97))


### Bug Fixes

* address PR [#18](https://github.com/hop-top/usp/issues/18) review feedback ([43397d3](https://github.com/hop-top/usp/commit/43397d343483720918f5be809f4594a8b35746c9))
* align sessions with kit projects and gemini resume ([612c6de](https://github.com/hop-top/usp/commit/612c6deeafc313dd2847847b8218da1e46321856))
* **ci:** mint release-please token from release-bot GitHub App ([d7cf8c3](https://github.com/hop-top/usp/commit/d7cf8c3c0f7b40b87adaab2c84edbc0fe1164d63))
* **ci:** mint release-please token from release-bot GitHub App ([856ae3d](https://github.com/hop-top/usp/commit/856ae3deedad7ccebb629fc32c455ef7a54dbf61))
* **claude:** extract tool_result content from user turns ([198021c](https://github.com/hop-top/usp/commit/198021c52146f9d2ad337a99ea1737799dd8e75c))
* **cli:** disable kit Layer-A validator until annotations adopted ([833c3b4](https://github.com/hop-top/usp/commit/833c3b4121559ff7596098d52544d4f395b66445))
* **cli:** ISO 8601 timestamps for JSON/YAML output (T-0115) ([b2bc9b6](https://github.com/hop-top/usp/commit/b2bc9b6fa580fc7a89eb85ddf15ef1ff8c64ed7a))
* **e2e:** address PR [#6](https://github.com/hop-top/usp/issues/6) review feedback ([114b51c](https://github.com/hop-top/usp/commit/114b51ccbe179a9914a2a1b68adab21667b5f83d))
* **e2e:** drop dead xrr CLI install from Dockerfile ([91b6b5a](https://github.com/hop-top/usp/commit/91b6b5a95b111f520892fdc952285be1b7839e8f))
* **e2e:** make xrr cassettes replay-safe ([bd3f362](https://github.com/hop-top/usp/commit/bd3f36256dfeb974aebfd5e540c44e9d027d041d))
* list codex sessions with stale index ([975b2ed](https://github.com/hop-top/usp/commit/975b2ed63e18d7dcb69a368c4dde25e04ccac932))
* **list:** emit empty JSON array when no sessions found ([d6d252b](https://github.com/hop-top/usp/commit/d6d252b6fb1f092ffed35b85ba356c65214e460b))
* **show:** route all formats through output.Render ([6ec99bd](https://github.com/hop-top/usp/commit/6ec99bd8aacc4fe03e0e63255a28e47d0ec0af3d))
* use gemini resume flag ([33bdf74](https://github.com/hop-top/usp/commit/33bdf74f84c967bb282839732c156e4fe08a47e2))

## Changelog

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
- `usp session tools` — lists tool calls across matching sessions,
  enriched with Kit's universal tool taxonomy.
- `CHANGELOG.md` — release-please-compatible structure for upcoming
  tagged releases. (T-0116)

#### Changed

- CLI-selection flags use `--cli` so they don't collide with
  model/tool invocation terminology.
- JSON / YAML output formats render timestamps as ISO 8601;
  relative time wording (`5m ago`) is reserved for `--format=table`.
  (T-0115)
- `go.mod` pins `hop.top/kit v0.4.0-alpha.2` from the published
  mirror at `github.com/hop-top/kit`; the prior local `replace`
  directive has been removed. (T-0118)

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
- `usp resume <id> --cli <cli>` — stream turns from one CLI's
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
  `USP_*` env → CLI flags. Initial keys: `default_cli`, `default_limit`.
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
