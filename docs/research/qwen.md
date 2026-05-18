# Qwen Code — Sessions Research Dossier

> Gemini CLI fork. SHA256-hashed project dirs under `~/.qwen/tmp/<hash>/`
> containing `chats/session-*.json`, `logs.json`, and `shell_history`.
> Nearly identical layout to Gemini CLI, different root dir.

Last verified: 2026-04-09. Author: jadb.

## Summary

- **Package:** `@qwen-code/qwen-code` v0.14.0 (Homebrew cask `qwen-code`)
- **Binary:** `/opt/homebrew/bin/qwen` → `Cellar/qwen-code/0.14.0/bin/qwen`
- **Store root:** `~/.qwen/tmp/<project-sha256>/`
- **Project key:** SHA256 hex of absolute project root (via
  `getProjectHash(projectRoot)`)
- **Session file:** `~/.qwen/tmp/<hash>/chats/session-<YYYY-MM-DD>T<HH-MM>-<short>.json`
- **Transcript format:** per-chat JSON file (snapshot, not append)
- **Logs:** `~/.qwen/tmp/<hash>/logs.json`
- **Shell history:** `~/.qwen/tmp/<hash>/shell_history`
- **History (checkpoint):** `~/.qwen/tmp/<hash>/checkpoints/` (if enabled)
- **Resume CLI:** `qwen --continue` | `qwen --resume <tag>`
- **Chat recording flag:** `--chat-recording` — when `false`,
  `--continue`/`--resume` are no-ops
- **Platform:** macOS + Linux + Windows

## Sources

- Package source: <https://github.com/QwenLM/qwen-code>
- Storage class: `packages/core/src/config/storage.ts`
  (verified via `gh api`)
- Path hash: `packages/core/src/utils/paths.ts` —
  `getProjectHash = sha256(projectRoot)` (lowercased on Windows)
- `sanitizeCwd = cwd.replace(/[^a-zA-Z0-9]/g, '-')` — used for
  `projects/` sub-dir (separate from `tmp/` hash)
- Local install: `/opt/homebrew/Cellar/qwen-code/0.14.0/libexec/lib/
  node_modules/@qwen-code/qwen-code/` (bundled `cli.js`, 21MB)
- CLI help: `qwen --help` (flag `--chat-recording`)
- Verified filesystem: `~/.qwen/` (only `skills/` populated locally,
  tmp/ absent — qwen has not been run with chat recording here)

## Layout

```pseudocode
~/.qwen/
├── tmp/                                     # runtime output root
│   └── <sha256-of-project-root>/            # per-project hash
│       ├── chats/
│       │   ├── session-2026-04-09T10-15-a1b2c3d4.json
│       │   └── session-2026-04-09T14-42-5e6f7a8b.json
│       ├── logs.json                        # event log
│       ├── shell_history                    # plain-text shell history
│       └── checkpoints/                     # if --checkpointing enabled
│           └── <checkpoint-files>
├── projects/                                # sanitizeCwd-named dirs
│   └── <sanitized-cwd>/                     # e.g. Users-jadb-...
│       └── <project-scoped state>
├── history/                                 # per-project history dir
│   └── <sha256-of-project-root>/
│       └── <history files>
├── settings.json                            # global config
├── oauth_creds.json                         # auth
├── installation_id                          # telemetry anchor
├── extensions/                              # qwen extensions
├── commands/                                # user slash commands
├── plans/
│   └── <session-id>.md
├── debug/
│   └── <session-id>.txt
├── ide/                                     # IDE integration state
├── arena/                                   # model arena?
├── bin/
├── mcp-oauth-tokens.json
├── google_accounts.json                     # (inherited from Gemini)
├── memory.md                                # global memory file
└── skills/                                  # user skills
```

- **Two project keyings exist:**
  - `tmp/<sha256>/` for runtime output (chats, logs, history)
  - `projects/<sanitize(cwd)>/` for project-scoped durable state
- No central `projects.json` like Gemini's cwd → alias map

## Project key encoding

### tmp/ and history/ — SHA256 hash

```pseudocode
def getProjectHash(projectRoot):
  normalized = projectRoot.lowercase() if win32 else projectRoot
  return sha256(normalized).hexdigest()
```

- Example: `/Users/jadb/.w/ideacrafterslabs/uhp/hops/main`
  → `<64-hex-chars>`
- Windows case-folds; Unix preserves case
- Opaque — reverse lookup requires external cwd→hash map (like
  OpenCode)

### projects/ — sanitizeCwd

```pseudocode
def sanitizeCwd(cwd):
  normalized = cwd.lowercase() if win32 else cwd
  return normalized.replace(/[^a-zA-Z0-9]/g, '-')
```

- Example: `/Users/jadb/.w/ideacrafterslabs/uhp/hops/main`
  → `-Users-jadb--w-ideacrafterslabs-uhp-hops-main`
- Similar to Claude Code's slash-to-dash but replaces ALL
  non-alphanumeric (dots → dash too)
- Human-decodable; used for durable project state, not transcripts

## Storage class — verified methods

```pseudocode
class Storage:
  # Global (no project scope)
  static getGlobalQwenDir()     # ~/.qwen
  static getGlobalTempDir()     # <runtime>/tmp
  static getGlobalDebugDir()    # <runtime>/debug
  static getGlobalIdeDir()      # <runtime>/ide
  static getPlansDir()          # ~/.qwen/plans
  static getPlanFilePath(sessionId)  # plans/<sessionId>.md
  static getDebugLogPath(sessionId)  # debug/<sessionId>.txt

  # Per-project (this.targetDir scoped)
  getProjectRoot()              # this.targetDir
  getProjectDir()               # <runtime>/projects/<sanitizeCwd>
  getProjectTempDir()           # <runtime>/tmp/<sha256>
  getProjectTempCheckpointsDir()# <runtime>/tmp/<sha256>/checkpoints
  getHistoryDir()               # <runtime>/history/<sha256>
  getQwenDir()                  # <project>/.qwen  (project-local)
  getWorkspaceSettingsPath()    # <project>/.qwen/settings.json
  getExtensionsDir()            # <project>/.qwen/extensions
```

- `getRuntimeBaseDir()` resolves via env `QWEN_RUNTIME_DIR` →
  context ALS → `setRuntimeBaseDir` → `getGlobalQwenDir()`
- Allows isolating concurrent sessions via AsyncLocalStorage

## Transcript schema

- File: `~/.qwen/tmp/<hash>/chats/session-<ISO>-<short>.json`
- Format: **per-chat snapshot JSON** (not appended JSONL)
- Filename pattern: `session-YYYY-MM-DDTHH-MM-<8-hex>.json` (from
  verified sibling Gemini CLI local files — qwen inherits naming)
- Content: JSON object with `history` array + metadata (unverified
  schema; inferred from Gemini CLI fork lineage)
- `logs.json`: sibling file with event log (also per-project)
- `shell_history`: plain text, one command per line

## Resume / continue semantics

- `qwen --continue` (`-C`) — resume most recent session for cwd
- `qwen --resume <tag>` (`-S`, `-r`) — resume specific session;
  without arg → interactive picker
- `qwen --chat-recording <bool>` — toggle recording; **when false,
  continue/resume become no-ops** (documented in `--help`)
- Resume is scoped via cwd → sha256 hash lookup → load latest
  `chats/session-*.json`
- Similar to Gemini's `/chat resume <tag>` pattern

## Project grouping

- **SHA256 opaque.** Reverse lookup requires:
  1. Walk `~/.qwen/tmp/*/`
  2. For each, cross-reference with the session file's embedded
     cwd field (if present) or compute `sha256(known_cwds)` and
     match
- No central map file (unlike Gemini's `projects.json`)
- `sanitizeCwd` dirs under `projects/` give human-readable project
  identification BUT hold durable state, not transcripts —
  adapter must correlate the two keyings

## Strengths

- Clean Gemini CLI lineage → similar semantics, easier adapter
  reuse
- SHA256 provides stable, collision-free keying (vs Gemini's
  basename collisions)
- Per-project `plans/<session-id>.md` and `debug/<session-id>.txt`
  enable artifact recovery
- `QWEN_RUNTIME_DIR` env var allows sandbox/isolation
- Schema is mostly inherited from Gemini → adapter code shares
- `--chat-recording` flag is explicit, surfacing the privacy control

## Known gotchas

- **No projects.json central map.** Unlike Gemini, there's no
  single file to read cwd ↔ hash mappings; must compute hashes
  from known cwds or walk + read embedded metadata
- **Two project keyings** (`tmp/<sha256>/` vs `projects/<sanitize>/`)
  — USP must pick one as canonical and cross-reference
- **`--chat-recording false` silently disables persistence** — if
  users set this, sessions simply vanish on exit
- Per-chat JSON snapshot model: each save is a full rewrite, not
  append-only — prone to data loss on crash mid-write
- No append-only event log for transcripts (only `logs.json` at
  project level, which is not the conversation itself)
- Windows case-folds project paths → macOS/Linux agents that
  migrate to WSL would lose session continuity
- Local install on this machine has never been run with recording
  enabled — `tmp/` is absent, so live verification of file
  contents was not possible

## Open questions

1. Exact schema of `chats/session-*.json` — does it inherit
   Gemini's format 1:1 or has Qwen added fields?
2. Is `logs.json` an event stream (append) or a snapshot?
3. Does `--continue` accept a cwd override, or strict current-cwd?
4. Is `projects/<sanitize>/` just settings override or does it
   contain any session-linked data?
5. Does checkpointing write to `tmp/<hash>/checkpoints/` during a
   session, and what format (git snapshots? full-tree dumps?)
6. Does Qwen emit a session ID in filenames that's preserved
   across resumes, or is it per-save?

## Integration notes for USP

Qwen is **easiest adapter after Gemini** (basically shared code).

Strategy:

1. Compute `sha256(cwd)` on the fly; walk
   `~/.qwen/tmp/<hash>/chats/` to find sessions per known cwd
2. Maintain a cwd → hash index locally (since no projects.json);
   rebuild by walking + reading first-seen cwd from embedded
   metadata if present
3. Watch `~/.qwen/tmp/` with fswatch; on new `chats/session-*.json`,
   emit `session.created`; on mtime update, emit turn events
4. Treat per-chat JSON as **snapshot**: diff against previous
   version to extract new turns; emit as USP events
5. Correlate `plans/<session-id>.md` and `debug/<session-id>.txt`
   as attached resources
6. Handle `--chat-recording false` by emitting `adapter.recording_off`
   status when tmp/ is empty despite active sessions

Expected adapter size: ~350 LOC Go (sha256 key + tmp/ walker +
per-chat snapshot differ + fswatch bridge). Large overlap with
Gemini adapter — consider a shared base adapter with per-CLI
config (root dir, project dir strategy).
