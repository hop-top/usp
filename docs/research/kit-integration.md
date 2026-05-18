# hop-top/kit — USP Integration Map

> Which USP concerns delegate to which kit package. Keeps USP thin.
> Living doc; update when kit ships new packages or changes APIs.

Last verified: 2026-04-11 against kit codebase on disk.
Kit README: <https://github.com/hop-top/kit/blob/main/README.md>

## Kit packages USP uses

| Package        | Purpose                                            | USP usage                                                         |
|----------------|----------------------------------------------------|-------------------------------------------------------------------|
| `kit/cli`      | cobra+fang+viper root command factory              | `usp` binary root + all subcommands; free `--format`/`--quiet`/`--no-color`/`--version` |
| `kit/config`   | Layered YAML config (system->user->project->env)   | `sessions.yaml` loader (global + project); adapter capability maps |
| `kit/log`      | Viper-configured `charm.land/log/v2` wrapper       | Daemon + adapter logging (structured, level-aware)                |
| `kit/xdg`      | XDG path resolution (config/data/cache/state)      | All paths: session db, index, pidfile, cache                      |
| `kit/output`   | table/json/yaml renderer (owns `--format`)         | `usp session list`, `show`, `doctor` outputs                      |
| `kit/sqlstore`  | Generic SQLite kv store with TTL                   | Project index (cwd -> native keys); session cache                 |
| `kit/tui`      | Themed bubbletea/v2 components                     | `usp install` wizard; interactive session browsing                |
| `kit/upgrade`  | Self-upgrade check/download/replace                | `usp upgrade` command                                             |
| `kit/markdown` | Glamour-based terminal markdown renderer           | `usp doctor` rich reports; session replay pretty-print            |
| `kit/toolspec` | Structured CLI tool knowledge registry             | Adapter tool discovery; error-pattern matching per CLI             |

## Kit packages USP may skip (v1)

| Package         | Reason                                            |
|-----------------|---------------------------------------------------|
| `kit/llm`       | USP doesn't call LLMs; handlers can if they want  |
| `kit/templates`  | Cross-lang build tooling; not needed at runtime   |
| `kit/proto`     | RouteLLM protobuf defs; not relevant to USP       |
| `kit/tracks`    | Kit-internal planning; not a Go package           |

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

- CLI bootstrap (`kit/cli`) — USP never touches cobra/viper directly
- Config loading (`kit/config`) — layered `sessions.yaml` merge
- Structured logging (`kit/log`) — every daemon/adapter log line
- XDG paths (`kit/xdg`) — state db, cache, logs
- Output formatting (`kit/output`) — `--format` flag handled for free
- Project index (`kit/sqlstore`) — cwd -> native-key map per-CLI
- Self-update (`kit/upgrade`)
- Interactive install + list browsing (`kit/tui`)
- Tool knowledge (`kit/toolspec`) — CLI command/flag/error registry

### USP-specific (can't delegate)

- Adapter readers for each CLI's native session store (JSONL
  tailers, SQLite readers, date-partition walkers)
- Project-key translators per CLI (slash-to-dash, basename
  lookup, SHA1, header extraction)
- Session graph builder (joining Claude sidechains, OpenCode
  5-way split, linking resumed sessions to parents)
- Filesystem watcher coordinator (fswatch/inotify bridges per
  adapter)
- Normalized USP session envelope schema + validator
- Cross-CLI session search (full-text + metadata)
- Interactive picker (kit/tui has no Picker; USP builds its own
  or composes with `tui.List`)

## CLI contract USP inherits free

From `kit/cli` every USP command gets:

- `--format {table,json,yaml}` — via `output.RegisterFlags()`
- `--quiet` — suppress non-essential output
- `--no-color` — disable ANSI
- `-v`/`--version` — root-only, handled by fang
- `-h`/`--help` — at any level, styled by fang
- Accent color (pick one for USP, e.g., `#7C5CFF`)
- Styled error output to stderr, non-zero exit, no stack traces

Note: no `--log-level` flag provided by kit/cli. USP must register
its own if needed, or rely on env-var config via `kit/log`.

## Paths (via `kit/xdg`)

All xdg functions return `(string, error)` — handle the error.

```pseudocode
dir, err := xdg.ConfigDir("usp")  // ~/.config/usp/
dir, err := xdg.DataDir("usp")    // ~/.local/share/usp/
dir, err := xdg.CacheDir("usp")   // ~/.cache/usp/
dir, err := xdg.StateDir("usp")   // ~/.local/state/usp/
```

Runtime socket: `$XDG_RUNTIME_DIR/usp.sock` — no `RuntimeDir` in
kit/xdg; USP resolves via `os.Getenv("XDG_RUNTIME_DIR")` or
stdlib fallback on darwin.

## Config layering (via `kit/config`)

`config.Options` uses named path fields, not a `Layers` slice:

```pseudocode
config.Load(&cfg, config.Options{
  SystemConfigPath:  "/etc/usp/config.yaml",
  UserConfigPath:    cfgDir + "/config.yaml",
  ProjectConfigPath: ".usp/config.yaml",
  EnvOverride: func(cfg any) {
    // apply USP_* env vars here
  },
})
```

Same pattern for `sessions.yaml` discovery — USP's adapter loader
consumes `kit/config` layers and merges adapter enable-lists +
per-adapter overrides.

## Logging (via `kit/log`)

```pseudocode
// kit/cli.New() does NOT wire log automatically.
// USP must create logger from root viper:
log := log.New(root.Viper)
log.Info("session discovered",
  "cli", "claude", "project", cwd, "id", sid)
log.Error("adapter read failed",
  "cli", "codex", "path", path, "err", err)
```

Log level controlled by viper config (no `--log-level` flag from
kit). USP can register its own flag and bind to viper, or use
`USP_LOG_LEVEL` env var via `EnvOverride`.

## Project index (via `kit/sqlstore`)

Central per-cwd index mapping USP's canonical project key to each
CLI's native representation.

`sqlstore.Put` signature: `Put(ctx, key, v any) error` — no Meta
param. Store is pure kv (key -> JSON blob). Custom queries via
`store.DB()` for anything beyond key lookup.

```pseudocode
store, err := sqlstore.Open(
  stateDir + "/projects.db",
  sqlstore.Options{},  // no TTL for project metadata
)

// Record per-CLI native keys for a cwd (as JSON value)
store.Put(ctx, "project:"+cwd, projectMeta)

// Lookup — unmarshals into dst, returns (found, err)
found, err := store.Get(ctx, "project:"+cwd, &meta)
```

For compound queries (cli+cwd+started), use `store.DB()` with
`Options.MigrateSQL` to create custom tables/indexes:

```pseudocode
store, err := sqlstore.Open(path, sqlstore.Options{
  MigrateSQL: `CREATE TABLE IF NOT EXISTS sessions (
    cli TEXT, cwd TEXT, started TEXT, sid TEXT,
    PRIMARY KEY (cli, sid)
  );
  CREATE INDEX IF NOT EXISTS idx_sessions_cwd
    ON sessions(cwd, started);`,
})
// Then query via store.DB().QueryContext(...)
```

## Session cache (via `kit/sqlstore`)

Optional denormalized session cache for fast search:

```pseudocode
cache, err := sqlstore.Open(
  cacheDir + "/sessions.db",
  sqlstore.Options{TTL: 90 * 24 * time.Hour},
)
cache.Put(ctx, "session:"+cli+":"+sid, sessionHeader)
```

For structured queries on session metadata, use `MigrateSQL` +
custom tables via `cache.DB()` as shown above.

## TUI components (via `kit/tui`)

Available components (all themed via `cli.Theme`):

- `tui.NewSpinner(theme)` — dot spinner styled with accent
- `tui.NewConfirm(prompt, theme)` — yes/no prompt (tea.Model)
- `tui.NewProgress(theme)` — gradient progress bar (0.0-1.0)
- `tui.NewList(height)` — scrollable item list with follow mode
- `tui.NewBadge(checker, theme)` — upgrade notification badge
- `tui.NewPillBar(pills...)` — styled tag/pill display
- `tui.NewStatus(theme, keymap)` — status bar component
- `tui.NewAnim(settings)` — animation component
- `tui.NewModel(theme, w, h)` — composite base model
- `dialog.NewOverlay()` — modal dialog overlay
- `styles.NewStyles(theme)` — style set for custom components

No Picker component exists. USP options:
1. Build picker from `tui.List` + key bindings
2. Use `tui.NewConfirm` for simple selection
3. Build custom bubbletea model using `tui/styles`

```pseudocode
// usp install claude
spinner := tui.NewSpinner(root.Theme)
// ... run as bubbletea model ...

confirm := tui.NewConfirm(
  "Index all? (~30s)", root.Theme,
)

prog := tui.NewProgress(root.Theme)
prog = prog.SetPercent(0.25)
```

## Tool knowledge (via `kit/toolspec`)

Registry of structured CLI tool specs — commands, flags, error
patterns, workflows. USP adapters can resolve tool metadata:

```pseudocode
reg := toolspec.NewRegistry(
  toolspec.WithSource(sources.Help{}),
  toolspec.WithCache(store),  // sqlstore satisfies Cache
)
spec, err := reg.Resolve("claude")
cmd := spec.FindCommand("chat")
```

Useful for: adapter auto-discovery, error-message matching,
generating help text for USP's cross-CLI interface.

## Open questions

1. ~~Does `kit/config` support hot reload via fsnotify?~~
   **No.** `config.Load` is one-shot. USP must layer fsnotify
   itself for live adapter enable/disable.

2. ~~Does `kit/sqlstore` support compound indexes?~~
   **Not built-in.** Store is pure kv. Use `Options.MigrateSQL`
   to create custom tables + indexes, then query via `DB()`.

3. ~~Is `kit/upgrade` compatible with per-adapter versioning?~~
   **Partially.** `upgrade.New()` accepts `WithBinary(name, ver)`
   per invocation. USP can create multiple `Checker` instances
   (one per adapter) but must manage the lifecycle. No built-in
   multi-binary coordination.

4. Does `@hop/kit-ts/cli` expose stdin/stdout JSON helpers useful
   for the USP SDK session query API? (Not verifiable on disk —
   TS port lives elsewhere.)

5. ~~Can `kit/tui.Picker` render dense multi-column rows?~~
   **Moot.** No Picker exists. USP must build its own using
   `tui.List` or a custom bubbletea model.

## Integration checklist (for Phase 2 bootstrap task)

- [ ] `go get hop.top/kit@latest`
- [ ] `cmd/usp/main.go` uses `cli.New(cli.Config{...})`
- [ ] All subcommand files use `root.Viper` for flags
- [ ] Replace ad-hoc `os.UserConfigDir()` with `xdg.*Dir("usp")`
- [ ] Replace `fmt.Printf` with `log.Info/Error` from `kit/log`
- [ ] Replace hand-rolled renderers with `output.Render(w, f, v)`
- [ ] Delete copy-pasted CLI contract code (fang handles it)
- [ ] Use `kit/sqlstore` for `projects.db` + `sessions.db`
- [ ] Use `kit/tui` for `install` wizard + session browsing
- [ ] Build custom picker from `tui.List` (no built-in Picker)
- [ ] Use `kit/upgrade` for `usp upgrade`
- [ ] Evaluate `kit/toolspec` for adapter tool discovery
- [ ] Register `--log-level` flag manually (not provided by kit)
- [ ] Handle `(string, error)` returns from all `xdg.*Dir` calls
- [ ] Use `config.Options` named paths (not `Layers` slice)
- [ ] Use `MigrateSQL` for session cache compound indexes
