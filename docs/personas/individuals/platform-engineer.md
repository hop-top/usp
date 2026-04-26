# Persona: Platform Engineer (usp Extension)

**Primary Role:** Owns the developer toolchain for a team or
org. Standardises how engineers use AI CLIs and provides
shared tooling for cross-CLI observability, audit, and
knowledge sharing.

---

## Base Goals

- Provide a uniform layer over heterogeneous developer tools
  so engineers don't reinvent integrations per project.
- Make AI-assisted work auditable and shareable across the
  team without leaking secrets.
- Reduce duplicated investigation across engineers by making
  past sessions discoverable.

## Base Pain Points

- Engineers use different AI CLIs by preference; transcripts
  scatter across `~/.claude/`, `~/.codex/`, copilot logs, etc.
- No structured way to audit which skills/agents an engineer
  invoked when reproducing a bug or onboarding a new hire.
- Cross-CLI lineage (claude resumed in codex, etc.) is
  invisible to current tooling, breaking incident timelines.

---

## usp Interaction Patterns

### Cross-CLI fleet search

```pseudocode
$ usp session search "incident-2026-04-15" --format json
// returns hits across claude, codex, copilot for any engineer
// running usp against shared session storage
```

### Lineage reconstruction for incident review

```pseudocode
$ usp session lineage <root-session-id> --format json
// orders predecessor → successor sessions across CLI boundaries
// → feeds into postmortem timeline
```

### Audit tool usage

```pseudocode
$ usp session show <id> --skills --format json
// lists skills invoked + queries; powers compliance / quality reviews
```

---

## User Stories

- As a platform engineer, I run a single search across every
  engineer's AI session transcripts so postmortems include
  the actual decision trail across CLIs.
- As a platform engineer, I reconstruct a session's lineage
  across CLI boundaries so incident timelines stay coherent
  when work hops tools mid-task.
- As a platform engineer, I audit which skills and agents a
  session invoked so I verify guardrails fired and identify
  gaps in our skill catalog.
- As a platform engineer, I review ctxt queries a session
  ran and whether they returned conclusive results so I tune
  knowledge-base coverage where it actually matters.

---

## Success Metrics

- Mean time to assemble cross-CLI incident timeline: < 5
  minutes from incident start to ordered transcript chain.
- Audit coverage: 100% of completed agent-driven tasks have
  retrievable skill/agent/ctxt usage data.
- Engineer self-serve rate: ≥ 90% of "how did this happen?"
  questions answered without escalating to platform team.
