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
usp session search auth  # find sessions mentioning "auth"
```

### Flags

```sh
--tool claude            # narrow to one assistant
--project /path/to/x     # explicit working directory
--since 7d               # sessions from the last 7 days
--limit 10               # cap results
--format json            # json | yaml | table
```

## Sample output

### `usp session list --since 2d`

```
ID             CLI     PROJECT                    STARTED  TURNS
8c470754-d69…  claude  ~/projects/tlc             1h ago   32
34534e3e-210…  claude  ~/projects/uhp             1h ago   499
827d6683-2f5…  claude  ~/projects/kit             1h ago   292
33ba162b-1e9…  claude  ~/projects/usp             2h ago   649
41c697cf-1f4…  claude  ~/projects/tep             4h ago   95
3f937c7d-7db…  claude  ~/projects/kit             5h ago   374
d6607e6d-d91…  claude  ~/projects/tip             10h ago  1919
cfe5b6d0-9a2…  claude  ~/projects/xray            21h ago  13
af1242d5-1c1…  claude  ~/projects/xray            21h ago  458
5f11d831-882…  claude  ~/skills                   22h ago  801
db43cfac-543…  claude  ~/projects/xray            1d ago   93
fe2eb947-eca…  claude  ~/projects/tlc             1d ago   142
```

### `usp session show fe2eb947-eca…`

```
Session: fe2eb947-ecab-4293-a26c-3485062e8e6a
CLI:     claude
Project: ~/projects/tlc
Started: 2026-04-10 04:40:25
Ended:   2026-04-10 05:08:20
Turns:   142

Turn 1 [system] 2026-04-10 04:40:25

Turn 2 [user] 2026-04-10 04:40:26
  ❯ tlc init libc_darwin.go:224: ...

Turn 3 [assistant] 2026-04-10 04:40:29
  That looks like a panic or crash in `tlc init`. Let me...
  Tool calls: Bash

Turn 4 [assistant] 2026-04-10 04:40:30
  Tool calls: Bash
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
