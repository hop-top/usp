# usp + kit 12fcc-leak: --confirm policy compatibility

Migration note for the `refactor/kit-12fcc-integration` cycle
(T-0122). Captures usp's current side-effect annotations, how each
leaf interacts with kit's global `--confirm` policy middleware, and
the compatibility story for the few places where a local flag still
exists.

## TL;DR

- **usp ships zero `destructive*` leaves today.** Kit's `gateConfirm`
  matrix (refuse on `--confirm=no` / non-TTY default; prompt on
  TTY; require `--confirm=yes` or `--confirm=auto` to proceed) only
  fires for `kit/side-effect ∈ {destructive, destructive-local,
  destructive-shared}`. Every mutating leaf in usp is annotated
  `write-local`, which kit explicitly excludes from the matrix.
- **`usp alias delete` is `write-local`, not `destructive-local`.**
  Alias entries live in `$XDG_CONFIG_HOME/usp/aliases.yaml` and are
  trivially recoverable by re-running `usp alias add`; they do not
  meet the irreversible-loss threshold for `destructive*`. The
  annotation is owned upstream by kit's `Root.AliasCmd` (kit/go/console/cli/alias_cmd.go:152).
- **`usp upgrade` keeps a local `--auto` flag as the bridge to
  `--confirm=yes`.** The leaf is `write-local` (binary swap is
  reversible by reinstall) so kit's confirm gate does not fire.
  `--auto` controls the interactive prompt inside kit's own
  `upgrade.RunCLI`, which is *separate* from `cli.gateConfirm` and
  accepts a third "snooze" answer that `--confirm` cannot express.

## Audit (T-0122)

| Leaf                | Side-effect   | Idempotent | Local force/yes? | Action |
|---------------------|---------------|------------|------------------|--------|
| `usp doctor`        | read          | yes        | —                | —      |
| `usp mcp`           | interactive   | no         | —                | —      |
| `usp resume`        | interactive   | no         | —                | —      |
| `usp session list`  | read          | yes        | —                | —      |
| `usp session search`| read          | yes        | —                | —      |
| `usp session show`  | read          | yes        | —                | —      |
| `usp session lineage` | read        | yes        | —                | —      |
| `usp session skills`| read          | yes        | —                | —      |
| `usp session tools` | read          | yes        | —                | —      |
| `usp setup`         | write-local   | no         | —                | —      |
| `usp upgrade`       | write-local   | no         | `--auto`         | bridge |
| `usp version`       | read          | yes        | —                | —      |
| `usp status`        | read          | yes        | —                | —      |
| `usp config path`   | read          | yes        | —                | —      |
| `usp config paths`  | read          | yes        | —                | —      |
| `usp alias`         | read (group)  | yes        | —                | —      |
| `usp alias list`    | read          | yes        | —                | —      |
| `usp alias add`     | write-local   | no         | —                | —      |
| `usp alias delete`  | write-local   | yes        | —                | —      |

Annotations were verified by `grep -rn SetSideEffect cmd/usp/
internal/ adapters/`. The kit-owned alias subtree is annotated in
`kit/go/console/cli/alias_cmd.go` (12fcc-leak).

## Interaction matrix with kit's `--confirm`

Kit's `gateConfirm` (kit/go/console/cli/policy_runE.go:311) reads
the resolved confirm mode and the leaf's side-effect, then:

| Side-effect class              | `--confirm=no` (or non-TTY default) | `--confirm=prompt` (or TTY default) | `--confirm=yes` / `--confirm=auto` |
|--------------------------------|-------------------------------------|-------------------------------------|------------------------------------|
| `read`                         | run                                 | run                                 | run                                |
| `write` / `write-local` / `write-shared` | run                       | run                                 | run                                |
| `destructive*`                 | **refuse (UNAUTHORIZED)**           | prompt; refuse on decline           | run                                |
| `interactive`                  | run                                 | run                                 | run                                |

Important: write-tier leaves are **never** gated by kit's confirm
matrix. Any prompt they raise is owned by the leaf's RunE (e.g. kit's
`upgrade.RunCLI` raises a y/N/snooze prompt for `usp upgrade`).

## Bridges and deprecations

### `usp upgrade --auto` (bridge, kept)

- Status: **bridge**, not deprecated.
- Why kept: kit's `upgrade.RunCLI` prompts y/N/**snooze**. The third
  answer has no `--confirm` equivalent (which only encodes
  auto|yes|no|prompt). Dropping `--auto` would lose the snooze path,
  which is a documented kit UX.
- Compatibility: `--auto` is announced in the leaf's help as
  "equivalent to `--confirm=yes` for this command". CI users can
  use either form; they pin different code paths but produce the
  same outcome (install the latest release without prompting).

### `usp alias delete` (no bridge)

- Status: **no flag added**.
- Why: alias deletion is `write-local` and the user can reverse it
  in one command (`usp alias add <name> <target>`). Kit's confirm
  gate is therefore not engaged and no opt-out flag is required.
- Compatibility: future scripts can pass `--confirm=yes` without
  changing behavior — it's accepted by kit's flag registration and
  ignored by the (non-firing) confirm gate.

### `usp setup` (no bridge)

- Status: **no flag added**.
- Why: setup re-seeds the local index DB; the most "destructive"
  action it takes is rewriting rows that re-running setup would
  re-create. Annotation is `write-local` and the gate does not
  fire.
- Compatibility: same as alias delete — `--confirm` flags accepted,
  inert.

## Sentinels

These invariants are encoded in
`cmd/usp/confirm_policy_test.go` so future drift surfaces in CI
rather than at runtime:

- `TestConfirmPolicy_NoDestructiveLeaves` — fails if anybody adds a
  `destructive*` leaf without a documented bridge.
- `TestConfirmPolicy_AliasDeleteIsWriteLocal` — fails if kit
  reclassifies `alias delete` upstream.
- `TestConfirmPolicy_WriteLocalLeavesNotGated` — pins `setup`,
  `upgrade`, and `alias delete` at `write-local`.
- `TestUpgradeCmd_AutoFlagBridge` — fails if the `--auto`/`--confirm`
  bridge note is dropped from the leaf's Long help or flag usage.

## References

- ADR-0021 (kit) — expanded side-effect ladder.
- `cli-conventions-with-kit.md` §3.5 — reserved annotations under `kit/`.
- `cli-conventions-with-kit.md` §8.6 — policy globals (`--confirm`,
  `--max-ops`, `--policy`, `--confirm-token`).
- `kit/go/console/cli/policy_runE.go` — gate implementation.
- `kit/go/console/cli/sideeffect.go` — class definitions +
  `isDestructiveLike`.
