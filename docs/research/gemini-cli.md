# Gemini CLI — Sessions Research Dossier

> Per-cwd history dirs with short auto-generated aliases mapped via
> central `projects.json`. Aliases collide across unrelated cwds,
> making backward lookup (alias → path) unreliable without the map.

Last verified: 2026-04-08.

## Summary

- **Store root:** `~/.gemini/history/<alias>/`
- **Project map:** `~/.gemini/projects.json` (cwd → alias)
- **Alias scheme:** basename of cwd, with numeric suffixes on collision
  (`main`, `main-1`, `main-2`, ..., `main-15`)
- **Transcript format:** per-chat JSON files + `.project_root` marker
- **Resume CLI:** `gemini chat resume <tag>` / `/chat resume <tag>`
- **Scope:** per-cwd; alias resolution via central map
- **Platform:** macOS + Linux + Windows

## Sources

- Docs index: <https://geminicli.com/docs/sessions/>
- Chat/history: <https://geminicli.com/docs/cli/chat/>
- Trusted folders: <https://geminicli.com/docs/configuration/trusted-folders/>

## Layout

```pseudocode
~/.gemini/
├── projects.json                     # cwd → alias map (THE index)
├── history/
│   ├── main/                         # alias for first seen "main" cwd
│   │   ├── .project_root             # absolute path marker
│   │   └── <chat-tag>.json           # per-saved-chat transcript
│   ├── main-1/                       # second "main" (collision)
│   │   └── ...
│   ├── main-15/                      # 16th collision
│   │   └── ...
│   ├── ideacrafterslabs/             # unique basename
│   ├── rania/
│   ├── gstack/
│   └── agent-skills/
├── settings.json
├── oauth_creds.json
├── state.json
└── trustedFolders.json
```

## projects.json — the central map

```pseudocode
{
  "projects": {
    "/Users/jadb/.w/ideacrafterslabs/tlc/hops/main": "main",
    "/Users/jadb/.w/ideacrafterslabs/clear": "clear",
    "/Users/jadb/.w/ideacrafterslabs/clear/hops/main": "main-1",
    "/Users/jadb/.w/ideacrafterslabs/owsf": "owsf",
    "/Users/jadb/.w/ideacrafterslabs/wsm/hops/main": "main-2",
    "/Users/jadb/.w/ideacrafterslabs/xray/hops/main": "main-4",
    "/Users/jadb/.w/ideacrafterslabs/aps/hops/main": "main-7",
    "/Users/jadb/.w/ideacrafterslabs/routellm/hops/main": "main-9",
    "/Users/jadb/.w/ideacrafterslabs/eva/hops/main": "main-10",
    ...
  }
}
```

- **Forward lookup** (cwd → alias): O(1) via map
- **Backward lookup** (alias → cwd): must scan all values
- Worktrees under `hops/main` all resolve to basename `main` → all
  collide → `main-1`, `main-2`, ..., `main-15` assigned in first-seen
  order
- Alias order depends on discovery history; not stable across users

## Alias generation rules

```pseudocode
def alias_for(cwd):
  base = basename(cwd)                     # e.g. "main" or "clear"
  if base not in used_aliases:
    return base
  for i in 1..:
    candidate = f"{base}-{i}"
    if candidate not in used_aliases:
      return candidate
```

- Collisions indexed by first-seen order
- No project metadata embedded in alias — `main-7` is meaningless
  without consulting `projects.json`

## Transcript storage

- Location: `~/.gemini/history/<alias>/`
- `.project_root` — plain text file containing the absolute cwd
  (redundant with projects.json; useful if map is corrupted)
- Saved chats: `<chat-tag>.json` (one per `/chat save <tag>` invocation)
- **Auto-save:** recent history retained as an implicit
  most-recent-chat; tag-less resume uses it
- Format: JSON object with `history` array + metadata

## Resume / continue semantics

- `gemini chat list` — list saved chats for current cwd (alias
  resolved from `projects.json`)
- `gemini chat resume <tag>` — resume specific saved chat
- `/chat save <tag>` — save current chat mid-session
- `/chat resume <tag>` — resume inside TUI
- Resume is scoped to current cwd's alias by default; cross-project
  resume requires `--history-dir` override

## Coverage vs Claude Code

| Claude concept         | Gemini equivalent                       |
|------------------------|-----------------------------------------|
| Project key encoding   | ≈ basename + suffix (via projects.json) |
| Per-project sessions   | ✅ per-alias dir                         |
| JSONL transcript       | 🔧 per-chat JSON (not append-stream)    |
| Resume by UUID         | 🔧 resume by user tag (named saves)     |
| Auto most-recent       | ✅ implicit untagged chat                |
| Worktree isolation     | ✅ each distinct cwd → distinct alias    |
| Memory subsystem       | ❌ none (GEMINI.md is instructions only) |
| Cross-project search   | ❌ no native search                      |

## Strengths

- Central `projects.json` is a clean, queryable index of every cwd
  Gemini has seen — best bootstrap for USP discovery
- Per-cwd history dir makes per-project filtering trivial once
  alias is resolved
- `.project_root` marker inside each history dir is a self-healing
  hint if `projects.json` drifts
- Named `/chat save <tag>` model is human-friendly

## Known gotchas

- **Basename collisions** are the dominant organizing principle
  under `hops/main` worktree conventions — `main-15` conveys nothing
- Alias is assigned once and never changes, even if cwd is renamed
  or moved — old alias keeps pointing at a dead path
- `.project_root` can disagree with `projects.json` if either is
  edited manually
- Worktrees created after the map is populated still get distinct
  aliases (good for isolation, bad for cross-worktree resume)
- No append-only stream format — each `/chat save` is a full
  snapshot; losing a save loses that state
- Trusted-folders gate (`trustedFolders.json`) can silently block
  chat history from being written for untrusted cwds

## Open questions

1. Does Gemini rewrite `projects.json` atomically, or is concurrent
   CLI invocation a race condition?
2. What happens when two cwds share basename AND one is deleted —
   does the alias get recycled, or is it permanently burned?
3. Does `/chat save` overwrite existing tags silently or prompt?
4. Is there a canonical event log (like Codex's history.jsonl) or
   only per-chat snapshots?

## Integration notes for USP

Gemini is the second-easiest adapter after Claude. Required work:

1. Parse `~/.gemini/projects.json` → build reverse map (alias → cwd)
2. Watch `~/.gemini/history/<alias>/*.json` for new/modified chats
3. Cross-reference with `.project_root` for integrity
4. Translate Gemini chat JSON to USP envelope (per-chat snapshot
   model; emit synthetic `session.created`/`session.ended` around
   each saved chat)
5. Handle alias collisions transparently — USP session key is the
   absolute cwd, not the alias

Expected adapter size: ~300 LOC Go.
