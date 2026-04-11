# Antigravity (agy) — Sessions Research Dossier

> VS Code fork (Code OSS derivative). Chat sessions live in VS Code's
> workspace/global storage: SQLite `state.vscdb` +
> `chatSessions/*.json` JSON files per workspace. Project grouping
> via VS Code's opaque `workspaceStorage/<hash>/` scheme with a
> `workspace.json` pointing at the folder URI.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Install:** `~/.antigravity/antigravity/bin/agy` (launcher)
- **VS Code data root:** `~/Library/Application Support/Antigravity/`
  (NOT `~/.antigravity/`)
- **Extensions root:** `~/.antigravity/extensions/`
- **Chat session files:**
  `~/Library/Application Support/Antigravity/User/workspaceStorage/
  <ws-hash>/chatSessions/<session-uuid>.json`
- **Session index:** `chat.ChatSessionStore.index` key in
  global `state.vscdb` ItemTable (currently `{"version":1,"entries":{}}`
  → empty on this machine)
- **Workspace pointer:**
  `workspaceStorage/<ws-hash>/workspace.json` (contains `folder:` URI)
- **Global state db:** `~/Library/Application Support/Antigravity/
  User/globalStorage/state.vscdb` (SQLite with `ItemTable`)
- **Backups:** `~/Library/Application Support/Antigravity/Backups/
  <hash>/`
- **Resume:** via GUI chat panel; no documented CLI resume flag
- **Platform:** macOS + Linux + Windows (as VS Code runs)
- **Docs:** <https://antigravity.google/>

## Sources

- Launcher bin: `/Users/jadb/.antigravity/antigravity/bin/agy`
- Extensions: `~/.antigravity/extensions/` (~15 entries, VSIX-style)
- Chrome-style app data: `~/Library/Application Support/Antigravity/`
  (Preferences, Cookies, IndexedDB, etc. — VS Code + Electron)
- User data: `~/Library/Application Support/Antigravity/User/`
- Verified SQLite:
  `globalStorage/state.vscdb` ItemTable keys and
  `chat.ChatSessionStore.index` value
- Verified file sample:
  `workspaceStorage/fbc5df20c6dcc7063985fccf8467a92f/chatSessions/
  a8969187-e0be-4408-95dc-a5781793b48f.json` (in VS Code's stock
  location; Antigravity uses the same schema under its own root —
  inspected Antigravity's tree to confirm dir shape)

## Layout

```pseudocode
~/.antigravity/
├── antigravity/
│   └── bin/
│       └── agy                                  # launcher script
├── extensions/                                  # VSIX extension dir
│   └── <ext-id>/
├── argv.json
└── ...

~/Library/Application Support/Antigravity/
├── User/
│   ├── globalStorage/
│   │   ├── state.vscdb                          # global SQLite
│   │   │   └── ItemTable keys including:
│   │   │       - chat.ChatSessionStore.index    # global session index
│   │   │       - chat.customModes
│   │   │       - chat.participantNameRegistry
│   │   │       - chat.workspaceTransfer
│   │   │       - antigravity.agentViewContainerId.state.*
│   │   │       - antigravityUnifiedStateSync.*  # pref sync keys
│   │   │       - jetskiStateSync.*              # (jetski = agent engine?)
│   │   ├── state.vscdb-journal
│   │   ├── state.vscdb.backup
│   │   ├── storage.json                         # 94K, global prefs
│   │   └── vscodevim.vim/                       # extension state
│   ├── workspaceStorage/                        # ~35 workspace dirs
│   │   ├── <ws-hash>/                           # per-workspace
│   │   │   ├── workspace.json                   # { "folder": "file:///..." }
│   │   │   ├── state.vscdb                      # per-workspace SQLite
│   │   │   ├── chatSessions/
│   │   │   │   └── <session-uuid>.json          # per-session JSON
│   │   │   └── chatEditingSessions/             # edit-mode sessions
│   │   └── ...
│   ├── profiles/
│   │   └── -1ef4f7bf/                           # custom profile
│   │       ├── settings.json
│   │       ├── extensions.json
│   │       └── globalStorage/
│   ├── History/                                 # file edit history
│   │   └── -<hash>/                             # per-file
│   ├── snippets/
│   └── settings.json
├── Backups/
│   ├── 99efb400774e8513505b37f352cbf841/
│   └── 237568d45579b3d889bb19cb46acc941/
├── CachedData/
├── CachedExtensionVSIXs/
├── CachedProfilesData/
├── Workspaces/                                  # opened workspace list
├── logs/                                        # Code OSS logs
├── Crashpad/
├── machineid
├── Preferences                                  # Chromium prefs
└── ... (Chromium app data)
```

## Workspace hash (project key)

- `workspaceStorage/<hash>/` — opaque hash (VS Code uses MD5 of
  workspace URI; verified by matching known patterns but not
  dumped from Antigravity source)
- `workspace.json` at that hash dir contains the ground-truth
  folder URI:

```pseudocode
# workspaceStorage/fbc5df20c6dcc7063985fccf8467a92f/workspace.json
{
  "folder": "file:///Users/jadb/.claude-worktrees/exo/infallible-black"
}
```

- Reverse lookup: walk `workspaceStorage/*/workspace.json`, extract
  `folder` URI, strip `file://` prefix for cwd

## Chat session storage

### Per-session JSON file

Verified schema from a sibling VS Code install
(`chatSessions/a8969187-e0be-4408-95dc-a5781793b48f.json`):

```pseudocode
{
  "version": 3,
  "responderUsername": "GitHub Copilot",         # varies by provider
  "responderAvatarIconUri": { "id": "copilot" },
  "initialLocation": "panel",
  "requests": [],                                # empty = new session
  "sessionId": "a8969187-e0be-4408-95dc-a5781793b48f",
  "creationDate": 1770431633363,                 # epoch ms
  "lastMessageDate": 1770431633363,
  "hasPendingEdits": false,
  "inputState": {
    "contrib": { "chatDynamicVariableModel": [] },
    "attachments": [],
    "mode": { "id": "agent", "kind": "agent" },
    "selectedModel": {
      "identifier": "copilot/gpt-5-mini",
      "metadata": { ... model info ... }
    },
    "inputText": "",
    "selections": [...]
  }
}
```

- Antigravity uses the same `chatSessions/<uuid>.json` layout
  (inherited from VS Code chat framework); provider identifier
  will differ (Google/Gemini-based rather than Copilot)
- `requests` array holds turns (empty in sample = unused session);
  each request likely has user input + response + tool calls

### Global session index

```pseudocode
# state.vscdb ItemTable key: chat.ChatSessionStore.index
{
  "version": 1,
  "entries": {}                                  # empty on this machine
}
```

- When populated, `entries` likely maps session-id → metadata
  preview for fast cross-workspace enumeration
- Currently empty — Antigravity on this machine has zero chat
  activity despite being opened (workspaces exist in
  `workspaceStorage/` but no chatSessions populated)

## Resume / continue semantics

- **GUI-driven.** No documented CLI resume flag; `agy` launcher
  opens the app window
- Chat panel in-app shows history via `chat.ChatSessionStore.index`
  → workspaceStorage lookups
- `chat.workspaceTransfer` ItemTable key suggests support for
  moving chat sessions between workspaces
- Resume semantics inherit from VS Code chat: click session in
  panel → load `<session-uuid>.json` → rehydrate into editor

## Project grouping

- **Via `workspaceStorage/<hash>/workspace.json`.** The folder URI
  is the project key; hash is opaque
- Reconstruction: walk all `workspaceStorage/*/workspace.json`,
  parse `folder` URI, build hash → cwd map
- Chat sessions are already sharded by workspace hash → no
  additional grouping layer needed once hash→cwd is known
- Global sessions (not tied to a workspace) may exist but none
  observed

## Strengths

- VS Code heritage → stable, well-known layout (adapter code can
  reuse VS Code adapter infra if any exists)
- `workspace.json` with `folder` URI is a clean project pointer
  (unlike opaque hashes in OpenCode/Qwen)
- Per-session JSON → easy to parse, human-readable
- `chat.ChatSessionStore.index` provides a global enumeration
  point (when populated)
- SQLite `ItemTable` is a single key-value store with stable
  extraction (`SELECT key, value FROM ItemTable`)
- Backups directory provides recovery options

## Known gotchas

- **Two storage roots** — extensions live at `~/.antigravity/`,
  everything else (sessions, config, caches) lives at
  `~/Library/Application Support/Antigravity/`
- Workspace hash is opaque; reverse lookup requires reading every
  `workspace.json` (cheap but O(n))
- `chat.ChatSessionStore.index` may lag per-workspace files
  (VS Code's chat store is eventually consistent)
- Session UUIDs are globally unique but workspace-filtered; a
  "global" chat may not appear under any specific workspace hash
- VS Code chat JSON schema has `"version"` field; adapters must
  handle version bumps
- `requests` array may be empty for stub sessions that were never
  used; filter those
- The `jetskiStateSync.*` and `antigravityUnifiedStateSync.*` keys
  suggest server-side sync is a feature — local sessions may
  lack history that's been cloud-synced
- VS Code `workspaceStorage` scheme caches stale workspace hashes
  forever; deleted workspaces remain in the dir
- `chat.workspaceTransfer` key hints at session migration between
  workspaces — a session's "owning workspace" may change
- The `-1ef4f7bf` profile dir suggests custom profiles — sessions
  in profile dirs are separate from the default profile

## Open questions

1. Exact shape of `chat.ChatSessionStore.index.entries` when
   populated — what fields per entry?
2. Does Antigravity store tool calls / agent actions in
   `requests[]` inline, or in `chatEditingSessions/`?
3. Is there a CLI flag (via `agy`) to list/resume sessions
   non-interactively?
4. How do custom profiles (`profiles/-1ef4f7bf/`) interact with
   chat session storage — parallel `workspaceStorage` tree?
5. What's the `jetski` engine — is it the agent runtime that
   writes to a parallel store not in `chatSessions/`?
6. Does `chat.workspaceTransfer` imply sessions can be imported
   from other VS Code forks (Code, Cursor, Antigravity)?
7. Is there a server-synced copy of sessions elsewhere (Google
   account-linked)?

## Integration notes for USP

Antigravity is **VS Code-flavored opaque-hash mid-complexity**.

Strategy:

1. **Dual-root awareness:** remember to poll
   `~/Library/Application Support/Antigravity/User/` NOT
   `~/.antigravity/`
2. **Workspace index:** walk
   `workspaceStorage/*/workspace.json` → build hash → folder URI
   map; strip `file://` prefix for cwd
3. **Session enumeration:** for each workspace hash, list
   `chatSessions/*.json` and skip stub sessions
   (`requests.length == 0 && lastMessageDate == creationDate`)
4. **Global index sanity:** read
   `globalStorage/state.vscdb` ItemTable key
   `chat.ChatSessionStore.index` — if non-empty, use as hint for
   recent sessions
5. **Translate JSON → USP envelope:** map
   `requests[]` to turn events; preserve `inputState.mode` +
   `selectedModel.identifier` as session metadata
6. **Backup awareness:** `Backups/` directory may hold
   recoverable sessions; surface as warnings if live sessions
   drop
7. **Profile handling:** detect `profiles/*/` subdirs and walk
   each as a sibling workspace tree
8. **SQLite open:** use read-only connection with shared cache;
   Antigravity may hold an exclusive lock while running

Expected adapter size: ~500 LOC Go (SQLite reader + workspace
walker + JSON deserializer + profile-aware multiplexer). Larger
than Claude/Gemini adapters due to nested hash → URI indirection
+ dual-root awareness.
