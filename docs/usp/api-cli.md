# `usp` CLI reference

Last updated: 2026-04-30
Author: $USER

Full subcommand reference for `usp`. Re-generate by running
`usp <cmd> --help` after each release.

## Global flags

Every command accepts these:

| Flag | Default | Effect |
|---|---|---|
| `--config <path>` | layered search | Path to YAML config file (overrides standard search). |
| `--format` | `table` | Output format: `table`, `json`, `yaml`. |
| `--no-color` | off | Disable ANSI colour. |
| `--no-hints` | off | Suppress next-step hint footer. |
| `--offline` | off | Disable network operations (placeholder; usp has no network ops yet). |
| `--quiet` | off | Suppress non-essential output. |
| `-h`, `--help` | — | Show command help. |
| `-v`, `--version` | — | Show binary version (root only). |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Generic error (runtime, IO). |
| `2` | Usage error (cobra default; bad args/flags). |
| `3` | Not found (e.g. `session show <missing-id>`). |
| `4` | Already exists (reserved). |
| `5` | Unauthorized (reserved). |

---

## `usp`

**Purpose** — root command. Prints help when called with no args.

**Synopsis**

```sh
usp [command] [--flags]
```

**Top-level commands** (grouped in help output)

| Group | Command | Purpose |
|---|---|---|
| KNOWLEDGE | [`session`](#usp-session) | Manage cross-CLI sessions (sub-tree). |
| LIFECYCLE | [`resume`](#usp-resume) | Continue a conversation from one CLI in another. |
| ORGANIZE | [`doctor`](#usp-doctor) | Health-check the environment for supported CLIs. |
| ORGANIZE | [`setup`](#usp-setup) | Detect CLIs and (re)build the index. |

**Examples**

```sh
usp                                  # show help
usp --version                        # print version
usp session list --format json       # use any global flag at the root
```

---

## `usp doctor`

**Purpose** — Check environment health for supported CLIs.

**Synopsis**

```sh
usp doctor [--tool <cli>] [--format <fmt>]
```

**Args** — none.

**Flags**

| Flag | Default | Effect |
|---|---|---|
| `--tool <cli>` | all CLIs | Restrict checks to a single CLI (`claude`, `codex`, `gemini`, `opencode`). |

**Examples**

```sh
usp doctor                           # check everything
usp doctor --tool claude             # check only the claude adapter
usp doctor --format json             # machine-readable health report
```

**Cross-refs** — [`usp setup`](#usp-setup) (run if doctor flags
missing index).

---

## `usp setup`

**Purpose** — Detect installed CLIs and index their sessions into the
local SQLite index.

**Synopsis**

```sh
usp setup [cli] [--format <fmt>]
```

**Args**

| Arg | Default | Effect |
|---|---|---|
| `cli` | all detected | Index a single CLI (`claude`, `codex`, `gemini`, `opencode`). Empty = all. |

**Flags** — none beyond the globals.

**Examples**

```sh
usp setup                            # index all detected CLIs
usp setup claude                     # index only Claude Code
usp setup --format json              # JSON summary of what was indexed
```

**Cross-refs** — [`usp doctor`](#usp-doctor) (run before setup to see
what's detectable); [`usp session list`](#usp-session-list) (verify the
index post-setup).

---

## `usp resume`

**Purpose** — Continue a conversation from one CLI in another. Streams
turns from the source session, injects them into the target CLI's
storage, records lineage, and `exec`s the target CLI.

**Synopsis**

```sh
usp resume [<id>] --tool <cli>
```

**Args**

| Arg | Default | Effect |
|---|---|---|
| `<id>` | most recent for cwd | Source session ID (full UUID). |

**Flags**

| Flag | Default | Effect |
|---|---|---|
| `--tool <cli>` | _required_ | Target CLI to resume in (`claude`, `codex`, `gemini`, `opencode`). |

**Examples**

```sh
usp resume --tool codex                       # most recent session for cwd → Codex
usp resume a1b2c3d4-... --tool gemini         # explicit source session
usp resume --tool claude                      # round-trip back to Claude
```

**Behaviour notes**

- The current process is **replaced** via `syscall.Exec` once the target
  CLI is found on `$PATH`. Any shell command chained after `usp resume
  ...` in the same line will not run.
- A new "usp session" UUID is minted and linked to both source and
  target native IDs in the lineage DB.
- Status output ("Resuming in <cli>…") goes to stderr; the target CLI
  takes over stdout.

**Cross-refs** — [`usp session lineage`](#usp-session-lineage) (after
resuming, see the chain); [`usp session list`](#usp-session-list)
(find the source ID).

---

## `usp session`

**Purpose** — Group command for cross-CLI session operations. Prints
help when called with no sub-command.

**Synopsis**

```sh
usp session [command] [--flags]
```

**Sub-commands**

| Command | Purpose |
|---|---|
| [`list`](#usp-session-list) | List sessions across all CLIs. |
| [`search`](#usp-session-search) | Search session content. |
| [`show`](#usp-session-show) | Display metadata + turn summary. |
| [`lineage`](#usp-session-lineage) | Show cross-CLI continuation chain. |

---

## `usp session list`

**Purpose** — Enumerate sessions across all supported CLIs, newest first.

**Synopsis**

```sh
usp session list [--tool <cli>] [--project <path>] [--since <when>] [--limit N] [--format <fmt>]
```

**Args** — none.

**Flags**

| Flag | Default | Effect |
|---|---|---|
| `--tool <cli>` | all | Filter to a single CLI. |
| `--project <path>` | all | Filter to a project directory. |
| `--since <when>` | all time | Cutoff: ISO date (`2026-04-01`) or duration (`7d`, `24h`). |
| `--limit N` | `20` | Maximum sessions to display. |

**Examples**

```sh
usp session list                                      # 20 most recent across everything
usp session list --tool claude --since 7d --limit 5   # last 5 Claude sessions in past week
usp session list --project ~/projects/myapp --format json
```

**Cross-refs** — [`usp session show`](#usp-session-show) (drill into one);
[`usp session search`](#usp-session-search) (find by content not date).

---

## `usp session search`

**Purpose** — Full-text search across indexed session content.

**Synopsis**

```sh
usp session search <query> [--tool <cli>] [--project <path>] \
                            [--since <when>] [--limit N] [--format <fmt>]
```

**Args**

| Arg | Required | Effect |
|---|---|---|
| `<query>` | yes | FTS5 query string. Quote multi-word queries. |

**Flags**

| Flag | Default | Effect |
|---|---|---|
| `--tool <cli>` | all | Restrict to one CLI. |
| `--project <path>` | all | Restrict to a project dir. |
| `--since <when>` | all time | Cutoff: ISO date or duration (e.g. `7d`). |
| `--limit N` | `20` | Maximum result rows. |

**Examples**

```sh
usp session search "auth refactor"
usp session search graphql --tool codex --since 30d
usp session search "panic" --format json --limit 5
```

**Cross-refs** — [`usp session show`](#usp-session-show) (open a result).

---

## `usp session show`

**Purpose** — Display session metadata and turn summary by ID.

**Synopsis**

```sh
usp session show <id> [--tool <cli>] [--project <path>] [--since <when>] [--format <fmt>]
```

**Args**

| Arg | Required | Effect |
|---|---|---|
| `<id>` | yes | Session ID. Full UUID required for cross-CLI lookup; partial-prefix matching is supported when the prefix is unique. |

**Flags**

| Flag | Default | Effect |
|---|---|---|
| `--tool <cli>` | all | Restrict prefix-match search to one CLI. |
| `--project <path>` | all | Narrow prefix match to a project dir. |
| `--since <when>` | all time | Narrow prefix match to a recency window. |

**Examples**

```sh
usp session show fe2eb947-ecab-4293-a26c-3485062e8e6a    # full UUID
usp session show fe2eb947 --project ~/projects/tlc       # disambiguate by project
usp session show fe2eb947 --tool claude --format json
```

**Cross-refs** — [`usp session list`](#usp-session-list) (find the ID);
[`usp session lineage`](#usp-session-lineage) (continuation chain);
[`usp resume`](#usp-resume) (continue elsewhere).

---

## `usp session lineage`

**Purpose** — Show the cross-CLI continuation chain (every CLI hop)
for a usp session ID.

**Synopsis**

```sh
usp session lineage <id> [--format <fmt>]
```

**Args**

| Arg | Required | Effect |
|---|---|---|
| `<id>` | yes | usp session ID (the UUID minted by `usp resume`). |

**Flags** — none beyond the globals.

**Examples**

```sh
usp session lineage a1b2c3d4-e5f6-7890-abcd-ef1234567890
usp session lineage a1b2c3d4-... --format json
```

**Cross-refs** — [`usp resume`](#usp-resume) (creates lineage rows).

---

## Re-generating this doc

There is no `make docs` target yet (justfile doesn't include one). To
regenerate manually:

```sh
for cmd in "" doctor setup resume session "session list" "session search" \
           "session show" "session lineage"; do
    echo "### usp $cmd"; usp $cmd --help; echo
done
```

Cobra-docgen integration could automate this; not yet wired.
