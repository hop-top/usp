# Session Introspection — Skills & Agents

**Track:** session-introspection
**Tasks:** T-0070 (skills), T-0071 (agents)
**Stories:** US-0001, US-0005
**Author:** $USER

Cross-CLI surface for "what skills did Claude invoke?" and "what
agents did it dispatch?" inside one or many sessions. Lands as a
new `usp session skills` (and follow-up `usp session agents`)
subcommand plus `--skills` / `--agents` flags on `usp session
show`.

This doc captures the design + schema agreed at planning time so
future contributors do not have to re-read source to understand
the contract.

---

## Goals

- One-shot answer to "did Claude invoke <skill> in any session?"
- Aggregate skill usage across sessions for audit / postmortem.
- Embed inline detail under `session show` for single-session
  diagnosis.
- Adapter-agnostic: every CLI that records skill invocations can
  surface them; CLIs that lack the primitive emit `unsupported:
  true` rows rather than a hard error.

## Non-Goals

- `--exec` aggregation (`summarize:tokens`, `count`,
  `group_by:<col>`) is a kit-level pipeline. T-0070 ships without
  it; the upstream `kit/cli` flag is a precondition.
- Tools usage (Bash, MCP, ctxt queries) — deferred to T-0072.
- Mutation: this surface is read-only.

---

## Surface

### Subcommand — N → view

```
usp session skills [flags]
```

Flags:

| Flag        | Purpose                                                     |
|-------------|-------------------------------------------------------------|
| `--session` | Restrict to a single session (full or prefix ID)            |
| `--cli`     | Restrict to claude / codex / gemini / opencode              |
| `--project` | Restrict to a project cwd                                   |
| `--name`    | Substring match on skill name                               |
| `--since`   | Lower time bound; accepts `7d`, `24h`, `2026-04-01`         |
| `--until`   | Upper time bound; same vocabulary as `--since`              |
| `--format`  | `table` (default) / `json` / `yaml`                         |

All filters AND-combine. Empty result on table mode prints "No
skill invocations found." to stderr; JSON/YAML emit `[]`.

### Flag — 1 → detail

```
usp session show <id> --skills [--format json|yaml]
```

Embeds the same skill rows under `Skills:` (table) or as
`skills` field on the JSON/YAML payload. Single-session views
inherit show's existing prefix-resolution / project / since flags.

---

## Data Model

```pseudocode
type SkillEvent {
    session_id        string       // CLI-native session ID
    cli               string       // claude / codex / ...
    ts                time         // RFC3339
    skill_name        string       // skill identifier (no leading slash)
    trigger_turn_id   string       // user turn UUID that triggered it
    trigger_query     string       // truncated trigger text (~240 chars)
    outcome           enum         // invoked | declined | errored
    unsupported       bool         // adapter cannot enumerate skills
}
```

`unsupported=true` rows omit name / turn / query and exist purely
so `--format json` consumers can distinguish "session has no
skills" from "this CLI's adapter has no skill primitives yet".

### Outcome semantics

| Outcome    | When                                                       |
|------------|------------------------------------------------------------|
| `invoked`  | Skill ran (slash command parsed, Skill tool returned ok)   |
| `declined` | Skill candidate surfaced but no tool call followed         |
| `errored`  | Skill tool returned `is_error=true` or transport failure   |

`declined` is a forward-looking state; T-0070 only emits
`invoked` and `errored` from Claude transcripts. Future work can
map model-only mentions (no tool_use) to `declined` once a
heuristic is agreed.

---

## Adapter Contract

```pseudocode
interface SkillExtractor extends SessionAdapter {
    ExtractSkills(sessionID) -> ([]SkillEvent, error)
}
```

Adapters implement this OPT-IN. The CLI layer detects support via
type assertion. Adapters that do not implement it cause the CLI
layer to emit one `unsupported: true` row per matched session.

### Claude (T-0070, shipped)

Two patterns surface skill invocations in `~/.claude/projects/<key>/<uuid>.jsonl`:

1. **Slash commands.** A user turn body contains
   `<command-name>/foo</command-name>` plus an optional
   `<command-args>...</command-args>` payload. The user turn UUID
   is the trigger; outcome is always `invoked`. Trigger query =
   the command-args contents (truncated).

2. **Skill tool calls.** An assistant turn includes a
   `tool_use` block with `name="Skill"` and
   `input.skill="<name>"`. The user turn IMMEDIATELY preceding
   the assistant turn is the trigger. Outcome resolves to
   `errored` if a downstream `tool_result` block (carried in a
   later user turn) reports `is_error=true`, otherwise `invoked`.

Trigger queries are truncated at 240 chars with a trailing
ellipsis to keep table rows usable.

### Codex / Gemini / OpenCode (T-0070)

Adapters do not implement `SkillExtractor`. CLI layer emits
`unsupported: true`. Future work: a Codex equivalent likely lives
in tool-call records; Gemini exposes some via tool annotations;
OpenCode currently has no skill primitive in its schema.

---

## Filtering Order

```pseudocode
sessions = if session != "":
              [resolve(session, project, since)]
           else:
              for adapter in adapters:
                  for s in adapter.ListSessions(project):
                      yield (s, adapter)

events = []
for (s, adapter) in sessions:
    if !since.zero && s.StartedAt.Before(since): continue
    if !until.zero && s.StartedAt.After(until):  continue
    if adapter not implements SkillExtractor:
        events.append(unsupportedRow(s))
        continue
    for e in adapter.ExtractSkills(s.id):
        if !since.zero && e.ts.Before(since): continue
        if !until.zero && e.ts.After(until):  continue
        events.append(e)
events = filter(events, name)   // substring match
events.sort(by ts ascending)
render(format, events)
```

Top-level `--since` / `--until` apply twice on purpose: once to
the session start (cheap session-level pruning), once to each
event timestamp (precise event-level filter so a long session
doesn't drag pre-window events into a narrow query).

---

## Diagrams

- ![Data flow v1](session-introspection/data-flow-v1.mmd)
- ![Adapter contract v1](session-introspection/adapter-contract-v1.mmd)

---

## Open Questions

- `declined` outcome heuristic for slash-command-style mentions
  in assistant text without a corresponding tool call.
- Cross-segment session: if `session.Segments` lists multiple
  CLIs, should `ExtractSkills` walk every segment? Current cut:
  no — caller passes the native session ID and only that segment
  is scanned. T-0072 may revisit.

---

## Related

- T-0071: agents view (mirror surface, dispatches via Task tool)
- T-0072: tools view (Bash, MCP, ctxt) — supersedes earlier
  ctxt-only proposal
- US-0001: recall past findings (search foundation)
- US-0005: this story
