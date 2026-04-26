# US-0004: Inspect a single session's transcript

**Persona:** Solo Developer, AI Agent
**Priority:** P0

## Story

As a solo developer (or AI agent loading prior context), I
run `usp session show <id>` so I read a turn-by-turn summary
of one session — system, user, assistant, and tool calls —
without parsing the CLI's native transcript format.

## Acceptance Criteria

- [ ] Given a valid session id, when I run `show <id>`, then
      output displays metadata header (cli, project, started,
      ended, turn count) followed by ordered turns
- [ ] Given turns include tool calls, when output renders,
      then each tool call shows tool name, inputs, and outputs
      distinct from regular assistant text
- [ ] Given `--format json`, when I parse the output, then
      every turn has `role`, `content`, optional `tool_calls`,
      and `ts` fields per the documented schema
- [ ] Given an id that exists in multiple CLIs (collision),
      when `show` runs, then the CLI source is disambiguated
      via prefix or `--cli` flag

## Notes

- Foundation for US-0001 (search → show) and US-0002 (archive).
- Existing command — story documents the typical-use contract.
