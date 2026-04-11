# E2E Cassettes

Author: $USER

Recorded CLI interactions for deterministic replay testing.
Uses [xrr](https://github.com/ideacrafterslabs/xrr) cassette
format v1.

## What Are Cassettes?

YAML file pairs capturing real CLI invocations:

```
<adapter>-<fingerprint>.req.yaml   # serialized request
<adapter>-<fingerprint>.resp.yaml  # serialized response
```

Fingerprint = `sha256(canonical(request))[:8]` (8 hex chars).
Envelope fields: `xrr`, `adapter`, `fingerprint`,
`recorded_at`, `payload`.

## Directory Layout

```
e2e/cassettes/
  latest/                # all 4 CLIs at latest version
    single_session/      # one CLI, one session lifecycle
    cross_resume/        # resume across CLIs
    filter_test/         # --since/--tool/--project filters
  mixed/                 # mixed CLI versions
    ...
  oldest/                # all 4 CLIs at oldest supported
    ...
```

## Recording

Build Docker image, then run scenario against real APIs:

```sh
bash e2e/docker/build.sh latest
bash e2e/docker/run-scenario.sh latest single_session
```

Cassettes land in `e2e/cassettes/<combo>/<scenario>/`.

## Replay

Replay test package planned for Phase 3. Once available:

```sh
XRR_MODE=replay go test ./e2e/replay/...
```

No network calls; xrr serves from cassette files.

## When to Re-record

- CLI version bump (any of the 4 providers)
- Scenario logic changes
- Weekly CI refresh (Phase 4: automated workflow planned)

## Rules

- Do NOT manually edit cassette YAML files
- Always commit cassettes: `git add e2e/cassettes/`
- Review diffs carefully; cassettes contain real output
