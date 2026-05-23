# usp × kit 12fcc-leak baseline (T-0120)

Captured against `kit/hops/12fcc-leak` after swapping the `replace
hop.top/kit => ...` line in `go.mod` to point at that worktree.
Replays use the binary at `/tmp/usp` built from this tree.

## Build / boot

- `/usr/bin/env go build -buildvcs=false ./...` → exit 0 (clean).
- `/usr/bin/env go build -buildvcs=false -o /tmp/usp ./cmd/usp` → exit 0.
- `/tmp/usp --help` → exit 2. Validation fails **at boot** (i.e. before
  cobra is allowed to render help / dispatch). Stderr message:

  ```
  cli validation failed: 15 leaf command(s) missing kit/side-effect
  annotation: usp alias add, usp alias delete, usp alias list, usp
  doctor, usp mcp, usp resume, usp session lineage, usp session list,
  usp session search, usp session show, usp session skills, usp session
  tools, usp setup, usp upgrade, usp version; 8 leaf command(s) missing
  kit/idempotent annotation: usp mcp, usp resume, usp session lineage,
  usp session skills, usp session tools, usp setup, usp upgrade, usp
  version; 13 leaf command(s) missing Long: usp alias, usp alias add,
  usp alias delete, usp alias list, usp doctor, usp mcp, usp resume,
  usp session lineage, usp session list, usp session search, usp
  session show, usp setup, usp version; 1 leaf command(s) missing
  reserved 'status' subcommand: usp; 7 leaf command(s) depth-1 leaf
  missing kit/top-level-verb: usp alias, usp doctor, usp mcp, usp
  resume, usp setup, usp upgrade, usp version
  ```

- `/tmp/usp status` → exit 2, same message. (`status` reserved subcmd
  is missing entirely, so kit short-circuits before any user-defined
  cmd can run.)
- Strict validation: **fails at boot** for every invocation. Kit
  defaults `Config.EnforceValidate=true` via `cli.New` unless the
  adopter sets `DisableValidate=true` (cf. `kit/go/console/cli/cli.go`
  lines 344–351 + 732–738). usp **does not** set `DisableValidate`
  today, so the strict gate fires on Execute pre-flight and routes
  through `ValidationFailureExit` → `os.Exit(2)`.
- `DisableValidate` does not currently exist anywhere under usp/ — no
  pre-existing opt-out to remove. Annotation work can be staged
  without any teardown.

## Failure buckets (per command path)

Annotations referenced below all live under the reserved `kit/` prefix
(see `kit/go/console/cli/sideeffect.go`, `idempotent.go`, `shape.go`).
For usp-owned files the `file:LINE` is the `cobra.Command{` declaration
line so an editor can land on the surface that needs an annotation
helper call (e.g. `cli.SetSideEffect(cmd, cli.SideEffectRead)`).

### Missing side-effect annotation (15 leaves)

- `usp alias` (group leaf — kit ships) — `kit/go/console/cli/alias_cmd.go:40`
- `usp alias add` — `kit/go/console/cli/alias_cmd.go:85` (write)
- `usp alias delete` — `kit/go/console/cli/alias_cmd.go:105` (destructive)
- `usp alias list` — `kit/go/console/cli/alias_cmd.go:56` (read)
- `usp doctor` — `cmd/usp/doctor.go:33` (read)
- `usp mcp` — `cmd/usp/mcp.go:12` (interactive / long-lived stdio)
- `usp resume` — `cmd/usp/resume.go:17` (interactive — `syscall.Exec`)
- `usp session lineage` — `cmd/usp/session_lineage.go:15` (read)
- `usp session list` — `cmd/usp/session_list.go:43` (read)
- `usp session search` — `cmd/usp/session_list.go:173` (read)
- `usp session show` — `cmd/usp/session_show.go:84` (read)
- `usp session skills` — `cmd/usp/session_skills.go:45` (read)
- `usp session tools` — `cmd/usp/session_tools.go:44` (read)
- `usp setup` — `cmd/usp/setup.go:26` (write — opens & seeds index DB)
- `usp upgrade` — `cmd/usp/upgrade.go:25` (write — overwrites binary)
- `usp version` — `cmd/usp/version.go:13` (read)

Note: kit's alias-cmd subtree (`alias`, `alias list`, `alias add`,
`alias delete`) is constructed inside `Root.AliasCmd(store)` and is
**not** annotated by kit itself in 12fcc-leak. usp cannot fix these
without either (a) calling `cli.SetSideEffect` on the returned
`*cobra.Command` post-construction, walking children, or (b) waiting
for kit to ship them annotated. Same applies to `kit/idempotent` /
`Long` discipline on those four leaves.

### Missing idempotency annotation (8 leaves)

- `usp mcp` — `cmd/usp/mcp.go:12`
- `usp resume` — `cmd/usp/resume.go:17` (non-idempotent: hands off
  argv to a target CLI)
- `usp session lineage` — `cmd/usp/session_lineage.go:15`
- `usp session skills` — `cmd/usp/session_skills.go:45`
- `usp session tools` — `cmd/usp/session_tools.go:44`
- `usp setup` — `cmd/usp/setup.go:26`
- `usp upgrade` — `cmd/usp/upgrade.go:25`
- `usp version` — `cmd/usp/version.go:13`

(Kit auto-applies a verb-default `kit/idempotent=yes` to `list`,
`show`, `search`, `doctor` — see `cli.go:796` "Auto-apply default
kit/idempotent before checking"; that's why those names do not appear
in this bucket even though the side-effect bucket lists them.)

### Missing/short Long help (13 leaves)

Every command whose `Long:` field is empty triggers Layer-A §2 hard
tier. Verified by grepping the `Use:` declarations in cmd/usp/*.go
and kit/.../alias_cmd.go.

- `usp alias` — kit `alias_cmd.go:40`
- `usp alias add` — kit `alias_cmd.go:85`
- `usp alias delete` — kit `alias_cmd.go:105`
- `usp alias list` — kit `alias_cmd.go:56`
- `usp doctor` — `cmd/usp/doctor.go:33`
- `usp mcp` — `cmd/usp/mcp.go:12`
- `usp resume` — `cmd/usp/resume.go:17`
- `usp session lineage` — `cmd/usp/session_lineage.go:15`
- `usp session list` — `cmd/usp/session_list.go:43`
- `usp session search` — `cmd/usp/session_list.go:173`
- `usp session show` — `cmd/usp/session_show.go:84`
- `usp setup` — `cmd/usp/setup.go:26`
- `usp version` — `cmd/usp/version.go:13`

(`usp session skills`, `usp session tools` already have a multi-line
`Long`. `usp upgrade` has a single-sentence `Long` ("Check for a newer
version of usp and optionally install it.") which kit accepts under
the current threshold — not flagged.)

### Missing status support (1 root)

- `usp` root — no `status` subcommand registered in `main.go`. Kit's
  reserved-status check (`§Layer-A: reserved 'status' subcommand
  present`) fires on `usp` itself. Fix: register `cli.StatusCmd(...)`
  or hand-roll a `usp status` cobra leaf that reports build/binary +
  index DB readiness.

### Shape violations / depth-1 leaf missing kit/top-level-verb (7 leaves)

Layer-A §3.5 requires every depth-1 leaf to either set
`kit/top-level-verb=true` (kit-blessed reserved verb) or be wrapped
under a noun group with `kit/hierarchical`. Today these depth-1
leaves carry no annotation:

- `usp alias` — kit `alias_cmd.go:40`
- `usp doctor` — `cmd/usp/doctor.go:33`
- `usp mcp` — `cmd/usp/mcp.go:12`
- `usp resume` — `cmd/usp/resume.go:17`
- `usp setup` — `cmd/usp/setup.go:26`
- `usp upgrade` — `cmd/usp/upgrade.go:25`
- `usp version` — `cmd/usp/version.go:13`

`usp session` (noun group) is NOT in the bucket because it has
children — kit treats it as a group, not a depth-1 leaf. `usp config`
is also exempt because its subtree comes from kit's
`RegisterPathSubcommands` which sets `kit/side-effect=read` +
`kit/idempotent=yes` (see `kit/go/console/cli/config/paths_cmd.go:69
+ 70 + 107 + 108`).

### Passthrough annotations needed

- None. `grep -rn "ArbitraryArgs|DisableFlagParsing" cmd/` returns
  nothing. No usp command forwards `argv -- <args>` to a sub-process
  through cobra; `usp resume` does `syscall.Exec(bin, argv, ...)` but
  builds argv internally (`res.Command` from the resume planner), so
  cobra's parser owns the flag handling end-to-end. No
  `kit/passthrough` work needed at this baseline.

### Local-global flag collisions (--force/--yes vs kit --confirm)

- `cmd/usp/upgrade.go:37` registers a local `--quiet`/`-q` Bool flag.
  Kit's `cli.New` already registers `--quiet` as a persistent global
  on the root (`kit/go/console/cli/cli.go:429`). This is a **local vs
  global flag collision** — cobra will accept the redeclaration on
  the leaf but the local binding shadows the kit global for `usp
  upgrade` only. Fix: drop the local registration and read via
  `viper.GetBool("quiet")`, mirroring how other usp leaves read
  `default_cli` / `default_limit` from rootViper.
- No `--force` or `--yes` flag anywhere in `cmd/`, `internal/`, or
  `adapters/`. usp has no destructive prompts at all today, so it
  does not collide with kit's `--confirm` / `--confirm-token` /
  `--max-ops` / `--policy` globals (see
  `kit/go/console/cli/policy_runE.go:21-25`). Adding `--confirm`
  semantics later means consuming kit's globals, not registering
  locals.

## Counts

- Total runnable leaf commands: 16
  (alias, alias add, alias delete, alias list, config path,
  config paths, doctor, mcp, resume, session lineage, session list,
  session search, session show, session skills, session tools, setup,
  upgrade, version) — 18 nominal minus the 2 already-annotated
  `config path` / `config paths` reserved leaves = 16 in scope for
  Layer-A annotation work; counts below treat the alias subtree as 4
  separate kit-owned leaves that usp must coordinate fixing upstream.
- Leaves needing side-effect annotation: 15
- Leaves needing idempotency annotation: 8
- Leaves needing Long: 13
- Leaves with destructive flags: 1 (`usp alias delete` — kit-owned;
  the only inherently destructive leaf in the tree today). `usp
  upgrade` overwrites the binary in place but the operation is
  user-driven and reversible by reinstalling, treat as write tier
  rather than destructive at first cut.

## Notes for downstream agents

### Subtree partitioning suggestion (3 roughly equal-sized groups)

Goal: keep each agent's diff under ~5 leaves of work and avoid
cross-agent file collisions. `cmd/usp/main.go` is the only file all
three need to coordinate on (each will edit it to wire annotations
onto its branch).

1. **Group A — session subtree (6 leaves, one file each)**
   - `cmd/usp/session.go` (group node — Long + kit/hierarchical)
   - `cmd/usp/session_list.go` (list, search — 2 leaves in this file)
   - `cmd/usp/session_show.go` (show)
   - `cmd/usp/session_lineage.go` (lineage)
   - `cmd/usp/session_skills.go` (skills — only needs side-effect +
     idempotent; Long already exists)
   - `cmd/usp/session_tools.go` (tools — same as skills)

2. **Group B — top-level read/management leaves (5 leaves)**
   - `cmd/usp/doctor.go` (doctor — top-level-verb + side-effect=read +
     Long)
   - `cmd/usp/mcp.go` (mcp — top-level-verb + side-effect +
     idempotent + Long)
   - `cmd/usp/version.go` (version — top-level-verb + side-effect=read
     + idempotent=yes + Long)
   - `cmd/usp/setup.go` (setup — top-level-verb + side-effect=write +
     idempotent=no + Long)
   - `cmd/usp/upgrade.go` (upgrade — top-level-verb + side-effect=write
     + idempotent=no + drop local --quiet collision)

3. **Group C — resume + alias coordination + reserved status (4–5
   leaves, mostly upstream pings)**
   - `cmd/usp/resume.go` (resume — top-level-verb + side-effect +
     idempotent=no + Long)
   - `cmd/usp/alias.go` + upstream kit patch: annotate the 4 kit-owned
     alias leaves (or, fast path: wrap `root.AliasCmd(store)` in a
     post-construction walk that injects annotations until kit lands
     them).
   - **New `cmd/usp/status.go`**: register `usp status` reserved leaf
     to clear the missing-status bucket. Likely thinnest possible
     impl: print binary version, kit version, index DB readiness, and
     the same `doctor`-style summary in JSON when `--format=json`.

### Gotchas

- **No `DisableValidate` to remove**: usp has zero opt-out today, so
  any agent that wants to land annotations incrementally must keep
  the binary buildable but cannot run `./usp <anything>` until every
  bucket above is empty. Suggest: agents test their slice via
  `go test ./...` calling `kitconformance.AssertCLI(t, root)`
  directly, not via the built binary.
- **Kit-owned alias subtree** is the highest-friction surface — fix
  upstream in `kit/hops/12fcc-leak` is the clean path; downstream
  monkey-patch in usp is the unblock-locally path. Group C carries
  the kit ticket.
- **`--quiet` collision** on `usp upgrade` (line 37) is silent today
  — cobra accepts the redeclaration. Easy to miss; flagged here so
  Group B does not leave it in.
- **`cmd/usp/upgrade.go` `--auto` flag**: not a kit reserved global,
  no collision, but consider whether `--confirm=yes` should replace
  `--auto` semantically when wiring upgrade into kit's policy
  middleware later.
- **`syscall.Exec` in resume**: kit's policy middleware never sees
  the post-handoff process. Whatever annotation usp picks for `usp
  resume` (likely `side-effect=interactive`, `idempotent=no`) is
  cosmetic past the exec boundary.
- **`api-version` / `--api-version` filtering** runs before
  validation but after `AutoRegisterFlags`. None of usp's commands
  carry `kit/min-api-version` today so this is a no-op for the
  baseline.
