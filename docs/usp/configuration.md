# `usp` configuration

Last updated: 2026-04-30
Author: $USER

> **Status:** target shape. As of `usp version dev` (commit `1381303`),
> `usp` ships with **no config-file support**. The only configurable
> knobs today are CLI flags. Layered config via `kit/config` is tracked
> as `hop-top/usp#T-0093` (F-11 in the parity review). This doc describes
> the post-T-0093 surface so callers can plan against it; once T-0093
> lands, the document should be moved out of "target shape" status.

## Loader precedence

When configuration support lands, the kit `config.Loader` resolves keys
in the following order, with **later sources winning** over earlier
ones:

1. Compiled-in defaults.
2. `/etc/usp/config.yaml` (system-wide).
3. `$XDG_CONFIG_HOME/usp/config.yaml` (default
   `~/.config/usp/config.yaml`).
4. `./.usp.yaml` (project-local; walks up from cwd to find the nearest).
5. `--config <path>` (explicit override; not yet implemented; tracked
   `hop-top/usp#T-0093`).
6. Environment variables (`USP_*`).
7. CLI flags.

A flag wins over an env var; an env var wins over any file; project
config wins over user config; user config wins over system config.

## Initial keys

| Key | Type | Default | Effect |
|---|---|---|---|
| `default_tool` | string | _unset_ | Default `--tool` filter for `session list`/`session search` and required `--tool` argument inference for `resume`. |
| `default_limit` | int | `20` | Default `--limit` for `session list`/`session search`. |

These are the keys that map cleanly to current flags. Additional keys
will be added as new flags land; each must keep flag/env/config name
consistent (kebab-case flag, SHOUT_SNAKE env, `snake_case` config).

## Environment variable mapping

The convention is `USP_` + the config key in SHOUT_SNAKE_CASE.

| Variable | Config key | Example |
|---|---|---|
| `USP_DEFAULT_TOOL` | `default_tool` | `USP_DEFAULT_TOOL=claude` |
| `USP_DEFAULT_LIMIT` | `default_limit` | `USP_DEFAULT_LIMIT=50` |

XDG variables that influence file layout (not configuration values
themselves) are documented in [`AGENTS.md`](../../AGENTS.md#environment-variables).

## Sample config file

`~/.config/usp/config.yaml`:

```yaml
# Default CLI for `--tool`-aware commands. Omit to scan every CLI.
default_tool: claude

# Default --limit for list/search.
default_limit: 50
```

A project-local override at `./.usp.yaml`:

```yaml
# In a Codex-heavy project, default to Codex.
default_tool: codex
```

With those two files, `usp session list` from inside the project
returns 50 most-recent Codex sessions. Outside the project, it returns
50 most-recent Claude sessions. Set `USP_DEFAULT_TOOL=gemini` for a
single shell to override both files; pass `--tool opencode` on the
command line to override everything.

## Validation rules

- `default_tool` must be one of the registered adapter names. The
  current set is `claude`, `codex`, `gemini`, `opencode`. Unknown values
  fail fast at load time with a hint listing the valid options.
- `default_limit` must be a positive integer. Non-positive values fail
  load.

## Cross-refs

- Loader order, key naming, and override semantics follow
  `~/.ops/docs/cli-conventions-with-kit.md` §7.
- File-system locations follow XDG; see [`AGENTS.md`](../../AGENTS.md)
  for the `XDG_*` matrix.
- Per-command flag reference: [`api-cli.md`](api-cli.md).
