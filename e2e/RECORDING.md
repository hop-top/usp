# E2E Recording Guide

Author: $USER

Step-by-step instructions for recording xrr cassettes
against real CLI endpoints.

## Prerequisites

- Docker (buildx enabled)
- API keys for all 4 CLI providers (env or `.env`)
- Go 1.22+ (for replay verification)

## 1. Build Docker Images

All combos:

```sh
bash e2e/docker/build.sh
```

Single combo:

```sh
bash e2e/docker/build.sh latest
```

Images tagged `usp-e2e:<combo>`.

## 2. Record Per Combo

```sh
bash e2e/docker/run-scenario.sh <combo> <scenario>
```

Examples:

```sh
bash e2e/docker/run-scenario.sh latest single_session
bash e2e/docker/run-scenario.sh mixed cross_resume
bash e2e/docker/run-scenario.sh oldest filter_test
```

Container runs scenario against live APIs; xrr writes
cassettes to `e2e/cassettes/<combo>/<scenario>/`.

## 3. Verify Cassettes

```sh
ls e2e/cassettes/latest/single_session/
# expect: exec-*.req.yaml + exec-*.resp.yaml pairs
```

Each interaction produces one `.req.yaml` + `.resp.yaml`
pair. Missing pairs = failed recording.

## 4. Replay Check

```sh
XRR_MODE=replay go test ./e2e/replay/... -v
```

All tests must pass without network access.

## 5. Commit

```sh
git add e2e/cassettes/
git commit -m "test(e2e): re-record cassettes"
```

## 6. CI Auto-Recording

Weekly workflow: `.github/workflows/e2e-record.yml`

- Builds all combos
- Records all scenarios
- Opens PR with updated cassettes if diff detected

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Missing .resp.yaml | API call failed; check keys |
| Fingerprint mismatch on replay | CLI output changed; re-record |
| Docker build fails | Check base image tags in matrix.yaml |
