# M0 interop spike — findings

Run: 2026-07-24 · engine `github.com/tobischo/gokeepasslib/v3 v3.6.2` · Go 1.26.5 · macOS arm64

**Verdict: GO.** Full three-way interop confirmed against a vault created by the Python
implementation (`skills/kdbx/kdbx.py`):

- Go **opened** the Python-created vault, decrypted it, and read a protected custom property.
- Go **wrote** an entry (protected `Password` + protected custom property) and re-opened it.
- **pykeepass** read the Go-written vault: new entry present, and the pre-existing entry's
  password and custom property were undamaged.
- **keepassxc-cli** (KeePassXC 2.7.x) listed the Go-written vault correctly.

## Engine facts later tasks depend on

| Question | Observed |
|----------|----------|
| Keyfile v2 (`<Data Hash="…">` hex) | `gokeepasslib.NewKeyCredentials(path)` parses our Python-minted `.keyx` directly. No custom parser needed for **reading** — `internal/keyfile` only needs to **mint** and validate. |
| KDBX4 detection on read | `NewDatabase()` with no options decodes a KDBX4 file fine; `db.Header.IsKdbx4()` returns true. For **create**, pass the minor version explicitly — `WithDatabaseKDBXVersion40()` since the engine bump below. |
| Group layout | Python vaults nest under a single top-level group literally named `"Root"`. Entry `api/openai` lives at `Root → api → openai`. The `isRootGroup` name check in `internal/vault` is therefore correct for real vaults — no change needed. |
| Protected custom properties | Round-trip intact in both directions. `ValueData{Key: "ORG_ID", Value: V{Content: …, Protected: NewBoolWrapper(true)}}`; read back with `entry.GetContent("ORG_ID")`. |
| Reserved-field key names | Python/pykeepass writes `Title`, `UserName`, `Password` (and `URL`, `Notes`). `Password` is protected; `Title`/`UserName` are not. Matches the `reserved` map in `internal/vault`. |
| Recycle Bin | A fresh Python vault has `Meta.RecycleBinEnabled = true` but `RecycleBinUUID` is all zeros — i.e. **enabled but not yet created**. `recycleBinName()` must treat a zero UUID as "no bin", and `ensureRecycleBin()` must create the group and set the UUID on first use. |
| Entry construction | `gokeepasslib.NewEntry()` needs `e.Times = gokeepasslib.NewTimeData()` set explicitly; omitting it risks nil-pointer marshal issues. |
| Lock/unlock discipline | `UnlockProtectedEntries()` after every decode, `LockProtectedEntries()` before every encode, and re-unlock after a write if the in-memory handle stays in use. |

## Engine upgrades since the spike

The spike above was run against `v3.6.2`. Each later engine bump is re-verified here, because
the on-disk format is a compatibility promise, not an implementation detail.

**`v3.6.2` → `v3.7.0`** (2026-07-30, macOS arm64, Go 1.26.5) — **safe, no format change.**

- The release adds **opt-in KDBX 4.1 support** (upstream PR #151) via a new
  `WithDatabaseKDBXVersion41()`. It does **not** change what we write:
  `WithDatabaseKDBXVersion4()` still delegates to `WithDatabaseKDBXVersion40()`.
- `WithDatabaseKDBXVersion4()` is now **deprecated** — staticcheck (SA1019) flags it.
  `internal/vault.Create` calls `WithDatabaseKDBXVersion40()` explicitly instead, so the minor
  version is a named decision rather than whatever an alias resolves to next.
- Verified: a freshly `kdbx init`-ed vault carries header bytes `00 00 04 00` (KDBX **4.0**,
  little-endian minor then major) and `keepassxc-cli 2.7.12` unlocks it key-file-only.
  `TestCreateWritesKdbx40OnDisk` in `internal/vault` now asserts those raw bytes, so a future
  bump that flips the default fails the suite instead of silently changing every vault.
- Also in the release: protected-field stream fix for KDBX 3.1 (upstream PR #152) — irrelevant
  to us, we only write 4.0 — and `golang.org/x/crypto` `v0.48.0` → `v0.54.0`.

## Consequences for the plan

1. **`internal/keyfile` scope narrows** — mint + validate only; loading delegates to
   `NewKeyCredentials`. (Spec D6 relaxed accordingly; validation is still ours so a missing
   or corrupt keyfile yields exit 3 with a clear message.)
2. **`recycleBinName()` must check for a zero UUID**, not just `RecycleBinEnabled`. A fresh
   vault reports enabled-with-zero-UUID, which would otherwise match no group and silently
   behave correctly — but the zero check makes the intent explicit and guards against a
   group whose UUID also happens to be zero.
3. **`isRootGroup` stays as a name check** (`"Root"`) — confirmed against real vaults.

## Parity findings from the build (resolved)

| Found in | Divergence | Resolution |
|----------|-----------|------------|
| Task 5 (`ojson`) | Go emitted raw UTF-8 for new string values; Python's `json.dumps` defaults to `ensure_ascii=True` and emits `\uXXXX`. A non-ASCII entry path would produce a spurious diff in the committed pointer file whenever the two implementations alternated. | Fixed — `encodeString` now `\uXXXX`-escapes non-ASCII (surrogate pairs above the BMP). Verified byte-identical against real `json.dumps` output. |
| Task 3 (`paths`) | Python's `expand_path` ends in `.resolve()` (follows symlinks); Go's `Expand` used `Abs`+`Clean` (did not). Worse than cosmetic: `Project()` falls back to the pointer directory's basename, so a repo reached via a symlink derived a different project name — and a different default vault path. | Fixed — `paths.Resolve` does non-strict symlink resolution (longest existing ancestor + remaining components), applied to `Expand`, `Find`, and the default-artifact branch. Verified: Go and Python report identical project name and vault path through a symlink. |
| Task 15 (`dotenv`) | `godotenv v1.5.1` interpolates `$VAR` inside double-quoted values, and expands only from the file's own bindings — so an unknown `$VAR` collapses to the **empty string**. A secret containing `$` would be silently destroyed by `kdbx import`, not merely leaked. | Avoided — `Parse` is hand-rolled to mirror python-dotenv's grammar with `interpolate=False` semantics. Zero dependencies added. Verified equal to `dotenv_values(interpolate=False)`, and Go's `Render` is byte-identical to Python's `render_dotenv`. |

## Deliberate divergences (kept, with reasons)

**Unterminated quoted value in a `.env` (Task 15).** python-dotenv prints a warning and
silently drops the binding (`A="never closed` → `{}`). Go returns `Preflight` (exit 7)
instead.

Kept deliberately. `kdbx import` exists to move secrets *into* the vault; silently dropping
one means a credential disappears with no signal, and the user discovers it when something
fails in production. Matching Python here would faithfully reproduce a bug. Failing loudly
on a malformed source file is the safer contract, and `import` is a human-run operation
where a clear error is actionable.

## Exit-code note (spec C6) — signal-killed children

`runner.Run` returns the child's raw exit code, matching Python's `subprocess.returncode`
for the normal case. Verified during Task 16: a child killed by a signal makes
`exec.ExitError.ExitCode()` return **`-1`** (Go collapses every signal death to -1), and
`Run` passes that through with a nil error.

⚠️ **Accepted divergence.** Python's `returncode` is `-N` for signal N (e.g. `-9` for
SIGKILL), so the two implementations differ in this one case after the process exits:

| Child killed by | Python `kdbx run` | Go `kdbx run` |
|-----------------|-------------------|---------------|
| SIGKILL (9)     | `sys.exit(-9)` → **247** | `os.Exit(-1)` → **255** |
| SIGTERM (15)    | `sys.exit(-15)` → **241** | `os.Exit(-1)` → **255** |

Left as-is deliberately. Neither implementation matches the shell's `128+N` convention
(137/143), so no script written against shell semantics works on either; reproducing
Python's `-N` exactly would need platform-specific `WaitStatus` handling for a value no
caller can portably use. Normal (non-signal) exit codes pass through identically, which is
the case that matters. Note that a child which *traps* a signal and exits normally reports
its own code correctly — confirmed at 7 in the Task 16 forwarding test.

Signal forwarding itself is verified, not assumed: SIGINT/SIGTERM sent to kdbx reach the
child, proven with a control run (same child, no forwarding goroutine, signal not
delivered) that rules out process-group spillover.
