# AGENTS.md

> Canonical guidance for anyone — human or AI — working **on** this repository.
> `CLAUDE.md` is a symlink to this file so there is one source of truth.
>
> Working *with* kdbx (using the CLI) is documented in [README.md](README.md). This file is
> about changing it.

## What this is

`github.com/yarrasys/kdbx` — a single Go binary (MIT) that stores a project's secrets in
per-project, per-environment KeePassXC vaults and injects them into child processes without
ever printing them. It is a port of the Python `kdbx` skill in
[`yarrasys/extensions`](https://github.com/yarrasys/extensions/tree/main/skills/kdbx), which
is now the frozen reference implementation.

## Current status — read this first

**Released and consumed.** The repo is public at <https://github.com/yarrasys/kdbx>, CI is
green on Linux, macOS and Windows, and the current release is **v0.1.2**. As of 2026-07-30:

- 19 packages, full suite green under `-race`, cross-compiles to darwin/linux/windows ×
  amd64/arm64. 8 CLI-contract scenarios pass.
- 15 commands: `init set get list delete mv run export import check envs rekey guard mcp
  completion`. `install-launcher` was removed — the binary is its own launcher.
- Vaults are standard KDBX4 (Argon2, key-file-only), so `keepassxc-cli` and the KeePassXC
  desktop app read Go-written vaults directly. The Python↔Go parity harness that verified the
  original port has been retired now that the Python reference is no longer a consumer.
- `golangci-lint` v2 runs clean locally and in CI. `main` is protected: the `lint` and
  three-OS `test` checks are required (admins may still push directly).
- **Every install route is live**: the curl installer, `go install`, `brew install
  yarrasys/tap/kdbx` (v0.1.1) and `ghcr.io/yarrasys/kdbx` (v0.1.2). Releases ship six
  archives plus `SHA256SUMS` and a keyless cosign signature — see [SECURITY.md](SECURITY.md).
- **The consuming change has landed**: `yarrasys/extensions` merged the skill-and-plugin
  switch to the binary in PR #36, freezing its Python implementation.

**Not done — this needs a human decision, so do not do it unprompted:**

1. **The normative design spec is published nowhere.** Golden rule 6 defers to
   `docs/superpowers/specs/2026-07-24-kdbx-go-standalone-design.md` in `yarrasys/extensions`,
   but that file exists only on a local `design/kdbx-go-standalone` branch that is on no
   remote. Until it is pushed — or moved into this repo's `docs/`, which is arguably where a
   contract governing this code belongs — the document this repo calls normative is
   unreachable to everyone but its author. Treat `docs/spike-notes.md`, the README and the
   `.txtar` contracts as the reachable statement of behavior in the meantime.

A dev build is installed at `~/.local/bin/kdbx` (`0.1.2-dev`), replacing the old Python
`install-launcher` shim, which has been deleted.

## Golden rules

**1. The engine boundary is absolute.** Only `internal/vault/` may import
`github.com/tobischo/gokeepasslib`. Every other package deals in plain types — paths,
strings, `[]byte`. This keeps the KDBX engine a single swap point instead of a decision
baked into thirty files. `internal/boundary_test.go` enforces it, including test and
external-test imports, so a violation is a red test rather than a review argument. If you
find yourself wanting engine types elsewhere, widen `internal/vault`'s API instead.

**2. Never author or observe a secret value.** This is the product's entire premise, and it
binds the code and whoever writes it:

- Secret intake is stdin, `--from-env`, or an interactive no-echo prompt. **Never argv.**
- A secret value must never reach stdout without `--reveal`/`--clip`, never reach stderr,
  never reach a log line, an error string, a JSON envelope, or a test failure message.
- Errors are scrubbed by construction: one stderr line, `kdbx: <op> failed: <Kind>`, no
  stack, no payload. `KDBX_DEBUG=1` is the only escape hatch, and it is opt-in.
- Tests build throwaway vaults in `t.TempDir()` with fake values. **Never** commit a
  `.kdbx`, `.keyx`, `.key` or `.env` file — `.gitignore` blocks them, don't work around it.
- If you are an agent: you may run `run`, `get` (masked), `list`, `check`, `envs`, `init`.
  You may not run `set`, `delete`, `mv`, `import`, `rekey`, `export`, or
  `get --reveal`/`--clip` — those are the human's, in the human's terminal. This is the
  same roles contract `kdbx guard` enforces, and it applies to you while you are editing
  the tool as much as while you are using it.

**3. TDD, always.** Write the failing test first, watch it fail for the right reason, then
make it pass. Every package is testable without a TTY and without network access; keep it
that way. CLI-level behavior (stdout, stderr, exit code) belongs in a `testdata/script/*.txtar`
testscript, not in a hand-rolled `exec.Command` test.

**4. Every error carries an exit code.** Command functions return `*kdbxerr.Error` (via
`kdbxerr.New*`/`Wrap`), never a bare `error`. The code and the stable kind name are part of
the public contract; a bare `error` silently becomes exit 1 and is a review failure.

**5. Clean formatting and lint, no exceptions.** `gofmt -l .` prints nothing, `go vet ./...`
is clean, `golangci-lint run` is clean. CI gates all three on Linux, macOS and Windows.

**6. The compatibility contract is normative, and it lives in the spec.** The design spec
(`docs/superpowers/specs/2026-07-24-kdbx-go-standalone-design.md` in `yarrasys/extensions`)
sections **C** (compatibility) and **N** (new surfaces) define pointer discovery, the entry
path grammar, the vault and key-file formats, the twelve operations' observable behavior,
the exit-code table, error scrubbing, permissions, and locking. **Changing observable
behavior means updating the spec in the same change** — the README and the `.txtar` contracts
descend from it. The known deliberate divergences from the original Python behavior are
recorded in [`docs/spike-notes.md`](docs/spike-notes.md) and the README's compatibility table.
Undocumented divergence is a bug, documented divergence is a decision.

## Repository map

| Path | Responsibility |
|------|----------------|
| `main.go` | calls `cmd.Execute()` and exits with its code — nothing else |
| `cmd/` | cobra wiring, one file per operation. Flag plumbing only; **no business logic** |
| `internal/vault/` | **engine boundary** — open/get/list/set/trash/purge/move/rekey/create |
| `internal/keyfile/` | mint and validate the KeePass XML v2 key file |
| `internal/pointer/` | `.keepassxc.json` discovery, schema, var-map edits, entry-path grammar |
| `internal/ojson/` | order-preserving JSON, so pointer rewrites stay diff-minimal |
| `internal/paths/` | KeePassXC dir resolution, `${KEEPASSXC_DIR}`/`~` expansion, sync-root detection |
| `internal/envctx/` | resolved environment context and the `ACTIVE ENV:` banner |
| `internal/secretio/` | secret intake, MASK, confirmation prompts, permissions, atomic writes, clipboard |
| `internal/locking/` | advisory lock plus SHA-256 capture/verify |
| `internal/dotenv/` | dotenv render and parse (hand-rolled — see spike notes for why) |
| `internal/shlex/` | POSIX-ish command-line splitting, used by the guard to read a hook payload |
| `internal/vaultvars/` | the single var-resolution implementation, shared by `run` and `export` |
| `internal/runner/` | PATH/PATHEXT lookup, env injection, spawn, signal forwarding, exit passthrough |
| `internal/kdbxerr/` | typed errors carrying exit codes and stable kind names |
| `internal/jsonout/` | `--json` envelopes |
| `internal/guard/` | `PreToolUse` decision function plus its stdin/stdout shell |
| `internal/mcpserver/` | stdio MCP server, five read-only tools |
| `testdata/script/*.txtar` | testscript CLI-contract tests |
| `docs/spike-notes.md` | verified engine facts, every engine upgrade, and every deliberate divergence, with reasons |
| `.goreleaser.yaml`, `install.sh`, `Dockerfile`, `.github/workflows/` | release engineering |
| `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` | the human on-ramp; both defer to this file for the rules |
| `.github/dependabot.yml` | weekly dependency PRs; the KDBX engine is deliberately ungrouped |

## Build and test

```sh
go build ./...
go test ./... -race          # the whole suite; no network, no TTY needed
go vet ./...
gofmt -l .                   # must print nothing
golangci-lint run

go test . -run TestScripts                          # CLI contract tests only (testdata/script)
```

Release pipeline, validated without publishing anything:

```sh
goreleaser check
goreleaser build --snapshot --clean --single-target
sh -n install.sh
```

Go 1.25 or newer is required to build. Dependencies stay permissive — MIT, BSD or
Apache-2.0 only, no copyleft (see [NOTICE](NOTICE)). Adding a dependency means updating
NOTICE in the same change.

## Before you open a PR

- [ ] a failing test came first, and the full suite is green
- [ ] `gofmt`, `go vet` and `golangci-lint` are clean
- [ ] every new error path returns a `*kdbxerr.Error` with a deliberate exit code
- [ ] no secret value can reach argv, stdout, stderr, a log, JSON output, or an error string
- [ ] observable behavior changes are reflected in the spec, the README and the `.txtar` contracts
- [ ] a new dependency is recorded in `NOTICE` with its license
- [ ] a `gokeepasslib` bump was re-read for format changes, re-verified against
      `keepassxc-cli`, and recorded under "Engine upgrades" in `docs/spike-notes.md`
- [ ] the change is described in `CHANGELOG.md` under `## [Unreleased]`
- [ ] no vault, key file, or `.env` is in the diff

`main` is protected: CI must be green on Linux, macOS **and** Windows before anything merges.

## Related

- [README.md](README.md) — the user-facing contract
- [SECURITY.md](SECURITY.md) — threat model and release verification
- [`docs/spike-notes.md`](docs/spike-notes.md) — engine facts and accepted divergences
- [`yarrasys/extensions`](https://github.com/yarrasys/extensions) — the frozen Python
  reference implementation, the Claude Code plugin, and the design spec
