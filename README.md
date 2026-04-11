# USP — Universal Sessions Protocol

Find any coding session across any AI assistant. One command.

## Jobs this solves

**"Which tool did I use for that auth fix last Tuesday?"**
You switch between Claude, Gemini, Codex, OpenCode. Each
buries sessions in different formats, different paths. USP
reads all of them and gives you one sorted list.

**"I need to replay what happened in that failed refactor."**
`usp session show <id>` — metadata, turns, tool calls.
Works regardless of which CLI created the session.

**"I'm building a devtool that needs session data."**
USP normalizes every CLI's native format into one envelope.
Implement one interface, get detection + diagnostics free.

**"New dev joining — which sessions touched this project?"**
`usp session list --project .` — every assistant's sessions
for this directory, newest first.

## Quickstart

```sh
go install hop.top/usp/cmd/usp@latest

usp doctor               # what's installed, what's readable
usp install              # index detected CLIs
usp session list         # all sessions, all CLIs, sorted
usp session show <id>    # full session detail
```

### Flags

```sh
--cli claude             # narrow to one assistant
--project /path/to/x     # explicit working directory
--limit 10               # cap results
--format json            # json | yaml | table
```

## Supported CLIs

| CLI | Store format | Status |
|-----|-------------|--------|
| Claude Code | JSONL per-session | v1 |
| Codex CLI | date-partitioned JSONL | v1 |
| Gemini CLI | JSON snapshots | v1 |
| OpenCode | 12-table SQLite | v1 |
| Copilot CLI | normalized SQLite | planned |
| Kimi Code | dual JSONL + metadata index | planned |
| Amp | server-side API | planned |
| Cursor Agent | SQLite CAS (metadata-only) | planned |
| Qwen, Vibe, Antigravity, Windsurf, Tabnine | various | planned |

## For devtool authors

Every session becomes:

```go
type Session struct {
    ID         string
    CLI        string        // "claude", "codex", ...
    ProjectCwd string
    StartedAt  time.Time
    EndedAt    *time.Time
    TurnCount  int
    Metadata   map[string]any
}
```

Add your CLI: implement `SessionAdapter` (detection, list,
get, stream turns). Shared infra handles the rest — registry,
project key derivation, doctor checks.

Adapters are thin: 250-600 LOC each. See
[docs/architecture.md](docs/architecture.md) for the contract
and data flow.

## Status

Alpha (`v0.0.1-alpha.1`). Envelope schema may change.
