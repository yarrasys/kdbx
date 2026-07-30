# Contributing to kdbx

Thanks for considering it. This file is the short path in; **[AGENTS.md](AGENTS.md)** (symlinked
as `CLAUDE.md`) is the authority on how this repository works, and reading it before you open a
PR will save you a review round.

## Before anything else: the two rules that get PRs rejected

1. **Never author or observe a secret value.** kdbx exists so that secrets never appear in a
   terminal, a log, or a process argument. A secret must not reach argv, stdout without
   `--reveal`/`--clip`, stderr, a log line, an error string, a JSON envelope, or a test failure
   message. Tests build throwaway vaults in `t.TempDir()` with fake values. **Never commit a
   `.kdbx`, `.keyx`, `.key` or `.env` file** — `.gitignore` blocks them; don't work around it.
2. **The engine boundary is absolute.** Only `internal/vault/` may import
   `github.com/tobischo/gokeepasslib`. Everything else deals in plain paths, strings and
   `[]byte`. `internal/boundary_test.go` enforces this, so a violation is a red test rather than
   a review argument.

## Setup

You need **Go 1.25 or newer** and, to match CI, **golangci-lint v2.12.2**:

```sh
git clone https://github.com/yarrasys/kdbx && cd kdbx
go build ./...
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
```

No network access and no TTY are needed to build or test. KeePassXC itself is not required —
handy for cross-checking a vault by hand, but nothing in the suite depends on it.

## The loop

```sh
go test ./... -race          # the whole suite
go test . -run TestScripts   # CLI-contract tests only (testdata/script/*.txtar)
go vet ./...
gofmt -l .                   # must print nothing
golangci-lint run
```

Write the failing test first and watch it fail for the right reason. CLI-level behavior — stdout,
stderr, exit code — belongs in a `testdata/script/*.txtar` testscript, not a hand-rolled
`exec.Command` test. Every error path returns a `*kdbxerr.Error` carrying a deliberate exit code;
a bare `error` silently becomes exit 1 and is a review failure.

## Opening the PR

The [pull request template](.github/PULL_REQUEST_TEMPLATE.md) carries the full checklist. In
short: a failing test came first and the suite is green, formatting and lint are clean, no secret
can escape, a new dependency is recorded in [NOTICE](NOTICE) with its license, and the change is
described in `CHANGELOG.md` under `## [Unreleased]`.

Observable behavior — flags, output, exit codes, on-disk formats — is a contract. Changing it
means updating the design spec, the README and the affected `.txtar` files in the same change.
Deliberate divergences get recorded in [docs/spike-notes.md](docs/spike-notes.md) with a reason;
an undocumented divergence is a bug.

`main` is protected. CI must be green on Linux, macOS **and** Windows before anything merges.

## Reporting things instead

Bugs and feature ideas go to [issues](https://github.com/yarrasys/kdbx/issues/new/choose) —
include `kdbx --version` and your platform, and redact everything. Security vulnerabilities go
privately through [GitHub Security Advisories](https://github.com/yarrasys/kdbx/security/advisories/new),
never a public issue; see [SECURITY.md](SECURITY.md).

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
