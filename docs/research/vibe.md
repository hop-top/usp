# Mistral Vibe — Sessions Research Dossier

> Timestamped session directories under `~/.vibe/logs/session/`,
> each containing `meta.json` + `messages.jsonl`. No project
> grouping — cwd embedded in meta.json only (like Codex).

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Package:** `mistral-vibe` 2.7.3 (Homebrew)
- **Binary:** `/opt/homebrew/bin/vibe` → Python launcher
  (`Cellar/mistral-vibe/2.7.3/libexec/bin/python`)
- **Store root:** `~/.vibe/logs/session/`
- **Session dir pattern:** `session_<YYYYMMDD>_<HHMMSS>_<id8>/`
- **Files per session:** `meta.json` + `messages.jsonl`
- **Project key:** ❌ none — `cwd` lives inside `meta.json` only
- **Resume CLI:** `vibe --resume [SESSION_ID]` | `vibe -c` (continue)
- **Trusted folders:** `~/.vibe/trusted_folders.toml`
- **Global log:** `~/.vibe/logs/vibe.log` (CLI diagnostic, not
  transcript)
- **Env override:** `VIBE_HOME` (defaults to `~/.vibe`)
- **Platform:** macOS + Linux + Windows

## Sources

- Package source: <https://github.com/mistralai/mistral-vibe>
- Local install: `/opt/homebrew/Cellar/mistral-vibe/2.7.3/libexec/
  lib/python3.14/site-packages/vibe/`
- Verified source files:
  - `vibe/core/paths/_vibe_home.py` — path constants
  - `vibe/core/config/_settings.py` — `SessionLoggingConfig` defaults
  - `vibe/core/session/session_logger.py` — directory/filename logic
  - `vibe/core/session/session_loader.py` — filename constants +
    validation + latest-session algorithm
- CLI help: `vibe --help`

## Layout

```pseudocode
~/.vibe/
├── logs/
│   ├── vibe.log                                    # CLI diagnostic log
│   └── session/                                    # session root
│       ├── session_20260409_001823_a1b2c3d4/       # one session dir
│       │   ├── meta.json                           # metadata + cwd
│       │   ├── messages.jsonl                      # append JSONL
│       │   └── agents/                             # sub-agent sessions
│       │       └── session_<...>/                  # recursive structure
│       │           ├── meta.json
│       │           └── messages.jsonl
│       └── session_20260409_010442_5e6f7a8b/
│           └── ...
├── trusted_folders.toml                            # trust allowlist
├── skills/                                         # user skills
├── .env                                            # optional env file
└── vibehistory                                     # prompt history
```

- On this machine, `~/.vibe/logs/session/` is absent — vibe has
  been installed but never run with session logging effective;
  only `~/.vibe/logs/vibe.log` (0 bytes) exists
- Paths derived from source verification:
  - `SESSION_LOG_DIR = VIBE_HOME / "logs" / "session"` (in `_vibe_home.py`)
  - `session_prefix = "session"` (in `_settings.py`, default)
  - Folder template: `f"{session_prefix}_{utc_now_YmdHMS}_{session_id[:8]}"`
    (in `session_logger.save_folder`)
  - Filenames: `METADATA_FILENAME = "meta.json"`,
    `MESSAGES_FILENAME = "messages.jsonl"` (in `session_loader.py`)

## Session dir naming

- Pattern: `session_<YYYYMMDD>_<HHMMSS>_<session_id[:8]>`
- Timestamp is UTC at session start
- `session_id` is the first 8 chars of a longer UUID/ULID (full
  id only in `meta.json`)
- Sub-agents recurse: `session_<id>/agents/session_<sub-id>/...`
  (confirmed from `vibe/core/tools/builtins/task.py:127` using
  `save_dir=str(ctx.session_dir / "agents")`)

## Transcript schema

### meta.json

Contains a `SessionMetadata` dict. Known fields from
`SessionInfo` TypedDict in `session_loader.py`:

```pseudocode
{
  "session_id": "<full-id>",
  "cwd": "/Users/jadb/.w/ideacrafterslabs/uhp",
  "title": "<optional title>",
  "end_time": "2026-04-09T00:45:12Z" | null,
  ...  # likely agent profile, start_time, stats
}
```

- `cwd` is the only project-identifying field
- `title` and `end_time` may be null (long-running session)

### messages.jsonl

- Append-only JSONL
- One line per LLM message (`LLMMessage` type)
- Fields likely include `role`, `content`, `tool_calls`, `timestamp`
  (standard OpenAI-style turn envelope; verification requires
  live capture)

## Resume / continue semantics

From `vibe --help`:

- `-c` / `--continue` — continue the most recent session
- `--resume [SESSION_ID]` — resume specific session; without arg
  → interactive picker
- `-c` and `--resume` are mutually exclusive (argparse group)

### Latest-session algorithm (verified from `session_loader.py`)

```pseudocode
def latest_session(session_dirs):
  # Sort by messages.jsonl mtime (newest first)
  # For each, validate via _is_valid_session
  # Return first valid
```

Validation requires both `meta.json` and `messages.jsonl` to exist,
`meta.json` to parse as dict, and `messages.jsonl` to have at
least one parseable line.

- `-c` scope: **not cwd-scoped at filesystem level.** The latest
  session picker walks ALL of `~/.vibe/logs/session/` and returns
  the most recently updated valid one, regardless of current cwd
  (unless cwd filtering happens above this layer — not verified)

## Project grouping

- **None native.** Sessions are flat under `~/.vibe/logs/session/`
- `meta.json.cwd` is the only per-session project identifier
- Same problem as Codex — reconstruction requires reading every
  `meta.json` to extract `cwd`
- No `projects.json`, no cwd-keyed dirs, no central index

## Strengths

- Append-only JSONL for messages → rehydration-friendly, streaming
  tail is natural
- `meta.json` + `messages.jsonl` split keeps metadata mutations
  cheap (don't rewrite turn history on title/end_time updates)
- Timestamped dir names are human-sortable AND lexicographic
- Sub-agents use the same pattern recursively → uniform handling
  (unlike Claude's inline `isSidechain` flag)
- Atomic writes via `NamedTemporaryFile` + rename (seen in
  `session_logger.py`)
- Periodic `.json.tmp` cleanup prevents stale temp files
- `VIBE_HOME` env var enables sandboxing

## Known gotchas

- **No project grouping dir** — flat store, like Codex. USP must
  index `meta.json.cwd` externally
- **`session_logging.enabled`** can be toggled via config; when
  `false`, `save_dir=None` and nothing is written (detected via
  `if not self.enabled or not self.save_dir` guards in source)
- `session_id[:8]` in dirname = 2^32 bit space → collision
  possible at scale (hours ≤ unlikely; weeks ≤ possible);
  full ID only in meta.json
- Latest-session picker uses `messages.jsonl` mtime, not
  `meta.json` mtime — an empty meta update won't re-rank the
  session
- Title generation is async; `title` may be null for live sessions
  — don't filter on title presence
- Sub-agent nesting creates deep trees; naive glob may miss them
- `~/.vibe/vibe.log` is 0 bytes on this machine → diagnostic log
  was never written; not a session source
- `trusted_folders.toml` with `trusted = []` + `untrusted = []`
  (default) may cause first-use prompts

## Open questions

1. Is there a config option to shard sessions by cwd under
   `~/.vibe/logs/session/<key>/`? (Inspection of
   `SessionLoggingConfig` showed only `save_dir` + `session_prefix`
   + `enabled`, no key strategy)
2. Does `--continue` walk all sessions or cwd-filter somewhere
   above `session_loader.latest_session`?
3. What exact fields live in `meta.json` beyond the `SessionInfo`
   TypedDict? (Full `SessionMetadata` type in `vibe/core/types.py`)
4. Does sub-agent recursion have a depth limit?
5. Is `vibehistory` a prompt history (like readline) or a session
   history index?
6. What's the atomic write guarantee on `messages.jsonl` — per-line
   fsync, or batched?

## Integration notes for USP

Vibe is **similar to Codex** — flat store, cwd in metadata.

Strategy:

1. **Bootstrap:** probe for `~/.vibe/logs/session/`; create-on-demand
2. **Index builder:** walk `session_*/meta.json`, extract `cwd`,
   build cwd → session[] map; persist as sidecar JSON in USP state
3. **Watcher:** fswatch `~/.vibe/logs/session/` for new dirs; on
   new `meta.json`, add to index; on new `messages.jsonl` lines,
   tail and emit as turn events
4. **Sub-agent handling:** recurse into `agents/` subdirs; emit
   linked `session.spawned_by=<parent-id>` events
5. **USP envelope:** map `LLMMessage` lines to
   `user|assistant|tool_call|tool_result` events; preserve
   `meta.json` as session header
6. **cwd normalization:** validate `meta.json.cwd` is absolute;
   warn on relative paths (shouldn't happen but sanity check)
7. Respect `session_logging.enabled` — if a user disables it,
   detect zero new sessions and emit `adapter.recording_off`

Expected adapter size: ~400 LOC Go (very similar to Codex adapter
— same date-partition-walker shape but with meta.json reads
instead of first-line JSONL parse). Could share base with Codex
adapter via a "flat-store + cwd-in-meta" abstraction.
