# US-0003: Audit cross-CLI work via session lineage

**Persona:** AI Agent, Platform Engineer
**Priority:** P1

## Story

As an AI agent (or platform engineer reconstructing an
incident), I run `usp session lineage <id>` so I follow a
session's predecessor/successor chain across CLI boundaries
(claude → codex → copilot) and see the full task arc in one
ordered list.

## Acceptance Criteria

- [ ] Given a session that continued from a prior session in
      a different CLI, when I run `lineage <id>`, then the
      chain returns sessions in chronological order with the
      `cli` field populated for each entry
- [ ] Given a standalone session (no predecessors), when I
      run `lineage <id>`, then a single-entry chain returns
      with exit code 0
- [ ] Given `--format json`, when results return, then each
      entry includes `session_id`, `cli`, `parent_id`,
      `child_ids`, `started_at`, and `ended_at`
- [ ] Given a chain spanning ≥ 2 CLIs, when I render output,
      then transitions between CLIs are visually marked
      (table) or structurally clear (json)

## Notes

- Lineage is reconstructed via parent_id pointers written by
  CLIs that support session continuation, plus heuristic
  matching (resume timestamp + project path) for CLIs that
  don't.
- Existing command — story documents the typical-use contract.
