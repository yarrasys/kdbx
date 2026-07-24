# `interop/` — Go ↔ Python parity and round-trip harness

This suite is the **migration promise, executed**. The Go `kdbx` replaces a Python
implementation that people already use daily, against vaults, keyfiles and
`.keepassxc.json` pointer files that already exist on their disks. The promise is
*zero migration*: point the new binary at the old artifacts and everything keeps
working.

Nothing in the Go unit-test suite can prove that — it only proves Go agrees with
itself. This suite runs **both implementations against the same artifacts** and
asserts they are interchangeable.

> ⚠️ **A failure here is a release blocker, not a test to adjust.**
> A round-trip failure means a real user's vault is at risk. Fix the
> implementation; never relax the assertion. A parity failure means the two CLIs
> disagree on an observable contract — decide which side is wrong (the **spec is
> authoritative**, see [Deliberate divergences](#deliberate-divergences)) and fix
> that side.

## What it proves

| File | Proves |
|------|--------|
| `test_roundtrip.py` | **Artifact compatibility.** A vault created and written by either implementation is fully readable and writable by the other — including protected custom properties, recycle-bin state, and keyfile rotation — and by `pykeepass` and `keepassxc-cli` directly. |
| `test_parity.py` | **Behavioral parity.** The same scenario yields the same observable contract from both CLIs: stdout shape, exit code, and the byte content of the rewritten pointer file. |

## Running it

```bash
cd /path/to/kdbx
go build -o kdbx .
uv run --with pytest --with pykeepass python -m pytest interop -v
```

Expected: everything passes except one documented `xfail`.

The Python side is invoked as `uv run --locked <extensions>/skills/kdbx/kdbx.py`,
so `uv` must be on `PATH`; it resolves the reference implementation's own
lockfile. No network access is needed beyond `uv`'s package cache, and every test
works in its own `tmp_path` with a private `$KEEPASSXC_DIR` — **no test ever
touches a real vault.**

### Environment variables

| Variable | Default | Meaning |
|----------|---------|---------|
| `KDBX_BIN` | `<repo>/kdbx` | The Go binary under test. |
| `KDBX_PY_REPO` | `/Users/nabsha/work/yarrasys/extensions` | Checkout of `yarrasys/extensions`, which holds the Python reference at `skills/kdbx/kdbx.py`. |

If the Python entrypoint is absent the Python-side tests **skip** rather than
fail, so the suite stays runnable on a machine without the extensions checkout.
`keepassxc-cli` is likewise optional — its one test skips when the binary is not
installed. CI installs both, so nothing is silently skipped there.

## Deliberate divergences

These are **decided, documented, and encoded** — see
[`../docs/spike-notes.md`](../docs/spike-notes.md) for the full reasoning. Do not
"fix" them.

| # | Divergence | Encoded as |
|---|-----------|------------|
| 1 | **Open failures that should be exit 3.** Python surfaces a missing keyfile as `FileNotFoundError` → exit **1**; Go returns the documented `Locked` → exit **3** (spec C6). Verified: Go prints `kdbx: get failed: Locked` (3), Python prints `kdbx: get failed: FileNotFoundError` (1). **Go is correct**; the spec is authoritative over the reference implementation. | `test_parity.py::test_missing_keyfile_exit_code`, `xfail(strict=False)` |
| 2 | **Signal-killed children.** Python's `subprocess.returncode` is `-N` (SIGKILL → exit 247); Go collapses every signal death to `-1` (→ exit 255). Normal exit codes pass through identically, which is the case callers can actually use. | Not asserted — `test_run_passes_the_child_exit_code_in_both` covers the normal path only. |
| 3 | **Unterminated quote in a `.env`.** `python-dotenv` warns and *silently drops the binding*; Go fails with `Preflight` (exit 7). Losing a secret silently during `import` is the worse failure. | Go-side only: `internal/dotenv` `TestParseRejectsUnterminatedQuote` |

---

## Coverage-parity checklist

Every test file in the Python implementation, mapped to what replaces it. This is
how we know nothing was silently dropped in the port.

Left column generated with:

```bash
ls <extensions>/skills/kdbx/tests/ <extensions>/plugins/kdbx/tests/
```

### `skills/kdbx/tests/`

| Python test file | Covers | Go / interop counterpart |
|------------------|--------|--------------------------|
| `conftest.py` | `built_vault` fixture (create a vault + keyfile in a temp dir) | `internal/vault/testsupport_test.go` — same fixture role for the Go suite |
| `test_confirm.py` | TTY-only confirmation; purge proceeds/refused; soft delete needs no confirm; rekey proceeds/refused | `internal/secretio/secretio_test.go` (`TestConfirmRefusesWithoutTTY`, `TestConfirmAcceptsYOnTTY`) + `testdata/script/delete_mv.txtar` and `rekey.txtar` (both assert the exit-4 `NotConfirmed` refusal end-to-end) |
| `test_context.py` | `ACTIVE ENV:` banner to stderr; banner suppressed for reads; `prod` is not gated; `$KDBX_ENV` recorded as the source | `internal/envctx/envctx_test.go` (`TestBannerFormatMatchesPython`, `TestResolveUsesPointerDefault`) + `testdata/script/envs.txtar` (selects `prod` via `$KDBX_ENV` with no gate, asserts each `source:`) + `interop/test_parity.py::test_read_ops_print_no_banner_in_either` |
| `test_integration.py` | Full lifecycle; secret never in argv; `--help` works; `keepassxc-cli` can read the vault | `testdata/script/*.txtar` (the whole lifecycle as CLI-contract tests) + `interop/test_roundtrip.py::test_keepassxc_cli_reads_a_go_written_vault`. Argv leakage is structurally impossible in Go: `cmd/set.go` is `cobra.ExactArgs(1)` (the path) with intake from stdin/`--from-env` only — asserted in `init_set.txtar` (`! stderr 'sk-test'`, `! stdout 'sk-test'`) |
| `test_launcher.py` | `install-launcher`: writes a shim, refuses foreign files, resolves the newest install across channels | **Removed — no counterpart needed (spec D5).** The Go binary *is* the launcher: a single static executable, so there is no uv/skill-cache version resolution to shim around. `testdata/script/help.txtar` asserts `install-launcher` is gone (`! stdout 'install-launcher'`) so the op cannot silently return |
| `test_locking.py` | `capture_state`/`verify_unchanged`; lock file is created | `internal/locking/locking_test.go` (`TestCaptureStateOfMissingFileIsEmpty`, `TestVerifyUnchangedDetectsModification`, `TestWithVaultLockRunsAndReleases`, `TestWithVaultLockPropagatesCallbackError`) |
| `test_ops_crud.py` | init→set→get; `--reveal`; delete then list; `set --var` rewrites the pointer; `envs` marks active; `mv` repoints vars and preserves the `:field` suffix and leaves unrelated mappings; `--var` name validation | `testdata/script/init_set.txtar`, `delete_mv.txtar`, `envs.txtar` + `internal/pointer/pointer_test.go` (`TestSetVarAndSavePreservesFormatting`, `TestRepointVarsPreservesFieldSuffix`) and `entrypath_test.go` (`TestValidVarName`) + `interop/test_parity.py::test_pointer_rewrite_produces_the_same_file` (byte-identical pointer output) |
| `test_ops_extra.py` | `run` injects env; `run` propagates the child exit code; `check` reports drift (exit 5); `export` round-trips a multi-line PEM | `internal/runner/runner_test.go` (`TestRunInjectsVariables`, `TestRunPassesThroughExitCode`) + `internal/vaultvars/vaultvars_test.go` (`TestResolveUnresolvedIsDrift`) + `internal/dotenv/dotenv_test.go` (`TestParseKeepsMultilineQuotedValues`, `TestRoundTrip`) + `testdata/script/run.txtar`, `export_import.txtar`, `read_ops.txtar` + `interop/test_parity.py::test_run_passes_the_child_exit_code_in_both`, `test_check_drift_exits_5_in_both`, `test_export_renders_identically` |
| `test_paths.py` | `$KEEPASSXC_DIR` override; Linux/Windows defaults; `${KEEPASSXC_DIR}` token expansion; sync-root detection | `internal/paths/paths_test.go` (`TestKeepassxcDirPrefersOverride`, `TestKeepassxcDirUsesXDGOnUnix`, `TestExpandSubstitutesKeepassxcDirToken`, `TestExpandExpandsTilde`, `TestUnderSyncRootDetectsKnownRoots`). Symlink resolution — a parity gap found in Task 3 — additionally gets `TestResolveFollowsSymlinksLikePythonResolve` and `interop/test_parity.py::test_symlinked_pointer_path_resolves_the_same` |
| `test_pointer.py` | Discovery walks up; env-selection precedence; derived defaults; entry-path grammar incl. rejecting a colon in a name; atomic pointer write preserving key order | `internal/pointer/pointer_test.go` (`TestFindWalksUpFromNestedDir`, `TestSelectEnvPrecedence`, `TestResolveEnvDefaultsPathsFromProject`, `TestFindResolvesSymlinksSoProjectNameMatchesPython`) + `entrypath_test.go` (`TestParseEntryPathDefaults`, `TestParseEntryPathRejectsAmbiguous`) + `internal/ojson/ojson_test.go` (order preservation and `\uXXXX` escaping identical to `json.dumps`) |
| `test_secretio.py` | `MASK` carries no value info; `--from-env` intake; stdin strips exactly one newline; atomic write is 0600; dotenv multi-line round-trip; scrubbed errors; empty/whitespace stdin rejected | `internal/secretio/secretio_test.go` (`TestReadSecretFromEnv`, `TestReadSecretFromStdinStripsOneTrailingNewline`, `TestReadSecretStripsCRLF`, `TestReadSecretEmptyStdinIsAnError`, `TestAtomicWriteSecretIsOwnerOnly`) + `internal/kdbxerr/kdbxerr_test.go` (`TestReportScrubsDetailWithoutDebug`, `TestReportIncludesDetailWithDebug`) + `internal/dotenv/dotenv_test.go` (`TestRoundTrip`). `Mask` itself is a const (`internal/secretio/secretio.go`) asserted verbatim by `read_ops.txtar` and `interop/test_parity.py::test_masked_get_prints_the_same_sentinel` |
| `test_vault_create.py` | Mint keyfile then open; 0600 perms on create; refuse to overwrite; save keeps 0600 | `internal/vault/vault_test.go` (`TestCreateProducesAnOpenableKdbx4Vault`, `TestCreateRefusesToOverwrite`, `TestOpenWithMissingKeyfileIsLocked`) + `internal/keyfile/keyfile_test.go` (`TestMintCreatesValidOwnerOnlyKeyfile`, `TestMintIsUnpredictable`, `TestMintRefusesToOverwrite`, `TestRenderXMLMatchesPythonByteForByte`) |
| `test_vault_crud.py` | set/get default password field; missing field raises; trash is recoverable and hidden; purge removes; move/rename; custom field round-trip; empty value refused | `internal/vault/vault_test.go` (`TestSetGetReservedAndCustomFields`, `TestGetFieldMissingFieldIsNotFound`, `TestListEntriesIsSortedAndExcludesTrash`, `TestMoveWithinAGroupRenamesExactlyOneEntry`, `TestMoveAcrossGroupsKeepsTheValue`, `TestSetFieldRefusesEmptyValue`) + `interop/test_roundtrip.py::test_protected_custom_property_survives_both_directions`, `test_recycle_bin_semantics_agree` |

### `plugins/kdbx/tests/`

| Python test file | Covers | Go / interop counterpart |
|------------------|--------|--------------------------|
| `test_plugin_guard.py` | Leak denials (`cat *.keyx`, command substitution, `VAR=` prefixes, `.bak` keyfiles); safe allows; role guard blocks agent writes / `--reveal` / `--clip`; allows masked reads and `run --`; `$KEEPASSXC_DIR` override; subprocess deny/allow/fail-open contract | `internal/guard/guard_test.go` (`TestDecideBlocksAgentWriteOps`, `TestDecideAllowsAgentReadOps`, `TestDecideBlocksNonKdbxToolsTouchingVaultFiles`, `TestDecideAllowsKdbxAndKeepassxcCliTouchingVaultFiles`, `TestDecideInspectsEveryShellSegment`, `TestRunEmitsDenyEnvelopeAndAlwaysExitsZero`, `TestRunAllowsSilently`, `TestRunFailsOpenOnGarbageInput`). The Python hook script is replaced by the built-in `kdbx guard` op |
| `test_plugin_mcp.py` | `get` is masked and never reveals; `run --` injection; `list`/`envs`; `check` status; no value-crossing ops registered; only safe tools exposed | `internal/mcpserver/mcpserver_test.go` (`TestExposesExactlyTheFiveReadOnlyTools` — also asserts `kdbx_set`/`delete`/`export`/`import`/`rekey`/`mv` are *never* exposed —, `TestGetToolNeverReturnsAValue`, `TestListToolReturnsPaths`, `TestEnvsToolMarksTheActiveEnv`, `TestCheckToolReportsResolution`). The standalone Python MCP server is replaced by the built-in `kdbx mcp` op |

### Result

**15 Python files: 14 mapped to a Go and/or interop counterpart, 1 justified as
removed** (`test_launcher.py` — spec D5, the binary is its own launcher).

**Residual gap (tracked, not blocking):** `test_plugin_mcp.py::test_run_injects_via_run_dashdash`
has no direct Go equivalent — `internal/mcpserver` tests the four read tools but not
`kdbx_run`'s injection path. The underlying behavior *is* covered at the CLI layer
by `internal/runner/runner_test.go::TestRunInjectsVariables` and
`testdata/script/run.txtar`; only the MCP tool's thin wrapper over it is untested.
Owned by the `kdbx mcp` workstream.
