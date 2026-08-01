# kdbx v1 — standalone Go binary — design

**Date:** 2026-07-24 · **Status:** implemented; normative for compatibility · **Owner:** nabsha
**Builds on:** the Python reference implementation (`skills/kdbx/`) and its docs, in the
now-archived [`yarrasys/extensions`](https://github.com/yarrasys/extensions).

> **Provenance.** Authored in `yarrasys/extensions` on 2026-07-24 and moved here on 2026-07-30,
> when that repo was archived and stopped accepting pushes. This is the **normative
> compatibility contract** that [AGENTS.md](../AGENTS.md) golden rule 6 defers to: sections
> **C** (compatibility) and **N** (new surfaces) define pointer discovery, the entry path
> grammar, the vault and key-file formats, the operations' observable behavior, the exit-code
> table, error scrubbing, permissions, and locking. **Changing observable behavior means
> updating this file in the same change.**
>
> The body is preserved as authored, so it describes intent at design time — a few things
> landed differently on purpose. Where this document and the shipped binary disagree, the
> deliberate divergences are the ones recorded in [spike-notes.md](spike-notes.md); anything
> else is a bug. The task-by-task plan is kept, unexecuted, as
> [history](history/2026-07-24-implementation-plan.md).

## Summary

Reimplement kdbx as a **standalone Go static binary** in a new repo
(`github.com/yarrasys/kdbx`, MIT). The binary is the product; the Claude Code skill and
plugin become thin consumers of it. Distribution is conventional (curl installer, Homebrew
tap, `go install`, GitHub release binaries, container image) instead of "install a skill,
then symlink". The existing Python implementation becomes the frozen **reference
implementation** that the port is verified against, and is deprecated once Go v1.0 ships.

**Motivation.** kdbx is the author's most-used command, but shipping it as a skill/plugin
caps adoption: users must install a skill, deal with skill-dir paths or a launcher shim, and
have `uv`. The actual usage pattern — `kdbx run -- claude` from any project with a
`.keepassxc.json` — is a plain CLI pattern and deserves a plain CLI distribution. Target
audiences (all): the author across machines, team/org colleagues, public OSS users, and
agents/CI images.

## Goals

1. `kdbx` on PATH via one conventional command (curl installer / brew / go install), no uv,
   no Python, no skill installation.
2. **100 % artifact compatibility** with the Python implementation: existing vaults,
   keyfiles, and `.keepassxc.json` pointers work unchanged, in both directions, with zero
   migration.
3. Behavioral compatibility: same ops, flags, exit codes, stdout/stderr contracts, and
   security invariants (documented below as the normative contract).
4. First-class integration surfaces built into the binary: `--json` output, `kdbx mcp`
   (stdio MCP server), `kdbx guard` (PreToolUse hook engine), shell completions.
5. MIT-clean dependency tree (no GPL) — enables brew/bundling without the pykeepass
   runtime-fetch workaround.
6. Signed, checksummed releases (cosign keyless + SHA256SUMS) — this tool opens password
   vaults; supply-chain trust is a feature.

## Non-goals

- No daemon/session mode (ssh-agent style) — `run` is per-invocation by design.
- No claude.ai web support — the keyfile-possession boundary makes kdbx a local-machine
  tool by design.
- No alternative backends (1Password, keychain, …) — KDBX4 only.
- Claude Desktop `.mcpb` extension bundle, macOS notarization, Windows code signing —
  explicitly *post-v1 follow-ups*, not in this cycle.
- No new vault features (attachments, TOTP, merge) — parity plus the listed new surfaces.

## Decision record

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Go** (≥ 1.25), not Rust | `gokeepasslib/v3` (MIT) has mature KDBX4 **write** support; `keepass-rs` write path is younger. Vault-write correctness is the dominant risk. Go also has an official MCP SDK. |
| D2 | Engine: `github.com/tobischo/gokeepasslib/v3`, imported **only** in `internal/vault/` | Preserves the Python codebase's engine-boundary rule (sole-importer module = single swap point). |
| D3 | New repo `github.com/yarrasys/kdbx`, `main` package at repo root, MIT | `go install github.com/yarrasys/kdbx@latest` works; releases/issues/identity decoupled from the skills monorepo (precedent: yarradev open-core split). |
| D4 | CLI framework: `spf13/cobra` | Subcommands, `--help` quality, and shell completions for free; Apache-2.0 (MIT-compatible). |
| D5 | **Drop `install-launcher`** | The binary *is* the launcher. No replacement op; op count goes 13 → 12 (+ new surfaces). |
| D6 | Keyfile handling is **our own code** (`internal/keyfile/`), not the engine's | We mint the KeePass XML v2 keyfile format ourselves today (see contract §C4); parsing it ourselves and handing raw key bytes to gokeepasslib removes a library-capability risk. |
| D7 | Python implementation is **frozen as the reference**: bug-fix only, drives the parity harness, deprecated at Go v1.0 | Two sources of truth is a bug factory; but the reference is needed to verify the port. |
| D8 | Plugin v2 (extensions repo) requires the binary; `guard.py` and `mcp/server.py` are retired in favor of `kdbx guard` / `kdbx mcp` | Removes uv from the hook hot path (fires on every Bash call) and deletes two lockfiles. |
| D9 | Release engineering: GoReleaser → GitHub Releases (darwin/linux/windows × amd64/arm64), `install.sh` (curl), Homebrew tap `yarrasys/homebrew-tap`, `ghcr.io/yarrasys/kdbx` (FROM scratch), SHA256SUMS + cosign keyless | Standard, automated from a tag. |
| D10 | Clipboard via the same external commands Python uses (`pbcopy` / `powershell Set-Clipboard` / `wl-copy` / `xclip`), auto-clear via detached self-re-exec (`kdbx internal-clear-clip`) | Zero extra deps; identical observable behavior; no cgo. |

## C. Compatibility contract (normative)

This section is the behavioral spec the Go implementation MUST satisfy. Source of truth for
ambiguity: the Python implementation (`skills/kdbx/kdbx_core/`) and `SKILL.md`. Where the
two disagree, the **documented** contract wins (one known case: exit 3, see §C6).

### C1. Pointer file — discovery, schema, rewriting

- Name `.keepassxc.json`; discovery walks up from cwd to filesystem root, first hit wins.
  No pointer → exit 2 with `no .keepassxc.json found (run from inside a configured repo)`.
- Schema (unchanged): `project` (string, optional — defaults to pointer-dir name),
  `defaultEnv` (string, optional — defaults to `"dev"`), `envs.<name>` with optional
  `vault`, `keyFile` (paths; support `${KEEPASSXC_DIR}` token, then `~` expansion, then
  absolutize) and optional `vars` (map ENV_VAR → entry path). Optional
  `run.allow` (array of command strings): when present, `run` only injects for an argv that
  exactly equals one of the entries after shell-splitting (no prefix matching — a `pytest`
  prefix would admit `pytest --pdb`, a REPL holding the secrets). Absent = no restriction;
  present-but-empty = nothing runs without `--any`; present-but-malformed = Preflight error
  (fail closed). `--any` bypasses the list and is denied to agents by the guard (N3).
- Default paths when `vault`/`keyFile` are omitted:
  `<keepassxc-dir>/<project>/<env>.kdbx` and `<env>.keyx`, where keepassxc-dir =
  `$KEEPASSXC_DIR` if set, else `%LOCALAPPDATA%\keepassxc` (Windows), else
  `$XDG_CONFIG_HOME/keepassxc` or `~/.config/keepassxc`.
- Env selection precedence: `--env` > `$KDBX_ENV` > pointer `defaultEnv` > `"dev"`; the
  selected source string (`--env` / `$KDBX_ENV` / `pointer`) is reported by `envs` and the
  banner. Unknown env → exit 2 (`env '<e>' not configured in pointer`).
- **Rewrites** (`set --var`, `mv` repointing, `import`) MUST be atomic
  (`.tmp` + rename), 2-space-indented JSON + trailing newline, and **preserve existing key
  order** (diff-minimal — the file is committed). ⚠️ Go `map[string]any` does not preserve
  order: use an order-preserving JSON representation (ordered decode or surgical edit).
  Each rewrite prints `modified tracked file .keepassxc.json — review and commit` (wording
  per op, see C7) to stderr.

### C2. Entry path grammar

`group/sub/Title[:field]` → (group path, title, field). Field defaults to `password`.
Reserved fields (case-insensitive, map to native entry attributes): `title`, `username`,
`password`, `url`, `notes`; any other field is a **protected custom property**. Reject:
more than one `:`, empty field after `:`, any empty `/`-segment. Var names registered with
`--var` must match `^[A-Z_][A-Z0-9_]*$` (violation → stderr message + exit 7).

### C3. Vault format

KDBX **4** + **Argon2** KDF, key-file-only credentials (no password). `init` refuses to
overwrite an existing vault or keyfile; creates parent dirs; restricts perms (C8). Vault
save is crash-safe: write `.tmp` → restrict perms → rename existing vault to `.bak` →
rename `.tmp` into place → restrict perms → delete `.bak`. Trash goes to the standard
KeePassXC Recycle Bin (create group + set meta RecycleBinUUID on first use; `list` excludes
recycled entries; `purge` finds entries in trash too).

### C4. Keyfile format

KeePass XML keyfile **v2**, minted by us:
32 random bytes; hex-uppercase in `<Data Hash="...">`, where Hash = first 4 bytes of
SHA-256(key), hex-uppercase; exact XML shape as `generate_keyfile_xml` in
`skills/kdbx/kdbx_core/vault.py`. Mint refuses to overwrite; file written 0600 atomically.
Go parses v2 (and accepts raw-32-byte/hex keyfiles for robustness) in `internal/keyfile/`
and passes derived key bytes to the engine.

### C5. Operations (12) — flags and observable behavior

All ops accept `--env E`. "Banner" = `ACTIVE ENV: <env>  vault=<path>  (source: <src>)` on
**stderr** (ops marked ✦ print it; read-only display ops don't).

| Op | Flags | Behavior (stdout / stderr / exit) |
|----|-------|------------------------------------|
| `init` ✦ | | create vault+keyfile; stderr `created <vault>` + KEYFILE backup warning; exists → refuse (≠0) |
| `set PATH` ✦ | `--var NAME`, `--from-env VAR`, `--raw` | value via `--from-env`, else getpass+confirm (TTY), else stdin (empty → error exit 1; strip one trailing `\n`/`\r\n` unless `--raw`); empty/whitespace value refused; `--var` updates pointer vars map (C1) |
| `get PATH` | `--reveal` \| `--clip` (mutually exclusive) | default: stdout `(set, hidden)` (the MASK constant — no length/prefix leak); `--reveal`: value on stdout + stderr warning; `--clip`: copy + auto-clear ≈15 s, stderr `copied to clipboard (clears shortly)`; entry/field missing → exit 2 |
| `list [GROUP]` | | sorted `group/…/title` lines, prefix-filtered by GROUP, Recycle Bin excluded; never values |
| `delete PATH` ✦ | `--purge` | default soft-delete to Recycle Bin; `--purge` prompts `y/N` (TTY only — non-TTY refuses, exit 4) then removes permanently |
| `mv SRC DST` ✦ | | move/retitle entry (create dest groups); re-point active env's var mappings that reference SRC, preserving `:field` suffix; stderr `re-pointed N var mapping(s) …` when any |
| `run` ✦ | `--allow-missing`, `--no-mask`, `--any`, `-- CMD…` | when the pointer has `run.allow` (C1), refuse an unlisted argv **before opening the vault** — kind `NotAllowed`, exit 7 — unless `--any`; resolve active env's vars map → inject into child env (parent env + overrides); resolve argv[0] via PATH lookup (Windows PATHEXT — the `shutil.which` lesson); spawn, wait, **pass through child exit code**; a child stream that is not a TTY has injected values (≥ 8 bytes) replaced with `***` on the way through — exact-match only, longest-first, chunking-invariant (`internal/maskio`); a TTY stream keeps the raw fd so interactive children are untouched; `--no-mask` disables masking and is denied to agents by the guard (N3); no command → exit 2; unresolved var → exit 5 unless `--allow-missing` |
| `export` ✦ | `--out F`, `--allow-missing` | render vars as dotenv (always double-quoted; escape `\\`, `\"`, `\n`); `--out`: atomic 0600 write + gitignore reminder; else stdout |
| `import FILE` ✦ | | parse dotenv (no interpolation); each KEY stored at `imported/KEY:password` + var mapping; stderr reminder to remove/rotate the source |
| `check` | | per missing mapping: stdout `MISSING VAR -> path`; exit 0 clean / 5 drift |
| `envs` | | stdout one line per env, active marked `* `; stderr `active: <env> (source: <src>)`; no pointer → exit 2 |
| `rekey` ✦ | | prompt `y/N` (TTY only, else exit 4); mint `.new` keyfile, re-key vault, atomic replace, delete old keyfile; stderr rotation reminder |

Removed: `install-launcher` (D5). Global: `--version` → `kdbx <semver>`.

### C6. Exit codes (unchanged)

`0` ok · `1` generic scrubbed failure · `2` not-found (pointer/env/entry/field/no-command)
· `3` locked / keyfile-missing / credential failure · `4` destructive op not confirmed ·
`5` drift (check / unresolved run|export var) · `6` vault changed underneath a write ·
`7` preflight (e.g. invalid `--var` name; also `run` refused by `run.allow`, which carries
its own kind `NotAllowed` so a policy refusal is distinguishable from malformed input). ⚠️ Known Python deviation: some open-failures
surface as 1 instead of the documented 3; **Go implements the documented 3** and the parity
harness asserts documented codes, not incidental Python behavior.

### C7. Error scrubbing & debug

Failures never print secret values or tracebacks: one stderr line
`kdbx: <op> failed: <ErrorKind>` (Go defines a stable kind-name set; parity maps Python
exception class names → kinds) and the mapped exit code. `KDBX_DEBUG=1` additionally prints
the full error/stack to stderr.

### C8. File permissions & secret hygiene

- Vault, keyfile, exported dotenv: 0600 on POSIX; on Windows `icacls` inheritance-stripped
  owner-only ACL. Atomic writes throughout (C1/C3).
- Secret values NEVER appear on argv, in logs, in JSON output, or in error text. Intake is
  stdin / `--from-env` / interactive getpass only.
- Env vars honored: `KDBX_ENV`, `KEEPASSXC_DIR`, `XDG_CONFIG_HOME`, `LOCALAPPDATA`,
  `KDBX_DEBUG`.
- Accepted limitation (same as Python): no guaranteed memory zeroization of secret strings.
- Cloud-sync-location check: Python defines `under_sync_root` (OneDrive/Dropbox/iCloud/…)
  but never wires it in. Go **does** wire it: `init` warns on stderr when the vault or
  keyfile path falls under a sync root (warning only, never a refusal).

### C9. Locking & concurrent-write safety

Every vault **write** takes an advisory lock on `<vault>.lock` (10 s acquisition timeout →
exit 3) and, for read-modify-write ops, captures a SHA-256 of the vault before open and
verifies it before save — mismatch → exit 6 (`vault changed underneath us; re-run`).
Go uses `gofrs/flock`; ⚠️ Python `filelock` and Go `flock` do not inter-lock reliably —
acceptable for the transition (single-user tool), noted in README.

### C10. Roles contract

Unchanged and normative in all docs: **agents read and use; humans write.** Agent-safe:
`run`, `get` (masked), `list`, `check`, `envs`, `init`. Human-only: `set`, `delete`, `mv`,
`import`, `rekey`, `export`, `get --reveal/--clip`. The binary itself does not enforce
roles (possession of the keyfile is the real boundary); `kdbx guard` (N3) enforces it in
harnesses that support hooks.

## N. New surfaces

### N1. `--json` (global flag)

Machine-readable stdout for read ops; secret values are never included.

- `list` → `{"entries":["api/openai", …]}`
- `check` → `{"ok":true,"missing":[]}` / `{"ok":false,"missing":[{"var":"X","path":"api/x:password"}]}` (exit still 5)
- `envs` → `{"envs":[{"name":"dev","active":true},…],"source":"pointer"}`
- `get` (masked only; `--json` + `--reveal` is an error, exit 7) → `{"path":"api/openai:password","set":true}`
- `--version --json` → `{"version":"1.0.0"}`
- On failure with `--json`: stdout `{"error":{"op":"check","exit":5,"kind":"Drift"}}`,
  same stderr line as C7, same exit code.

### N2. `kdbx mcp`

Stdio MCP server (official Go SDK `modelcontextprotocol/go-sdk`), replacing
`plugins/kdbx/mcp/server.py`. Same five read-only tools with the same names and
descriptions as the Python server: list, envs, check, get (masked), run. Works with Claude
Code (`.mcp.json`: `{"command":"kdbx","args":["mcp"]}`), Claude Desktop, and any MCP
client. No write tools — the roles contract (C10) applies to machines too.

### N3. `kdbx guard`

Port of `plugins/kdbx/hooks/guard.py` `decide()` semantics as a built-in: reads PreToolUse
JSON on stdin, prints a `permissionDecision: deny` envelope (same wording) or nothing;
**always exits 0 / fails open**. Blocks (a) agent-issued human-only ops (C10 list, incl.
`get --reveal/--clip`, `run --no-mask` and `run --any` — flags after `run`'s `--` belong to
the child and are ignored), (b) non-kdbx programs touching `*.kdbx`/`*.keyx` or the KeePassXC
config dir, (c) shell writes to the committed pointer file `.keepassxc.json`: output
redirection onto it, in-place editors (`sed -i`, `perl -i`), `tee`, interactive editors,
and `mv`/`cp` with the pointer as destination. Pointer reads stay allowed (the file is
committed and holds no secrets), and kdbx itself remains a recognized invoker so `init`
and pointer rewrites through kdbx are unaffected. Detection is heuristic and fail-open by
design. Recognized invokers: `kdbx`, `kdbx.py`, `keepassxc-cli`, `keepassxc`, uv-run
forms (transition). Plugin v2's `hooks.json` invokes `kdbx guard --hook pretooluse`.

### N4. `kdbx completion [bash|zsh|fish|powershell]`

Cobra-generated completions; documented in README and brew formula.

## Architecture

```
github.com/yarrasys/kdbx
├── main.go                    # thin: cmd.Execute()
├── cmd/                       # cobra wiring, one file per op; no business logic
├── internal/
│   ├── vault/                 # ENGINE BOUNDARY — sole importer of gokeepasslib.
│   │                          #   Public API: plain types only (paths, strings, []byte).
│   ├── keyfile/               # mint/parse XML v2 keyfile (D6)
│   ├── pointer/               # discovery, schema, order-preserving rewrite (C1, C2 grammar)
│   ├── envctx/                # env resolution + banner (C1 precedence)
│   ├── paths/                 # keepassxc dir, ${KEEPASSXC_DIR}/~ expansion, sync-root check
│   ├── secretio/              # stdin/getpass/from-env intake, MASK, confirm, perms,
│   │                          #   atomic writes, clipboard (D10), error scrub (C7)
│   ├── locking/               # flock + sha256 capture/verify (C9)
│   ├── runner/                # run: PATH/PATHEXT lookup, env inject, spawn/wait,
│   │                          #   signal forwarding, exit passthrough
│   ├── dotenv/                # render (exact C5 quoting) + parse (godotenv, no interpolation)
│   ├── jsonout/               # N1 envelopes
│   ├── guard/                 # N3 decide() port (pure function + stdin/stdout shell)
│   └── mcpserver/             # N2
├── interop/                   # uv-run pytest parity harness (dev/CI only, not shipped)
├── testdata/
├── install.sh  .goreleaser.yaml  .github/workflows/{ci,release}.yml
└── README.md  SECURITY.md  NOTICE  CHANGELOG.md  AGENTS.md
```

Rules carried over from the Python codebase: engine boundary (D2); every package testable
without a TTY or network; `guard.decide` and `pointer`/`dotenv` parsing are pure functions.

## Testing strategy

1. **Go unit tests** per package (table-driven), TDD per repo norms.
2. **CLI-level tests** with `rogpeppe/go-internal/testscript`: every op's stdout/stderr/exit
   contract from C5, golden-file style, on all three OSes.
3. **Interop & parity harness** (`interop/`, pytest via uv — dev/CI only):
   - Round-trip: vault created by Go → opened/mutated by pykeepass and (where installed)
     `keepassxc-cli` → re-opened by Go; and the reverse, starting from the frozen Python
     kdbx. Covers: protected custom properties, Recycle Bin semantics, Argon2 params,
     keyfile v2, rekey.
   - Parity: identical scenario scripts run against both binaries; asserts the C5/C6
     contract (documented behavior, not incidental Python quirks).
   - **Coverage-parity checklist**: every existing test in `skills/kdbx/tests/` and
     `plugins/kdbx/tests/` maps to a named Go/interop counterpart; the implementation plan
     enumerates the mapping; no test dropped without a recorded reason.
4. **CI** (GitHub Actions): linux/macos/windows matrix — `go test ./...`, `go vet`,
   `golangci-lint`, `goreleaser check`, interop job (installs uv; `keepassxc-cli` interop
   on linux+macos, skipped on windows), license audit of the module graph (M6).

## Distribution & release engineering

- GoReleaser from tag `v*`: archives for darwin/linux/windows × amd64/arm64,
  `SHA256SUMS` + cosign keyless signatures, Homebrew tap formula
  (`brew install yarrasys/tap/kdbx`), `ghcr.io/yarrasys/kdbx` (FROM scratch, binary only),
  GitHub Release notes from CHANGELOG.
- `install.sh` (curl one-liner) — OS/arch detect, verify checksum, install to
  `~/.local/bin`, PATH hint. Served from the repo (raw URL) and release assets.
- `go install github.com/yarrasys/kdbx@latest` works by construction (D3).
- Versioning: semver from v0.1.0; **v1.0.0 = contract C+N frozen** and parity suite green.
  `--version` embeds version via ldflags.
- NOTICE: full dependency license inventory (all MIT/BSD/Apache-2.0; no GPL anywhere).

## Extensions-repo integration (final milestone)

In `yarrasys/extensions`, after the binary is releasable:

1. `skills/kdbx/SKILL.md` v2: invocation section becomes "requires the `kdbx` binary on
   PATH" + install one-liner; ops table updated (install-launcher removed); roles/security
   sections unchanged. Python entrypoint remains documented in a short "legacy (uv)
   fallback" appendix until v1.0.
2. `plugins/kdbx/` v2: `hooks.json` → `kdbx guard --hook pretooluse`; `.mcp.json` →
   `kdbx mcp`; delete `mcp/server.py` (+lock) and `hooks/guard.py`; README rewritten;
   version bumps in `plugin.json` + `marketplace.json`.
3. Python implementation frozen (D7): CHANGELOG note, AGENTS.md banner ("reference
   implementation — bug-fix only; canonical: github.com/yarrasys/kdbx"), tests keep
   running in CI until v1.0, then archived.

## Milestones

| M | Deliverable | Gate |
|---|-------------|------|
| M0 | Repo bootstrap + **interop spike**: gokeepasslib opens a real Python-kdbx vault+keyx; writes an entry (incl. protected custom property); pykeepass and keepassxc-cli read it back. CI skeleton. | **Go/no-go.** Any gap → fix inside `internal/vault`/`keyfile` or revisit D1. |
| M1 | Foundation: `pointer` (incl. order-preserving rewrite), `paths`, `envctx`, `secretio` core, exit-code/error model, cobra skeleton, `--version`, `envs` | unit + testscript green |
| M2 | Read path: `vault` open/get/list, `get` (masked/reveal/clip), `list`, `check`, `--json` | parity: read scenarios |
| M3 | `run` + `export` (the crown jewel: injection, PATHEXT, exit passthrough, signals) | parity: run/export scenarios on 3 OSes |
| M4 | Write path: `init`, `set`, `delete`, `mv`, `import`, `rekey` + locking (C9) + pointer rewrites | full interop round-trip green |
| M5 | Integration surfaces: `mcp`, `guard`, `completion` | MCP smoke vs Claude Code; guard test vectors ported |
| M6 | Release engineering: GoReleaser, install.sh, tap, cosign, ghcr, README/SECURITY/NOTICE, license audit | v0.x tagged; curl install works on a clean machine |
| M7 | Extensions-repo integration (above) | plugin v2 works end-to-end with the binary |

## Risks & mitigations

| Risk | Mitigation |
|------|------------|
| gokeepasslib gap (Argon2 params, keyfile v2, protected custom props, Recycle Bin) | M0 spike is a hard gate against the *author's real vaults*; keyfile parsing is ours (D6); engine boundary keeps a swap possible |
| Silent serialization bug corrupts a vault | crash-safe save dance (C3), interop round-trips in CI, `.bak` retained until rename lands |
| Pointer rewrite scrambles committed JSON | explicit order-preserving requirement (C1) + golden-file tests on diff-minimality |
| Windows spawn/exit-code/PATHEXT regressions | dedicated runner package + windows CI leg + testscript coverage (the deepseek `shutil.which` lesson is encoded in C5 `run`) |
| Python↔Go lock non-interop during transition | documented (C9); transition window short; single-user tool |
| Supply-chain trust for a secrets tool | checksums + cosign from the first tag (D9); SECURITY.md; reproducible-build flags |
| Scope creep into two-implementation maintenance | D7 freeze; v1.0 archives the Python path |
