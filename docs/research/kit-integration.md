# hop-top/kit — USP Integration Map

> Which USP concerns delegate to which kit package. Keeps USP thin.
> Living doc; update when kit ships new packages or changes APIs.

Last verified: 2026-04-08. Kit README:
<https://github.com/hop-top/kit/blob/main/README.md>

## Kit packages USP uses

| Package        | Purpose                                            | USP usage                                                         |
|----------------|----------------------------------------------------|-------------------------------------------------------------------|
| `kit/cli`      | fang+cobra+viper root command factory              | `usp` binary root + all subcommands; free `--format`/`--quiet`/`--no-color`/`--version` |
| `kit/config`   | Layered config loader (system→user→project→env)    | `sessions.yaml` loader (global + project); adapter capability maps |
| `kit/log`      | Viper-configured `charm.land/log/v2` wrapper       | Daemon + adapter logging (structured, level-aware)                |
| `kit/xdg`      | XDG path resolution (config/data/cache/state)      | All paths: session db, index, pidfile, cache                      |
| `kit/output`   | table/json/yaml renderer (owns `--format`)         | `usp session list`, `show`, `doctor` outputs                      |
| `kit/sqlstore` | Generic SQLite kv store with TTL                    | Project index (cwd → native keys); session cache                  |
| `kit/tui`      | Themed bubbletea/v2 components (spinner/confirm)    | `usp install` wizard; `usp session pick` interactive picker       |
| `kit/upgrade`  | Self-upgrade check/download/replace                | `usp upgrade` command                                             |
| `kit/markdown` | Glamour v2 terminal markdown renderer              | `usp doctor` rich reports; session replay pretty-print            |

## Kit packages USP may skip (v1)

| Package        | Reason                                             |
|----------------|----------------------------------------------------|
| `kit/llm`      | USP doesn't call LLMs; handlers can if they want   |
| `kit/templates`| No install-time templating required for v1        |

## Cross-language kit

USP also uses kit's TS and Python ports for the SDKs:

| Kit package          | USP SDK using it           |
|----------------------|----------------------------|
| `@hop/kit-ts/cli`    | `sdk/ts/` (commander wrapper for session query helpers) |
| `@hop/kit-ts/output` | SDK output formatting      |
| `hop_kit.cli`        | `sdk/py/` (Typer baseline) |
| `hop_kit.output`     | SDK output formatting      |

## What USP builds from scratch vs delegates

### Delegates to kit

- CLI bootstrap (`kit/cli`) — USP never touches fang/cobra/viper directly
- Config loading (`kit/config`) — layered `sessions.yaml` merge
- Structured logging (`kit/log`) — every daemon/adapter log line
- XDG paths (`kit/xdg`) — state db, cache, logs
- Output formatting (`kit/output`) — `--format` flag handled for free
- Project index (`kit/sqlstore`) — cwd → native-key map per-CLI
- Self-update (`kit/upgrade`)
- Interactive install + picker (`kit/tui`)

### USP-specific (can't delegate)

- Adapter readers for each CLI's native session store (JSONL tailers,
  SQLite readers, date-partition walkers)
- Project-key translators per CLI (slash-to-dash, basename lookup,
  SHA1, header extraction)
- Session graph builder (joining Claude sidechains, OpenCode 5-way
  split, linking resumed sessions to parents)
- Filesystem watcher coordinator (fswatch/inotify bridges per adapter)
- Normalized USP session envelope schema + validator
- Cross-CLI session search (full-text + metadata)

## CLI contract USP inherits free

From `kit/cli` every USP command gets:

- `--format {table,json,yaml}` — rendered via `kit/output.Render()`
- `--quiet` — suppress non-essential output
- `--no-color` — disable ANSI
- `-v`/`--version` — root-only, handled by fang
- `-h`/`--help` — at any level, styled by fang
- Accent color (pick one for USP, e.g., `#7C5CFF`)
- Structured error output to stderr, non-zero exit, no stack traces

## Paths (via `kit/xdg`)

```pseudocode
xdg.ConfigDir("usp")    -> ~/.config/usp/       # sessions.yaml, adapter config
xdg.DataDir("usp")      -> ~/.local/share/usp/  # installed adapters
xdg.CacheDir("usp")     -> ~/.cache/usp/        # session search index
xdg.StateDir("usp")     -> ~/.local/state/usp/  # pidfile, projects.db, logs
```

Runtime socket: `$XDG_RUNTIME_DIR/usp.sock` (resolved via kit/xdg or
stdlib fallback on darwin).

## Config layering (via `kit/config`)

```pseudocode
config.Load(&cfg, config.Options{
  Name: "usp",
  Layers: []string{
    "/etc/usp/config.yaml",                # system (optional)
    xdg.ConfigDir("usp") + "/config.yaml",  # user global
    ".usp/config.yaml",                     # project
    // env vars prefixed USP_*
  },
})
```

Same pattern for `sessions.yaml` discovery — USP's adapter loader
consumes `kit/config` layers and merges adapter enable-lists +
per-adapter overrides.

## Logging (via `kit/log`)

```pseudocode
// kit/cli.New() wires log automatically from viper flags
log := log.New(root.Viper)
log.Info("session discovered", "cli", "claude", "project", cwd, "id", sid)
log.Error("adapter read failed", "cli", "codex", "path", path, "err", err)
```

Log level controlled by `--log-level` flag (free from kit). No custom
logger needed.

## Project index (via `kit/sqlstore`)

Central per-cwd index mapping USP's canonical project key (absolute
cwd) to each CLI's native representation:

```pseudocode
store := sqlstore.Open(xdg.StateDir("usp") + "/projects.db", sqlstore.Options{
  # no TTL — project metadata is long-lived
})

# Record per-CLI native keys for a cwd
store.Put("project:"+cwd, projectMetaJSON, sqlstore.Meta{
  "claude_key":    "-Users-jadb--w-ideacrafterslabs-uhp",
  "gemini_alias":  "main-7",
  "codex_count":   42,            # updated by walker
  "opencode_hash": "4e2f4754d19ed904f11553caadcd97fca696dcaa",
  "last_seen":     now,
})

# Lookup native key for a cwd
store.Query(sqlstore.Filter{Key: "project:"+cwd})
```

Benefits:
- Single source of truth for cross-CLI project identity
- Reverse-lookup cache for Codex (expensive first-line scans)
- Feeds `usp session list --project <cwd>` across all CLIs

## Session cache (via `kit/sqlstore`)

Optional denormalized session cache for fast search:

```pseudocode
cache := sqlstore.Open(xdg.CacheDir("usp") + "/sessions.db", sqlstore.Options{
  TTL: 90 * 24 * time.Hour,
})

cache.Put("session:"+cli+":"+sid, sessionHeaderJSON, sqlstore.Meta{
  "cli":     cli,
  "cwd":     cwd,
  "started": startedAt,
  "ended":   endedAt,       # null if active
  "turns":   turnCount,
})
```

## TUI wizard + picker (via `kit/tui`)

```pseudocode
# usp install claude
spinner := tui.NewSpinner("Scanning ~/.claude/projects/...")
# ...walk...
spinner.Success("Found 136 project dirs, 412 sessions")

if !tui.Confirm("Index all? (might take ~30s)") { return }

prog := tui.NewProgress(4, "Indexing Claude sessions")
prog.Step("Building project-key map")
prog.Step("Reading JSONL headers")
prog.Step("Populating projects.db")
prog.Step("Verifying socket reachable")
prog.Done("Claude adapter installed (14 native dimensions)")
```

```pseudocode
# usp session pick
picker := tui.NewPicker("Recent sessions")
for _, s := range recentSessions {
  picker.Add(s.ID, fmt.Sprintf("%s — %s (%s)", s.CLI, s.Title, s.StartedAt))
}
selected := picker.Run()
# launch: usp session show <selected>
```

## Open questions

1. Does `kit/config` support hot reload via fsnotify, or does USP need
   to layer that itself for live adapter enable/disable?
2. Does `kit/sqlstore` support compound indexes for fast
   `cli+cwd+started` filtering, or only single-key lookups?
3. Is `kit/upgrade` compatible with per-adapter versioning (USP core
   + per-CLI adapter versions)?
4. Does `@hop/kit-ts/cli` expose stdin/stdout JSON helpers useful
   for the USP SDK session query API?
5. Can `kit/tui.Picker` render dense multi-column rows (cli, cwd,
   started, turns, size), or should USP render manually?

## Integration checklist (for Phase 2 bootstrap task)

- [ ] `go get hop.top/kit@latest`
- [ ] `cmd/usp/main.go` uses `cli.New(cli.Config{Name:"usp", Accent:"#7C5CFF"})`
- [ ] All subcommand files use `root.Viper` for flags
- [ ] Replace any ad-hoc `os.UserConfigDir()` with `xdg.ConfigDir("usp")`
- [ ] Replace any `fmt.Printf` with `log.Info/Error` from `kit/log`
- [ ] Replace any hand-rolled renderers with `output.Render(w, format, v)`
- [ ] Delete any copy-pasted CLI contract code (fang handles it)
- [ ] Use `kit/sqlstore` for `projects.db` + `sessions.db`
- [ ] Use `kit/tui` for `install` wizard + `session pick`
- [ ] Use `kit/upgrade` for `usp upgrade`
