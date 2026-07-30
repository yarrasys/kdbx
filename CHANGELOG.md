# Changelog

All notable changes to kdbx are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and kdbx uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions are cut from a `v*` tag.

## [Unreleased]

### Changed

- **KDBX engine updated** to `gokeepasslib` v3.7.0 (from v3.6.2). **No change to the vault
  format**: the release adds *opt-in* KDBX 4.1 support, and kdbx continues to write KDBX 4.0.
  `internal/vault.Create` now names `WithDatabaseKDBXVersion40()` explicitly instead of the
  newly deprecated `WithDatabaseKDBXVersion4()` alias. Re-verified against
  `keepassxc-cli` 2.7.12; see [docs/spike-notes.md](docs/spike-notes.md).
- MCP SDK updated to `modelcontextprotocol/go-sdk` v1.7.0, and `golang.org/x/crypto` to v0.54.0.
- **Container images now carry OCI metadata** (`org.opencontainers.image.source` and friends),
  so the `ghcr.io/yarrasys/kdbx` package links back to this repository and declares its MIT
  license. Set from `.goreleaser.yaml` so version and revision come from the release tag.

### Added

- `TestCreateWritesKdbx40OnDisk` asserts the KDBX major/minor version in the raw header bytes
  of a freshly created vault, so an engine upgrade cannot silently change the on-disk format.

### Fixed

- CI and release workflows moved off several end-of-life action majors (`checkout` v4 → v7,
  `setup-go` v5 → v7, `golangci-lint-action` v7 → v9, the `docker/*` actions v3 → v4,
  `goreleaser-action` v6 → v7). CI now declares least-privilege `contents: read`.
- **cosign pinned to the 2.x line** in the release workflow. `cosign-installer@v4` defaults to
  cosign 3, which enables `--new-bundle-format` and *ignores* the
  `--output-signature`/`--output-certificate` flags the `signs` block relies on — signing would
  have appeared to succeed while publishing no `SHA256SUMS.sig`/`.pem`, the artifacts
  SECURITY.md tells users to verify.

## [0.1.2] - 2026-07-25

### Added

- **Container image** — a multi-arch (`linux/amd64`, `linux/arm64`) `FROM scratch` image at
  `ghcr.io/yarrasys/kdbx`, built and pushed by the release pipeline:
  `docker run --rm ghcr.io/yarrasys/kdbx:latest --version`.

## [0.1.1] - 2026-07-25

### Added

- **Homebrew cask** — `brew install yarrasys/tap/kdbx`, published to
  [`yarrasys/homebrew-tap`](https://github.com/yarrasys/homebrew-tap) by the release
  pipeline. (The `ghcr.io/yarrasys/kdbx` container image remains deferred until the release
  workflow gains multi-arch build support.)

## [0.1.0] - 2026-07-25

The first release of kdbx as a standalone Go binary. It reimplements the Python `kdbx` skill
from [`yarrasys/extensions`](https://github.com/yarrasys/extensions/tree/main/skills/kdbx),
which is now the frozen reference implementation.

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
  darwin/linux/windows × amd64/arm64, `SHA256SUMS` with a cosign keyless signature, and a
  checksum-verifying `install.sh` for `curl … | sh` installation. A Homebrew cask and a
  `FROM scratch` `ghcr.io/yarrasys/kdbx` image are configured but not published in this
  release (the tap repo, its token, and multi-arch container CI are not yet set up).

### Compatibility

- Vaults (KDBX4 + Argon2, key-file-only), KeePass XML v2 key files, and `.keepassxc.json`
  pointer files are the standard formats KeePassXC uses, so `keepassxc-cli` and the KeePassXC
  desktop app read Go-written vaults directly. Any vault, key file or pointer created by the
  original Python skill is read without a migration step.

### Breaking changes vs. the Python implementation

- **`install-launcher` is removed.** The binary is its own launcher — there is no shim to
  install, no skill directory to resolve, and no replacement operation. Anything that
  invoked `kdbx install-launcher` should simply install the binary instead.
- **Failing to open a vault now exits 3, not 1.** Exit 3 (locked / key file missing /
  credential failure) has always been the documented contract; the Python implementation
  surfaced some open failures as the generic exit 1. Go implements the documented code.
  Scripts that branched on exit 1 for a missing or unreadable key file must branch on 3.

[Unreleased]: https://github.com/yarrasys/kdbx/compare/v0.1.2...main
[0.1.2]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.2
[0.1.1]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.1
[0.1.0]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.0
