# Amp — Sessions Research Dossier

> Server-side thread store. Local `~/.amp/` holds only config, binary,
> and empty thread-id marker dirs under `file-changes/`. No local
> transcripts. Project grouping nonexistent locally.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Store root:** server-side (ampcode.com); no local transcripts
- **Local markers:** `~/.amp/file-changes/T-<uuid>/` (empty dirs)
- **Thread ID format:** `T-<uuid-v7>` (e.g. `T-019cfc47-32f4-72ae-...`)
- **Project key:** ❌ none (server tracks it via workspace/visibility)
- **Transcript format:** opaque (server-only); `amp threads markdown`
  renders on demand from API
- **Resume CLI:** `amp threads continue <id>` / `amp threads c`
- **List CLI:** `amp threads list` (hits server)
- **Config:** `~/.config/amp/settings.json` (may not exist by default)
- **Logs:** `~/.cache/amp/logs/cli.log` (not a transcript; CLI telemetry)
- **Platform:** macOS + Linux + Windows

## Sources

- Product: <https://ampcode.com/>
- Binary wrapper: `/Users/jadb/.local/bin/amp` → `~/.amp/bin/amp` (bash)
- CLI help: `amp threads --help`, `amp threads list`
- Verified filesystem: `~/.amp/`, `~/.config/amp/`, `~/.cache/amp/`

## Layout

```pseudocode
~/.amp/
├── bin/
│   ├── amp                                # bash launcher
│   ├── bun                                # bundled runtime
│   └── rg                                 # bundled ripgrep
├── file-changes/
│   ├── T-019cfc47-32f4-72ae-8596-b991b309118f/   # empty marker dir
│   ├── T-68124e4a-e03a-4baa-9f19-ac93d13e7f57/   # per-thread
│   └── T-e4d1c1fc-1998-4907-b28e-4aaff9cf62a3/
├── package/                               # node package metadata
└── .package.backup/

~/.config/amp/
└── settings.json                          # optional; may not exist

~/.cache/amp/
└── logs/
    └── cli.log                            # telemetry; not transcripts
```

- `file-changes/T-<uuid>/` dirs are always present but empty on this
  machine — likely cache scratch for per-thread file-change tracking,
  not conversation storage
- Four thread-id dirs exist locally but `amp threads list` reports
  only one active thread — stale dirs never cleaned

## Thread ID format

- Pattern: `T-<uuid-v7>`
- UUIDv7 embeds creation timestamp; lexicographically time-ordered
- Minted server-side; CLI receives ID on `amp threads new`

## Transcript schema

- **Not stored locally.** Server-side only.
- `amp threads markdown <id>` fetches + renders to stdout; no cache
- Schema is API-opaque; USP cannot parse without hitting API

## Resume / continue semantics

- `amp threads continue <id>` / alias `amp threads c`
- `amp threads new` — mint new thread, print ID
- `amp threads handoff` — create handoff thread from existing
- **All operations hit the server.** Offline resume impossible.

## Project grouping

- **None locally.** Server may track `workspace`/`group`/`visibility`
  as a bucket taxonomy, but it's not exposed via filesystem
- `amp threads visibility` sub-command sets per-repository default
  visibility (private|public|workspace|group)
- No `~/.amp/projects/` or per-cwd dir

## Strengths

- Zero local state drift — server is source of truth
- Thread IDs are globally unique and time-ordered (UUIDv7)
- Cross-machine continuity natural (log in, resume anywhere)
- No filesystem scan needed for discovery — just hit API

## Known gotchas

- **No offline access.** Losing connectivity = losing sessions.
- **No filesystem transcript.** USP cannot mirror without API client.
- `file-changes/T-*/` dirs are empty and stale; don't treat as session
  markers — they're not authoritative
- `~/.cache/amp/logs/cli.log` is CLI noise (skill loading, etc.),
  not conversation content
- `amp threads list` output is ephemeral; no local cache to reread
- Thread visibility is a server concept, not a local one —
  per-project filtering requires API round-trip

## Open questions

1. Does `amp threads markdown <id>` have a local cache, or always
   round-trip to server?
2. What populates `file-changes/T-*/` dirs — uncommitted edit
   provenance? Are contents written during a live session and
   cleaned on exit?
3. Is there a public API endpoint for thread listing USP could poll,
   or is authentication tied to the binary's OAuth state?
4. Does Amp expose a webhook / event stream for new threads, or is
   polling the only option?
5. Is `workspace` a first-class project grouping in the API, and
   how is the workspace id derived — from git remote, cwd, or manual?

## Integration notes for USP

Amp is **the hardest adapter** because the store is remote. Options:

1. **API adapter:** USP reuses amp's OAuth token (from `~/.amp/` or
   settings) to hit the thread list endpoint; polls on interval
2. **CLI wrapper:** USP shells out to `amp threads list --include-archived`
   + `amp threads markdown <id>` per thread; slow but no API surface
3. **Skip Amp:** document as "cloud-only, unsupported for local mirror"

Recommendation: Option 2 for MVP (CLI-wrapped, rate-limited), then
Option 1 once an API spec is documented. Project grouping must be
synthesized from thread metadata or abandoned — no cwd-based
reconstruction possible.

Expected adapter size: ~200 LOC Go (CLI shell-out + JSON parse) or
~500 LOC (direct API + auth).
