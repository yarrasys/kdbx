# Changelog

All notable changes to kdbx are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and kdbx uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Versions are cut from a `v*` tag.

## [Unreleased]

### Added

- **`kdbx init` can start a project now.** `kdbx init --here` writes a minimal
  `.keepassxc.json` in the current directory (project named after it, `dev`/`prod`
  environments, `dev` active) and creates the vault in one step; plain `kdbx init` does the
  same when there is no pointer anywhere above, a path that could previously only fail.
  Inside another project's tree, plain `init` keeps targeting that project (the
  cloned-repo case) but now prints a note naming it and pointing at `--here`, so the
  walk-up trap explains itself instead of dying with a bare `Preflight`. This bit three
  real projects in two weeks; the tool was the right place to fix it.

## [0.2.1] - 2026-08-01

Release-engineering only; the binary is identical to 0.2.0 in behavior. Cut so the
container image carries the label the MCP Registry requires, which cannot be added to an
already-published multi-arch tag.

### Fixed

- **MCP Registry publishing works now, and failures are loud.** The v0.2.0 release's
  registry publish failed with a 400, and `continue-on-error` hid it behind a green run,
  exactly the failure mode that step was flagged for. Three registry-side rules were
  stricter than the published schema: no `registryBaseUrl` on OCI packages, no package
  `version` field (the canonical `image:tag` identifier carries it), and the image itself
  must bear an `io.modelcontextprotocol.server.name` label proving intent. `server.json`
  is now canonical, the label ships in the image via `.goreleaser.yaml`, and publishing
  moved to its own `mcp-publish` workflow, triggered by release publication and by manual
  dispatch, so a registry-side failure is a red run that can be retried or backfilled
  without cutting a new tag.

## [0.2.0] - 2026-08-01

The launch-feedback release. Readers on r/AI_Agents demonstrated that "kdbx never prints a
secret" was a property of the wrapper, not the system: `kdbx run -- env` printed every
injected value, and the MCP server's `kdbx_run` did the same straight into a model
transcript. This release closes the demonstrated leaks and adds an opt-in strict policy
profile, designed under one honest constraint: an agent running as your uid cannot be
*contained* by kdbx, so these features stop accidents and make tampering loud, and the docs
say exactly that (see issue #11 for the full analysis).

**Behavior change** (the minor-version bump): when `run`'s child output is captured rather
than on a terminal, injected values now appear as `***`. Piped output that expected raw
values needs `--no-mask`, which the guard reserves for humans.

### Added

- **Policy modes: `standard` and `strict`**
  ([#11](https://github.com/yarrasys/kdbx/issues/11)). `kdbx init --mode strict` opts an
  environment into the locked-down profile, recorded as `policy.mode` in the pointer.
  Strict requires the `run.allow` list, refuses `--any`/`--no-mask` outright, appends an
  audit line for every run decision (timestamp, command, injected variable names — never
  values — to `<vault>.audit.log`, 0600), and gates injection on a policy hash anchored in
  the vault's custom data. `kdbx policy bless` is the human approval act that writes the
  anchor: interactive-only like `rekey`, denied to agents by the guard. An out-of-band edit
  to the policy or allowlist is refused at the next `run` (`PolicyDrift`, exit 5) instead of
  silently obeyed. Anchored vaults verified compatible with `keepassxc-cli` 2.7.12,
  including surviving a KeePassXC-side write; see `docs/spike-notes.md`.
- **The MCP server's `kdbx_run` now goes through the same gate as the CLI** and always
  masks. Previously it returned raw captured output to the model, so `kdbx_run` with
  command `env` leaked every injected value into the transcript with no allowlist, no
  policy, and no masking, while its description claimed secrets were never printed. The
  shared gate lives in `internal/runpolicy`, so the two surfaces cannot drift; the MCP path
  has no escape hatches at all.

- **The pointer can pin `run` to an allowlist**
  ([#11](https://github.com/yarrasys/kdbx/issues/11)). An optional per-env
  `run.allow` array in `.keepassxc.json` makes `run` refuse any command whose argv does not
  exactly equal a listed entry (shell-split; no prefix matching, since a `pytest` prefix
  would admit `pytest --pdb`). Refusal happens before the vault is opened and carries its
  own kind, `NotAllowed` (exit 7). `--any` bypasses the list for ad hoc human use and is
  denied to agents by the guard, like `--no-mask`. Absent list = no restriction; empty
  list = nothing runs without `--any`; malformed list = error, never silently ignored.
  Combined with the guard's pointer-write denial, an agent widening its own allowlist is
  a visible diff on a tracked file rather than an invisible one-liner.

- **`run` now masks injected values in captured child output**
  ([#14](https://github.com/yarrasys/kdbx/issues/14)). When a child stream is not a TTY (a
  pipe, an agent harness, a log), each injected value of 8 bytes or more is replaced with
  `***` on the way through, so `kdbx run -- env` under an agent yields masks instead of
  values, with no configuration. A terminal keeps the raw fd, so interactive children are
  untouched; `--no-mask` restores raw piped output for humans and is denied to agents by the
  guard, like `get --reveal`. Matching is exact-value, longest-first, and chunking-invariant
  (a value split across writes still masks; property fuzz-tested in `internal/maskio`).
  Encoded values pass through: this stops accidents, not a hostile agent, same as the rest
  of the tool.

### Security

- **The guard now denies agent writes to `.keepassxc.json`**
  ([#13](https://github.com/yarrasys/kdbx/issues/13)). Output redirection onto the pointer,
  `sed -i`/`perl -i`, `tee`, interactive editors, and `mv`/`cp` with the pointer as
  destination are denied; reads stay allowed, and kdbx itself remains a recognized invoker.
  The pointer selects which vault and key file kdbx opens, so an agent editing it is editing
  its own permissions. Heuristic and fail-open, like the rest of the guard.

### Fixed

- **README no longer claims an agent "never sees its value".** It does not hold: `run` injects
  into whatever argv it is handed, so `kdbx run -- env` prints every injected value, as does a
  test script the agent edited beforehand. kdbx not printing a secret is a property of kdbx,
  not of the system around it. The roles section now states what the split does buy (an agent
  will not stumble into disclosure, and cannot author, rotate or export a credential) and what
  it does not (containment of an agent that has decided to read one), and points at
  [#11](https://github.com/yarrasys/kdbx/issues/11). Raised by readers on r/AI_Agents.

## [0.1.3] - 2026-07-31

A maintenance release, and the first to carry fixes for bugs found by fuzzing and by the
Windows CI matrix. Two of them could alter or strand data: `shlex.Split` handed `exec` an
argument other than the one it was given, and a failed `rekey` left a freshly minted key file
on disk. Vault format is unchanged — existing vaults, key files and pointers keep working.

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
- [CONTRIBUTING.md](CONTRIBUTING.md) and a Contributor Covenant 2.1
  [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
- `.github/dependabot.yml` — weekly Go module and GitHub Actions updates. The KDBX engine is
  excluded from the grouped Go PR so its bumps arrive alone and get re-verified.
- **The normative design spec now lives in this repository** at
  [`docs/kdbx-go-standalone-design.md`](docs/kdbx-go-standalone-design.md). It was authored in
  `yarrasys/extensions`, which was archived on 2026-07-30 and is read-only, so the contract this
  repo defers to is finally reachable to anyone reading the code it governs. The completed port
  plan came with it as [`docs/history/`](docs/history/) — historical, not normative.

### Fixed

- **`init` and `rekey` no longer leave a handle open on the key file** (Windows). Both used
  `gokeepasslib.NewKeyCredentials`, which opens the key file and never closes it; `Open` was
  fixed for this previously and these two were missed. The consequence went beyond an untidy
  handle: `rekey` removes the newly minted key file when a rekey fails, and on Windows the
  leaked handle blocked that removal — stranding secret material on disk after an operation the
  user watched fail. `TestKeyfileIsNotHeldOpen` now covers `Create`, `Open` and `Rekey`.
- **`shlex.Split` no longer corrupts bytes it cannot decode** (found by fuzzing). It converted its input to
  `[]rune` first, which replaces every byte that is not valid UTF-8 with U+FFFD, so an argument
  containing such a byte was executed as something other than what the caller asked for — with
  no error. Splitting is now byte-oriented, like a real shell; valid multi-byte characters are
  unaffected, since every character `Split` treats specially is ASCII.
- **`ojson.Parse` now caps nested-object depth at 32** (found by fuzzing). Decoding captured each nested value and
  re-scanned it one level down, making the work quadratic in depth: 24 ms at 1,000 levels,
  1.5 s at 10,000. Pointer discovery walks up from the working directory, so kdbx reads
  `.keepassxc.json` files it did not write — checking out a hostile repository should not be
  able to wedge `kdbx run`. A real pointer nests three levels.
- CI and release workflows moved off several end-of-life action majors (`checkout` v4 → v7,
  `setup-go` v5 → v7, `golangci-lint-action` v7 → v9, the `docker/*` actions v3 → v4,
  `goreleaser-action` v6 → v7). CI now declares least-privilege `contents: read`.
- **cosign pinned to the 2.x line** in the release workflow. `cosign-installer@v4` defaults to
  cosign 3, which enables `--new-bundle-format` and *ignores* the
  `--output-signature`/`--output-certificate` flags the `signs` block relies on — signing would
  have appeared to succeed while publishing no `SHA256SUMS.sig`/`.pem`, the artifacts
  SECURITY.md tells users to verify.
- **`.gitignore` now covers `*.env`.** It ignored `.env` and `.env.*` but not `prod.env` or
  `secrets.env` — natural names for a `kdbx export --out` target, and exactly the files that
  must never be committed. `.env.example` stays committable.

### Security

- **Release archives are now byte-reproducible.** `mod_timestamp` stamps archive member mtimes
  from the commit instead of the clock. `-trimpath` already made the binary itself identical
  across builds, but the archives were not — so `SHA256SUMS` changed on a rebuild of the same
  tag and a published checksum could not be independently reproduced. Verified: two builds of
  one commit now produce identical archives and an identical `SHA256SUMS`.
- **A `release-dryrun` workflow rehearses the release pipeline** on demand and weekly, without
  publishing. `release.yml` only ever runs on a `v*` tag, so a broken action pin, config error
  or cosign regression used to surface mid-release — after the archives were uploaded and before
  the signatures existed. Snapshot mode does run the `signs` block, so the dry run signs for
  real and then *verifies* the signature, rather than trusting the signing step's exit code; it
  also rebuilds to confirm the checksums are reproducible.
- **Private vulnerability reporting enabled.** SECURITY.md and the issue-template config already
  routed disclosures to GitHub Security Advisories, but the setting was off — so an outside
  reporter had no way to open one.
- **Fuzz targets for all four hand-rolled parsers** (`dotenv`, `ojson`, `shlex`, and `pointer`'s
  entry-path grammar), each asserting a property rather than merely the absence of a panic —
  Parse/Render are inverses, re-encoding is idempotent, unquoted splitting matches whitespace
  splitting, a rejected path returns nothing usable. Seed corpora run as part of `go test ./...`.
- **CodeQL** (`security-and-quality` queries) on push, PR and weekly.
- **Every GitHub Action is pinned to a commit SHA** with its version in a trailing comment. A
  tag can be repointed at new code after review; a SHA cannot.
- **`release.yml` no longer grants write permission workflow-wide.** `contents`, `packages` and
  `id-token` write are scoped to the one job that publishes; the workflow default is now
  `contents: read`.
- **`govulncheck` now runs in CI** on every push and PR, plus weekly so a newly published
  advisory against an unchanged dependency surfaces without waiting for a code change. It fails
  the build only on a vulnerability kdbx actually *calls*; module-level advisories against
  required-but-uncalled code are reported, not fatal.
- **OpenSSF Scorecard** runs weekly and on branch-protection changes, publishing its results and
  uploading findings to the Security tab.
- Repository security features enabled: **secret scanning**, **push protection** (a secret cannot
  be pushed in the first place), and **Dependabot automated security fixes**.

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

[Unreleased]: https://github.com/yarrasys/kdbx/compare/v0.2.1...main
[0.2.1]: https://github.com/yarrasys/kdbx/releases/tag/v0.2.1
[0.2.0]: https://github.com/yarrasys/kdbx/releases/tag/v0.2.0
[0.1.3]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.3
[0.1.2]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.2
[0.1.1]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.1
[0.1.0]: https://github.com/yarrasys/kdbx/releases/tag/v0.1.0
