# US-0002: Archive a session transcript to a track folder

**Persona:** Solo Developer
**Priority:** P1

## Story

As a solo developer, I export a single session's transcript
to my track folder via `usp session show <id> >
tracks/<track-id>/session.md` so I have a durable, version-
controlled record of how the work happened.

## Acceptance Criteria

- [ ] Given a valid session id, when I run
      `usp session show <id>`, then output is human-readable
      markdown suitable for committing alongside track docs
- [ ] Given the same id with `--format json`, when I redirect
      to a file, then the JSON is well-formed and round-trips
      through `jq .` without error
- [ ] Given a non-existent id, when I run `show`, then exit
      code is non-zero and stderr explains the lookup failed
- [ ] Given a long session, when I export it, then the output
      preserves turn ordering and tool-call boundaries

## Notes

- Pairs with track convention in `~/.ops/CLAUDE.md`: track
  folders MAY include `session.md` archives for completed
  tracks.
- Existing command — story documents the typical-use contract.

Covered by: `e2e/session_show_test.go`
