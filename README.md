# USP — Universal Sessions Protocol

Find any coding session across any AI assistant. One command.

## Jobs this solves

**"Which tool did I use for that auth fix last Tuesday?"**
You switch between Claude, Gemini, Codex, OpenCode. Each
buries sessions in different formats, different paths. USP
reads all of them, gives you one sorted list — and can
resume where you left off in any other tool.

**"I need to replay what happened in that failed refactor."**
`usp session show <id>` — metadata, turns, tool calls.
Works regardless of which CLI created the session.

**"I'm building a devtool that needs session data."**
USP normalizes every CLI's native format into one envelope.
Implement one interface, get detection + diagnostics free.

**"New dev joining — which sessions touched this project?"**
`usp session list --project .` — every assistant's sessions
for this directory, newest first.

**"I want my session history queryable as part of my knowledge
graph."**
USP ships a companion bridge, `usp-ctxt`, that walks every
detected CLI's session list and projects each session into a
[ctxt](https://github.com/jadb/ContextHelp) knowledge object.
Once bridged, your sessions are searchable next to your notes,
documents, and other captured context — and they participate in
ctxt's entity graph via stable mentions like
`@usp.session.<uuid>`, `@cli.<name>`, and `@agent.<id>`.

## The ctxt bridge

`usp-ctxt sync` is a separate binary in this repo
([cmd/usp-ctxt/](cmd/usp-ctxt/)). It's idempotent: re-running
over the same session updates the matching ctxt entity rather
than creating duplicates, and a high-water-mark file at
`~/.local/share/usp-ctxt/last_run.json` tracks per-CLI
progress so subsequent runs only ingest new sessions.

```sh
go install hop.top/usp/cmd/usp-ctxt@latest

usp-ctxt sync                        # walk all CLIs since last run
usp-ctxt sync --tool claude          # only Claude Code sessions
usp-ctxt sync --project ~/code/myapp # only sessions in this project
usp-ctxt sync --dry-run              # project + log; do not write
```

Run it on cron, after each work block, or whenever you want
your session history reflected in ctxt. The full ingestion +
retrieval design — including the live-capture path that runs
on top of UHP, and the three retrieval shapes built on the
combined data — lives in §4 of the spec at
<https://github.com/hop-top/hop/blob/main/docs/ingestion-retrieval/spec.md>.

## The killer feature: cross-CLI resume

Start in one assistant, continue in another. Full context
carries over — no copy-paste, no re-explaining.

```sh
# Start a session in Claude
claude
# ... work on auth feature, exit

# Resume the same conversation in Codex
usp resume --tool codex
# Codex picks up with full conversation context

# Resume again in Gemini
usp resume --tool gemini
# Gemini continues where Codex left off

# See the full lineage
usp session lineage <id>
```

Sample lineage output:

```
Session: a1b2c3d4-e5f6-7890-abcd-ef1234567890
Project: ~/projects/myapp

  #  CLI       Native ID     Started              Turns
  1  claude    fe2eb947-ec…  2026-04-10 04:40:25  142
  2  codex     019d70b5-83…  2026-04-10 05:10:00   38
  3  gemini    usp-resume-…  2026-04-10 05:30:00   12

Total: 192 turns across 3 CLIs
```

## Quickstart

```sh
go install hop.top/usp/cmd/usp@latest

usp doctor               # what's installed, what's readable
usp install              # index detected CLIs
usp session list         # all sessions, all CLIs, sorted
usp session show <id>    # full session detail
usp session show <id> --skills  # detail + skill invocations
usp session search auth  # find sessions mentioning "auth"
usp session skills --name review  # every session that fired the review skill

usp resume --tool codex        # resume last session in Codex
usp session lineage <id>      # cross-CLI conversation history
```

See [docs/architecture/session-introspection.md](docs/architecture/session-introspection.md)
for the `session skills` data model + adapter contract.

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
