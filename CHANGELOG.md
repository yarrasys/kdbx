# Changelog

All notable changes to kdbx are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and kdbx uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions are cut from a `v*` tag; v1.0.0 will mean the compatibility contract is frozen and
the interop suite is green. Nothing has been released yet.

## [Unreleased]

The first version of kdbx as a standalone Go binary. It reimplements the Python `kdbx` skill
from [`yarrasys/extensions`](https://github.com/yarrasys/extensions/tree/main/skills/kdbx),
which becomes the frozen reference implementation.

### Added

- **The `kdbx` binary** — a single static executable for macOS, Linux and Windows on amd64
  and arm64. No Python, no `uv`, no skill installation, no launcher shim.
- **Pointer-based discovery** — walks up from the working directory to a committed
  `.keepassxc.json`, resolves the active environment (`--env` › `$KDBX_ENV` › `defaultEnv`),
  and derives the vault and key-file paths.
- **Vault operations**: `init`, `set`, `get`, `list`, `delete`, `mv`, `run`, `export`,
  `import`, `check`, `envs`, `rekey`. `run` injects the active environment's mapped secrets
  into a child process and passes its exit code through.
- **`--json`** on read operations (`list`, `check`, `envs`, `get`) for machine consumers.
  Secret values are never included.
- **`kdbx mcp`** — a read-only MCP server over stdio exposing `kdbx_list`, `kdbx_envs`,
  `kdbx_check`, `kdbx_get` and `kdbx_run`. No write tools, deliberately.
- **`kdbx guard`** — a `PreToolUse` hook engine that denies agent-issued human-only
  operations and blocks non-kdbx programs reaching for vault or key-file paths. Always
  exits 0 (fails open).
- **`kdbx completion`** for bash, zsh, fish and powershell.
- **Crash-safe, concurrency-safe writes** — advisory lock on `<vault>.lock`, a SHA-256
  capture/verify around read-modify-write operations (exit 6 if the vault changed
  underneath), and a temp-file → rename save with the previous vault retained as `.bak`
  until the rename lands.
- **Order-preserving pointer rewrites** — `set --var`, `mv` and `import` edit
  `.keepassxc.json` in place with 2-space indentation, a trailing newline, existing key
  order intact, and non-ASCII escaped as `\uXXXX`, so the committed file stays diff-minimal
  and byte-identical to what the Python implementation would have written.
- **Cloud-sync warning on `init`** — warns (never refuses) when the vault or key file lands
  under OneDrive, Dropbox, iCloud Drive or similar. The Python implementation had the
  detection but never wired it in.
- **Release engineering** — GoReleaser pipeline producing archives for
  darwin/linux/windows × amd64/arm64, `SHA256SUMS` with a cosign keyless signature, a
  Homebrew cask in `yarrasys/homebrew-tap`, a `FROM scratch` image on
  `ghcr.io/yarrasys/kdbx`, and a checksum-verifying `install.sh` for
  `curl … | sh` installation.

### Compatibility

- Vaults (KDBX4 + Argon2, key-file-only), KeePass XML v2 key files, and `.keepassxc.json`
  pointer files are **fully interchangeable** with the Python implementation in both
  directions, with no migration step. Verified against `pykeepass` and `keepassxc-cli` —
  see `docs/spike-notes.md` and the `interop/` suite.
- ⚠️ Advisory locking does **not** interoperate: Python's `filelock` and Go's
  `gofrs/flock` do not reliably block one another. Do not run concurrent writes from both
  implementations against the same vault.

### Breaking changes vs. the Python implementation

- **`install-launcher` is removed.** The binary is its own launcher — there is no shim to
  install, no skill directory to resolve, and no replacement operation. Anything that
  invoked `kdbx install-launcher` should simply install the binary instead.
- **Failing to open a vault now exits 3, not 1.** Exit 3 (locked / key file missing /
  credential failure) has always been the documented contract; the Python implementation
  surfaced some open failures as the generic exit 1. Go implements the documented code, and
  the parity harness asserts the documented contract rather than the Python behavior.
  Scripts that branched on exit 1 for a missing or unreadable key file must branch on 3.

[Unreleased]: https://github.com/yarrasys/kdbx/commits/main
