# `usp` configuration

Last updated: 2026-04-30
Author: $USER

> **Status:** live. Layered config via `kit/config` shipped in T-0093.

## Loader precedence

Keys are resolved in the following order, with **later sources winning**
over earlier ones:

1. Compiled-in defaults.
2. `/etc/usp/config.yaml` (system-wide).
3. `$XDG_CONFIG_HOME/usp/config.yaml` (default
   `~/.config/usp/config.yaml`).
4. `./.usp.yaml` (project-local; walks up from cwd to find the nearest).
5. `--config <path>` (explicit override).
6. Environment variables (`USP_*`).
7. CLI flags.

A flag wins over an env var; an env var wins over any file; project
config wins over user config; user config wins over system config.

## Initial keys

| Key | Type | Default | Effect |
|---|---|---|---|
| `default_cli` | string | _unset_ | Default `--cli` filter for `session list`/`session search` and required `--cli` argument inference for `resume`. |
| `default_limit` | int | `20` | Default `--limit` for `session list`/`session search`. |

These are the keys that map cleanly to current flags. Additional keys
will be added as new flags land; each must keep flag/env/config name
consistent (kebab-case flag, SHOUT_SNAKE env, `snake_case` config).

## Environment variable mapping

The convention is `USP_` + the config key in SHOUT_SNAKE_CASE.

| Variable | Config key | Example |
|---|---|---|
| `USP_DEFAULT_CLI` | `default_cli` | `USP_DEFAULT_CLI=claude` |
| `USP_DEFAULT_LIMIT` | `default_limit` | `USP_DEFAULT_LIMIT=50` |

XDG variables that influence file layout (not configuration values
themselves) are documented in [`AGENTS.md`](../../AGENTS.md#environment-variables).

## Sample config file

`~/.config/usp/config.yaml`:

```yaml
# Default CLI for `--cli`-aware commands. Omit to scan every CLI.
default_cli: claude

# Default --limit for list/search.
default_limit: 50
```

A project-local override at `./.usp.yaml`:

```yaml
# In a Codex-heavy project, default to Codex.
default_cli: codex
```

With those two files, `usp session list` from inside the project
returns 50 most-recent Codex sessions. Outside the project, it returns
50 most-recent Claude sessions. Set `USP_DEFAULT_CLI=gemini` for a
single shell to override both files; pass `--cli opencode` on the
command line to override everything.

## Validation rules

- `default_cli` must be one of the registered adapter names. The
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
