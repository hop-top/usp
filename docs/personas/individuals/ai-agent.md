# Persona: AI Agent (usp Extension)

**Primary Role:** Non-human autonomous agent that recovers
prior-session context, traces cross-CLI continuations, and
audits its own tool usage to inform current-task decisions.

---

## Base Goals

- Discover available capabilities at runtime via structured
  metadata (schemas, manifests, registries).
- Execute multi-step workflows with predictable,
  machine-readable responses at every boundary.
- Maintain session continuity across interruptions without
  losing accumulated context or partial results.
- Minimise token/context-window spend per task while preserving
  decision-relevant information.

## Base Pain Points

- Unstructured or inconsistent output forces brittle parsing
  and increases failure rates.
- Each CLI stores transcripts in its own format/location;
  cross-tool work is invisible without a unified view.
- Re-deriving past decisions by re-reading raw JSONL files
  burns context and is non-deterministic.

---

## usp Interaction Patterns

### Recall prior decisions

```pseudocode
results = usp.session.search(query: "auth migration", format: "json")
// returns list of {session_id, cli, turn_id, snippet, ts}
for r in results:
    if relevant(r.snippet):
        ctx.append(r)
```

### Trace cross-CLI lineage

```pseudocode
chain = usp.session.lineage(id: current_session_id, format: "json")
// returns ordered list of predecessor sessions across CLIs
context = merge(chain)  // earliest first; reconstruct task arc
```

### Inspect a single transcript

```pseudocode
session = usp.session.show(id: uuid, format: "json")
// session.turns: [{role, content, tool_calls, ts}]
```

---

## User Stories

- As an AI agent, I search past sessions across all CLIs so I
  can recover decisions without re-reading raw transcripts.
- As an AI agent, I follow a session lineage chain so I
  reconstruct task continuity that spans claude → codex → copilot.
- As an AI agent, I read structured JSON output so parsing
  never fails mid-workflow.
- As an AI agent, I list which skills/agents/ctxts a session
  invoked so I audit my own tool usage and avoid repeating
  known-bad patterns.

---

## Success Metrics

- 100% of `--format json` responses conform to the documented
  schema.
- Cross-CLI lineage reconstruction completes < 500ms for chains
  of ≤ 10 sessions.
- Recall queries return at least one relevant result for ≥ 80%
  of follow-up tasks within the same project scope.
