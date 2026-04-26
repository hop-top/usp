# US-0005: Inspect skills and agents used in a session

**Persona:** Solo Developer, AI Agent, Platform Engineer
**Priority:** P1

## Story

As a solo developer (or platform engineer reviewing an
incident, or AI agent auditing its own tool usage), I run
`usp session skills` and `usp session agents` so I see what
skills Claude invoked and which agents it dispatched inside
matching sessions — so I diagnose why an outcome was good or
bad and avoid repeating mistakes.

## Acceptance Criteria

- [ ] Given a session that invoked ≥ 1 skill, when I run
      `usp session skills --session <id>`, then results
      include `skill_name`, `trigger_turn_id`,
      `trigger_query` (truncated), and `outcome` (invoked /
      declined / errored)
- [ ] Given the same session, when I run
      `usp session show <id> --skills`, then the
      single-session detail view embeds the same data inline
      with the rest of the transcript
- [ ] Given filters `--cli`, `--project`, `--since`,
      `--until`, when I combine them, then results narrow
      additively (AND semantics) to the intersection
- [ ] Given `--name <skill-name>` on `session skills`, when
      results return, then only matching skills appear
- [ ] Given a session that dispatched subagents via the Task
      tool, when I run `usp session agents --session <id>`,
      then results include `agent_type`, `agent_name`,
      `agent_description`, `prompt` (truncated),
      `exit_status`, and `duration_ms`
- [ ] Given `--type <agent-type>` and/or `--name <agent-name>`
      on `session agents`, when results return, then both
      filters AND-combine and only matching rows appear
- [ ] Given `--exec 'summarize:tokens(N)'` (or
      `--exec 'count'` or `--exec 'group_by:<col>'`) on either
      view, when output renders, then aggregation replaces the
      raw row list per the chosen executor — see notes for
      kit-level dependency
- [ ] Given top-level `--since` and an embedded
      `filter(since[...])` inside `--exec`, when both are set,
      then the effective window is the intersection (AND), not
      one overriding the other
- [ ] Given `--format json` on either view, when I parse the
      output, then every row conforms to the documented schema
      in the track's `architecture/session-introspection.md`
- [ ] Given a CLI whose adapter does not yet support skills /
      agents primitives (e.g. opencode), when I run a view
      against its sessions, then results include
      `unsupported: true` per row rather than a hard error

## Notes

- Implemented under track `session-introspection`
  (T-0070 skills, T-0071 agents).
- Each view is a standalone subcommand for N→view queries
  AND a flag (`--skills`, `--agents`) on `session show` for
  single-session detail.
- `--exec` is a kit-level flag (executor pipeline syntax)
  that usp inherits; it requires the upstream kit/cli flag
  to ship first. Aggregation ACs may be deferred to a
  follow-up release if kit work isn't ready when
  T-0070/T-0071 land.

## Deferred

- **Tools-usage view (incl. ctxt queries + classification)** —
  originally part of this story. ctxt is one of many tools
  invoked in a session (Bash subcommands, MCP calls, gh, tlc,
  …); a unified "tools" view will subsume the ctxt-only case.
  Tracked under brainstorm task **T-0072** (unlinked from
  this track). A successor story will land alongside the
  feature task(s) that come out of the brainstorm.
