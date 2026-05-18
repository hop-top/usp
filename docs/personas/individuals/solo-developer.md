# Persona: Solo Developer (usp Extension)

**Primary Role:** Independent engineer using multiple AI CLIs
(claude, codex, copilot, gemini, opencode) across projects.
Wants a single tool to recall, audit, and archive their AI-
assisted work without per-CLI plumbing.

---

## Base Goals

- Ship features fast across many concurrent projects without
  per-tool friction.
- Recover prior context (decisions, dead-ends, fixes) without
  re-doing investigation.
- Keep a durable record of AI-assisted work for audit, blog
  posts, and onboarding future contributors.

## Base Pain Points

- Each CLI hides transcripts in a different path/format; no
  single command surfaces everything.
- "What did I figure out last week about X?" requires grepping
  multiple JSONL trees with bespoke jq.
- Track/PR write-ups need session excerpts; copy-pasting from
  raw files is tedious and lossy.

---

## usp Interaction Patterns

### Recall by topic

```pseudocode
$ usp session search "elasticsearch migration"
// table: session_id, cli, project, ts, snippet
```

### Archive a session for a track

```pseudocode
$ usp session show <session-id> > tracks/<track-id>/session.md
// human-readable transcript, committed alongside track docs
```

### List recent sessions across all CLIs

```pseudocode
$ usp session list --format json | jq '.[] | select(.project == "myrepo")'
```

---

## User Stories

- As a solo dev, I search across all my CLI sessions for a
  topic so I recover prior decisions in one command instead
  of grepping JSONL trees per tool.
- As a solo dev, I export a single session transcript to my
  track folder so I have a durable, version-controlled record
  of how the work happened.
- As a solo dev, I list all my recent sessions filtered by
  project so I quickly find the conversation that produced
  a specific change.
- As a solo dev, I inspect which skills, agents, and ctxt
  queries a past session used so I diagnose why an outcome
  was good or bad and avoid repeating mistakes.

---

## Success Metrics

- Time to recall a prior decision: < 30 seconds from question
  to relevant snippet.
- Track folders include session archives for ≥ 80% of
  completed tracks within 30 days.
- Zero per-CLI shell aliases or jq scripts needed for routine
  recall/audit workflows.
