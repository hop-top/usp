# Review: session-introspection track + supporting docs

**Date:** 2026-04-26
**Reviewer:** feature-dev:code-reviewer (Agent)
**Track:** session-introspection (T-0070, T-0071, T-0072)
**Stories:** US-0001..US-0004
**Personas:** ai-agent, solo-developer, platform-engineer (usp extensions)

## Critical

### 1. Task descriptions hedge the subcommand-vs-flag decision (confidence: 88)

`plan.md` `## Open questions` resolves: "standalone subcommands
for filter-heavy queries; flags on `show` for single-session
detail — implement both." But task descriptions still read:

> `usp session skills <filter>` (or equivalent flag on
> `session show` / `session list`)

Per `~/.ops/CLAUDE.md`, every task description MUST end with a
single concrete action — no "either/or" for the implementer.

**Fix:** Remove "or equivalent flag on …" hedging from T-0070,
T-0071, T-0072 descriptions. Add a closing line to each:
"Implement as standalone `usp session <view>` subcommand; also
expose `--<view>` flag on `usp session show` for single-session
detail."

## Important

### 2. Solo Dev persona story 4 has no US-NNNN coverage (confidence: 85)

`solo-developer.md` last user story:

> As a solo dev, I inspect which skills, agents, and ctxt
> queries a past session used so I diagnose why an outcome
> was good or bad…

US-0001..US-0004 cover existing commands only. The new
introspection surface (this track) lacks a story file. Track
acceptance gate references US-0001 and US-0003 — neither covers
the new commands.

**Fix:** Create `US-0005-inspect-session-skill-agent-ctxt-usage.md`
covering the 3 new introspection commands. Link from track plan
acceptance gate.

### 3. T-0072 (ctxt) should be blocked-by both T-0070 and T-0071 (confidence: 80)

Plan declares `blocked-by: [0]` for ctxt task. Build sequence
states ctxt reuses both adapter scaffolding (task 0) AND
turn-correlation model introduced in agents view (task 1).

**Fix:** Change T-0072 `blocked-by: [0]` to `blocked-by: [0, 1]`.

## Notes

- Platform Engineer persona's `show --skills` flag pattern is
  consistent with the plan resolution — no action needed once
  task descriptions are fixed (issue 1).
- Stories US-0001..US-0004 ACs match observable behavior of
  existing `usp session list/show/search/lineage` commands per
  `--help` output. Good.
- Persona project-extension structure follows the c12n pattern
  correctly. Extended-By links updated in base personas.

## Outcome

3 fixes required before track is ready for execution. None block
the docs commit (personas + stories are independently valid);
plan changes can land in same commit or follow-up.

## Post-review descope (2026-04-26)

After review, the ctxts task was **removed from the track** and
converted to a brainstorm task (T-0072, unlinked). Rationale:
ctxt is one of many tools invoked in a session (Bash subcommands,
MCP calls, gh, tlc, …); a unified "tools usage" view should
subsume the ctxt-only case rather than being shipped one-off.

Knock-on changes applied:

- Track renamed: "skills/agents/ctxt usage" → "skills + agents
  usage".
- T-0072 retagged `type:brainstorm`, effort `S`, track unlinked,
  blockers cleared.
- Plan tasks frontmatter shrunk to 2 entries; ctxts content
  moved to `## Future / Deferred`.
- US-0005 file renamed to `US-0005-inspect-session-skills-and-
  agents.md`; ACs trimmed; "Deferred" section added.
- Filter renames adopted across remaining tasks: `--skill` →
  `--name` (skills); `--agent-type` → `--type` (agents); added
  `--name` to agents.
- `--summary` flag replaced with kit-level `--exec` executor-
  pipeline syntax (repeatable, AND-intersect with top-level
  filters). Kit work item not yet filed.

Issue-1 fix (subcommand-vs-flag hedging) and Issue-3 fix
(blocked-by) above remain valid and were applied. Issue-2 fix
(US-0005) was applied and then refined in the descope.
