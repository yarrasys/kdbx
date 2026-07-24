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
| KDBX4 detection on read | `NewDatabase()` with no options decodes a KDBX4 file fine; `db.Header.IsKdbx4()` returns true. For **create**, pass `WithDatabaseKDBXVersion4()` explicitly. |
| Group layout | Python vaults nest under a single top-level group literally named `"Root"`. Entry `api/openai` lives at `Root → api → openai`. The `isRootGroup` name check in `internal/vault` is therefore correct for real vaults — no change needed. |
| Protected custom properties | Round-trip intact in both directions. `ValueData{Key: "ORG_ID", Value: V{Content: …, Protected: NewBoolWrapper(true)}}`; read back with `entry.GetContent("ORG_ID")`. |
| Reserved-field key names | Python/pykeepass writes `Title`, `UserName`, `Password` (and `URL`, `Notes`). `Password` is protected; `Title`/`UserName` are not. Matches the `reserved` map in `internal/vault`. |
| Recycle Bin | A fresh Python vault has `Meta.RecycleBinEnabled = true` but `RecycleBinUUID` is all zeros — i.e. **enabled but not yet created**. `recycleBinName()` must treat a zero UUID as "no bin", and `ensureRecycleBin()` must create the group and set the UUID on first use. |
| Entry construction | `gokeepasslib.NewEntry()` needs `e.Times = gokeepasslib.NewTimeData()` set explicitly; omitting it risks nil-pointer marshal issues. |
| Lock/unlock discipline | `UnlockProtectedEntries()` after every decode, `LockProtectedEntries()` before every encode, and re-unlock after a write if the in-memory handle stays in use. |

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
| Task 3 (`paths`) | Python's `expand_path` ends in `.resolve()` (follows symlinks); Go's `Expand` uses `Abs`+`Clean` (does not). Path comparisons could disagree when a pointer path traverses a symlink. | Open by design — behavior is equivalent for non-symlinked paths. A dedicated interop test (`test_symlinked_pointer_path_resolves_the_same`) asserts both implementations still reach the same vault through a symlink. |

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
