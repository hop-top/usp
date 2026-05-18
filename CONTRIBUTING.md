# Contributing to usp

Thanks for considering a contribution. This page is organized by
what you came here to do.

## I found a bug

[Open an issue](https://github.com/hop-top/usp/issues/new) with:

- What you ran (exact command + flags)
- What you expected
- What happened instead (paste output verbatim — including any
  error)
- `usp --version` output
- Your OS + Go version (`go version`)

If the bug touches a specific AI CLI's session format, include
that CLI's name and version too.

For **security** issues, do not open a public issue — see
[SECURITY.md](SECURITY.md).

## I want to propose a change

For anything beyond a typo or one-liner, open an issue first so we
can agree on direction before you spend time. Tiny fixes can go
straight to a PR.

When you open the PR:

- Reference the related issue
- Keep the PR focused on one concern
- Use [Conventional Commits](https://conventionalcommits.org) for
  the commit message — `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`, etc. `release-please` generates the changelog
  from these.
- Update `CHANGELOG.md` under `## Unreleased` if your change is
  user-visible
- Ensure CI passes (build, tests, lint)

## I want to set up a dev environment

```sh
git clone https://github.com/hop-top/usp.git
cd usp
make build test    # build + test (matches what blocking CI gates on)
```

See [`make help`](Makefile) for the full target list. The most
common ones:

| Target | What it does |
|---|---|
| `make build` | Build `usp` + `usp-ctxt` into `bin/` |
| `make test` | Run all Go tests |
| `make lint` | `go vet`, plus `golangci-lint` if installed |
| `make check` | `build` + `test` + `lint` |
| `make tidy` | `go mod tidy` |
| `make install` | Build + install to `$XDG_BIN_HOME` (default `~/.local/bin`) |

You'll need **Go 1.26+**. `golangci-lint` is optional but
recommended — install per
<https://golangci-lint.run/welcome/install/> to surface the same
findings CI reports as advisory. Note: usp currently carries 26
pre-existing lint findings (errcheck, misspell, staticcheck,
gofmt, ineffassign); `make check` will fail until those are
cleared — use `make build test` until then.

## I want to understand how usp is organized

Top-level layout:

- `cmd/usp/` — the main CLI binary
- `cmd/usp-ctxt/` — the ctxt bridge binary
- `adapters/` — per-CLI session readers (Claude, Codex, Gemini,
  OpenCode)
- `session/` — the normalized session envelope
- `index/`, `lineage/` — search + cross-CLI continuation tracking
- `internal/` — package-private helpers
- `e2e/` — Docker-based end-to-end test harness

Conventions usp inherits from [hop.top/kit](https://github.com/hop-top/kit):
CLI shape, output formats, config layering, exit codes. The CLI
spec lives in [docs/usp/api-cli.md](docs/usp/api-cli.md).

## Code of Conduct

By participating, you agree to abide by the
[Code of Conduct](CODE_OF_CONDUCT.md).
