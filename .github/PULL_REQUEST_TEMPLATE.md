<!--
  Read AGENTS.md before opening a PR — it carries the engine boundary, the
  secret-hygiene invariants, and the test discipline this checklist enforces.
-->

## What & why

<!-- One or two sentences: what changes, and what it fixes or enables. Link any issue. -->

## Checklist

- [ ] A failing test came first, and the full suite is green (`go test ./... -race`)
- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `golangci-lint run` are clean
- [ ] Every new error path returns a `*kdbxerr.Error` with a deliberate exit code
- [ ] No secret value can reach argv, stdout, stderr, a log, JSON output, or an error string
- [ ] Observable-behavior changes are reflected in the spec, the README and the `.txtar` contracts
- [ ] A new dependency is recorded in `NOTICE` with its license
- [ ] The change is described in `CHANGELOG.md` under `## [Unreleased]`
- [ ] No vault, key file, or `.env` is in the diff
