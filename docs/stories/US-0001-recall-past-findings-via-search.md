# US-0001: Recall past-session findings via search

**Persona:** Solo Developer, AI Agent
**Priority:** P0

## Story

As a solo developer (or AI agent recovering context), I run
`usp session search "<topic>"` so I find every relevant past
session across all my AI CLIs in one command, instead of
grepping multiple JSONL trees with per-tool jq.

## Acceptance Criteria

- [ ] Given session files exist for ≥ 2 supported CLIs,
      when I run `usp session search "<query>"`, then
      results from all CLIs appear in a single ranked list
- [ ] Given results exist, when I add `--format json`, then
      each hit includes `session_id`, `cli`, `project`,
      `turn_id`, `snippet`, and `ts` fields
- [ ] Given no matching sessions, when the query returns
      zero hits, then the command exits 0 with an empty
      result set (table: header only; json: `[]`)
- [ ] Given a query string with shell-special chars, when
      I quote it correctly, then the search runs without
      shell-injection failures

## Notes

- Backed by per-CLI adapters that normalise transcripts into
  a shared schema before searching.
- Search scope: full content across system / user / assistant
  / tool-call turns.
- Existing command — story documents the typical-use contract.
- Post adapter-signal-extraction (T-0078..T-0083): once
  sessions are projected into ctxt, equivalent searches
  surface as `ctxt find "@file.<slug>"`, `ctxt find
  "@model.<slug>"`, `ctxt find "#cost:high"`, broadening the
  recall surface beyond raw text. See README "Signals".
