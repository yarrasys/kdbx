# kdbx v1 — Standalone Go Binary — Implementation Plan

> ## ⚠️ HISTORICAL — COMPLETED 2026-07-25. DO NOT EXECUTE.
>
> **This plan has already been carried out.** kdbx shipped from it and is released through
> v0.1.2. It is kept only as the record of how the port was sequenced and why.
>
> **Agents: do not treat the unchecked `- [ ]` boxes as work to do.** The original header here
> instructed an agentic worker to execute this plan task-by-task; that instruction has been
> removed because following it now would rebuild a repository that already exists. The current
> state of the code is the source of truth, and [AGENTS.md](../../AGENTS.md) is the guide to
> changing it.
>
> Parts of this plan were superseded during the build and never landed as written — the
> `pytest` interop harness was retired, `joho/godotenv` was replaced by a hand-rolled
> `internal/dotenv`, and the milestone sequencing shifted. Nothing here is normative. The
> normative contract is [the design spec](../kdbx-go-standalone-design.md); the verified engine
> facts and accepted divergences are in [spike-notes.md](../spike-notes.md).
>
> Moved from the now-archived `yarrasys/extensions` on 2026-07-30, when that repo became
> read-only and this document's only remaining copy was on one laptop. Absolute paths from the
> author's machine were replaced with `$KDBX_REPO` and `$EXTENSIONS_REPO` in the same move; the
> text is otherwise as written.

**Goal:** Reimplement kdbx as a standalone, statically-linked Go CLI (`github.com/yarrasys/kdbx`) that is 100 % artifact- and behavior-compatible with the existing Python implementation, installable via curl/brew/`go install`, with built-in `--json`, MCP server, and hook-guard surfaces.

**Architecture:** A cobra-based CLI at the repo root delegates to focused `internal/` packages. `internal/vault` is the **sole importer** of the KDBX engine (`gokeepasslib/v3`) and exposes only plain types, keeping a single swap point. Pure-logic packages (`pointer`, `dotenv`, `guard`, `paths`, `keyfile`) are table-tested; the CLI contract (stdout/stderr/exit codes) is tested with `testscript`; artifact compatibility is proven by a pytest interop harness that round-trips vaults between Go, `pykeepass`, and `keepassxc-cli`.

**Tech Stack:** Go 1.25+ · `github.com/tobischo/gokeepasslib/v3` (MIT, KDBX4+Argon2) · `github.com/spf13/cobra` (Apache-2.0) · `github.com/gofrs/flock` (BSD-3) · `github.com/joho/godotenv` (MIT) · `github.com/modelcontextprotocol/go-sdk` (MIT) · `github.com/rogpeppe/go-internal/testscript` (BSD-3) · GoReleaser · cosign

**Spec:** [`docs/kdbx-go-standalone-design.md`](../kdbx-go-standalone-design.md) (moved into this repo alongside this plan). Section references below (C1–C10, N1–N4, D1–D10, M0–M7) point at that spec.

---

## Global Constraints

Every task's requirements implicitly include this section.

- **Repo location:** `$KDBX_REPO` (new git repo, sibling of `extensions`). Module path `github.com/yarrasys/kdbx`. License MIT.
- **Reference implementation:** the Python source at `$EXTENSIONS_REPO/skills/kdbx/`. When behavior is ambiguous, read it. Where the Python code contradicts the spec's documented contract, **the spec wins** (known case: exit 3 vs 1, spec C6).
- **Go version floor:** 1.25 (`go.mod` says `go 1.25.0`). Not 1.22 — the KDBX engine
  `gokeepasslib v3.6.2` itself declares `go 1.25.0`, as do `x/term` and `go-internal`.
  Release binaries impose no toolchain requirement on users; only building from source does.
- **Engine boundary (spec D2):** only `internal/vault/*.go` may import `gokeepasslib`. Any other package importing it is a build-breaking review failure. Enforced by a test in Task 22.
- **Secrets never leak:** no secret value may appear on argv, in JSON output, in error strings, or in a log line. Value intake is stdin / `--from-env` / interactive prompt only (spec C8).
- **Every error returned to `main` carries an exit code** via the `kdbxerr` package (Task 5). Bare `error` returns from command functions are a review failure.
- **TDD:** every task starts with a failing test. Never write implementation before a red test.
- **Formatting/lint:** `gofmt -l .` must print nothing; `go vet ./...` clean; `golangci-lint run` clean (config in Task 1).
- **Commits:** conventional-commit style (`feat:`, `fix:`, `test:`, `chore:`, `docs:`), one per task step where indicated. Never commit a `.kdbx`, `.keyx`, `.key`, or `.env` file — `.gitignore` from Task 1 blocks them.
- **No network at test time.** Tests that need a vault create one in `t.TempDir()`.
- **Exit-code table (spec C6), used verbatim throughout:** `0` ok · `1` generic scrubbed failure · `2` not-found · `3` locked/keyfile-missing/credential failure · `4` destructive op not confirmed · `5` drift · `6` vault changed underneath a write · `7` preflight.

---

## File Structure

| Path | Responsibility |
|------|----------------|
| `main.go` | 6 lines: call `cmd.Execute()`, `os.Exit` with its code |
| `cmd/root.go` | cobra root, global flags (`--env`, `--json`), version, error→exit-code mapping |
| `cmd/<op>.go` | one file per op: flag wiring + calling into `internal/`; **no business logic** |
| `internal/kdbxerr/` | typed errors carrying exit codes + stable "kind" names; scrubbing (C6, C7) |
| `internal/paths/` | KeePassXC dir resolution, `${KEEPASSXC_DIR}`/`~` expansion, sync-root detection (C1, C8) |
| `internal/ojson/` | order-preserving JSON object model (needed by C1 pointer rewrites) |
| `internal/pointer/` | `.keepassxc.json` discovery, schema, env selection, var-map edits, entry-path grammar (C1, C2) |
| `internal/envctx/` | resolved env context + `ACTIVE ENV:` banner (C1, C5) |
| `internal/secretio/` | MASK, secret intake, confirm, perms, atomic writes, clipboard (C5, C8, D10) |
| `internal/keyfile/` | mint + validate KeePass XML keyfile v2 (C4) |
| `internal/locking/` | flock + SHA-256 capture/verify (C9) |
| `internal/vault/` | **engine boundary** — open/get/list/set/trash/purge/move/rekey/create (C3) |
| `internal/dotenv/` | render (exact quoting) + parse (C5) |
| `internal/runner/` | PATH/PATHEXT lookup, env injection, spawn, signal forwarding, exit passthrough (C5) |
| `internal/jsonout/` | `--json` envelopes (N1) |
| `internal/guard/` | PreToolUse decision function + stdin/stdout shell (N3) |
| `internal/mcpserver/` | stdio MCP server, 5 read-only tools (N2) |
| `testdata/script/*.txtar` | testscript CLI-contract tests |
| `interop/` | pytest parity + round-trip harness (dev/CI only, not shipped) |
| `.goreleaser.yaml`, `install.sh`, `.github/workflows/` | release engineering (M6) |

---

## Task 1: Repo bootstrap, CLI skeleton, CI

**Files:**
- Create: `$KDBX_REPO/go.mod`, `main.go`, `cmd/root.go`, `cmd/root_test.go`, `.gitignore`, `LICENSE`, `.golangci.yml`, `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: nothing.
- Produces: `cmd.Execute() int` — runs the CLI, returns the process exit code. `cmd.Version` (string, ldflags-injected, default `"dev"`). `cmd.RootCmd() *cobra.Command` — builds a fresh root command tree (test helper; every later task registers its subcommand here).

- [ ] **Step 1: Create the repo and module**

```bash
mkdir -p $KDBX_REPO && cd $KDBX_REPO
git init -b main
go mod init github.com/yarrasys/kdbx
go get github.com/spf13/cobra@latest
```

⚠️ `go mod init` writes the **installed toolchain's** version into the `go` directive (e.g. `go 1.26.5`), not the project's floor. CI pins `go-version: "1.25.x"`, and a module declaring a newer version than the toolchain fails to build. Edit `go.mod` so the directive reads a real floor, not the toolchain version. After Task 10 `go mod tidy`
will settle it at `go 1.25.0` (the engine's own requirement).

⚠️ `go get` marks every new requirement `// indirect` until something imports it. Run `go mod tidy` once real imports exist (end of Task 10) so `cobra`, `gokeepasslib`, `term`, `flock`, `godotenv`, and the MCP SDK are correctly listed as direct dependencies — a release artifact whose whole dependency set claims to be indirect looks broken and confuses license auditing (Task 23).

- [ ] **Step 2: Write the failing test**

Create `cmd/root_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionFlagPrintsVersion(t *testing.T) {
	Version = "1.2.3"
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "kdbx 1.2.3" {
		t.Fatalf("got %q, want %q", got, "kdbx 1.2.3")
	}
}

func TestUnknownCommandIsAnError(t *testing.T) {
	root := RootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"definitely-not-a-command"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./cmd/...`
Expected: FAIL — `undefined: Version`, `undefined: RootCmd`.

- [ ] **Step 4: Implement the skeleton**

Create `main.go`:

```go
package main

import (
	"os"

	"github.com/yarrasys/kdbx/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
```

Create `cmd/root.go`:

```go
// Package cmd wires the kdbx command tree. Command files hold flag plumbing
// only — behavior lives in internal/.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags "-X github.com/yarrasys/kdbx/cmd.Version=…".
var Version = "dev"

// Global flags shared by every subcommand.
type globals struct {
	env  string
	json bool
}

var opts globals

// RootCmd builds a fresh command tree. Tests call it to get an isolated root.
func RootCmd() *cobra.Command {
	opts = globals{}
	root := &cobra.Command{
		Use:           "kdbx",
		Short:         "Per-project, per-env KeePassXC credentials",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
	}
	root.SetVersionTemplate("kdbx {{.Version}}\n")
	root.PersistentFlags().StringVar(&opts.env, "env", "", "environment name (overrides $KDBX_ENV and the pointer default)")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "machine-readable output (read operations only)")
	register(root)
	return root
}

// register hooks every subcommand onto root. Each cmd/<op>.go appends to
// registrars in its init(); this keeps RootCmd free of a growing import list.
var registrars []func(*cobra.Command)

func register(root *cobra.Command) {
	for _, r := range registrars {
		r(root)
	}
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	root := RootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "kdbx: %v\n", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./cmd/... && go vet ./...`
Expected: PASS, no vet output.

- [ ] **Step 6: Add hygiene files**

Create `.gitignore`:

```
# never commit vault material
*.kdbx
*.keyx
*.key
.env
.env.*
!.env.example

# build output
/dist/
/kdbx
/kdbx.exe
```

Create `.golangci.yml`:

```yaml
version: "2"
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - misspell
    - revive
formatters:
  enable:
    - gofmt
```

Create `LICENSE` with the standard MIT text, `Copyright (c) 2026 yarrasys`.

- [ ] **Step 7: Add CI**

Create `.github/workflows/ci.yml`:

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - run: go build ./...
      - run: go test ./... -race
      - run: go vet ./...
      - name: gofmt
        if: matrix.os == 'ubuntu-latest'
        run: test -z "$(gofmt -l .)"

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25.x"
      - uses: golangci/golangci-lint-action@v6
```

- [ ] **Step 8: Commit**

```bash
cd $KDBX_REPO
git add -A
git commit -m "chore: bootstrap kdbx Go module, cobra skeleton, CI"
```

---

## Task 2: Interop spike — the go/no-go gate (spec M0)

**Purpose:** prove `gokeepasslib` can open and write the author's *real* existing vaults before any port work. If this fails, stop and revisit spec D1.

**Files:**
- Create: `internal/vault/spike_test.go` (deleted at the end of Task 9 — it is scaffolding, its assertions get absorbed into the real vault tests)
- Create: `docs/spike-notes.md`

**Interfaces:**
- Consumes: nothing.
- Produces: `docs/spike-notes.md` — the engine facts later tasks depend on (whether `NewDatabase()` defaults to KDBX4, exact Argon2 parameter defaults, protected-custom-property round-trip behavior, recycle-bin representation).

- [ ] **Step 1: Add the engine dependency**

```bash
cd $KDBX_REPO
go get github.com/tobischo/gokeepasslib/v3@latest
```

- [ ] **Step 2: Write the spike test**

Create `internal/vault/spike_test.go`. It is skipped unless `KDBX_SPIKE_VAULT` and `KDBX_SPIKE_KEYFILE` point at a real Python-kdbx-created vault, so CI stays green.

```go
package vault

import (
	"os"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// TestSpikeOpenRealPythonVault is the M0 go/no-go gate: the Go engine must open
// a vault+keyfile produced by the Python implementation, read a protected value,
// write a new entry with a protected custom property, and re-open it.
func TestSpikeOpenRealPythonVault(t *testing.T) {
	vaultPath := os.Getenv("KDBX_SPIKE_VAULT")
	keyPath := os.Getenv("KDBX_SPIKE_KEYFILE")
	if vaultPath == "" || keyPath == "" {
		t.Skip("set KDBX_SPIKE_VAULT and KDBX_SPIKE_KEYFILE to run the interop spike")
	}

	creds, err := gokeepasslib.NewKeyCredentials(keyPath)
	if err != nil {
		t.Fatalf("NewKeyCredentials on a Python-minted v2 keyfile: %v", err)
	}

	f, err := os.Open(vaultPath)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	defer f.Close()

	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		t.Fatalf("decode Python-created vault: %v", err)
	}
	if !db.Header.IsKdbx4() {
		t.Fatalf("expected KDBX4, got header version %v", db.Header.Signature)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	t.Logf("groups at root: %d", len(db.Content.Root.Groups))
	for _, g := range db.Content.Root.Groups {
		t.Logf("group %q entries=%d subgroups=%d", g.Name, len(g.Entries), len(g.Groups))
	}
	t.Logf("RecycleBinEnabled=%v RecycleBinUUID=%x",
		db.Content.Meta.RecycleBinEnabled.Bool, db.Content.Meta.RecycleBinUUID)

	// Write path: add an entry with a protected custom property into a copy.
	e := gokeepasslib.NewEntry()
	e.Values = append(e.Values,
		gokeepasslib.ValueData{Key: "Title", Value: gokeepasslib.V{Content: "kdbx-spike"}},
		gokeepasslib.ValueData{Key: "Password", Value: gokeepasslib.V{
			Content: "spike-value", Protected: w.NewBoolWrapper(true)}},
		gokeepasslib.ValueData{Key: "CUSTOM_TOKEN", Value: gokeepasslib.V{
			Content: "custom-value", Protected: w.NewBoolWrapper(true)}},
	)
	db.Content.Root.Groups[0].Entries = append(db.Content.Root.Groups[0].Entries, e)

	out := t.TempDir() + "/spike.kdbx"
	of, err := os.Create(out)
	if err != nil {
		t.Fatalf("create out: %v", err)
	}
	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := gokeepasslib.NewEncoder(of).Encode(db); err != nil {
		t.Fatalf("encode: %v", err)
	}
	of.Close()

	// Re-open what we wrote.
	rf, err := os.Open(out)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer rf.Close()
	creds2, _ := gokeepasslib.NewKeyCredentials(keyPath)
	db2 := gokeepasslib.NewDatabase()
	db2.Credentials = creds2
	if err := gokeepasslib.NewDecoder(rf).Decode(db2); err != nil {
		t.Fatalf("decode round-trip: %v", err)
	}
	if err := db2.UnlockProtectedEntries(); err != nil {
		t.Fatalf("unlock round-trip: %v", err)
	}
	var found *gokeepasslib.Entry
	for i := range db2.Content.Root.Groups[0].Entries {
		if db2.Content.Root.Groups[0].Entries[i].GetTitle() == "kdbx-spike" {
			found = &db2.Content.Root.Groups[0].Entries[i]
		}
	}
	if found == nil {
		t.Fatal("spike entry missing after round-trip")
	}
	if got := found.GetContent("CUSTOM_TOKEN"); got != "custom-value" {
		t.Fatalf("protected custom property round-trip: got %q, want %q", got, "custom-value")
	}
	t.Logf("SPIKE OK: %s round-tripped through gokeepasslib", vaultPath)
	t.Logf("SPIKE OUTPUT VAULT (verify with pykeepass/keepassxc-cli): %s", out)
}
```

- [ ] **Step 3: Run the spike against a Python-generated vault**

Generate a **throwaway** vault with the Python implementation, then point the spike at it. Do **not** use the human's real vaults — a disposable vault created by the same code proves the same interop.

```bash
cd $KDBX_REPO
SPIKE=$(mktemp -d)
mkdir -p "$SPIKE/proj"
cat > "$SPIKE/proj/.keepassxc.json" <<'JSON'
{"project":"spike","defaultEnv":"dev","envs":{"dev":{}}}
JSON
export KEEPASSXC_DIR="$SPIKE/kpxc"
PY=$EXTENSIONS_REPO/skills/kdbx/kdbx.py
(cd "$SPIKE/proj" && uv run --locked "$PY" init)
(cd "$SPIKE/proj" && printf 'sk-spike-value\n' | uv run --locked "$PY" set api/openai --var OPENAI_API_KEY)
(cd "$SPIKE/proj" && printf 'org-spike\n' | uv run --locked "$PY" set api/openai:ORG_ID)

KDBX_SPIKE_VAULT="$SPIKE/kpxc/spike/dev.kdbx" \
KDBX_SPIKE_KEYFILE="$SPIKE/kpxc/spike/dev.keyx" \
go test ./internal/vault/ -run TestSpikeOpenRealPythonVault -v
```

Expected: PASS, with `SPIKE OK:` and a `SPIKE OUTPUT VAULT:` path in the log. Keep `$SPIKE` for Step 4.

*(Optional human confirmation, for extra assurance only — the human may re-run the same `go test` command with `KDBX_SPIKE_VAULT`/`KDBX_SPIKE_KEYFILE` pointed at one of their own vaults. The agent must never do this.)*

- [ ] **Step 4: Verify the Go-written vault from the other side**

Take the `SPIKE OUTPUT VAULT:` path printed in Step 3 and read it back with pykeepass:

```bash
uv run --with pykeepass python -c "
from pykeepass import PyKeePass
import sys
kp = PyKeePass(sys.argv[1], keyfile=sys.argv[2])
e = kp.find_entries(title='kdbx-spike', first=True)
assert e is not None, 'entry missing'
assert e.get_custom_property('CUSTOM_TOKEN') == 'custom-value', 'custom prop lost'
orig = kp.find_entries(title='openai', first=True)
assert orig is not None and orig.password == 'sk-spike-value', 'pre-existing entry damaged'
print('pykeepass round-trip OK')
" <SPIKE_OUTPUT_VAULT> "$SPIKE/kpxc/spike/dev.keyx"
```

Expected: `pykeepass round-trip OK`.

If `keepassxc-cli` is installed, also run:
`keepassxc-cli ls -R --no-password -k "$SPIKE/kpxc/spike/dev.keyx" <SPIKE_OUTPUT_VAULT>`
Expected: an entry listing including `kdbx-spike` and `openai`.

- [ ] **Step 5: Record the findings**

Create `docs/spike-notes.md` documenting, with the observed values:
- whether `gokeepasslib.NewDatabase()` defaults to KDBX4 or needs `WithDatabaseKDBXVersion4()`,
- the Argon2 KDF parameters gokeepasslib writes vs. what the Python/pykeepass vault used,
- how the Recycle Bin group appears (`Meta.RecycleBinUUID`, `RecycleBinEnabled`),
- whether `UnlockProtectedEntries` must be called before *every* read and `LockProtectedEntries` before *every* write (yes — note the pattern),
- any surprise (e.g. entry `Times` requiring `NewTimeData()` to avoid nil-pointer marshal panics).

**⚠️ Gate:** if any part of Steps 3–4 fails, STOP. Do not start Task 3. Report the failure and revisit spec D1/D2 with the human.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "test: M0 interop spike — gokeepasslib round-trips a Python-created vault"
```

---

## Task 3: `internal/paths` — OS-aware path resolution

**Files:**
- Create: `internal/paths/paths.go`, `internal/paths/paths_test.go`
- Reference: `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/paths.py`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `paths.KeepassxcDir() string` — `$KEEPASSXC_DIR`, else `%LOCALAPPDATA%\keepassxc` on Windows, else `$XDG_CONFIG_HOME/keepassxc`, else `~/.config/keepassxc`.
  - `paths.Expand(raw string) (string, error)` — substitutes `${KEEPASSXC_DIR}`, expands a leading `~`, returns an absolute cleaned path.
  - `paths.UnderSyncRoot(p string) string` — name of a cloud-sync root in the path, or `""`.

- [ ] **Step 1: Write the failing tests**

Create `internal/paths/paths_test.go`:

```go
package paths

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKeepassxcDirPrefersOverride(t *testing.T) {
	t.Setenv("KEEPASSXC_DIR", filepath.Join("/custom", "vaults"))
	if got, want := KeepassxcDir(), filepath.Join("/custom", "vaults"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeepassxcDirUsesXDGOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only default")
	}
	t.Setenv("KEEPASSXC_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, want := KeepassxcDir(), filepath.Join("/xdg", "keepassxc"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandSubstitutesKeepassxcDirToken(t *testing.T) {
	t.Setenv("KEEPASSXC_DIR", "/base")
	got, err := Expand("${KEEPASSXC_DIR}/proj/dev.kdbx")
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if want := filepath.Join("/base", "proj", "dev.kdbx"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandExpandsTilde(t *testing.T) {
	got, err := Expand("~/vaults/dev.kdbx")
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if strings.HasPrefix(got, "~") {
		t.Fatalf("tilde not expanded: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("not absolute: %q", got)
	}
}

func TestUnderSyncRootDetectsKnownRoots(t *testing.T) {
	cases := map[string]string{
		filepath.Join("/Users", "n", "OneDrive", "v.kdbx"):          "OneDrive",
		filepath.Join("/Users", "n", "Dropbox", "v.kdbx"):           "Dropbox",
		filepath.Join("/Users", "n", ".config", "kpxc", "v.kdbx"):   "",
		filepath.Join("/c", "Users", "n", "AppData", "Roaming", "x"): "AppData/Roaming",
	}
	for in, want := range cases {
		if got := UnderSyncRoot(in); got != want {
			t.Fatalf("UnderSyncRoot(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/paths/...`
Expected: FAIL — `undefined: KeepassxcDir`.

- [ ] **Step 3: Implement**

Create `internal/paths/paths.go`:

```go
// Package paths resolves the KeePassXC config directory and expands pointer paths.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var syncRoots = []string{"OneDrive", "Dropbox", "iCloud", "iCloudDrive", "Nextcloud", "Google Drive"}

// KeepassxcDir is the base directory holding <project>/<env>.{kdbx,keyx}.
func KeepassxcDir() string {
	if v := os.Getenv("KEEPASSXC_DIR"); v != "" {
		return filepath.Clean(v)
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(base, "keepassxc")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "keepassxc")
}

// Expand resolves a pointer path: ${KEEPASSXC_DIR} token, then ~, then absolutize.
func Expand(raw string) (string, error) {
	s := strings.ReplaceAll(raw, "${KEEPASSXC_DIR}", KeepassxcDir())
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		s = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(s, "~"), string(os.PathSeparator)))
	}
	abs, err := filepath.Abs(s)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// UnderSyncRoot returns the name of a cloud-sync root present in p, or "".
func UnderSyncRoot(p string) string {
	parts := map[string]bool{}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Clean(p)), "/") {
		parts[seg] = true
	}
	for _, root := range syncRoots {
		if parts[root] {
			return root
		}
	}
	if parts["AppData"] && parts["Roaming"] {
		return "AppData/Roaming"
	}
	return ""
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/paths/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/paths
git commit -m "feat(paths): OS-aware KeePassXC dir, path expansion, sync-root detection"
```

---

## Task 4: `internal/kdbxerr` — typed errors and exit codes

**Files:**
- Create: `internal/kdbxerr/kdbxerr.go`, `internal/kdbxerr/kdbxerr_test.go`
- Modify: `cmd/root.go` (replace the placeholder error handling in `Execute`)

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `kdbxerr.Error` struct with fields `Kind string`, `Code int`, `Msg string`, `Err error`; methods `Error() string`, `Unwrap() error`.
  - Constructors: `kdbxerr.NotFound(msg string, args ...any) *Error` (code 2, kind `NotFound`), `kdbxerr.Locked(...)` (3, `Locked`), `kdbxerr.NotConfirmed(...)` (4, `NotConfirmed`), `kdbxerr.Drift(...)` (5, `Drift`), `kdbxerr.Changed(...)` (6, `VaultChanged`), `kdbxerr.Preflight(...)` (7, `Preflight`), `kdbxerr.Runtime(...)` (1, `Runtime`).
  - `kdbxerr.Wrap(err error, kind string, code int, msg string, args ...any) *Error`.
  - `kdbxerr.CodeOf(err error) int` — 0 for nil, the carried code for a `*Error`, else 1.
  - `kdbxerr.KindOf(err error) string` — `""` for nil, carried kind, else `Runtime`.
  - `kdbxerr.Report(w io.Writer, op string, err error)` — writes exactly `kdbx: <op> failed: <Kind>\n`, plus the full error and stack when `KDBX_DEBUG` is non-empty (spec C7).

- [ ] **Step 1: Write the failing tests**

Create `internal/kdbxerr/kdbxerr_test.go`:

```go
package kdbxerr

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestConstructorsCarryDocumentedCodes(t *testing.T) {
	cases := []struct {
		err  *Error
		code int
		kind string
	}{
		{NotFound("nope"), 2, "NotFound"},
		{Locked("nope"), 3, "Locked"},
		{NotConfirmed("nope"), 4, "NotConfirmed"},
		{Drift("nope"), 5, "Drift"},
		{Changed("nope"), 6, "VaultChanged"},
		{Preflight("nope"), 7, "Preflight"},
		{Runtime("nope"), 1, "Runtime"},
	}
	for _, c := range cases {
		if CodeOf(c.err) != c.code {
			t.Errorf("%s: code %d, want %d", c.kind, CodeOf(c.err), c.code)
		}
		if KindOf(c.err) != c.kind {
			t.Errorf("kind %q, want %q", KindOf(c.err), c.kind)
		}
	}
}

func TestCodeOfPlainErrorIsOne(t *testing.T) {
	if got := CodeOf(errors.New("boom")); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
	if got := CodeOf(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}

func TestWrappedErrorIsUnwrappable(t *testing.T) {
	base := errors.New("underlying")
	e := Wrap(base, "Locked", 3, "opening vault")
	if !errors.Is(e, base) {
		t.Fatal("errors.Is should find the wrapped error")
	}
	if CodeOf(e) != 3 {
		t.Fatalf("code %d, want 3", CodeOf(e))
	}
}

func TestReportScrubsDetailWithoutDebug(t *testing.T) {
	t.Setenv("KDBX_DEBUG", "")
	var buf bytes.Buffer
	Report(&buf, "get", Wrap(errors.New("SUPER-SECRET-VALUE"), "NotFound", 2, "entry missing"))
	got := buf.String()
	if got != "kdbx: get failed: NotFound\n" {
		t.Fatalf("got %q, want the single scrubbed line", got)
	}
	if strings.Contains(got, "SUPER-SECRET") {
		t.Fatal("scrubbed output leaked error detail")
	}
}

func TestReportIncludesDetailWithDebug(t *testing.T) {
	t.Setenv("KDBX_DEBUG", "1")
	var buf bytes.Buffer
	Report(&buf, "get", fmt.Errorf("detailed context"))
	if !strings.Contains(buf.String(), "detailed context") {
		t.Fatalf("KDBX_DEBUG should reveal detail, got %q", buf.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/kdbxerr/...`
Expected: FAIL — `undefined: Error`.

- [ ] **Step 3: Implement**

Create `internal/kdbxerr/kdbxerr.go`:

```go
// Package kdbxerr carries a stable error "kind" and the documented exit code
// (spec C6) alongside an error, and reports failures without leaking detail
// that could contain a secret (spec C7).
package kdbxerr

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// Error is an error that knows its exit code and its user-visible kind.
type Error struct {
	Kind string
	Code int
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func newErr(kind string, code int, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Msg: fmt.Sprintf(format, args...)}
}

// NotFound — pointer, env, entry, field, or required argument missing (exit 2).
func NotFound(format string, args ...any) *Error { return newErr("NotFound", 2, format, args...) }

// Locked — vault locked, keyfile missing, credentials rejected, lock timeout (exit 3).
func Locked(format string, args ...any) *Error { return newErr("Locked", 3, format, args...) }

// NotConfirmed — a destructive operation was not confirmed (exit 4).
func NotConfirmed(format string, args ...any) *Error {
	return newErr("NotConfirmed", 4, format, args...)
}

// Drift — a mapped var does not resolve (exit 5).
func Drift(format string, args ...any) *Error { return newErr("Drift", 5, format, args...) }

// Changed — the vault changed underneath a read-modify-write (exit 6).
func Changed(format string, args ...any) *Error { return newErr("VaultChanged", 6, format, args...) }

// Preflight — bad input caught before touching the vault (exit 7).
func Preflight(format string, args ...any) *Error { return newErr("Preflight", 7, format, args...) }

// Runtime — anything else (exit 1).
func Runtime(format string, args ...any) *Error { return newErr("Runtime", 1, format, args...) }

// Wrap attaches a kind and code to an existing error.
func Wrap(err error, kind string, code int, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Msg: fmt.Sprintf(format, args...), Err: err}
}

// CodeOf returns the exit code for err: 0 if nil, the carried code for a *Error,
// otherwise 1.
func CodeOf(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if asError(err, &e) {
		return e.Code
	}
	return 1
}

// KindOf returns the stable kind name for err.
func KindOf(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if asError(err, &e) {
		return e.Kind
	}
	return "Runtime"
}

// Report writes the single scrubbed failure line for op, plus full detail when
// KDBX_DEBUG is set.
func Report(w io.Writer, op string, err error) {
	if err == nil {
		return
	}
	if os.Getenv("KDBX_DEBUG") != "" {
		fmt.Fprintf(w, "kdbx: %s failed: %v\n%s", op, err, debug.Stack())
		return
	}
	fmt.Fprintf(w, "kdbx: %s failed: %s\n", op, KindOf(err))
}
```

Add the small `asError` helper in the same file (kept separate so `errors` usage is obvious):

```go
func asError(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/kdbxerr/... -v`
Expected: all PASS.

- [ ] **Step 5: Wire it into `Execute`**

In `cmd/root.go`, replace the body of `Execute` with:

```go
// Execute runs the CLI and returns the process exit code (spec C6).
func Execute() int {
	root := RootCmd()
	err := root.Execute()
	if err == nil {
		return 0
	}
	op := "kdbx"
	if c, _, ferr := root.Find(os.Args[1:]); ferr == nil && c != nil {
		op = c.Name()
	}
	kdbxerr.Report(os.Stderr, op, err)
	return kdbxerr.CodeOf(err)
}
```

Add the import `"github.com/yarrasys/kdbx/internal/kdbxerr"` and drop the now-unused `fmt` import if nothing else uses it.

- [ ] **Step 6: Run the full suite and commit**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: PASS.

```bash
git add internal/kdbxerr cmd/root.go
git commit -m "feat(kdbxerr): typed errors carrying documented exit codes and scrubbed reporting"
```

---

## Task 5: `internal/ojson` — order-preserving JSON objects

**Why:** `.keepassxc.json` is a committed file. Go maps randomize key order, so a naive round-trip would scramble the human's file on every `set --var` (spec C1). This package is the fix.

**Files:**
- Create: `internal/ojson/ojson.go`, `internal/ojson/ojson_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Object struct{ … }` with `UnmarshalJSON([]byte) error` and `MarshalJSON() ([]byte, error)` that preserve insertion/file order at every nesting level.
  - `func Parse(b []byte) (*Object, error)`
  - `func (o *Object) Keys() []string`
  - `func (o *Object) Str(key string) string` — `""` if absent or not a string.
  - `func (o *Object) SetString(key, val string)` — updates in place, appends at the end if new.
  - `func (o *Object) Obj(key string) *Object` — nil if absent or not an object.
  - `func (o *Object) EnsureObj(key string) *Object` — creates an empty object at key if absent.
  - `func (o *Object) Delete(key string)`
  - `func (o *Object) Indent() ([]byte, error)` — 2-space-indented bytes with a trailing newline, HTML escaping disabled.

- [ ] **Step 1: Write the failing tests**

Create `internal/ojson/ojson_test.go`:

```go
package ojson

import (
	"strings"
	"testing"
)

const sample = `{
  "project": "ideas",
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vault": "~/v/dev.kdbx",
      "vars": {
        "ZED_KEY": "api/zed:password",
        "ALPHA_KEY": "api/alpha:password"
      }
    },
    "prod": {}
  }
}
`

func TestRoundTripPreservesKeyOrder(t *testing.T) {
	o, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := o.Indent()
	if err != nil {
		t.Fatalf("Indent: %v", err)
	}
	if string(got) != sample {
		t.Fatalf("round-trip changed the file:\n--- got ---\n%s\n--- want ---\n%s", got, sample)
	}
}

func TestSetStringAppendsWithoutReordering(t *testing.T) {
	o, _ := Parse([]byte(sample))
	o.Obj("envs").Obj("dev").EnsureObj("vars").SetString("NEW_KEY", "api/new:password")
	got, _ := o.Indent()
	s := string(got)
	if !strings.Contains(s, `"NEW_KEY": "api/new:password"`) {
		t.Fatalf("new var missing:\n%s", s)
	}
	iZed := strings.Index(s, "ZED_KEY")
	iAlpha := strings.Index(s, "ALPHA_KEY")
	iNew := strings.Index(s, "NEW_KEY")
	if !(iZed < iAlpha && iAlpha < iNew) {
		t.Fatalf("order not preserved (ZED then ALPHA then NEW):\n%s", s)
	}
	if !strings.HasPrefix(s, "{\n  \"project\": \"ideas\",") {
		t.Fatalf("top-level key order changed — project must remain first:\n%s", s)
	}
}

func TestSetStringOverwritesInPlace(t *testing.T) {
	o, _ := Parse([]byte(sample))
	o.Obj("envs").Obj("dev").Obj("vars").SetString("ZED_KEY", "api/zed2:password")
	got, _ := o.Indent()
	s := string(got)
	if !strings.Contains(s, `"ZED_KEY": "api/zed2:password"`) {
		t.Fatalf("value not updated:\n%s", s)
	}
	if strings.Index(s, "ZED_KEY") > strings.Index(s, "ALPHA_KEY") {
		t.Fatal("overwriting must not move the key to the end")
	}
}

func TestEnsureObjCreatesMissingLevels(t *testing.T) {
	o, _ := Parse([]byte(`{"envs":{}}`))
	o.Obj("envs").EnsureObj("staging").EnsureObj("vars").SetString("K", "p:password")
	got, _ := o.Indent()
	if !strings.Contains(string(got), `"staging"`) || !strings.Contains(string(got), `"K": "p:password"`) {
		t.Fatalf("nested creation failed:\n%s", got)
	}
}

func TestObjReturnsNilForMissingOrNonObject(t *testing.T) {
	o, _ := Parse([]byte(`{"a": "string"}`))
	if o.Obj("a") != nil {
		t.Fatal("string value must not be returned as an object")
	}
	if o.Obj("missing") != nil {
		t.Fatal("missing key must return nil")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/ojson/...`
Expected: FAIL — `undefined: Parse`.

- [ ] **Step 3: Implement**

Create `internal/ojson/ojson.go`:

```go
// Package ojson provides a JSON object model that preserves key order, so that
// rewriting a committed .keepassxc.json produces a minimal diff (spec C1).
package ojson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Object is a JSON object with stable key order. Values are either *Object
// (nested objects) or json.RawMessage (everything else, kept verbatim).
type Object struct {
	keys []string
	vals map[string]any
}

// New returns an empty Object.
func New() *Object {
	return &Object{vals: map[string]any{}}
}

// Parse decodes b into an order-preserving Object.
func Parse(b []byte) (*Object, error) {
	o := New()
	if err := json.Unmarshal(b, o); err != nil {
		return nil, err
	}
	return o, nil
}

// UnmarshalJSON decodes a JSON object, recording key order.
func (o *Object) UnmarshalJSON(b []byte) error {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("ojson: expected a JSON object, got %v", tok)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("ojson: non-string object key %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		var val any
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			child := New()
			if err := child.UnmarshalJSON(trimmed); err != nil {
				return err
			}
			val = child
		} else {
			val = raw
		}
		if _, exists := o.vals[key]; !exists {
			o.keys = append(o.keys, key)
		}
		o.vals[key] = val
	}
	_, err = dec.Token() // consume '}'
	return err
}

// MarshalJSON re-emits the object in its recorded key order.
func (o *Object) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := encodeString(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		switch v := o.vals[k].(type) {
		case *Object:
			vb, err := v.MarshalJSON()
			if err != nil {
				return nil, err
			}
			buf.Write(vb)
		case json.RawMessage:
			buf.Write(bytes.TrimSpace(v))
		default:
			return nil, fmt.Errorf("ojson: unsupported value type %T for key %q", v, k)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeString marshals s as a JSON string without HTML escaping.
func encodeString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Keys returns the keys in file order.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	out := make([]string, len(o.keys))
	copy(out, o.keys)
	return out
}

// Str returns the string value at key, or "" if absent or not a string.
func (o *Object) Str(key string) string {
	if o == nil {
		return ""
	}
	raw, ok := o.vals[key].(json.RawMessage)
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// SetString sets key to val, updating in place if the key already exists.
func (o *Object) SetString(key, val string) {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	b, err := encodeString(val)
	if err != nil {
		return
	}
	if _, exists := o.vals[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = json.RawMessage(b)
}

// Obj returns the nested object at key, or nil if absent or not an object.
func (o *Object) Obj(key string) *Object {
	if o == nil {
		return nil
	}
	child, _ := o.vals[key].(*Object)
	return child
}

// EnsureObj returns the nested object at key, creating an empty one if needed.
func (o *Object) EnsureObj(key string) *Object {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if child, ok := o.vals[key].(*Object); ok {
		return child
	}
	child := New()
	if _, exists := o.vals[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = child
	return child
}

// Delete removes key.
func (o *Object) Delete(key string) {
	if o == nil {
		return
	}
	if _, exists := o.vals[key]; !exists {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Indent renders the object with 2-space indentation and a trailing newline,
// matching Python's json.dumps(indent=2) + "\n".
func (o *Object) Indent() ([]byte, error) {
	compact, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, compact, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/ojson/... -v`
Expected: all PASS. If `TestRoundTripPreservesKeyOrder` fails on the empty-object case, confirm `json.Indent` renders `{}` as `{}` and adjust nothing else — do not special-case.

- [ ] **Step 5: Commit**

```bash
git add internal/ojson
git commit -m "feat(ojson): order-preserving JSON object model for minimal pointer diffs"
```

---

## Task 6: `internal/pointer` — discovery, schema, entry-path grammar

**Files:**
- Create: `internal/pointer/pointer.go`, `internal/pointer/entrypath.go`, `internal/pointer/pointer_test.go`, `internal/pointer/entrypath_test.go`
- Reference: `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/pointer.py`

**Interfaces:**
- Consumes: `paths.Expand`, `paths.KeepassxcDir` (Task 3); `kdbxerr.NotFound`, `kdbxerr.Preflight` (Task 4); `ojson.*` (Task 5).
- Produces:
  - `const pointer.Name = ".keepassxc.json"`
  - `func pointer.Find(startDir string) (string, error)` — walks up from `startDir`; `kdbxerr.NotFound("no .keepassxc.json found (run from inside a configured repo)")` if none.
  - `func pointer.Load(path string) (*Pointer, error)`
  - `type Pointer struct{ Path string; … }`
  - `func (p *Pointer) SelectEnv(cliEnv string) (name, source string)` — precedence `--env` > `$KDBX_ENV` > `defaultEnv` > `"dev"`; source is `"--env"`, `"$KDBX_ENV"`, or `"pointer"`.
  - `func (p *Pointer) EnvNames() []string`
  - `type EnvPaths struct{ Vault, KeyFile string; Vars map[string]string; VarOrder []string }`
  - `func (p *Pointer) ResolveEnv(env string) (EnvPaths, error)` — `kdbxerr.NotFound("env '%s' not configured in pointer", env)` if absent.
  - `func (p *Pointer) SetVar(env, varName, entryPath string)`
  - `func (p *Pointer) RepointVars(env, srcEntry, dstEntry string) int` — repoints mappings whose entry part equals `srcEntry`, preserving any `:field` suffix; returns the count changed.
  - `func (p *Pointer) Save() error` — atomic `.tmp`+rename, 2-space indent, trailing newline.
  - `func pointer.ParseEntryPath(raw string) (groupPath []string, title, field string, err error)`
  - `func pointer.ValidVarName(name string) bool` — `^[A-Z_][A-Z0-9_]*$`.

- [ ] **Step 1: Write the failing entry-path tests**

Create `internal/pointer/entrypath_test.go`:

```go
package pointer

import (
	"reflect"
	"testing"
)

func TestParseEntryPathDefaults(t *testing.T) {
	g, title, field, err := ParseEntryPath("api/openai")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(g, []string{"api"}) {
		t.Fatalf("group %v, want [api]", g)
	}
	if title != "openai" || field != "password" {
		t.Fatalf("title=%q field=%q, want openai/password", title, field)
	}
}

func TestParseEntryPathNestedGroupsAndField(t *testing.T) {
	g, title, field, err := ParseEntryPath("a/b/c/Title:CUSTOM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(g, []string{"a", "b", "c"}) {
		t.Fatalf("group %v", g)
	}
	if title != "Title" || field != "CUSTOM" {
		t.Fatalf("title=%q field=%q", title, field)
	}
}

func TestParseEntryPathBareTitleHasEmptyGroup(t *testing.T) {
	g, title, _, err := ParseEntryPath("Solo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g) != 0 || title != "Solo" {
		t.Fatalf("group=%v title=%q, want empty group and Solo", g, title)
	}
}

func TestParseEntryPathRejectsAmbiguous(t *testing.T) {
	for _, bad := range []string{"a:b:c", "api/openai:", "api//openai", "/api/openai", "api/openai/"} {
		if _, _, _, err := ParseEntryPath(bad); err == nil {
			t.Errorf("ParseEntryPath(%q) should have failed", bad)
		}
	}
}

func TestValidVarName(t *testing.T) {
	ok := []string{"A", "_A", "OPENAI_API_KEY", "K9"}
	bad := []string{"", "9K", "lower", "WITH-DASH", "WITH SPACE", "WITH.DOT"}
	for _, s := range ok {
		if !ValidVarName(s) {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range bad {
		if ValidVarName(s) {
			t.Errorf("%q should be invalid", s)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/pointer/...`
Expected: FAIL — `undefined: ParseEntryPath`.

- [ ] **Step 3: Implement the grammar**

Create `internal/pointer/entrypath.go`:

```go
package pointer

import (
	"regexp"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

var varNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidVarName reports whether name is a legal environment-variable name for a
// pointer var mapping (spec C2).
func ValidVarName(name string) bool { return varNameRe.MatchString(name) }

// ParseEntryPath splits "group/sub/Title[:field]" into its parts. The field
// defaults to "password". A name component may not contain ':' and no component
// may be empty (spec C2).
func ParseEntryPath(raw string) (groupPath []string, title, field string, err error) {
	field = "password"
	body := raw
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		body, field = raw[:i], raw[i+1:]
		if strings.Contains(body, ":") {
			return nil, "", "", kdbxerr.Preflight("ambiguous path (multiple ':'): %q", raw)
		}
		if field == "" {
			return nil, "", "", kdbxerr.Preflight("empty field: %q", raw)
		}
	}
	segments := strings.Split(body, "/")
	for _, seg := range segments {
		if seg == "" {
			return nil, "", "", kdbxerr.Preflight("empty path component: %q", raw)
		}
	}
	title = segments[len(segments)-1]
	groupPath = segments[:len(segments)-1]
	return groupPath, title, field, nil
}

// EntryOf returns the entry portion of a var mapping value (everything before
// an optional ":field").
func EntryOf(mapping string) string {
	if i := strings.Index(mapping, ":"); i >= 0 {
		return mapping[:i]
	}
	return mapping
}
```

- [ ] **Step 4: Run to verify the grammar tests pass**

Run: `go test ./internal/pointer/ -run 'ParseEntryPath|ValidVarName' -v`
Expected: PASS.

- [ ] **Step 5: Write the failing pointer tests**

Create `internal/pointer/pointer_test.go`:

```go
package pointer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePointer(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, Name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const fixture = `{
  "project": "ideas",
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vault": "${KEEPASSXC_DIR}/ideas/dev.kdbx",
      "keyFile": "${KEEPASSXC_DIR}/ideas/dev.keyx",
      "vars": {
        "OPENAI_API_KEY": "api/openai:password"
      }
    },
    "prod": {}
  }
}
`

func TestFindWalksUpFromNestedDir(t *testing.T) {
	root := t.TempDir()
	writePointer(t, root, fixture)
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	wantSuffix := filepath.Join(filepath.Base(root), Name)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("found %q, want a path ending in %q", got, wantSuffix)
	}
}

func TestFindReturnsNotFoundExitCode2(t *testing.T) {
	dir := t.TempDir()
	_, err := Find(dir)
	if err == nil {
		t.Fatal("expected an error when no pointer exists")
	}
	if !strings.Contains(err.Error(), "no .keepassxc.json found") {
		t.Fatalf("message %q lacks the documented wording", err.Error())
	}
}

func TestSelectEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	p, err := Load(writePointer(t, dir, fixture))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KDBX_ENV", "")
	if name, src := p.SelectEnv(""); name != "dev" || src != "pointer" {
		t.Fatalf("default: got %q/%q, want dev/pointer", name, src)
	}
	t.Setenv("KDBX_ENV", "prod")
	if name, src := p.SelectEnv(""); name != "prod" || src != "$KDBX_ENV" {
		t.Fatalf("env var: got %q/%q, want prod/$KDBX_ENV", name, src)
	}
	if name, src := p.SelectEnv("staging"); name != "staging" || src != "--env" {
		t.Fatalf("flag: got %q/%q, want staging/--env", name, src)
	}
}

func TestResolveEnvExpandsTokensAndReadsVars(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, fixture))

	ep, err := p.ResolveEnv("dev")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if want := filepath.Join(base, "ideas", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
	if want := filepath.Join(base, "ideas", "dev.keyx"); ep.KeyFile != want {
		t.Fatalf("keyfile %q, want %q", ep.KeyFile, want)
	}
	if ep.Vars["OPENAI_API_KEY"] != "api/openai:password" {
		t.Fatalf("vars %v", ep.Vars)
	}
}

func TestResolveEnvDefaultsPathsFromProject(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, `{"project":"ideas","defaultEnv":"dev","envs":{"dev":{}}}`))

	ep, err := p.ResolveEnv("dev")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if want := filepath.Join(base, "ideas", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
}

func TestResolveEnvProjectFallsBackToPointerDirName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myrepo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, `{"defaultEnv":"dev","envs":{"dev":{}}}`))

	ep, _ := p.ResolveEnv("dev")
	if want := filepath.Join(base, "myrepo", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
}

func TestResolveEnvUnknownEnvIsNotFound(t *testing.T) {
	dir := t.TempDir()
	p, _ := Load(writePointer(t, dir, fixture))
	if _, err := p.ResolveEnv("nope"); err == nil {
		t.Fatal("expected an error for an unconfigured env")
	}
}

func TestSetVarAndSavePreservesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, fixture)
	p, _ := Load(path)
	p.SetVar("dev", "NEW_KEY", "api/new:password")
	if err := p.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.HasPrefix(s, "{\n  \"project\": \"ideas\",") {
		t.Fatalf("top-level key order or indentation changed:\n%s", s)
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Fatalf("missing trailing newline:\n%q", s)
	}
	if !strings.Contains(s, `"NEW_KEY": "api/new:password"`) {
		t.Fatalf("var not written:\n%s", s)
	}
}

func TestSetVarCreatesMissingEnvAndVarsLevels(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, fixture)
	p, _ := Load(path)
	p.SetVar("prod", "PROD_KEY", "api/prod:password")
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := Load(path)
	ep, err := reloaded.ResolveEnv("prod")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Vars["PROD_KEY"] != "api/prod:password" {
		t.Fatalf("vars %v", ep.Vars)
	}
}

func TestRepointVarsPreservesFieldSuffix(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, `{
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vars": {
        "A": "old/entry:password",
        "B": "old/entry:CUSTOM",
        "C": "other/entry:password"
      }
    }
  }
}
`)
	p, _ := Load(path)
	n := p.RepointVars("dev", "old/entry", "new/place")
	if n != 2 {
		t.Fatalf("repointed %d, want 2", n)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := Load(path)
	ep, _ := reloaded.ResolveEnv("dev")
	if ep.Vars["A"] != "new/place:password" {
		t.Fatalf("A = %q", ep.Vars["A"])
	}
	if ep.Vars["B"] != "new/place:CUSTOM" {
		t.Fatalf("B = %q (field suffix must be preserved)", ep.Vars["B"])
	}
	if ep.Vars["C"] != "other/entry:password" {
		t.Fatalf("C = %q (unrelated mapping must not change)", ep.Vars["C"])
	}
}

func TestEnvNamesInFileOrder(t *testing.T) {
	dir := t.TempDir()
	p, _ := Load(writePointer(t, dir, fixture))
	got := p.EnvNames()
	if len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Fatalf("EnvNames = %v, want [dev prod]", got)
	}
}
```

- [ ] **Step 6: Run to verify failure**

Run: `go test ./internal/pointer/...`
Expected: FAIL — `undefined: Find`, `undefined: Load`.

- [ ] **Step 7: Implement**

Create `internal/pointer/pointer.go`:

```go
// Package pointer discovers, reads, and rewrites the committed .keepassxc.json
// pointer file (spec C1).
package pointer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/ojson"
	"github.com/yarrasys/kdbx/internal/paths"
)

// Name is the pointer file's fixed name.
const Name = ".keepassxc.json"

// Pointer is a loaded pointer file that can be edited and saved without
// disturbing key order or formatting.
type Pointer struct {
	Path string
	root *ojson.Object
}

// EnvPaths are the resolved artifacts for one environment.
type EnvPaths struct {
	Vault    string
	KeyFile  string
	Vars     map[string]string
	VarOrder []string
}

// Find walks up from startDir looking for the pointer file.
func Find(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", kdbxerr.Wrap(err, "Runtime", 1, "resolving %s", startDir)
	}
	for {
		cand := filepath.Join(dir, Name)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", kdbxerr.NotFound("no .keepassxc.json found (run from inside a configured repo)")
		}
		dir = parent
	}
}

// Load reads and parses the pointer file at path.
func Load(path string) (*Pointer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "NotFound", 2, "reading %s", filepath.Base(path))
	}
	root, err := ojson.Parse(b)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "Preflight", 7, "%s is not valid JSON", filepath.Base(path))
	}
	return &Pointer{Path: path, root: root}, nil
}

// Dir is the directory holding the pointer file (the project root).
func (p *Pointer) Dir() string { return filepath.Dir(p.Path) }

// Project is the pointer's project name, defaulting to the pointer directory's name.
func (p *Pointer) Project() string {
	if v := p.root.Str("project"); v != "" {
		return v
	}
	return filepath.Base(p.Dir())
}

// SelectEnv resolves the active environment and where the choice came from.
func (p *Pointer) SelectEnv(cliEnv string) (name, source string) {
	if cliEnv != "" {
		return cliEnv, "--env"
	}
	if v := os.Getenv("KDBX_ENV"); v != "" {
		return v, "$KDBX_ENV"
	}
	if v := p.root.Str("defaultEnv"); v != "" {
		return v, "pointer"
	}
	return "dev", "pointer"
}

// EnvNames lists configured environments in file order.
func (p *Pointer) EnvNames() []string {
	envs := p.root.Obj("envs")
	if envs == nil {
		return nil
	}
	return envs.Keys()
}

// ResolveEnv returns the artifact paths and var mappings for env.
func (p *Pointer) ResolveEnv(env string) (EnvPaths, error) {
	envs := p.root.Obj("envs")
	if envs == nil {
		return EnvPaths{}, kdbxerr.NotFound("env '%s' not configured in pointer", env)
	}
	cfg := envs.Obj(env)
	if cfg == nil {
		found := false
		for _, k := range envs.Keys() {
			if k == env {
				found = true
			}
		}
		if !found {
			return EnvPaths{}, kdbxerr.NotFound("env '%s' not configured in pointer", env)
		}
		cfg = ojson.New()
	}

	defaultDir := filepath.Join(paths.KeepassxcDir(), p.Project())
	vault, err := resolveArtifact(cfg.Str("vault"), filepath.Join(defaultDir, env+".kdbx"))
	if err != nil {
		return EnvPaths{}, err
	}
	keyfile, err := resolveArtifact(cfg.Str("keyFile"), filepath.Join(defaultDir, env+".keyx"))
	if err != nil {
		return EnvPaths{}, err
	}

	out := EnvPaths{Vault: vault, KeyFile: keyfile, Vars: map[string]string{}}
	if vars := cfg.Obj("vars"); vars != nil {
		for _, k := range vars.Keys() {
			out.Vars[k] = vars.Str(k)
			out.VarOrder = append(out.VarOrder, k)
		}
	}
	return out, nil
}

func resolveArtifact(configured, fallback string) (string, error) {
	if configured == "" {
		return filepath.Clean(fallback), nil
	}
	return paths.Expand(configured)
}

// SetVar records varName -> entryPath under env, creating levels as needed.
func (p *Pointer) SetVar(env, varName, entryPath string) {
	p.root.EnsureObj("envs").EnsureObj(env).EnsureObj("vars").SetString(varName, entryPath)
}

// RepointVars rewrites env's mappings that point at srcEntry so they point at
// dstEntry, preserving each mapping's ":field" suffix. Returns how many changed.
func (p *Pointer) RepointVars(env, srcEntry, dstEntry string) int {
	envs := p.root.Obj("envs")
	if envs == nil {
		return 0
	}
	cfg := envs.Obj(env)
	if cfg == nil {
		return 0
	}
	vars := cfg.Obj("vars")
	if vars == nil {
		return 0
	}
	changed := 0
	for _, k := range vars.Keys() {
		val := vars.Str(k)
		if EntryOf(val) != srcEntry {
			continue
		}
		vars.SetString(k, dstEntry+strings.TrimPrefix(val, srcEntry))
		changed++
	}
	return changed
}

// Save atomically rewrites the pointer file with 2-space indentation.
func (p *Pointer) Save() error {
	data, err := p.root.Indent()
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "rendering %s", Name)
	}
	tmp := p.Path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "writing %s", Name)
	}
	if err := os.Rename(tmp, p.Path); err != nil {
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "replacing %s", Name)
	}
	return nil
}
```

- [ ] **Step 8: Run to verify pass**

Run: `go test ./internal/pointer/... -v`
Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/pointer
git commit -m "feat(pointer): discovery, env resolution, entry-path grammar, order-preserving var edits"
```

---

## Task 7: `internal/envctx` + the `envs` command + `--json`

**Files:**
- Create: `internal/envctx/envctx.go`, `internal/envctx/envctx_test.go`, `internal/jsonout/jsonout.go`, `internal/jsonout/jsonout_test.go`, `cmd/envs.go`
- Modify: `cmd/root.go` (nothing structural — subcommands self-register via `registrars`)

**Interfaces:**
- Consumes: `pointer.*` (Task 6), `kdbxerr.*` (Task 4).
- Produces:
  - `type envctx.Context struct{ Env, Source string; Vault, KeyFile string; Vars map[string]string; VarOrder []string; Pointer *pointer.Pointer }`
  - `func envctx.Resolve(cliEnv, startDir string) (*Context, error)`
  - `func (c *Context) WriteBanner(w io.Writer)` — writes `ACTIVE ENV: <env>  vault=<path>  (source: <src>)\n` (two spaces before `vault=`, exactly as Python).
  - `func jsonout.Write(w io.Writer, v any) error` — compact JSON + newline, HTML escaping off.
  - `func jsonout.WriteError(w io.Writer, op string, err error) error` — `{"error":{"op":…,"exit":…,"kind":…}}`.

- [ ] **Step 1: Write the failing tests**

Create `internal/envctx/envctx_test.go`:

```go
package envctx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupProject(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveUsesPointerDefault(t *testing.T) {
	t.Setenv("KDBX_ENV", "")
	t.Setenv("KEEPASSXC_DIR", t.TempDir())
	dir := setupProject(t, `{"project":"p","defaultEnv":"dev","envs":{"dev":{"vars":{"K":"api/k:password"}}}}`)
	c, err := Resolve("", dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.Env != "dev" || c.Source != "pointer" {
		t.Fatalf("env=%q source=%q", c.Env, c.Source)
	}
	if c.Vars["K"] != "api/k:password" {
		t.Fatalf("vars %v", c.Vars)
	}
}

func TestResolveNoPointerIsNotFound(t *testing.T) {
	if _, err := Resolve("", t.TempDir()); err == nil {
		t.Fatal("expected NotFound when no pointer exists")
	}
}

func TestBannerFormatMatchesPython(t *testing.T) {
	t.Setenv("KDBX_ENV", "")
	t.Setenv("KEEPASSXC_DIR", t.TempDir())
	dir := setupProject(t, `{"project":"p","defaultEnv":"dev","envs":{"dev":{}}}`)
	c, _ := Resolve("", dir)
	var buf bytes.Buffer
	c.WriteBanner(&buf)
	got := buf.String()
	if !strings.HasPrefix(got, "ACTIVE ENV: dev  vault=") {
		t.Fatalf("banner prefix wrong: %q", got)
	}
	if !strings.HasSuffix(got, "(source: pointer)\n") {
		t.Fatalf("banner suffix wrong: %q", got)
	}
}
```

Create `internal/jsonout/jsonout_test.go`:

```go
package jsonout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

func TestWriteEmitsCompactJSONWithNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteError(&buf, "check", kdbxerr.Drift("drifted")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{`"op":"check"`, `"exit":5`, `"kind":"Drift"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("envelope %q missing %q", got, want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/envctx/... ./internal/jsonout/...`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement `jsonout`**

Create `internal/jsonout/jsonout.go`:

```go
// Package jsonout renders the --json output envelopes (spec N1). It never
// carries a secret value.
package jsonout

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Write emits v as a single compact JSON line.
func Write(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Op   string `json:"op"`
	Exit int    `json:"exit"`
	Kind string `json:"kind"`
}

// WriteError emits the failure envelope for op.
func WriteError(w io.Writer, op string, err error) error {
	return Write(w, errEnvelope{Error: errBody{
		Op:   op,
		Exit: kdbxerr.CodeOf(err),
		Kind: kdbxerr.KindOf(err),
	}})
}
```

- [ ] **Step 4: Implement `envctx`**

Create `internal/envctx/envctx.go`:

```go
// Package envctx resolves the active environment for a command invocation and
// prints the ACTIVE ENV banner (spec C1, C5).
package envctx

import (
	"fmt"
	"io"

	"github.com/yarrasys/kdbx/internal/pointer"
)

// Context is the resolved environment for one command invocation.
type Context struct {
	Env      string
	Source   string
	Vault    string
	KeyFile  string
	Vars     map[string]string
	VarOrder []string
	Pointer  *pointer.Pointer
}

// Resolve finds the pointer from startDir, selects the environment, and
// resolves its artifact paths and var mappings.
func Resolve(cliEnv, startDir string) (*Context, error) {
	path, err := pointer.Find(startDir)
	if err != nil {
		return nil, err
	}
	p, err := pointer.Load(path)
	if err != nil {
		return nil, err
	}
	env, source := p.SelectEnv(cliEnv)
	ep, err := p.ResolveEnv(env)
	if err != nil {
		return nil, err
	}
	return &Context{
		Env: env, Source: source,
		Vault: ep.Vault, KeyFile: ep.KeyFile,
		Vars: ep.Vars, VarOrder: ep.VarOrder,
		Pointer: p,
	}, nil
}

// WriteBanner tells the operator which vault is about to be touched.
func (c *Context) WriteBanner(w io.Writer) {
	fmt.Fprintf(w, "ACTIVE ENV: %s  vault=%s  (source: %s)\n", c.Env, c.Vault, c.Source)
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/envctx/... ./internal/jsonout/... -v`
Expected: all PASS.

- [ ] **Step 6: Add the `envs` command**

Create `cmd/envs.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/pointer"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "envs",
			Short: "List configured environments and mark the active one",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runEnvs(c)
			},
		})
	})
}

type envRow struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

func runEnvs(c *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, err := pointer.Find(cwd)
	if err != nil {
		return err
	}
	p, err := pointer.Load(path)
	if err != nil {
		return err
	}
	active, source := p.SelectEnv(opts.env)

	if opts.json {
		rows := []envRow{}
		for _, name := range p.EnvNames() {
			rows = append(rows, envRow{Name: name, Active: name == active})
		}
		return jsonout.Write(c.OutOrStdout(), map[string]any{"envs": rows, "source": source})
	}
	for _, name := range p.EnvNames() {
		marker := "  "
		if name == active {
			marker = "* "
		}
		fmt.Fprintf(c.OutOrStdout(), "%s%s\n", marker, name)
	}
	fmt.Fprintf(c.ErrOrStderr(), "active: %s (source: %s)\n", active, source)
	return nil
}

var _ = envctx.Resolve // envs deliberately does not require resolvable artifacts
```

Remove the trailing `var _ =` line if `envctx` is not otherwise imported — it exists only to document intent; prefer deleting both it and the import.

- [ ] **Step 7: Add the CLI-contract test harness**

```bash
go get github.com/rogpeppe/go-internal/testscript@latest
```

Create `main_test.go` at the repo root:

```go
package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/yarrasys/kdbx/cmd"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"kdbx": cmd.Execute,
	}))
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(e *testscript.Env) error {
			e.Setenv("KEEPASSXC_DIR", e.WorkDir+"/kpxc")
			e.Setenv("KDBX_ENV", "")
			return nil
		},
	})
}
```

Create `testdata/script/envs.txtar`:

```
# envs lists configured environments, marks the active one, and reports the source
exec kdbx envs
stdout '^\* dev$'
stdout '^  prod$'
stderr 'active: dev \(source: pointer\)'

# $KDBX_ENV overrides the pointer default
env KDBX_ENV=prod
exec kdbx envs
stdout '^\* prod$'
stderr 'active: prod \(source: \$KDBX_ENV\)'

# --env beats $KDBX_ENV
exec kdbx --env dev envs
stderr 'active: dev \(source: --env\)'

# --json emits a machine-readable listing
env KDBX_ENV=
exec kdbx --json envs
stdout '"envs":\[\{"name":"dev","active":true\},\{"name":"prod","active":false\}\]'

# no pointer anywhere -> exit 2
cd $WORK/elsewhere
! exec kdbx envs
stderr 'no .keepassxc.json found'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {},
    "prod": {}
  }
}
-- elsewhere/keep --
placeholder
```

⚠️ The `elsewhere` directory is inside `$WORK`, which is itself inside a temp dir — confirm no `.keepassxc.json` exists above it. If testscript's work dir is nested under a directory that has one, set `e.Setenv("HOME", e.WorkDir)` in Setup and create `elsewhere` outside `$WORK` instead.

- [ ] **Step 8: Run the contract test**

Run: `go test . -run TestScripts -v`
Expected: PASS, with `envs.txtar` reported.

- [ ] **Step 9: Commit**

```bash
git add internal/envctx internal/jsonout cmd/envs.go main_test.go testdata go.mod go.sum
git commit -m "feat(envs): env resolution, ACTIVE ENV banner, --json envelopes, testscript harness"
```

---

## Task 8: `internal/secretio` — masked output, secret intake, perms, atomic writes

**Files:**
- Create: `internal/secretio/secretio.go`, `internal/secretio/perms.go`, `internal/secretio/perms_windows.go`, `internal/secretio/secretio_test.go`
- Reference: `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/secretio.py`

**Interfaces:**
- Consumes: `kdbxerr.*` (Task 4).
- Produces:
  - `const secretio.Mask = "(set, hidden)"`
  - `type ReadOpts struct{ FromEnv string; Raw bool; Stdin io.Reader; IsTTY bool; PromptFn func(string) (string, error) }`
  - `func secretio.ReadSecret(o ReadOpts) (string, error)` — `--from-env` › interactive prompt+confirm (when `IsTTY`) › stdin; strips one trailing `\n` or `\r\n` unless `Raw`; empty/whitespace-only stdin → `kdbxerr.Runtime` with the documented stderr guidance.
  - `func secretio.Confirm(prompt string, in io.Reader, out io.Writer, isTTY bool) bool` — non-TTY always refuses, printing `<prompt>: refused — needs an interactive terminal to confirm`.
  - `func secretio.RestrictPerms(path string) error` — 0600 POSIX / owner-only ACL on Windows.
  - `func secretio.AtomicWriteSecret(path string, data []byte) error` — create 0600, write, restrict.
  - `func secretio.IsTerminal(f *os.File) bool`

- [ ] **Step 1: Write the failing tests**

Create `internal/secretio/secretio_test.go`:

```go
package secretio

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadSecretFromEnv(t *testing.T) {
	t.Setenv("MY_SECRET_SRC", "s3cr3t")
	got, err := ReadSecret(ReadOpts{FromEnv: "MY_SECRET_SRC"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "s3cr3t" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretFromEnvUnsetIsAnError(t *testing.T) {
	os.Unsetenv("DEFINITELY_UNSET_VAR")
	if _, err := ReadSecret(ReadOpts{FromEnv: "DEFINITELY_UNSET_VAR"}); err == nil {
		t.Fatal("expected an error when --from-env names an unset variable")
	}
}

func TestReadSecretFromStdinStripsOneTrailingNewline(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("got %q, want %q", got, "value")
	}
}

func TestReadSecretStripsCRLF(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\r\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretRawKeepsTrailingNewline(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\n"), Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value\n" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretEmptyStdinIsAnError(t *testing.T) {
	if _, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("   \n")}); err == nil {
		t.Fatal("expected an error for whitespace-only stdin")
	}
}

func TestReadSecretPromptsAndConfirmsOnTTY(t *testing.T) {
	calls := 0
	got, err := ReadSecret(ReadOpts{
		IsTTY: true,
		PromptFn: func(prompt string) (string, error) {
			calls++
			return "typed", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "typed" {
		t.Fatalf("got %q", got)
	}
	if calls != 2 {
		t.Fatalf("prompt called %d times, want 2 (value + confirm)", calls)
	}
}

func TestReadSecretMismatchedConfirmationFails(t *testing.T) {
	seq := []string{"one", "two"}
	i := 0
	_, err := ReadSecret(ReadOpts{
		IsTTY: true,
		PromptFn: func(string) (string, error) {
			v := seq[i]
			i++
			return v, nil
		},
	})
	if err == nil {
		t.Fatal("expected an error when the confirmation does not match")
	}
}

func TestConfirmRefusesWithoutTTY(t *testing.T) {
	var errBuf bytes.Buffer
	if Confirm("purge it?", strings.NewReader("y\n"), &errBuf, false) {
		t.Fatal("must refuse without a TTY")
	}
	if !strings.Contains(errBuf.String(), "needs an interactive terminal to confirm") {
		t.Fatalf("stderr %q lacks the documented wording", errBuf.String())
	}
}

func TestConfirmAcceptsYOnTTY(t *testing.T) {
	var errBuf bytes.Buffer
	if !Confirm("go?", strings.NewReader("y\n"), &errBuf, true) {
		t.Fatal("y should confirm")
	}
	if Confirm("go?", strings.NewReader("n\n"), &errBuf, true) {
		t.Fatal("n should refuse")
	}
	if Confirm("go?", strings.NewReader("\n"), &errBuf, true) {
		t.Fatal("empty should refuse (default N)")
	}
}

func TestAtomicWriteSecretIsOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "secret.txt")
	if err := AtomicWriteSecret(p, []byte("data")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "data" {
		t.Fatalf("content %q err %v", b, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode %o, want 600", perm)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/secretio/...`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement the core**

Create `internal/secretio/secretio.go`:

```go
// Package secretio handles every path a secret value takes in or out of kdbx:
// intake that never crosses argv, masked display, confirmations, and 0600
// atomic writes (spec C5, C8).
package secretio

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"golang.org/x/term"
)

// Mask is the constant stand-in printed instead of a value. It encodes neither
// length nor prefix.
const Mask = "(set, hidden)"

// ReadOpts describes where a secret value should come from.
type ReadOpts struct {
	FromEnv  string
	Raw      bool
	Stdin    io.Reader
	IsTTY    bool
	PromptFn func(prompt string) (string, error)
}

// ReadSecret obtains a secret without it ever crossing argv.
func ReadSecret(o ReadOpts) (string, error) {
	if o.FromEnv != "" {
		v, ok := os.LookupEnv(o.FromEnv)
		if !ok {
			return "", kdbxerr.Preflight("--from-env %s is not set", o.FromEnv)
		}
		return trim(v, o.Raw), nil
	}

	if o.IsTTY {
		prompt := o.PromptFn
		if prompt == nil {
			prompt = promptHidden
		}
		v, err := prompt("value: ")
		if err != nil {
			return "", kdbxerr.Wrap(err, "Runtime", 1, "reading value")
		}
		again, err := prompt("confirm: ")
		if err != nil {
			return "", kdbxerr.Wrap(err, "Runtime", 1, "reading confirmation")
		}
		if v != again {
			return "", kdbxerr.Runtime("values did not match")
		}
		return v, nil
	}

	src := o.Stdin
	if src == nil {
		src = os.Stdin
	}
	b, err := io.ReadAll(src)
	if err != nil {
		return "", kdbxerr.Wrap(err, "Runtime", 1, "reading stdin")
	}
	v := string(b)
	if strings.TrimSpace(v) == "" {
		fmt.Fprint(os.Stderr, "kdbx: no value provided — stdin is empty "+
			"(pipe a value, use --from-env, or run interactively from a terminal)\n")
		return "", kdbxerr.Runtime("no value provided via stdin")
	}
	return trim(v, o.Raw), nil
}

func trim(v string, raw bool) string {
	if raw {
		return v
	}
	v = strings.TrimSuffix(v, "\n")
	return strings.TrimSuffix(v, "\r")
}

func promptHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// Confirm asks for interactive y/N approval of an irreversible operation. There
// is no non-interactive override: writes are a human role.
func Confirm(prompt string, in io.Reader, errOut io.Writer, isTTY bool) bool {
	if !isTTY {
		fmt.Fprintf(errOut, "%s: refused — needs an interactive terminal to confirm\n", prompt)
		return false
	}
	fmt.Fprintf(errOut, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// AtomicWriteSecret writes data to path with owner-only permissions.
func AtomicWriteSecret(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating %s", path)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return kdbxerr.Wrap(err, "Runtime", 1, "writing %s", path)
	}
	if err := f.Close(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "closing %s", path)
	}
	return RestrictPerms(path)
}
```

Install the terminal dependency:

```bash
go get golang.org/x/term@latest
```

- [ ] **Step 4: Implement permissions per platform**

Create `internal/secretio/perms.go`:

```go
//go:build !windows

package secretio

import (
	"os"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// RestrictPerms makes path readable and writable only by its owner.
func RestrictPerms(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "restricting permissions on %s", path)
	}
	return nil
}
```

Create `internal/secretio/perms_windows.go`:

```go
//go:build windows

package secretio

import (
	"os"
	"os/exec"
)

// RestrictPerms strips inherited ACLs and grants the current user full control.
// Failure is not fatal: the file already exists with default ACLs, and a hard
// failure here would make kdbx unusable on locked-down machines.
func RestrictPerms(path string) error {
	user := os.Getenv("USERNAME")
	if user == "" {
		return nil
	}
	cmd := exec.Command("icacls", path, "/inheritance:r", "/grant:r", user+":F")
	_ = cmd.Run()
	return nil
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/secretio/... -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/secretio go.mod go.sum
git commit -m "feat(secretio): transcript-safe secret intake, masked constant, confirmations, 0600 writes"
```

---

## Task 9: `internal/keyfile` — mint and validate KeePass XML keyfile v2

**Files:**
- Create: `internal/keyfile/keyfile.go`, `internal/keyfile/keyfile_test.go`
- Reference: `generate_keyfile_xml` / `mint_keyfile` in `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/vault.py`

**Note from the Task 2 spike:** `gokeepasslib` parses KeyFile v2 correctly (hex data + first-4-bytes-of-SHA256 hash attribute), so **loading** delegates to the engine inside `internal/vault`. This package owns **minting** (the engine cannot create keyfiles) and a standalone validator used by tests and preflight checks.

**Interfaces:**
- Consumes: `secretio.AtomicWriteSecret` (Task 8), `kdbxerr.*` (Task 4).
- Produces:
  - `func keyfile.RenderXML(key []byte) string` — byte-identical to the Python `generate_keyfile_xml`.
  - `func keyfile.Mint(path string) error` — 32 random bytes → XML v2 → atomic 0600 write; refuses to overwrite an existing file.
  - `func keyfile.Validate(path string) error` — exists, parses, hash matches; `kdbxerr.Locked` on any failure (spec C6 exit 3).

- [ ] **Step 1: Write the failing tests**

Create `internal/keyfile/keyfile_test.go`:

```go
package keyfile

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderXMLMatchesPythonByteForByte(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	sum := sha256.Sum256(key)
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<KeyFile>\n\t<Meta>\n\t\t<Version>2.0</Version>\n\t</Meta>\n" +
		"\t<Key>\n\t\t<Data Hash=\"" + strings.ToUpper(hex.EncodeToString(sum[:4])) + "\">" +
		strings.ToUpper(hex.EncodeToString(key)) + "</Data>\n\t</Key>\n</KeyFile>\n"

	if got := RenderXML(key); got != want {
		t.Fatalf("keyfile XML drifted from the Python format:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestMintCreatesValidOwnerOnlyKeyfile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev.keyx")
	if err := Mint(p); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate on freshly minted keyfile: %v", err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "<Version>2.0</Version>") {
		t.Fatalf("not a v2 keyfile:\n%s", b)
	}
}

func TestMintIsUnpredictable(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.keyx")
	b := filepath.Join(dir, "b.keyx")
	if err := Mint(a); err != nil {
		t.Fatal(err)
	}
	if err := Mint(b); err != nil {
		t.Fatal(err)
	}
	ab, _ := os.ReadFile(a)
	bb, _ := os.ReadFile(b)
	if string(ab) == string(bb) {
		t.Fatal("two minted keyfiles are identical — the key is not random")
	}
}

func TestMintRefusesToOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev.keyx")
	if err := Mint(p); err != nil {
		t.Fatal(err)
	}
	if err := Mint(p); err == nil {
		t.Fatal("Mint must refuse to overwrite an existing keyfile")
	}
}

func TestValidateRejectsCorruptHash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.keyx")
	body := RenderXML(make([]byte, 32))
	body = strings.Replace(body, "Hash=\"", "Hash=\"AA", 1)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err == nil {
		t.Fatal("Validate must reject a keyfile whose hash does not match")
	}
}

func TestValidateMissingFileIsLockedError(t *testing.T) {
	if err := Validate(filepath.Join(t.TempDir(), "absent.keyx")); err == nil {
		t.Fatal("Validate must fail for a missing keyfile")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/keyfile/...`
Expected: FAIL — `undefined: RenderXML`.

- [ ] **Step 3: Implement**

Create `internal/keyfile/keyfile.go`:

```go
// Package keyfile mints and validates KeePass XML keyfiles (version 2.0), the
// sole credential for a kdbx vault (spec C4). Losing a keyfile makes its vault
// unrecoverable.
package keyfile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/secretio"
)

const keyLen = 32

// RenderXML produces the exact KeyFile v2 document the Python implementation
// writes: uppercase hex key data, plus a Hash attribute holding the uppercase
// hex of the first four bytes of SHA-256(key).
func RenderXML(key []byte) string {
	sum := sha256.Sum256(key)
	data := strings.ToUpper(hex.EncodeToString(key))
	checksum := strings.ToUpper(hex.EncodeToString(sum[:4]))
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<KeyFile>\n\t<Meta>\n\t\t<Version>2.0</Version>\n\t</Meta>\n" +
		fmt.Sprintf("\t<Key>\n\t\t<Data Hash=\"%s\">%s</Data>\n\t</Key>\n</KeyFile>\n", checksum, data)
}

// Mint writes a new random keyfile at path. It refuses to overwrite.
func Mint(path string) error {
	if _, err := os.Stat(path); err == nil {
		return kdbxerr.Preflight("keyfile already exists: %s", path)
	}
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "generating key material")
	}
	return secretio.AtomicWriteSecret(path, []byte(RenderXML(key)))
}

type xmlKeyFile struct {
	XMLName xml.Name `xml:"KeyFile"`
	Meta    struct {
		Version string `xml:"Version"`
	} `xml:"Meta"`
	Key struct {
		Data struct {
			Hash  string `xml:"Hash,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"Key"`
}

// Validate checks that path exists and is a self-consistent v2 keyfile. Any
// failure is a Locked error (exit 3) — the vault cannot be opened without it.
func Validate(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile unreadable: %s", path)
	}
	var kf xmlKeyFile
	if err := xml.Unmarshal(b, &kf); err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile is not valid XML: %s", path)
	}
	if !strings.HasPrefix(kf.Meta.Version, "2") {
		return kdbxerr.Locked("unsupported keyfile version %q in %s", kf.Meta.Version, path)
	}
	raw := strings.Join(strings.Fields(kf.Key.Data.Value), "")
	key, err := hex.DecodeString(raw)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile data is not hex: %s", path)
	}
	sum := sha256.Sum256(key)
	if want := strings.ToUpper(hex.EncodeToString(sum[:4])); !strings.EqualFold(want, kf.Key.Data.Hash) {
		return kdbxerr.Locked("keyfile checksum mismatch: %s", path)
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/keyfile/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/keyfile
git commit -m "feat(keyfile): mint and validate KeePass XML keyfile v2 byte-compatibly with Python"
```

---

## Task 10: `internal/vault` — engine boundary, read path

**Files:**
- Create: `internal/vault/vault.go`, `internal/vault/vault_test.go`, `internal/vault/testsupport_test.go`
- Delete: `internal/vault/spike_test.go` (its assertions are now covered)

**Interfaces:**
- Consumes: `kdbxerr.*`, `keyfile.Validate`.
- Produces (all plain types — no `gokeepasslib` type crosses this boundary):
  - `type Handle struct{ … }` — an opened database plus the paths it came from.
  - `func vault.Open(vaultPath, keyPath string) (*Handle, error)` — validates the keyfile first (exit 3 on failure), decodes, asserts KDBX4, unlocks protected entries.
  - `func (h *Handle) GetField(groupPath []string, title, field string) (string, error)` — reserved fields (`title`/`username`/`password`/`url`/`notes`, case-insensitive) map to native values; anything else is a custom property. Missing entry or empty/absent field → `kdbxerr.NotFound` (exit 2).
  - `func (h *Handle) ListEntries() ([]string, error)` — sorted `group/…/title`, Recycle Bin excluded.
  - `func (h *Handle) Close() error`
  - `func vault.Create(vaultPath, keyPath string) error` — refuses if either exists; mints keyfile; writes a KDBX4+Argon2 database; restricts perms.

- [ ] **Step 1: Write the test support helper**

Create `internal/vault/testsupport_test.go`:

```go
package vault

import (
	"path/filepath"
	"testing"
)

// newVault creates an empty vault + keyfile in a temp dir and returns their paths.
func newVault(t *testing.T) (vaultPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	vaultPath = filepath.Join(dir, "dev.kdbx")
	keyPath = filepath.Join(dir, "dev.keyx")
	if err := Create(vaultPath, keyPath); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return vaultPath, keyPath
}
```

- [ ] **Step 2: Write the failing tests**

Create `internal/vault/vault_test.go`:

```go
package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestCreateProducesAnOpenableKdbx4Vault(t *testing.T) {
	v, k := newVault(t)
	if _, err := os.Stat(v); err != nil {
		t.Fatalf("vault not created: %v", err)
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer h.Close()
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(v)
		if perm := st.Mode().Perm(); perm != 0o600 {
			t.Fatalf("vault mode %o, want 600", perm)
		}
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	v, k := newVault(t)
	if err := Create(v, k); err == nil {
		t.Fatal("Create must refuse when the vault already exists")
	}
}

func TestOpenWithMissingKeyfileIsLocked(t *testing.T) {
	v, _ := newVault(t)
	_, err := Open(v, filepath.Join(t.TempDir(), "absent.keyx"))
	if err == nil {
		t.Fatal("expected an error opening with a missing keyfile")
	}
}

func TestSetGetReservedAndCustomFields(t *testing.T) {
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "openai", "password", "sk-test"); err != nil {
		t.Fatalf("SetField password: %v", err)
	}
	if err := SetField(v, k, []string{"api"}, "openai", "ORG_ID", "org-123"); err != nil {
		t.Fatalf("SetField custom: %v", err)
	}

	h, err := Open(v, k)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	if got, err := h.GetField([]string{"api"}, "openai", "password"); err != nil || got != "sk-test" {
		t.Fatalf("password = %q, err %v", got, err)
	}
	if got, err := h.GetField([]string{"api"}, "openai", "ORG_ID"); err != nil || got != "org-123" {
		t.Fatalf("custom = %q, err %v", got, err)
	}
	if got, err := h.GetField([]string{"api"}, "openai", "PASSWORD"); err != nil || got != "sk-test" {
		t.Fatalf("reserved field lookup must be case-insensitive: %q %v", got, err)
	}
}

func TestGetFieldMissingEntryIsNotFound(t *testing.T) {
	v, k := newVault(t)
	h, _ := Open(v, k)
	defer h.Close()
	if _, err := h.GetField([]string{"api"}, "nope", "password"); err == nil {
		t.Fatal("expected NotFound for a missing entry")
	}
}

func TestGetFieldMissingFieldIsNotFound(t *testing.T) {
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "openai", "password", "sk-test"); err != nil {
		t.Fatal(err)
	}
	h, _ := Open(v, k)
	defer h.Close()
	if _, err := h.GetField([]string{"api"}, "openai", "ABSENT"); err == nil {
		t.Fatal("expected NotFound for an absent custom field")
	}
}

func TestSetFieldRefusesEmptyValue(t *testing.T) {
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "x", "password", "   "); err == nil {
		t.Fatal("SetField must refuse a whitespace-only value")
	}
}

func TestListEntriesIsSortedAndExcludesTrash(t *testing.T) {
	v, k := newVault(t)
	for _, p := range [][]string{
		{"api", "zeta"},
		{"api", "alpha"},
		{"db", "primary"},
	} {
		if err := SetField(v, k, p[:1], p[1], "password", "value"); err != nil {
			t.Fatal(err)
		}
	}
	if err := Trash(v, k, []string{"api"}, "zeta"); err != nil {
		t.Fatalf("Trash: %v", err)
	}

	h, _ := Open(v, k)
	defer h.Close()
	got, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api/alpha", "db/primary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListEntries = %v, want %v", got, want)
	}
}
```

⚠️ This test file references `SetField` and `Trash`, implemented in Task 12. Write the tests now, implement `Create`/`Open`/`GetField`/`ListEntries` in this task, and mark the two write-dependent tests with `t.Skip("implemented in Task 12")` until then — remove the skips as the final step of Task 12.

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/vault/...`
Expected: FAIL — `undefined: Create`.

- [ ] **Step 4: Implement the engine boundary**

Create `internal/vault/vault.go`. Use the facts recorded in `docs/spike-notes.md` (Task 2) for engine specifics.

```go
// Package vault is the ONLY package permitted to import the KDBX engine
// (spec D2). Its public interface uses plain types exclusively, so swapping the
// engine touches this package alone.
package vault

import (
	"os"
	"sort"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/keyfile"
	"github.com/yarrasys/kdbx/internal/secretio"
)

var reserved = map[string]string{
	"title":    "Title",
	"username": "UserName",
	"password": "Password",
	"url":      "URL",
	"notes":    "Notes",
}

// Handle is an opened, unlocked database.
type Handle struct {
	db        *gokeepasslib.Database
	vaultPath string
	keyPath   string
}

// Create writes a new KDBX4+Argon2 vault and mints its keyfile. It refuses if
// either artifact already exists.
func Create(vaultPath, keyPath string) error {
	if _, err := os.Stat(vaultPath); err == nil {
		return kdbxerr.Preflight("refusing to overwrite an existing vault: %s", vaultPath)
	}
	if _, err := os.Stat(keyPath); err == nil {
		return kdbxerr.Preflight("refusing to overwrite an existing keyfile: %s", keyPath)
	}
	if err := os.MkdirAll(dirOf(vaultPath), 0o700); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating vault directory")
	}
	if err := os.MkdirAll(dirOf(keyPath), 0o700); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating keyfile directory")
	}
	if err := keyfile.Mint(keyPath); err != nil {
		return err
	}

	creds, err := gokeepasslib.NewKeyCredentials(keyPath)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "building credentials from %s", keyPath)
	}
	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion4())
	db.Credentials = creds
	root := gokeepasslib.NewGroup()
	root.Name = "Root"
	db.Content.Root = &gokeepasslib.RootData{Groups: []gokeepasslib.Group{root}}

	return writeDB(db, vaultPath)
}

// Open decodes and unlocks the vault at vaultPath using keyPath.
func Open(vaultPath, keyPath string) (*Handle, error) {
	if err := keyfile.Validate(keyPath); err != nil {
		return nil, err
	}
	creds, err := gokeepasslib.NewKeyCredentials(keyPath)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "building credentials from %s", keyPath)
	}
	f, err := os.Open(vaultPath)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "NotFound", 2, "opening vault %s", vaultPath)
	}
	defer f.Close()

	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "decrypting vault %s", vaultPath)
	}
	if !db.Header.IsKdbx4() {
		return nil, kdbxerr.Locked("vault %s is not KDBX4", vaultPath)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "unlocking protected entries")
	}
	return &Handle{db: db, vaultPath: vaultPath, keyPath: keyPath}, nil
}

// Close releases the handle. It never writes.
func (h *Handle) Close() error { h.db = nil; return nil }

// GetField reads one field of one entry.
func (h *Handle) GetField(groupPath []string, title, field string) (string, error) {
	e := h.findEntry(groupPath, title, false)
	if e == nil {
		return "", kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
	}
	key := field
	if native, ok := reserved[strings.ToLower(field)]; ok {
		key = native
	}
	val := e.GetContent(key)
	if val == "" {
		return "", kdbxerr.NotFound(
			"field not set: %s (entry %s exists but '%s' is empty/absent)",
			field, joinPath(groupPath, title), field)
	}
	return val, nil
}

// ListEntries returns every live entry path, sorted, excluding the Recycle Bin.
func (h *Handle) ListEntries() ([]string, error) {
	var out []string
	walk(h.db.Content.Root.Groups, nil, h.recycleBinName(), func(path []string, e *gokeepasslib.Entry) {
		out = append(out, strings.Join(append(append([]string{}, path...), e.GetTitle()), "/"))
	})
	sort.Strings(out)
	return out, nil
}
```

Add the unexported helpers in the same file:

```go
func dirOf(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i <= 0 {
		return "."
	}
	return p[:i]
}

func joinPath(groupPath []string, title string) string {
	return strings.Join(append(append([]string{}, groupPath...), title), "/")
}

// recycleBinName returns the name of the group designated as the Recycle Bin,
// or "" when the vault has none.
func (h *Handle) recycleBinName() string {
	meta := h.db.Content.Meta
	if meta == nil || !meta.RecycleBinEnabled.Bool {
		return ""
	}
	for _, g := range h.db.Content.Root.Groups {
		if g.UUID.Compare(meta.RecycleBinUUID) {
			return g.Name
		}
		for _, sub := range g.Groups {
			if sub.UUID.Compare(meta.RecycleBinUUID) {
				return sub.Name
			}
		}
	}
	return ""
}

// walk visits every entry outside the named recycle-bin group. The path passed
// to fn excludes the synthetic root group, matching Python's e.path semantics.
func walk(groups []gokeepasslib.Group, prefix []string, recycleBin string,
	fn func(path []string, e *gokeepasslib.Entry)) {
	for gi := range groups {
		g := &groups[gi]
		if recycleBin != "" && g.Name == recycleBin && len(prefix) == 0 {
			continue
		}
		path := prefix
		if len(prefix) > 0 || !isRootGroup(g) {
			path = append(append([]string{}, prefix...), g.Name)
		}
		for ei := range g.Entries {
			fn(path, &g.Entries[ei])
		}
		walk(g.Groups, path, recycleBin, fn)
	}
}

// isRootGroup reports whether g is the synthetic top-level container that does
// not appear in entry paths.
func isRootGroup(g *gokeepasslib.Group) bool { return g.Name == "Root" }

// findEntry locates an entry by group path and title.
func (h *Handle) findEntry(groupPath []string, title string, includeTrash bool) *gokeepasslib.Entry {
	var found *gokeepasslib.Entry
	rb := h.recycleBinName()
	if includeTrash {
		rb = ""
	}
	walk(h.db.Content.Root.Groups, nil, rb, func(path []string, e *gokeepasslib.Entry) {
		if found != nil {
			return
		}
		if e.GetTitle() != title {
			return
		}
		if len(path) != len(groupPath) {
			if includeTrash && len(groupPath) == 0 {
				found = e
			}
			return
		}
		for i := range path {
			if path[i] != groupPath[i] {
				return
			}
		}
		found = e
	})
	return found
}

// writeDB locks protected entries and writes the database crash-safely
// (spec C3): tmp -> restrict -> rename old to .bak -> rename tmp into place ->
// restrict -> drop .bak.
func writeDB(db *gokeepasslib.Database, vaultPath string) error {
	if err := db.LockProtectedEntries(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "locking protected entries")
	}
	tmp := vaultPath + ".tmp"
	bak := vaultPath + ".bak"

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating %s", tmp)
	}
	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "encoding vault")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "closing %s", tmp)
	}
	if err := secretio.RestrictPerms(tmp); err != nil {
		return err
	}
	if _, err := os.Stat(vaultPath); err == nil {
		if err := os.Rename(vaultPath, bak); err != nil {
			return kdbxerr.Wrap(err, "Runtime", 1, "backing up %s", vaultPath)
		}
	}
	if err := os.Rename(tmp, vaultPath); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "installing %s", vaultPath)
	}
	if err := secretio.RestrictPerms(vaultPath); err != nil {
		return err
	}
	_ = os.Remove(bak)
	// Re-unlock so the in-memory handle stays usable after a write.
	return db.UnlockProtectedEntries()
}

// unused keeps the wrappers import meaningful before Task 12 lands writes.
var _ = w.NewBoolWrapper
```

⚠️ `isRootGroup` matching on the literal name `"Root"` is the pragmatic choice that matches how both pykeepass and this package create vaults. If `docs/spike-notes.md` shows the author's real vaults name the top group differently, change `walk` to treat *the single top-level group* as the root instead of matching by name, and note it in the commit.

- [ ] **Step 5: Add the skips and run**

Add `t.Skip("write path lands in Task 12")` at the top of `TestSetGetReservedAndCustomFields`, `TestGetFieldMissingFieldIsNotFound`, `TestSetFieldRefusesEmptyValue`, and `TestListEntriesIsSortedAndExcludesTrash`.

Run: `go test ./internal/vault/... -v`
Expected: `TestCreateProducesAnOpenableKdbx4Vault`, `TestCreateRefusesToOverwrite`, `TestOpenWithMissingKeyfileIsLocked`, `TestGetFieldMissingEntryIsNotFound` PASS; the rest SKIP.

- [ ] **Step 6: Delete the spike and commit**

```bash
rm internal/vault/spike_test.go
go test ./... && go vet ./...
git add -A
git commit -m "feat(vault): engine boundary with create/open/get/list; retire the M0 spike"
```

---

## Task 11: `get`, `list`, `check` commands + clipboard

**Files:**
- Create: `cmd/get.go`, `cmd/list.go`, `cmd/check.go`, `internal/secretio/clipboard.go`, `internal/secretio/clipboard_test.go`, `testdata/script/read_ops.txtar`

**Interfaces:**
- Consumes: `envctx.Resolve`, `vault.Open`, `vault.Handle.GetField/ListEntries`, `pointer.ParseEntryPath`, `secretio.Mask`, `jsonout.*`, `kdbxerr.*`.
- Produces:
  - `func secretio.ClipboardCommand() []string` — platform copy command, nil when unavailable.
  - `func secretio.ClipboardCopy(value string, clearAfter time.Duration) error` — copies, then spawns a detached `kdbx internal-clear-clip` that clears after the delay.
  - `cmd` helpers `mustContext(c *cobra.Command, banner bool) (*envctx.Context, error)` and `openVault(ctx *envctx.Context) (*vault.Handle, error)` in `cmd/common.go`, reused by every later command.

- [ ] **Step 1: Write the failing clipboard test**

Create `internal/secretio/clipboard_test.go`:

```go
package secretio

import (
	"runtime"
	"testing"
)

func TestClipboardCommandIsPlatformAppropriate(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	cmd := ClipboardCommand()
	switch runtime.GOOS {
	case "darwin":
		if len(cmd) == 0 || cmd[0] != "pbcopy" {
			t.Fatalf("darwin should use pbcopy, got %v", cmd)
		}
	case "windows":
		if len(cmd) == 0 || cmd[0] != "powershell" {
			t.Fatalf("windows should use powershell, got %v", cmd)
		}
	default:
		if cmd != nil {
			t.Fatalf("headless linux should have no clipboard backend, got %v", cmd)
		}
	}
}

func TestClipboardCommandPrefersWaylandWhenPresent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if cmd := ClipboardCommand(); len(cmd) == 0 || cmd[0] != "wl-copy" {
		t.Fatalf("got %v, want wl-copy", cmd)
	}
}

func TestClipboardCopyFailsCleanlyWithoutBackend(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only headless case")
	}
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	if err := ClipboardCopy("value", 0); err == nil {
		t.Fatal("expected an error when no clipboard backend exists")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/secretio/ -run Clipboard`
Expected: FAIL — `undefined: ClipboardCommand`.

- [ ] **Step 3: Implement the clipboard**

Create `internal/secretio/clipboard.go`:

```go
package secretio

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// DefaultClipboardClear is how long a copied secret lives on the clipboard.
const DefaultClipboardClear = 15 * time.Second

// ClipboardCommand returns the platform's copy command, or nil if none applies.
func ClipboardCommand() []string {
	switch {
	case runtime.GOOS == "darwin":
		return []string{"pbcopy"}
	case runtime.GOOS == "windows":
		return []string{"powershell", "-NoProfile", "-Command", "Set-Clipboard"}
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return []string{"wl-copy"}
	case os.Getenv("DISPLAY") != "":
		return []string{"xclip", "-selection", "clipboard"}
	}
	return nil
}

// ClipboardCopy places value on the clipboard and schedules a detached clear.
func ClipboardCopy(value string, clearAfter time.Duration) error {
	argv := ClipboardCommand()
	if argv == nil {
		return kdbxerr.Runtime("no clipboard backend available")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "copying to clipboard")
	}
	if clearAfter <= 0 {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil // copied successfully; we simply cannot schedule the clear
	}
	clear := exec.Command(self, "internal-clear-clip",
		"--after", strconv.Itoa(int(clearAfter/time.Second)))
	detach(clear)
	_ = clear.Start()
	_ = clear.Process.Release()
	return nil
}
```

Create `internal/secretio/detach.go`:

```go
//go:build !windows

package secretio

import (
	"os/exec"
	"syscall"
)

// detach puts the helper in its own session so it outlives the parent.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
```

Create `internal/secretio/detach_windows.go`:

```go
//go:build windows

package secretio

import "os/exec"

// detach is a no-op on Windows: a started process already survives the parent.
func detach(c *exec.Cmd) {}
```

- [ ] **Step 4: Add the shared command helpers**

Create `cmd/common.go`:

```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/vault"
)

// mustContext resolves the active environment, optionally announcing it.
func mustContext(c *cobra.Command, banner bool) (*envctx.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctx, err := envctx.Resolve(opts.env, cwd)
	if err != nil {
		return nil, err
	}
	if banner && !opts.json {
		ctx.WriteBanner(c.ErrOrStderr())
	}
	return ctx, nil
}

// openVault opens the active environment's vault for reading.
func openVault(ctx *envctx.Context) (*vault.Handle, error) {
	return vault.Open(ctx.Vault, ctx.KeyFile)
}
```

- [ ] **Step 5: Implement `get`**

Create `cmd/get.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var reveal, clip bool
		cmd := &cobra.Command{
			Use:   "get PATH",
			Short: "Read a secret (masked by default)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runGet(c, args[0], reveal, clip)
			},
		}
		cmd.Flags().BoolVar(&reveal, "reveal", false, "print the value to stdout")
		cmd.Flags().BoolVar(&clip, "clip", false, "copy the value to the clipboard (auto-clears)")
		cmd.MarkFlagsMutuallyExclusive("reveal", "clip")
		root.AddCommand(cmd)
		root.AddCommand(clearClipCmd())
	})
}

func runGet(c *cobra.Command, path string, reveal, clip bool) error {
	if opts.json && reveal {
		return kdbxerr.Preflight("--json cannot be combined with --reveal")
	}
	group, title, field, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	val, err := h.GetField(group, title, field)
	if err != nil {
		return err
	}

	switch {
	case clip:
		if err := secretio.ClipboardCopy(val, secretio.DefaultClipboardClear); err != nil {
			return err
		}
		fmt.Fprintln(c.ErrOrStderr(), "copied to clipboard (clears shortly)")
	case reveal:
		fmt.Fprintln(c.OutOrStdout(), val)
		fmt.Fprintln(c.ErrOrStderr(), "WARNING: value printed to stdout (scrollback/CI logs)")
	case opts.json:
		return jsonout.Write(c.OutOrStdout(), map[string]any{"path": path, "set": true})
	default:
		fmt.Fprintln(c.OutOrStdout(), secretio.Mask)
	}
	return nil
}

// clearClipCmd is the detached helper ClipboardCopy schedules. It is hidden
// because it is an implementation detail, not part of the CLI contract.
func clearClipCmd() *cobra.Command {
	var after int
	cmd := &cobra.Command{
		Use:    "internal-clear-clip",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return secretio.ClearClipboardAfter(after)
		},
	}
	cmd.Flags().IntVar(&after, "after", 15, "seconds to wait before clearing")
	return cmd
}
```

Add to `internal/secretio/clipboard.go`:

```go
// ClearClipboardAfter sleeps for seconds, then blanks the clipboard.
func ClearClipboardAfter(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	argv := ClipboardCommand()
	if argv == nil {
		return nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader("")
	_ = cmd.Run()
	return nil
}
```

- [ ] **Step 6: Implement `list` and `check`**

Create `cmd/list.go`:

```go
package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/jsonout"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "list [GROUP]",
			Short: "List entry paths (never values)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				group := ""
				if len(args) == 1 {
					group = args[0]
				}
				return runList(c, group)
			},
		})
	})
}

func runList(c *cobra.Command, group string) error {
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	all, err := h.ListEntries()
	if err != nil {
		return err
	}
	kept := []string{}
	for _, p := range all {
		if group == "" || strings.HasPrefix(p, group) {
			kept = append(kept, p)
		}
	}
	if opts.json {
		return jsonout.Write(c.OutOrStdout(), map[string]any{"entries": kept})
	}
	for _, p := range kept {
		fmt.Fprintln(c.OutOrStdout(), p)
	}
	return nil
}
```

Create `cmd/check.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "check",
			Short: "Verify every mapped var resolves",
			Args:  cobra.NoArgs,
			RunE:  runCheck,
		})
	})
}

type missingVar struct {
	Var  string `json:"var"`
	Path string `json:"path"`
}

func runCheck(c *cobra.Command, _ []string) error {
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	missing := []missingVar{}
	for _, name := range ctx.VarOrder {
		path := ctx.Vars[name]
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			missing = append(missing, missingVar{Var: name, Path: path})
			continue
		}
		if _, gerr := h.GetField(group, title, field); gerr != nil {
			missing = append(missing, missingVar{Var: name, Path: path})
		}
	}

	if opts.json {
		if err := jsonout.Write(c.OutOrStdout(), map[string]any{
			"ok": len(missing) == 0, "missing": missing,
		}); err != nil {
			return err
		}
	} else {
		for _, m := range missing {
			fmt.Fprintf(c.OutOrStdout(), "MISSING %s -> %s\n", m.Var, m.Path)
		}
	}
	if len(missing) > 0 {
		return kdbxerr.Drift("%d mapped var(s) do not resolve", len(missing))
	}
	return nil
}
```

⚠️ `check` returning a `Drift` error means `Execute` prints `kdbx: check failed: Drift` to stderr and exits 5. That matches spec C6/C7 — the `MISSING` lines are the contract, the stderr line is the scrubbed summary. Confirm the testscript expectations below encode exactly this.

- [ ] **Step 7: Write the CLI-contract test**

Create `testdata/script/read_ops.txtar`:

```
# Seed a vault: init, then set two entries (the harness acts as the human here).
exec kdbx init
stderr 'ACTIVE ENV: dev'
stderr 'KEYFILE:'

stdin secret1.txt
exec kdbx set api/openai --var OPENAI_API_KEY
stdin secret2.txt
exec kdbx set api/anthropic --var ANTHROPIC_API_KEY

# list prints sorted paths and never a value
exec kdbx list
stdout '^api/anthropic$'
stdout '^api/openai$'
! stdout 'sk-'

# list filters by group prefix
exec kdbx list api/open
stdout '^api/openai$'
! stdout 'anthropic'

# get is masked by default
exec kdbx get api/openai
stdout '^\(set, hidden\)$'
! stdout 'sk-'

# --reveal prints the value and warns
exec kdbx get api/openai --reveal
stdout '^sk-test-one$'
stderr 'WARNING: value printed to stdout'

# --json masks and never carries a value
exec kdbx --json get api/openai
stdout '"set":true'
! stdout 'sk-'

# --json with --reveal is a preflight error (exit 7)
! exec kdbx --json get api/openai --reveal
stderr 'kdbx: get failed: Preflight'

# a missing entry is exit 2
! exec kdbx get api/nope
stderr 'kdbx: get failed: NotFound'

# check passes when every mapping resolves
exec kdbx check
! stdout 'MISSING'

# check reports drift and exits 5
stdin secret1.txt
exec kdbx set api/orphan --var ORPHAN_KEY
exec kdbx delete api/orphan --purge --yes
! exec kdbx check
stdout 'MISSING ORPHAN_KEY -> api/orphan'
stderr 'kdbx: check failed: Drift'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {}
  }
}
-- secret1.txt --
sk-test-one
-- secret2.txt --
sk-test-two
```

⚠️ This script uses `init`, `set`, and `delete`, which land in Tasks 13–14. Until then, keep `read_ops.txtar` out of `testdata/script/` — stage it at `testdata/pending/read_ops.txtar` and move it into place as the final step of Task 14. The `--yes` flag on `delete --purge` is introduced in Task 14 for non-interactive confirmation in tests only; it does **not** exist in the Python CLI, so it must be `Hidden: true` and documented as test-only in the command's help.

- [ ] **Step 8: Run and commit**

Run: `go test ./... && go vet ./...`
Expected: PASS (with `read_ops.txtar` still in `testdata/pending/`).

```bash
git add -A
git commit -m "feat(get,list,check): read operations with masking, --json, clipboard, drift exit 5"
```

---

## Task 12: `internal/locking` + the vault write path

**Files:**
- Create: `internal/locking/locking.go`, `internal/locking/locking_test.go`, `internal/vault/write.go`
- Modify: `internal/vault/vault_test.go` (remove the four `t.Skip` calls added in Task 10)
- Reference: `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/locking.py` and the write half of `vault.py`

**Interfaces:**
- Consumes: `kdbxerr.*`, `vault.Open`, `vault.writeDB` (Task 10), `keyfile.Mint` (Task 9).
- Produces:
  - `func locking.WithVaultLock(vaultPath string, fn func() error) error` — advisory `<vault>.lock`, 10 s timeout → `kdbxerr.Locked`.
  - `func locking.CaptureState(vaultPath string) (string, error)` — SHA-256 hex of the file, `""` when absent.
  - `func locking.VerifyUnchanged(vaultPath, captured string) error` — `kdbxerr.Changed` (exit 6) on mismatch.
  - `func vault.SetField(vaultPath, keyPath string, groupPath []string, title, field, value string) error` — refuses whitespace-only values; creates missing groups; reserved fields native, others protected custom properties.
  - `func vault.Trash(vaultPath, keyPath string, groupPath []string, title string) error` — moves to the Recycle Bin, creating it on first use.
  - `func vault.Purge(vaultPath, keyPath string, groupPath []string, title string) error` — permanent, searches the Recycle Bin too.
  - `func vault.Move(vaultPath, keyPath, src, dst string) error`
  - `func vault.Rekey(vaultPath, oldKeyPath, newKeyPath string) error`

- [ ] **Step 1: Write the failing locking tests**

```bash
cd $KDBX_REPO
go get github.com/gofrs/flock@latest
```

Create `internal/locking/locking_test.go`:

```go
package locking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureStateOfMissingFileIsEmpty(t *testing.T) {
	got, err := CaptureState(filepath.Join(t.TempDir(), "absent.kdbx"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestVerifyUnchangedDetectsModification(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	if err := os.WriteFile(p, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureState(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnchanged(p, captured); err != nil {
		t.Fatalf("unchanged file should verify: %v", err)
	}
	if err := os.WriteFile(p, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnchanged(p, captured); err == nil {
		t.Fatal("modified file must fail verification")
	}
}

func TestWithVaultLockRunsAndReleases(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	ran := false
	if err := WithVaultLock(p, func() error { ran = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("callback did not run")
	}
	// The lock must be released, so a second acquisition succeeds immediately.
	if err := WithVaultLock(p, func() error { return nil }); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestWithVaultLockPropagatesCallbackError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	sentinel := os.ErrClosed
	if err := WithVaultLock(p, func() error { return sentinel }); err == nil {
		t.Fatal("callback error must propagate")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/locking/...`
Expected: FAIL — `undefined: CaptureState`.

- [ ] **Step 3: Implement locking**

Create `internal/locking/locking.go`:

```go
// Package locking prevents two kdbx writes from racing and detects a vault that
// changed underneath a read-modify-write (spec C9).
package locking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"

	"github.com/gofrs/flock"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// LockTimeout bounds how long a write waits for a competing kdbx process.
const LockTimeout = 10 * time.Second

// WithVaultLock runs fn while holding the advisory lock for vaultPath.
func WithVaultLock(vaultPath string, fn func() error) error {
	lock := flock.New(vaultPath + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), LockTimeout)
	defer cancel()

	got, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "acquiring the vault lock")
	}
	if !got {
		return kdbxerr.Locked("another kdbx process holds the vault lock; try again")
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

// CaptureState returns a content hash of the vault, or "" if it does not exist.
func CaptureState(vaultPath string) (string, error) {
	b, err := os.ReadFile(vaultPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", kdbxerr.Wrap(err, "Runtime", 1, "reading the vault for integrity capture")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyUnchanged fails when the vault differs from its captured state.
func VerifyUnchanged(vaultPath, captured string) error {
	now, err := CaptureState(vaultPath)
	if err != nil {
		return err
	}
	if now != captured {
		return kdbxerr.Changed("vault changed underneath us; re-run")
	}
	return nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/locking/... -v`
Expected: all PASS.

- [ ] **Step 5: Implement the vault write path**

Create `internal/vault/write.go`:

```go
package vault

import (
	"os"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/keyfile"
	"github.com/yarrasys/kdbx/internal/locking"
	"github.com/yarrasys/kdbx/internal/pointer"
)

// recycleBinName is the group KeePassXC uses for soft-deleted entries.
const recycleBinGroupName = "Recycle Bin"

// mutate opens the vault under lock, verifies it has not changed, applies fn,
// and writes it back crash-safely (spec C3, C9).
func mutate(vaultPath, keyPath string, fn func(h *Handle) error) error {
	return locking.WithVaultLock(vaultPath, func() error {
		captured, err := locking.CaptureState(vaultPath)
		if err != nil {
			return err
		}
		h, err := Open(vaultPath, keyPath)
		if err != nil {
			return err
		}
		defer h.Close()
		if err := locking.VerifyUnchanged(vaultPath, captured); err != nil {
			return err
		}
		if err := fn(h); err != nil {
			return err
		}
		return writeDB(h.db, vaultPath)
	})
}

// SetField stores value at the given entry+field, creating groups and the entry
// as needed. Non-reserved fields become protected custom properties.
func SetField(vaultPath, keyPath string, groupPath []string, title, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return kdbxerr.Preflight("refusing to write empty value — provide a non-empty secret")
	}
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		grp := h.ensureGroup(groupPath)
		e := h.findEntry(groupPath, title, false)
		if e == nil {
			ne := gokeepasslib.NewEntry()
			ne.Times = gokeepasslib.NewTimeData()
			ne.Values = append(ne.Values, gokeepasslib.ValueData{
				Key: "Title", Value: gokeepasslib.V{Content: title},
			})
			grp.Entries = append(grp.Entries, ne)
			e = &grp.Entries[len(grp.Entries)-1]
		}
		key := field
		protected := true
		if native, ok := reserved[strings.ToLower(field)]; ok {
			key = native
			protected = native == "Password"
		}
		setValue(e, key, value, protected)
		return nil
	})
}

// setValue writes or replaces one key on an entry.
func setValue(e *gokeepasslib.Entry, key, value string, protected bool) {
	for i := range e.Values {
		if e.Values[i].Key == key {
			e.Values[i].Value = gokeepasslib.V{
				Content: value, Protected: w.NewBoolWrapper(protected),
			}
			return
		}
	}
	e.Values = append(e.Values, gokeepasslib.ValueData{
		Key: key, Value: gokeepasslib.V{Content: value, Protected: w.NewBoolWrapper(protected)},
	})
}

// ensureGroup walks (creating as needed) to the group holding an entry.
func (h *Handle) ensureGroup(groupPath []string) *gokeepasslib.Group {
	root := h.rootGroup()
	cur := root
	for _, name := range groupPath {
		var next *gokeepasslib.Group
		for i := range cur.Groups {
			if cur.Groups[i].Name == name {
				next = &cur.Groups[i]
				break
			}
		}
		if next == nil {
			ng := gokeepasslib.NewGroup()
			ng.Name = name
			cur.Groups = append(cur.Groups, ng)
			next = &cur.Groups[len(cur.Groups)-1]
		}
		cur = next
	}
	return cur
}

// rootGroup returns the synthetic top-level container, creating it if absent.
func (h *Handle) rootGroup() *gokeepasslib.Group {
	if len(h.db.Content.Root.Groups) == 0 {
		g := gokeepasslib.NewGroup()
		g.Name = "Root"
		h.db.Content.Root.Groups = append(h.db.Content.Root.Groups, g)
	}
	return &h.db.Content.Root.Groups[0]
}

// Trash soft-deletes an entry into the Recycle Bin.
func Trash(vaultPath, keyPath string, groupPath []string, title string) error {
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		e, owner := h.locate(groupPath, title, false)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
		}
		bin := h.ensureRecycleBin()
		moved := *e
		removeEntry(owner, title)
		bin.Entries = append(bin.Entries, moved)
		return nil
	})
}

// Purge permanently removes an entry, including one already in the Recycle Bin.
func Purge(vaultPath, keyPath string, groupPath []string, title string) error {
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		e, owner := h.locate(groupPath, title, true)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
		}
		removeEntry(owner, title)
		return nil
	})
}

// Move relocates and/or retitles an entry.
func Move(vaultPath, keyPath, src, dst string) error {
	sg, st, _, err := pointer.ParseEntryPath(src)
	if err != nil {
		return err
	}
	dg, dt, _, err := pointer.ParseEntryPath(dst)
	if err != nil {
		return err
	}
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		e, owner := h.locate(sg, st, false)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(sg, st))
		}
		// Copy the value slice, not just the entry struct: `moved := *e` shares
		// its Values backing array with the entry still in the tree, so retitling
		// the copy renames the original too. removeEntry (matching on title) then
		// finds nothing, and the two entries' shared protected values get locked
		// twice on write — the moved secret decrypts to nothing. Silent data loss.
		moved := *e
		moved.Values = append([]gokeepasslib.ValueData(nil), e.Values...)
		setValue(&moved, "Title", dt, false)
		removeEntry(owner, st)
		target := h.ensureGroup(dg)
		target.Entries = append(target.Entries, moved)
		return nil
	})
}

// Rekey mints newKeyPath, re-encrypts the vault under it, and removes the old key.
func Rekey(vaultPath, oldKeyPath, newKeyPath string) error {
	if err := keyfile.Mint(newKeyPath); err != nil {
		return err
	}
	err := locking.WithVaultLock(vaultPath, func() error {
		h, err := Open(vaultPath, oldKeyPath)
		if err != nil {
			return err
		}
		defer h.Close()
		creds, err := gokeepasslib.NewKeyCredentials(newKeyPath)
		if err != nil {
			return kdbxerr.Wrap(err, "Locked", 3, "building credentials from the new keyfile")
		}
		h.db.Credentials = creds
		return writeDB(h.db, vaultPath)
	})
	if err != nil {
		_ = os.Remove(newKeyPath)
		return err
	}
	return nil
}
```

Add the remaining unexported helpers to `internal/vault/write.go`:

```go
// locate finds an entry and the group that owns it.
func (h *Handle) locate(groupPath []string, title string, includeTrash bool) (*gokeepasslib.Entry, *gokeepasslib.Group) {
	var (
		hit   *gokeepasslib.Entry
		owner *gokeepasslib.Group
	)
	rb := h.recycleBinName()
	if includeTrash {
		rb = ""
	}
	var visit func(groups []gokeepasslib.Group, prefix []string)
	visit = func(groups []gokeepasslib.Group, prefix []string) {
		for gi := range groups {
			g := &groups[gi]
			if rb != "" && g.Name == rb && len(prefix) == 0 {
				continue
			}
			path := prefix
			if len(prefix) > 0 || !isRootGroup(g) {
				path = append(append([]string{}, prefix...), g.Name)
			}
			for ei := range g.Entries {
				if hit != nil {
					return
				}
				if g.Entries[ei].GetTitle() != title {
					continue
				}
				if !includeTrash && !samePath(path, groupPath) {
					continue
				}
				hit, owner = &g.Entries[ei], g
				return
			}
			visit(g.Groups, path)
		}
	}
	visit(h.db.Content.Root.Groups, nil)
	return hit, owner
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// removeEntry deletes the first entry with the given title from g.
func removeEntry(g *gokeepasslib.Group, title string) {
	if g == nil {
		return
	}
	for i := range g.Entries {
		if g.Entries[i].GetTitle() == title {
			g.Entries = append(g.Entries[:i], g.Entries[i+1:]...)
			return
		}
	}
}

// ensureRecycleBin returns the Recycle Bin group, creating it and recording its
// UUID in Meta on first use, exactly as KeePassXC does.
func (h *Handle) ensureRecycleBin() *gokeepasslib.Group {
	root := h.rootGroup()
	for i := range root.Groups {
		if root.Groups[i].Name == recycleBinGroupName {
			h.db.Content.Meta.RecycleBinEnabled = w.NewBoolWrapper(true)
			h.db.Content.Meta.RecycleBinUUID = root.Groups[i].UUID
			return &root.Groups[i]
		}
	}
	bin := gokeepasslib.NewGroup()
	bin.Name = recycleBinGroupName
	root.Groups = append(root.Groups, bin)
	created := &root.Groups[len(root.Groups)-1]
	h.db.Content.Meta.RecycleBinEnabled = w.NewBoolWrapper(true)
	h.db.Content.Meta.RecycleBinUUID = created.UUID
	return created
}
```

Also delete the `var _ = w.NewBoolWrapper` placeholder line added at the end of `internal/vault/vault.go` in Task 10, and drop the now-duplicate `w` import there if `vault.go` no longer uses it.

- [ ] **Step 6: Remove the skips and run**

Delete the four `t.Skip("write path lands in Task 12")` lines from `internal/vault/vault_test.go`.

Run: `go test ./internal/vault/... -v`
Expected: all PASS, including `TestListEntriesIsSortedAndExcludesTrash`.

- [ ] **Step 7: Add a concurrency regression test**

Append to `internal/vault/vault_test.go`:

```go
func TestConcurrentSetsDoNotCorruptTheVault(t *testing.T) {
	v, k := newVault(t)
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(n int) {
			done <- SetField(v, k, []string{"api"}, "entry"+string(rune('a'+n)), "password", "value")
		}(i)
	}
	failures := 0
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			failures++ // a losing writer may legitimately see VaultChanged (exit 6)
		}
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("vault unreadable after concurrent writes: %v", err)
	}
	defer h.Close()
	entries, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries)+failures != 4 {
		t.Fatalf("entries=%d failures=%d, want them to sum to 4", len(entries), failures)
	}
}
```

Run: `go test ./internal/vault/ -run Concurrent -race -count=3 -v`
Expected: PASS every run; the vault must always remain readable.

- [ ] **Step 8: Commit**

```bash
git add internal/locking internal/vault go.mod go.sum
git commit -m "feat(vault): locking, integrity capture, and the set/trash/purge/move/rekey write path"
```

---

## Task 13: `init` and `set` commands

**Files:**
- Create: `cmd/init.go`, `cmd/set.go`, `testdata/script/init_set.txtar`

**Interfaces:**
- Consumes: `vault.Create`, `vault.SetField`, `secretio.ReadSecret`, `pointer.ParseEntryPath`, `pointer.ValidVarName`, `paths.UnderSyncRoot`, `envctx.Resolve`.
- Produces: no new exported Go API; the CLI surface for `init` and `set` per spec C5.

- [ ] **Step 1: Implement `init`**

Create `cmd/init.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/paths"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "init",
			Short: "Create the vault and keyfile for an environment",
			Args:  cobra.NoArgs,
			RunE:  runInit,
		})
	})
}

func runInit(c *cobra.Command, _ []string) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		return err
	}
	errOut := c.ErrOrStderr()
	fmt.Fprintf(errOut, "created %s\n", ctx.Vault)
	fmt.Fprintf(errOut,
		"KEYFILE: %s — back this up; losing it makes the vault unrecoverable.\n", ctx.KeyFile)
	for _, p := range []string{ctx.Vault, ctx.KeyFile} {
		if root := paths.UnderSyncRoot(p); root != "" {
			fmt.Fprintf(errOut,
				"WARNING: %s is inside %s — cloud sync can corrupt a vault and copies the keyfile off this machine.\n",
				p, root)
			break
		}
	}
	return nil
}
```

- [ ] **Step 2: Implement `set`**

Create `cmd/set.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var varName, fromEnv string
		var raw bool
		cmd := &cobra.Command{
			Use:   "set PATH",
			Short: "Store a secret (value via stdin or --from-env, never argv)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runSet(c, args[0], varName, fromEnv, raw)
			},
		}
		cmd.Flags().StringVar(&varName, "var", "", "also map this env-var name to the entry")
		cmd.Flags().StringVar(&fromEnv, "from-env", "", "read the value from this environment variable")
		cmd.Flags().BoolVar(&raw, "raw", false, "do not strip a trailing newline")
		root.AddCommand(cmd)
	})
}

func runSet(c *cobra.Command, path, varName, fromEnv string, raw bool) error {
	if varName != "" && !pointer.ValidVarName(varName) {
		fmt.Fprintf(c.ErrOrStderr(),
			"kdbx set: --var '%s' is not a valid env-var name (expected pattern: ^[A-Z_][A-Z0-9_]*$)\n",
			varName)
		return kdbxerr.Preflight("invalid --var name")
	}
	group, title, field, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	value, err := secretio.ReadSecret(secretio.ReadOpts{
		FromEnv: fromEnv,
		Raw:     raw,
		Stdin:   c.InOrStdin(),
		IsTTY:   fromEnv == "" && secretio.IsTerminal(os.Stdin),
	})
	if err != nil {
		return err
	}
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, group, title, field, value); err != nil {
		return err
	}
	if varName != "" {
		ctx.Pointer.SetVar(ctx.Env, varName, path)
		if err := ctx.Pointer.Save(); err != nil {
			return err
		}
		fmt.Fprintf(c.ErrOrStderr(),
			"modified tracked file %s — review and commit\n", pointer.Name)
	}
	return nil
}
```

- [ ] **Step 3: Write the CLI-contract test**

Create `testdata/script/init_set.txtar`:

```
# init creates the vault and warns about the keyfile
exec kdbx init
stderr 'ACTIVE ENV: dev  vault='
stderr 'created '
stderr 'KEYFILE: .* back this up'

# init refuses to clobber an existing vault
! exec kdbx init
stderr 'kdbx: init failed: Preflight'

# set reads the value from stdin and never echoes it
stdin secret.txt
exec kdbx set api/openai --var OPENAI_API_KEY
stderr 'modified tracked file .keepassxc.json'
! stderr 'sk-test'
! stdout 'sk-test'

# the pointer gained the mapping and kept its formatting
grep '"OPENAI_API_KEY": "api/openai"' .keepassxc.json
grep '^\{$' .keepassxc.json
grep '^  "project": "demo",$' .keepassxc.json

# the stored value round-trips
exec kdbx get api/openai --reveal
stdout '^sk-test-value$'

# an invalid --var name is a preflight failure (exit 7) before any write
stdin secret.txt
! exec kdbx set api/other --var 'not-valid'
stderr 'is not a valid env-var name'
stderr 'kdbx: set failed: Preflight'

# empty stdin is refused
stdin empty.txt
! exec kdbx set api/empty
stderr 'no value provided'

# --from-env takes the value from the environment
env SRC_VALUE=from-env-value
exec kdbx set api/fromenv --from-env SRC_VALUE
exec kdbx get api/fromenv --reveal
stdout '^from-env-value$'

# a custom field is stored as a protected property
stdin secret.txt
exec kdbx set api/openai:ORG_ID
exec kdbx get api/openai:ORG_ID --reveal
stdout '^sk-test-value$'
exec kdbx get api/openai --reveal
stdout '^sk-test-value$'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {}
  }
}
-- secret.txt --
sk-test-value
-- empty.txt --

```

- [ ] **Step 4: Run to verify**

Run: `go test . -run TestScripts/init_set -v`
Expected: PASS.

⚠️ If `set` blocks waiting on a terminal inside testscript, the `IsTTY` detection is wrong: testscript gives the child a non-TTY stdin, so `secretio.IsTerminal(os.Stdin)` must be false there. Do not "fix" this by removing the TTY branch — verify with `go run . set x` in a real terminal that the interactive prompt still appears.

- [ ] **Step 5: Commit**

```bash
git add cmd/init.go cmd/set.go testdata/script/init_set.txtar
git commit -m "feat(init,set): vault creation with sync-root warning; transcript-safe secret storage"
```

---

## Task 14: `delete` and `mv` commands

**Files:**
- Create: `cmd/delete.go`, `cmd/mv.go`, `testdata/script/delete_mv.txtar`
- Move: `testdata/pending/read_ops.txtar` → `testdata/script/read_ops.txtar`

**Interfaces:**
- Consumes: `vault.Trash`, `vault.Purge`, `vault.Move`, `pointer.RepointVars`, `pointer.EntryOf`, `secretio.Confirm`.
- Produces: a hidden, **test-only** `--yes` flag on `delete --purge` and (in Task 19) `rekey`, so scripted tests can pass the confirmation gate. It must be `Hidden: true` with help text `test-only: bypass the interactive confirmation`.

- [ ] **Step 1: Implement `delete`**

Create `cmd/delete.go`:

```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var purge, yes bool
		cmd := &cobra.Command{
			Use:   "delete PATH",
			Short: "Soft-delete an entry to the Recycle Bin (--purge removes it permanently)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runDelete(c, args[0], purge, yes)
			},
		}
		cmd.Flags().BoolVar(&purge, "purge", false, "permanently remove the entry")
		cmd.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = cmd.Flags().MarkHidden("yes")
		root.AddCommand(cmd)
	})
}

func runDelete(c *cobra.Command, path string, purge, yes bool) error {
	group, title, _, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if purge && !yes {
		ok := secretio.Confirm(
			"permanently purge '"+path+"'? this cannot be undone",
			c.InOrStdin(), c.ErrOrStderr(), secretio.IsTerminal(os.Stdin))
		if !ok {
			return kdbxerr.NotConfirmed("purge not confirmed")
		}
	}
	if purge {
		return vault.Purge(ctx.Vault, ctx.KeyFile, group, title)
	}
	return vault.Trash(ctx.Vault, ctx.KeyFile, group, title)
}
```

- [ ] **Step 2: Implement `mv`**

Create `cmd/mv.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "mv SRC DST",
			Short: "Rename or move an entry, repointing affected var mappings",
			Args:  cobra.ExactArgs(2),
			RunE: func(c *cobra.Command, args []string) error {
				return runMv(c, args[0], args[1])
			},
		})
	})
}

func runMv(c *cobra.Command, src, dst string) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if err := vault.Move(ctx.Vault, ctx.KeyFile, src, dst); err != nil {
		return err
	}
	srcEntry, dstEntry := pointer.EntryOf(src), pointer.EntryOf(dst)
	if n := ctx.Pointer.RepointVars(ctx.Env, srcEntry, dstEntry); n > 0 {
		if err := ctx.Pointer.Save(); err != nil {
			return err
		}
		fmt.Fprintf(c.ErrOrStderr(),
			"re-pointed %d var mapping(s) %s -> %s in %s — review and commit\n",
			n, srcEntry, dstEntry, pointer.Name)
	}
	return nil
}
```

- [ ] **Step 3: Write the CLI-contract test**

Create `testdata/script/delete_mv.txtar`:

```
exec kdbx init
stdin secret.txt
exec kdbx set api/openai --var OPENAI_API_KEY
stdin secret.txt
exec kdbx set api/openai:ORG_ID

# soft delete hides the entry from list but keeps it recoverable
exec kdbx delete api/openai
exec kdbx list
! stdout 'api/openai'

# purge without a TTY and without --yes refuses with exit 4
stdin secret.txt
exec kdbx set api/temp
! exec kdbx delete api/temp --purge
stderr 'needs an interactive terminal to confirm'
stderr 'kdbx: delete failed: NotConfirmed'

# purge with the test-only --yes succeeds
exec kdbx delete api/temp --purge --yes
exec kdbx list
! stdout 'api/temp'

# mv repoints var mappings and preserves the :field suffix
stdin secret.txt
exec kdbx set db/primary --var DB_URL
exec kdbx mv db/primary db/main
stderr 're-pointed 1 var mapping\(s\) db/primary -> db/main'
grep '"DB_URL": "db/main"' .keepassxc.json
exec kdbx list
stdout '^db/main$'
! stdout 'db/primary'

# the moved entry still holds its value
exec kdbx get db/main --reveal
stdout '^sk-test-value$'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {}
  }
}
-- secret.txt --
sk-test-value
```

- [ ] **Step 4: Activate the deferred read-ops script**

```bash
mkdir -p testdata/script
git mv testdata/pending/read_ops.txtar testdata/script/read_ops.txtar 2>/dev/null || mv testdata/pending/read_ops.txtar testdata/script/read_ops.txtar
rmdir testdata/pending 2>/dev/null || true
```

- [ ] **Step 5: Run the whole contract suite**

Run: `go test . -run TestScripts -v`
Expected: `envs`, `init_set`, `delete_mv`, and `read_ops` all PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat(delete,mv): recycle-bin delete, confirmed purge, entry moves with var repointing"
```

---

## Task 15: `internal/dotenv` — render and parse

**Files:**
- Create: `internal/dotenv/dotenv.go`, `internal/dotenv/dotenv_test.go`
- Reference: `render_dotenv` / `parse_dotenv` in `$EXTENSIONS_REPO/skills/kdbx/kdbx_core/secretio.py`

**Interfaces:**
- Consumes: `kdbxerr.*`.
- Produces:
  - `func dotenv.Render(order []string, items map[string]string) string` — one `KEY="value"` line per var **in `order`**; escapes `\` → `\\`, `"` → `\"`, newline → `\n`.
  - `func dotenv.Parse(text string) (map[string]string, []string, error)` — returns values and key order; no variable interpolation.

- [ ] **Step 1: Write the failing tests**

Create `internal/dotenv/dotenv_test.go`:

```go
package dotenv

import (
	"reflect"
	"testing"
)

func TestRenderQuotesAndEscapes(t *testing.T) {
	got := Render(
		[]string{"SIMPLE", "WITH_QUOTE", "WITH_BACKSLASH", "WITH_NEWLINE"},
		map[string]string{
			"SIMPLE":         "value",
			"WITH_QUOTE":     `a"b`,
			"WITH_BACKSLASH": `a\b`,
			"WITH_NEWLINE":   "a\nb",
		},
	)
	want := "SIMPLE=\"value\"\n" +
		"WITH_QUOTE=\"a\\\"b\"\n" +
		"WITH_BACKSLASH=\"a\\\\b\"\n" +
		"WITH_NEWLINE=\"a\\nb\"\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderPreservesGivenOrder(t *testing.T) {
	got := Render([]string{"Z", "A"}, map[string]string{"A": "1", "Z": "2"})
	if got != "Z=\"2\"\nA=\"1\"\n" {
		t.Fatalf("order not preserved: %q", got)
	}
}

func TestParseReadsSimpleAndQuotedValues(t *testing.T) {
	vals, order, err := Parse("A=1\nB=\"two\"\n# comment\nC='three'\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "1", "B": "two", "C": "three"}
	if !reflect.DeepEqual(vals, want) {
		t.Fatalf("got %v, want %v", vals, want)
	}
	if !reflect.DeepEqual(order, []string{"A", "B", "C"}) {
		t.Fatalf("order %v", order)
	}
}

func TestParseDoesNotInterpolate(t *testing.T) {
	t.Setenv("OUTER", "leaked")
	vals, _, err := Parse("A=\"$OUTER\"\nB=\"${OUTER}\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if vals["A"] != "$OUTER" || vals["B"] != "${OUTER}" {
		t.Fatalf("interpolation happened: %v", vals)
	}
}

func TestRoundTrip(t *testing.T) {
	order := []string{"A", "B"}
	items := map[string]string{"A": `weird "value" \ here`, "B": "line1\nline2"}
	vals, _, err := Parse(Render(order, items))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vals, items) {
		t.Fatalf("round-trip lost data:\ngot  %q\nwant %q", vals, items)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/dotenv/...`
Expected: FAIL — `undefined: Render`.

- [ ] **Step 3: Implement**

```bash
go get github.com/joho/godotenv@latest
```

Create `internal/dotenv/dotenv.go`:

```go
// Package dotenv renders and parses .env files for kdbx export/import. Values
// are always double-quoted on output and never interpolated on input (spec C5).
package dotenv

import (
	"strings"

	"github.com/joho/godotenv"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Render writes one KEY="value" line per name in order.
func Render(order []string, items map[string]string) string {
	var b strings.Builder
	for _, k := range order {
		v, ok := items[k]
		if !ok {
			continue
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(quote(v))
		b.WriteString("\n")
	}
	return b.String()
}

func quote(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return `"` + r.Replace(v) + `"`
}

// Parse reads dotenv text, returning values and the key order they appeared in.
func Parse(text string) (map[string]string, []string, error) {
	vals, err := godotenv.Unmarshal(text)
	if err != nil {
		return nil, nil, kdbxerr.Wrap(err, "Preflight", 7, "parsing dotenv input")
	}
	var order []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if _, exists := vals[key]; !exists {
			continue
		}
		if !contains(order, key) {
			order = append(order, key)
		}
	}
	return vals, order, nil
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
```

⚠️ `godotenv.Unmarshal` does expand `${VAR}` inside **double-quoted** values in some versions. Run `TestParseDoesNotInterpolate` first: if it fails, replace the `godotenv` call with a hand-rolled line parser (strip comments, split on the first `=`, unquote `"`/`'`, unescape `\n`, `\"`, `\\`) rather than accepting interpolation — the Python implementation passes `interpolate=False` and matching it is the contract.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/dotenv/... -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dotenv go.mod go.sum
git commit -m "feat(dotenv): exact-quoting renderer and non-interpolating parser"
```

---

## Task 16: `internal/runner` — child process spawning

**Why it matters:** `kdbx run -- <cmd>` is the tool's most-used operation. The Windows `PATHEXT` case is a known production bug class — the Python codebase hit exactly this (`shutil.which` fix) in a sibling skill.

**Files:**
- Create: `internal/runner/runner.go`, `internal/runner/runner_test.go`

**Interfaces:**
- Consumes: `kdbxerr.*`.
- Produces:
  - `func runner.Lookup(name string) (string, error)` — resolves `name` via PATH (honoring `PATHEXT` on Windows); returns `kdbxerr.NotFound` when unresolvable.
  - `func runner.Run(argv []string, inject map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error)` — merges `inject` over the parent environment, spawns, forwards `SIGINT`/`SIGTERM` to the child, waits, and returns the child's exit code (128+signal when signalled).

- [ ] **Step 1: Write the failing tests**

Create `internal/runner/runner_test.go`:

```go
package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildEcho compiles a tiny helper that prints an env var and exits with a code.
func buildEcho(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "helper.go")
	code := `package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	fmt.Println(os.Getenv("KDBX_TEST_VAR"))
	if len(os.Args) > 1 {
		n, _ := strconv.Atoi(os.Args[1])
		os.Exit(n)
	}
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "helper")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v\n%s", err, out)
	}
	return bin
}

func TestRunInjectsVariables(t *testing.T) {
	bin := buildEcho(t)
	var out bytes.Buffer
	code, err := Run([]string{bin}, map[string]string{"KDBX_TEST_VAR": "injected"}, nil, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if strings.TrimSpace(out.String()) != "injected" {
		t.Fatalf("child saw %q", out.String())
	}
}

func TestRunPassesThroughExitCode(t *testing.T) {
	bin := buildEcho(t)
	var out bytes.Buffer
	code, err := Run([]string{bin, "42"}, nil, nil, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 42 {
		t.Fatalf("exit code %d, want 42", code)
	}
}

func TestRunInheritsParentEnvironment(t *testing.T) {
	t.Setenv("KDBX_TEST_VAR", "from-parent")
	bin := buildEcho(t)
	var out bytes.Buffer
	if _, err := Run([]string{bin}, nil, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "from-parent" {
		t.Fatalf("child saw %q, want the inherited value", out.String())
	}
}

func TestRunInjectionOverridesParentEnvironment(t *testing.T) {
	t.Setenv("KDBX_TEST_VAR", "from-parent")
	bin := buildEcho(t)
	var out bytes.Buffer
	if _, err := Run([]string{bin}, map[string]string{"KDBX_TEST_VAR": "override"}, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "override" {
		t.Fatalf("child saw %q, want the injected value", out.String())
	}
}

func TestLookupResolvesViaPath(t *testing.T) {
	bin := buildEcho(t)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := Lookup("helper")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if filepath.Base(got) != filepath.Base(bin) {
		t.Fatalf("resolved %q, want %q", got, bin)
	}
}

func TestLookupUnknownCommandIsNotFound(t *testing.T) {
	if _, err := Lookup("definitely-not-a-real-binary-xyz"); err == nil {
		t.Fatal("expected NotFound for an unresolvable command")
	}
}

func TestRunUnresolvableCommandIsNotFound(t *testing.T) {
	if _, err := Run([]string{"definitely-not-a-real-binary-xyz"}, nil, nil, nil, nil); err == nil {
		t.Fatal("expected an error for an unresolvable command")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/runner/...`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Implement**

Create `internal/runner/runner.go`:

```go
// Package runner spawns the child process for `kdbx run`, injecting resolved
// secrets into its environment and passing its exit status straight through
// (spec C5).
package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Lookup resolves name against PATH. On Windows exec.LookPath already honors
// PATHEXT, so a .bat/.cmd shim resolves correctly — the failure mode that bit
// the Python implementation.
func Lookup(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", kdbxerr.Wrap(err, "NotFound", 2, "command not found: %s", name)
	}
	return p, nil
}

// Run executes argv with inject merged over the parent environment and returns
// the child's exit code.
func Run(argv []string, inject map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(argv) == 0 {
		return 0, kdbxerr.NotFound("kdbx run: no command given (use: run -- <cmd> ...)")
	}
	exe, err := Lookup(argv[0])
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(exe, argv[1:]...)
	cmd.Env = mergeEnv(os.Environ(), inject)
	cmd.Stdin = orDefault(stdin, os.Stdin)
	cmd.Stdout = orDefaultW(stdout, os.Stdout)
	cmd.Stderr = orDefaultW(stderr, os.Stderr)

	if err := cmd.Start(); err != nil {
		return 0, kdbxerr.Wrap(err, "Runtime", 1, "starting %s", argv[0])
	}

	// Forward interrupts so Ctrl-C reaches the child, not just kdbx.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()

	err = cmd.Wait()
	close(done)
	signal.Stop(sigs)

	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	return 0, kdbxerr.Wrap(err, "Runtime", 1, "waiting for %s", argv[0])
}

// mergeEnv layers overrides on top of base, replacing existing assignments.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]bool{}
	for _, kv := range base {
		name := kv
		if i := indexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if v, ok := overrides[name]; ok {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name+"="+v)
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func orDefault(r io.Reader, def io.Reader) io.Reader {
	if r == nil {
		return def
	}
	return r
}

func orDefaultW(w io.Writer, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}
```

⚠️ `ee.ExitCode()` returns `-1` when the child was killed by a signal. On POSIX, convert that to `128 + int(status.Signal())` using `ee.Sys().(syscall.WaitStatus)`. Add that branch and a test only if a real use case demands it — the Python implementation returns the raw `returncode`, so matching it is acceptable for v1; document the choice in `docs/spike-notes.md`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/runner/... -v`
Expected: all PASS on the current OS. CI runs the same on Windows.

- [ ] **Step 5: Commit**

```bash
git add internal/runner
git commit -m "feat(runner): PATH/PATHEXT-aware child spawn with env injection and exit passthrough"
```

---

## Task 17: `run` command

**Files:**
- Create: `cmd/run.go`, `internal/vaultvars/vaultvars.go`, `internal/vaultvars/vaultvars_test.go`, `testdata/script/run.txtar`

**Interfaces:**
- Consumes: `envctx.Context`, `vault.Open`, `pointer.ParseEntryPath`, `runner.Run`, `kdbxerr.Drift`.
- Produces:
  - `func vaultvars.Resolve(ctx *envctx.Context, allowMissing bool) (map[string]string, []string, error)` — resolves every mapped var to its value; returns values, the ordered var names actually resolved, and `kdbxerr.Drift` (exit 5) on the first unresolved var unless `allowMissing`. **Shared by `run` and `export`** — do not duplicate this loop.

- [ ] **Step 1: Write the failing test**

Create `internal/vaultvars/vaultvars_test.go`:

```go
package vaultvars

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/vault"
)

func setup(t *testing.T, vars string) *envctx.Context {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEEPASSXC_DIR", filepath.Join(dir, "kpxc"))
	t.Setenv("KDBX_ENV", "")
	body := `{"project":"demo","defaultEnv":"dev","envs":{"dev":{"vars":` + vars + `}}}`
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := envctx.Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestResolveReturnsValuesInPointerOrder(t *testing.T) {
	ctx := setup(t, `{"ZED":"api/zed:password","ALPHA":"api/alpha:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "zed", "password", "z"); err != nil {
		t.Fatal(err)
	}
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "alpha", "password", "a"); err != nil {
		t.Fatal(err)
	}
	vals, order, err := Resolve(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if vals["ZED"] != "z" || vals["ALPHA"] != "a" {
		t.Fatalf("values %v", vals)
	}
	if len(order) != 2 || order[0] != "ZED" || order[1] != "ALPHA" {
		t.Fatalf("order %v, want [ZED ALPHA]", order)
	}
}

func TestResolveUnresolvedIsDrift(t *testing.T) {
	ctx := setup(t, `{"MISSING":"api/absent:password"}`)
	if _, _, err := Resolve(ctx, false); err == nil {
		t.Fatal("expected Drift for an unresolved mapping")
	}
}

func TestResolveAllowMissingSkipsUnresolved(t *testing.T) {
	ctx := setup(t, `{"PRESENT":"api/there:password","MISSING":"api/absent:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "there", "password", "v"); err != nil {
		t.Fatal(err)
	}
	vals, order, err := Resolve(ctx, true)
	if err != nil {
		t.Fatalf("allowMissing should not fail: %v", err)
	}
	if len(vals) != 1 || vals["PRESENT"] != "v" {
		t.Fatalf("values %v", vals)
	}
	if len(order) != 1 || order[0] != "PRESENT" {
		t.Fatalf("order %v", order)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/vaultvars/...`
Expected: FAIL — `undefined: Resolve`.

- [ ] **Step 3: Implement**

Create `internal/vaultvars/vaultvars.go`:

```go
// Package vaultvars resolves an environment's mapped variables to their secret
// values. It is the single implementation shared by `run` and `export`.
package vaultvars

import (
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

// Resolve reads every mapped var. Unless allowMissing is set, the first var
// that does not resolve is a Drift error (exit 5).
func Resolve(ctx *envctx.Context, allowMissing bool) (map[string]string, []string, error) {
	h, err := vault.Open(ctx.Vault, ctx.KeyFile)
	if err != nil {
		return nil, nil, err
	}
	defer h.Close()

	vals := map[string]string{}
	var order []string
	for _, name := range ctx.VarOrder {
		path := ctx.Vars[name]
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			if allowMissing {
				continue
			}
			return nil, nil, kdbxerr.Drift("unresolved var %s -> %s", name, path)
		}
		v, gerr := h.GetField(group, title, field)
		if gerr != nil {
			if allowMissing {
				continue
			}
			return nil, nil, kdbxerr.Drift("unresolved var %s -> %s", name, path)
		}
		vals[name] = v
		order = append(order, name)
	}
	return vals, order, nil
}
```

- [ ] **Step 4: Implement the `run` command**

Create `cmd/run.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/runner"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var allowMissing bool
		cmd := &cobra.Command{
			Use:   "run [--allow-missing] -- CMD [ARGS...]",
			Short: "Inject this environment's secrets and exec a command",
			RunE: func(c *cobra.Command, args []string) error {
				return runRun(c, args, allowMissing)
			},
		}
		cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "skip vars that do not resolve")
		cmd.Flags().SetInterspersed(false)
		root.AddCommand(cmd)
	})
}

// runExitCode carries a child's exit status out through cobra's error path
// without printing a failure line — the child already reported for itself.
type runExitCode struct{ code int }

func (e *runExitCode) Error() string { return "" }

func runRun(c *cobra.Command, args []string, allowMissing bool) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return kdbxerr.NotFound("kdbx run: no command given (use: run -- <cmd> ...)")
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	vals, _, err := vaultvars.Resolve(ctx, allowMissing)
	if err != nil {
		return err
	}
	code, err := runner.Run(args, vals, c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr())
	if err != nil {
		return err
	}
	if code != 0 {
		return &runExitCode{code: code}
	}
	return nil
}
```

Teach `Execute` about the pass-through code. In `cmd/root.go`, inside `Execute`, before the `kdbxerr.Report` call:

```go
	var passthrough *runExitCode
	if errors.As(err, &passthrough) {
		return passthrough.code
	}
```

Add `"errors"` to that file's imports.

- [ ] **Step 5: Write the CLI-contract test**

Create `testdata/script/run.txtar`:

```
exec kdbx init
stdin secret.txt
exec kdbx set api/openai --var OPENAI_API_KEY

# run injects the mapped var into the child
exec kdbx run -- printenv-shim OPENAI_API_KEY
stdout '^sk-test-value$'
stderr 'ACTIVE ENV: dev'

# run passes the child's exit code straight through
! exec kdbx run -- exit-shim 7
stdout '^exiting 7$'

# run with no command is exit 2
! exec kdbx run
stderr 'kdbx: run failed: NotFound'

# an unresolvable mapping is drift (exit 5)
exec kdbx delete api/openai --purge --yes
! exec kdbx run -- printenv-shim OPENAI_API_KEY
stderr 'kdbx: run failed: Drift'

# --allow-missing lets the child run without the var
exec kdbx run --allow-missing -- printenv-shim OPENAI_API_KEY
stdout '^$'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {}
  }
}
-- secret.txt --
sk-test-value
```

Register the two shims in `main_test.go`'s command map so the script can call them:

```go
func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"kdbx": cmd.Execute,
		"printenv-shim": func() int {
			if len(os.Args) > 1 {
				fmt.Println(os.Getenv(os.Args[1]))
			}
			return 0
		},
		"exit-shim": func() int {
			n := 0
			if len(os.Args) > 1 {
				n, _ = strconv.Atoi(os.Args[1])
			}
			fmt.Printf("exiting %d\n", n)
			return n
		},
	}))
}
```

Add `"fmt"` and `"strconv"` to `main_test.go`'s imports.

- [ ] **Step 6: Run and commit**

Run: `go test . -run TestScripts/run -v && go test ./internal/vaultvars/... -v`
Expected: PASS.

```bash
git add cmd/run.go cmd/root.go internal/vaultvars main_test.go testdata/script/run.txtar
git commit -m "feat(run): inject mapped secrets into a child process and pass its exit code through"
```

---

## Task 18: `export` and `import` commands

**Files:**
- Create: `cmd/export.go`, `cmd/import.go`, `testdata/script/export_import.txtar`

**Interfaces:**
- Consumes: `vaultvars.Resolve`, `dotenv.Render`, `dotenv.Parse`, `secretio.AtomicWriteSecret`, `vault.SetField`, `pointer.SetVar`.
- Produces: no new Go API.

- [ ] **Step 1: Implement `export`**

Create `cmd/export.go`:

```go
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/dotenv"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var out string
		var allowMissing bool
		cmd := &cobra.Command{
			Use:   "export",
			Short: "Render mapped vars as a dotenv file (plaintext — handle with care)",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runExport(c, out, allowMissing)
			},
		}
		cmd.Flags().StringVar(&out, "out", "", "write to this file (0600) instead of stdout")
		cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "skip vars that do not resolve")
		root.AddCommand(cmd)
	})
}

func runExport(c *cobra.Command, out string, allowMissing bool) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	vals, order, err := vaultvars.Resolve(ctx, allowMissing)
	if err != nil {
		return err
	}
	text := dotenv.Render(order, vals)
	if out == "" {
		fmt.Fprint(c.OutOrStdout(), text)
		return nil
	}
	fmt.Fprintf(c.ErrOrStderr(),
		"NOTE: ensure %s is gitignored (it holds plaintext secrets)\n", filepath.Base(out))
	if err := secretio.AtomicWriteSecret(out, []byte(text)); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(), "wrote %d vars to %s (0600)\n", len(order), out)
	return nil
}
```

- [ ] **Step 2: Implement `import`**

Create `cmd/import.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/dotenv"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "import FILE",
			Short: "Read an existing .env into the vault and register var mappings",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runImport(c, args[0])
			},
		})
	})
}

func runImport(c *cobra.Command, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return kdbxerr.Wrap(err, "NotFound", 2, "reading %s", file)
	}
	vals, order, err := dotenv.Parse(string(b))
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	for _, name := range order {
		path := "imported/" + name + ":password"
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			return perr
		}
		if err := vault.SetField(ctx.Vault, ctx.KeyFile, group, title, field, vals[name]); err != nil {
			return err
		}
		ctx.Pointer.SetVar(ctx.Env, name, path)
	}
	if err := ctx.Pointer.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(),
		"imported %d vars. Reminder: remove/gitignore the source .env; rotate anything ever committed.\n",
		len(order))
	return nil
}
```

- [ ] **Step 3: Write the CLI-contract test**

Create `testdata/script/export_import.txtar`:

```
exec kdbx init

# import reads a .env, stores each value, and registers mappings
exec kdbx import legacy.env
stderr 'imported 2 vars'
stderr 'rotate anything ever committed'
grep '"API_KEY": "imported/API_KEY:password"' .keepassxc.json
grep '"DB_URL": "imported/DB_URL:password"' .keepassxc.json

exec kdbx list
stdout '^imported/API_KEY$'
stdout '^imported/DB_URL$'

# export round-trips the imported values, in pointer order
exec kdbx export
stdout '^API_KEY="sk-imported"$'
stdout '^DB_URL="postgres://localhost/db"$'

# --out writes a file and warns about gitignore
exec kdbx export --out .env.generated
stderr 'ensure .env.generated is gitignored'
stderr 'wrote 2 vars'
grep '^API_KEY="sk-imported"$' .env.generated

# a drifting mapping fails export with exit 5
exec kdbx delete imported/API_KEY --purge --yes
! exec kdbx export
stderr 'kdbx: export failed: Drift'

# --allow-missing exports what remains
exec kdbx export --allow-missing
stdout '^DB_URL='
! stdout 'API_KEY='

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {}
  }
}
-- legacy.env --
# a legacy dotenv file
API_KEY="sk-imported"
DB_URL="postgres://localhost/db"
```

- [ ] **Step 4: Run and commit**

Run: `go test . -run TestScripts/export_import -v`
Expected: PASS.

```bash
git add cmd/export.go cmd/import.go testdata/script/export_import.txtar
git commit -m "feat(export,import): dotenv round-trip with drift detection and 0600 output"
```

---

## Task 19: `rekey` command

**Files:**
- Create: `cmd/rekey.go`, `testdata/script/rekey.txtar`

**Interfaces:**
- Consumes: `vault.Rekey`, `secretio.Confirm`, `kdbxerr.NotConfirmed`.
- Produces: no new Go API. Reuses the hidden test-only `--yes` flag pattern from Task 14.

- [ ] **Step 1: Implement**

Create `cmd/rekey.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var yes bool
		cmd := &cobra.Command{
			Use:   "rekey",
			Short: "Rotate the environment's keyfile",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runRekey(c, yes)
			},
		}
		cmd.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = cmd.Flags().MarkHidden("yes")
		root.AddCommand(cmd)
	})
}

func runRekey(c *cobra.Command, yes bool) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if !yes {
		ok := secretio.Confirm(
			fmt.Sprintf("rotate the key file for env '%s'? the old key file is deleted", ctx.Env),
			c.InOrStdin(), c.ErrOrStderr(), secretio.IsTerminal(os.Stdin))
		if !ok {
			return kdbxerr.NotConfirmed("rekey not confirmed")
		}
	}
	newKey := ctx.KeyFile + ".new"
	if err := vault.Rekey(ctx.Vault, ctx.KeyFile, newKey); err != nil {
		return err
	}
	if err := os.Rename(newKey, ctx.KeyFile); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "installing the new keyfile")
	}
	fmt.Fprintln(c.ErrOrStderr(),
		"rekeyed. A prior keyfile+vault leak means secrets are already exposed — rotate at source.")
	return nil
}
```

⚠️ `vault.Rekey` (Task 12) deletes nothing; this command performs the atomic `.new` → live rename, matching the Python flow. Confirm `Rekey` leaves the old keyfile intact so a failed rename is recoverable, then verify the old keyfile no longer opens the vault (test below).

- [ ] **Step 2: Write the CLI-contract test**

Create `testdata/script/rekey.txtar`:

```
exec kdbx init
stdin secret.txt
exec kdbx set api/openai --var OPENAI_API_KEY
cp $WORK/kpxc/demo/dev.keyx $WORK/old.keyx

# rekey without a TTY and without --yes refuses (exit 4)
! exec kdbx rekey
stderr 'needs an interactive terminal to confirm'
stderr 'kdbx: rekey failed: NotConfirmed'

# rekey rotates the keyfile and the vault still opens
exec kdbx rekey --yes
stderr 'rekeyed'
exec kdbx get api/openai --reveal
stdout '^sk-test-value$'

# the old keyfile no longer opens the vault
cp $WORK/old.keyx $WORK/kpxc/demo/stale.keyx
env KDBX_ENV=stale
! exec kdbx get api/openai
stderr 'kdbx: get failed: Locked'

-- .keepassxc.json --
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {},
    "stale": {
      "vault": "${KEEPASSXC_DIR}/demo/dev.kdbx",
      "keyFile": "${KEEPASSXC_DIR}/demo/stale.keyx"
    }
  }
}
-- secret.txt --
sk-test-value
```

- [ ] **Step 3: Run and commit**

Run: `go test . -run TestScripts/rekey -v`
Expected: PASS.

```bash
git add cmd/rekey.go testdata/script/rekey.txtar
git commit -m "feat(rekey): confirmed keyfile rotation with atomic replacement"
```

---

## Task 20: `kdbx guard` — the PreToolUse hook engine (spec N3)

**Files:**
- Create: `internal/guard/guard.go`, `internal/guard/guard_test.go`, `cmd/guard.go`
- Reference: `$EXTENSIONS_REPO/plugins/kdbx/hooks/guard.py` — port `decide()` exactly, including wording.

**Interfaces:**
- Consumes: `paths.KeepassxcDir`.
- Produces:
  - `func guard.Decide(command string) string` — deny reason, or `""` to allow.
  - `func guard.Run(stdin io.Reader, stdout io.Writer) int` — reads PreToolUse JSON, writes the deny envelope when applicable, **always returns 0** (fail open).

- [ ] **Step 1: Write the failing tests**

Create `internal/guard/guard_test.go`:

```go
package guard

import (
	"bytes"
	"strings"
	"testing"
)

func TestDecideBlocksAgentWriteOps(t *testing.T) {
	for _, cmd := range []string{
		"kdbx set api/openai",
		"kdbx delete api/openai --purge",
		"kdbx mv a b",
		"kdbx import .env",
		"kdbx rekey",
		"kdbx export --out .env",
		"kdbx get api/openai --reveal",
		"kdbx get api/openai --clip",
	} {
		if got := Decide(cmd); got == "" {
			t.Errorf("Decide(%q) allowed a human-only operation", cmd)
		} else if !strings.Contains(got, "human-only operation") {
			t.Errorf("Decide(%q) = %q, want the role-guard wording", cmd, got)
		}
	}
}

func TestDecideAllowsAgentReadOps(t *testing.T) {
	for _, cmd := range []string{
		"kdbx run -- npm test",
		"kdbx get api/openai",
		"kdbx list",
		"kdbx check",
		"kdbx envs",
		"kdbx init",
		"echo kdbx set api/openai",
		"npm test",
		"",
	} {
		if got := Decide(cmd); got != "" {
			t.Errorf("Decide(%q) denied an allowed command: %s", cmd, got)
		}
	}
}

func TestDecideBlocksNonKdbxToolsTouchingVaultFiles(t *testing.T) {
	for _, cmd := range []string{
		"cat ~/.config/keepassxc/demo/dev.kdbx",
		"xxd dev.keyx",
		"cp dev.KDBX /tmp/",
		"base64 $(cat dev.keyx)",
	} {
		if got := Decide(cmd); got == "" {
			t.Errorf("Decide(%q) allowed a vault read via a non-kdbx tool", cmd)
		} else if !strings.Contains(got, "leak-guard") {
			t.Errorf("Decide(%q) = %q, want the leak-guard wording", cmd, got)
		}
	}
}

func TestDecideAllowsKdbxAndKeepassxcCliTouchingVaultFiles(t *testing.T) {
	for _, cmd := range []string{
		"kdbx run -- printenv",
		"keepassxc-cli ls --no-password -k dev.keyx dev.kdbx",
		"uv run kdbx.py list",
	} {
		if got := Decide(cmd); got != "" {
			t.Errorf("Decide(%q) denied an allowed invoker: %s", cmd, got)
		}
	}
}

func TestDecideInspectsEveryShellSegment(t *testing.T) {
	if Decide("npm test && cat dev.kdbx") == "" {
		t.Fatal("must inspect commands after &&")
	}
	if Decide("echo hi; kdbx set api/x") == "" {
		t.Fatal("must inspect commands after ;")
	}
	if Decide("kdbx list | grep api") != "" {
		t.Fatal("a piped allowed command must stay allowed")
	}
}

func TestRunEmitsDenyEnvelopeAndAlwaysExitsZero(t *testing.T) {
	var out bytes.Buffer
	code := Run(strings.NewReader(`{"tool_input":{"command":"kdbx set api/x"}}`), &out)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (the guard must never break the shell)", code)
	}
	s := out.String()
	for _, want := range []string{`"hookEventName":"PreToolUse"`, `"permissionDecision":"deny"`, "human-only operation"} {
		if !strings.Contains(s, want) {
			t.Fatalf("envelope %q missing %q", s, want)
		}
	}
}

func TestRunAllowsSilently(t *testing.T) {
	var out bytes.Buffer
	if code := Run(strings.NewReader(`{"tool_input":{"command":"kdbx list"}}`), &out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("allow must produce no output, got %q", out.String())
	}
}

func TestRunFailsOpenOnGarbageInput(t *testing.T) {
	var out bytes.Buffer
	if code := Run(strings.NewReader("not json at all"), &out); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("garbage input must fail open silently, got %q", out.String())
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/guard/...`
Expected: FAIL — `undefined: Decide`.

- [ ] **Step 3: Implement**

Create `internal/guard/guard.go`. Port the Python logic faithfully; the shell tokenizer must handle quotes well enough that `$(cat dev.keyx)` still matches.

```go
// Package guard implements the PreToolUse decision that keeps an agent inside
// the read-only half of the kdbx role contract (spec C10, N3). It fails open:
// anything that is not a clear violation is allowed, because a guard must never
// brick the user's shell.
package guard

import (
	"encoding/json"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/yarrasys/kdbx/internal/paths"
)

// allowedInvokers may touch vault material: kdbx itself and the KeePassXC CLI,
// neither of which prints a secret by default.
var allowedInvokers = map[string]bool{
	"kdbx": true, "kdbx.py": true, "keepassxc-cli": true, "keepassxc": true,
}

// blockedOps mutate the vault or expose a value — a human role.
var blockedOps = map[string]bool{
	"set": true, "delete": true, "mv": true, "import": true, "rekey": true, "export": true,
}

var (
	secretRe   = regexp.MustCompile(`(?i)\.(kdbx|keyx)\b`)
	segSplitRe = regexp.MustCompile(`\|\||&&|[;|\n]`)
)

// Decide returns a deny reason for command, or "" to allow it.
func Decide(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	for _, rawSeg := range segSplitRe.Split(command, -1) {
		seg := strings.TrimSpace(rawSeg)
		if seg == "" {
			continue
		}
		tokens := tokenize(seg)

		if op := blockedKdbxOp(tokens); op != "" {
			return "kdbx role-guard: '" + op + "' is a human-only operation (it mutates the vault " +
				"or exposes a secret value). Don't run it as the agent — give the command " +
				"to the human to run in their own terminal (or via `!kdbx ...`)."
		}

		norm := strings.ReplaceAll(seg, `\`, "/")
		hit := secretRe.FindString(norm)
		frag := matchedConfigFragment(strings.ToLower(norm))
		if hit == "" && frag == "" {
			continue
		}
		prog := programOf(tokens)
		allowed := allowedInvokers[prog] ||
			((prog == "uv" || prog == "uvx") && strings.Contains(strings.ToLower(seg), "kdbx"))
		if allowed {
			continue
		}
		what := hit
		if what == "" {
			what = "a KeePassXC config path"
		}
		name := prog
		if name == "" {
			name = "command"
		}
		return "kdbx leak-guard: '" + name + "' would read a KeePassXC vault/keyfile (" + what + "). " +
			"Use `kdbx run -- ...` to inject secrets without printing them."
	}
	return ""
}

// tokenize splits a shell segment into words, stripping simple quoting.
func tokenize(seg string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote rune
	)
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range seg {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// programOf returns the basename of the invoked program, skipping leading
// VAR=value assignments.
func programOf(tokens []string) string {
	for _, tok := range tokens {
		head, _, isAssign := strings.Cut(tok, "=")
		if isAssign && !strings.HasPrefix(tok, "-") && !strings.Contains(head, "/") {
			continue
		}
		return path.Base(strings.ReplaceAll(tok, `\`, "/"))
	}
	return ""
}

// kdbxOp returns the subcommand when this segment actually invokes the kdbx CLI.
func kdbxOp(tokens []string) (string, bool) {
	if len(tokens) == 0 {
		return "", false
	}
	prog := programOf(tokens)
	for i, tok := range tokens {
		base := path.Base(strings.ReplaceAll(tok, `\`, "/"))
		if base != "kdbx" && base != "kdbx.py" {
			continue
		}
		if base == prog || prog == "uv" || prog == "uvx" {
			for _, next := range tokens[i+1:] {
				if !strings.HasPrefix(next, "-") {
					return next, true
				}
			}
			return "", true
		}
		return "", false
	}
	return "", false
}

// blockedKdbxOp names the offending operation, or "" when the segment is fine.
func blockedKdbxOp(tokens []string) string {
	op, isKdbx := kdbxOp(tokens)
	if !isKdbx {
		return ""
	}
	if blockedOps[op] {
		return op
	}
	if op == "get" {
		for _, t := range tokens {
			if t == "--reveal" || t == "--clip" {
				return "get --reveal/--clip"
			}
		}
	}
	return ""
}

// matchedConfigFragment reports which KeePassXC config-dir fragment appears in
// the lowercased, forward-slashed segment.
func matchedConfigFragment(lowerSeg string) string {
	frags := []string{".config/keepassxc/", "appdata/local/keepassxc/"}
	if v := paths.KeepassxcDir(); v != "" {
		frags = append(frags, strings.ToLower(strings.ReplaceAll(v, `\`, "/"))+"/")
	}
	for _, env := range []string{"XDG_CONFIG_HOME", "LOCALAPPDATA"} {
		if v := os.Getenv(env); v != "" {
			frags = append(frags,
				strings.ToLower(strings.TrimRight(strings.ReplaceAll(v, `\`, "/"), "/"))+"/keepassxc/")
		}
	}
	for _, f := range frags {
		if strings.Contains(lowerSeg, f) {
			return f
		}
	}
	return ""
}

type hookInput struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// Run reads a PreToolUse payload and emits a deny envelope when warranted. It
// always returns 0.
func Run(stdin io.Reader, stdout io.Writer) int {
	b, err := io.ReadAll(stdin)
	if err != nil {
		return 0
	}
	var in hookInput
	if err := json.Unmarshal(b, &in); err != nil {
		return 0
	}
	reason := Decide(in.ToolInput.Command)
	if reason == "" {
		return 0
	}
	var out hookOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
	return 0
}
```

- [ ] **Step 4: Add the command**

Create `cmd/guard.go`:

```go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/guard"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var hook string
		cmd := &cobra.Command{
			Use:   "guard",
			Short: "Agent role-guard: decide a PreToolUse hook payload read from stdin",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				guard.Run(c.InOrStdin(), c.OutOrStdout())
				return nil
			},
		}
		cmd.Flags().StringVar(&hook, "hook", "pretooluse", "hook event to evaluate")
		root.AddCommand(cmd)
		_ = os.Stdin
	})
}
```

Remove the `_ = os.Stdin` line and the `os` import — they exist only to show the file compiled standalone; delete both before committing.

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/guard/... -v && go build ./...`
Expected: all PASS.

```bash
git add internal/guard cmd/guard.go
git commit -m "feat(guard): built-in PreToolUse role/leak guard replacing the plugin's Python hook"
```

---

## Task 21: `kdbx mcp` — stdio MCP server (spec N2)

**Files:**
- Create: `internal/mcpserver/mcpserver.go`, `internal/mcpserver/mcpserver_test.go`, `cmd/mcp.go`
- Reference: `$EXTENSIONS_REPO/plugins/kdbx/mcp/server.py` — same five tool names and descriptions.

**Interfaces:**
- Consumes: `envctx.Resolve`, `vault.Open`, `vaultvars.Resolve`, `runner.Run`, `pointer.*`, `secretio.Mask`.
- Produces:
  - `func mcpserver.Tools() []ToolSpec` where `type ToolSpec struct{ Name, Description string; Handler func(ctx context.Context, args map[string]any) (string, error) }` — the five read-only tools: `kdbx_list`, `kdbx_envs`, `kdbx_check`, `kdbx_get` (masked only), `kdbx_run`.
  - `func mcpserver.Serve(ctx context.Context) error` — runs the server over stdio.

- [ ] **Step 1: Write the failing tests**

Create `internal/mcpserver/mcpserver_test.go`:

```go
package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/vault"
)

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEEPASSXC_DIR", filepath.Join(dir, "kpxc"))
	t.Setenv("KDBX_ENV", "")
	body := `{"project":"demo","defaultEnv":"dev","envs":{"dev":{"vars":{"API_KEY":"api/openai:password"}},"prod":{}}}`
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := envctx.Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		t.Fatal(err)
	}
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "openai", "password", "sk-secret"); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func handler(t *testing.T, name string) func(context.Context, map[string]any) (string, error) {
	t.Helper()
	for _, spec := range Tools() {
		if spec.Name == name {
			return spec.Handler
		}
	}
	t.Fatalf("tool %q not registered", name)
	return nil
}

func TestExposesExactlyTheFiveReadOnlyTools(t *testing.T) {
	got := map[string]bool{}
	for _, spec := range Tools() {
		got[spec.Name] = true
		if spec.Description == "" {
			t.Errorf("tool %q has no description", spec.Name)
		}
	}
	want := []string{"kdbx_list", "kdbx_envs", "kdbx_check", "kdbx_get", "kdbx_run"}
	if len(got) != len(want) {
		t.Fatalf("registered %v, want exactly %v", got, want)
	}
	for _, n := range want {
		if !got[n] {
			t.Errorf("missing tool %q", n)
		}
	}
	for _, forbidden := range []string{"kdbx_set", "kdbx_delete", "kdbx_export", "kdbx_import", "kdbx_rekey", "kdbx_mv"} {
		if got[forbidden] {
			t.Errorf("write tool %q must never be exposed over MCP", forbidden)
		}
	}
}

func TestGetToolNeverReturnsAValue(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_get")(context.Background(), map[string]any{"path": "api/openai"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("kdbx_get leaked the value: %q", out)
	}
	if !strings.Contains(out, "(set, hidden)") {
		t.Fatalf("expected the mask, got %q", out)
	}
}

func TestListToolReturnsPaths(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_list")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "api/openai") {
		t.Fatalf("list output %q", out)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatal("list leaked a value")
	}
}

func TestEnvsToolMarksTheActiveEnv(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_envs")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("envs output %q", out)
	}
}

func TestCheckToolReportsResolution(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_check")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "MISSING") {
		t.Fatalf("expected a clean check, got %q", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/mcpserver/...`
Expected: FAIL — `undefined: Tools`.

- [ ] **Step 3: Implement**

```bash
go get github.com/modelcontextprotocol/go-sdk@latest
```

Create `internal/mcpserver/mcpserver.go`:

```go
// Package mcpserver exposes kdbx's read-only operations over MCP (spec N2). The
// role contract applies to machines too: there are no write tools, and no tool
// ever returns a secret value.
package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/runner"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

// ToolSpec is one MCP tool, kept transport-agnostic so it is directly testable.
type ToolSpec struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func currentContext() (*envctx.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return envctx.Resolve("", cwd)
}

// Tools returns every tool this server exposes.
func Tools() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "kdbx_list",
			Description: "List entry paths in the active vault. Never returns secret values.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				entries, err := h.ListEntries()
				if err != nil {
					return "", err
				}
				group := str(args, "group")
				var b bytes.Buffer
				for _, p := range entries {
					if group == "" || strings.HasPrefix(p, group) {
						b.WriteString(p + "\n")
					}
				}
				return b.String(), nil
			},
		},
		{
			Name:        "kdbx_envs",
			Description: "List configured environments and mark the active one.",
			Handler: func(_ context.Context, _ map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				var b bytes.Buffer
				for _, name := range ctx.Pointer.EnvNames() {
					marker := "  "
					if name == ctx.Env {
						marker = "* "
					}
					b.WriteString(marker + name + "\n")
				}
				fmt.Fprintf(&b, "active: %s (source: %s)\n", ctx.Env, ctx.Source)
				return b.String(), nil
			},
		},
		{
			Name:        "kdbx_check",
			Description: "Verify every mapped variable resolves in the active environment.",
			Handler: func(_ context.Context, _ map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				var b bytes.Buffer
				missing := 0
				for _, name := range ctx.VarOrder {
					path := ctx.Vars[name]
					group, title, field, perr := pointer.ParseEntryPath(path)
					if perr == nil {
						if _, gerr := h.GetField(group, title, field); gerr == nil {
							continue
						}
					}
					missing++
					fmt.Fprintf(&b, "MISSING %s -> %s\n", name, path)
				}
				if missing == 0 {
					b.WriteString("ok: every mapped var resolves\n")
				}
				return b.String(), nil
			},
		},
		{
			Name: "kdbx_get",
			Description: "Confirm a secret exists at PATH. Always masked — this tool never " +
				"returns a secret value. Use kdbx_run to actually use a secret.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				path := str(args, "path")
				group, title, field, err := pointer.ParseEntryPath(path)
				if err != nil {
					return "", err
				}
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				if _, err := h.GetField(group, title, field); err != nil {
					return "", err
				}
				return secretio.Mask + "\n", nil
			},
		},
		{
			Name: "kdbx_run",
			Description: "Run a command with the active environment's secrets injected. The " +
				"secrets are never printed.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				line := str(args, "command")
				argv := strings.Fields(line)
				if len(argv) == 0 {
					return "", fmt.Errorf("no command given")
				}
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				vals, _, err := vaultvars.Resolve(ctx, false)
				if err != nil {
					return "", err
				}
				var out bytes.Buffer
				code, err := runner.Run(argv, vals, nil, &out, &out)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&out, "\n[exit %d]\n", code)
				return out.String(), nil
			},
		},
	}
}

type toolArgs struct {
	Path    string `json:"path,omitempty" jsonschema:"entry path, e.g. api/openai"`
	Group   string `json:"group,omitempty" jsonschema:"optional group prefix filter"`
	Command string `json:"command,omitempty" jsonschema:"command line to run"`
}

// Serve runs the stdio MCP server until the client disconnects.
func Serve(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{Name: "kdbx", Version: "1"}, nil)
	for _, spec := range Tools() {
		spec := spec
		mcp.AddTool(server,
			&mcp.Tool{Name: spec.Name, Description: spec.Description},
			func(ctx context.Context, req *mcp.CallToolRequest, in toolArgs) (
				*mcp.CallToolResult, any, error) {
				out, err := spec.Handler(ctx, map[string]any{
					"path": in.Path, "group": in.Group, "command": in.Command,
				})
				if err != nil {
					return nil, nil, err
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: out}},
				}, nil, nil
			})
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
```

⚠️ The `mcp.AddTool` signature moves between SDK releases. Run `go doc github.com/modelcontextprotocol/go-sdk/mcp AddTool` and adapt `Serve` to the installed version — the `Tools()` function and its tests are the contract, and they must not change to accommodate the SDK.

- [ ] **Step 4: Add the command**

Create `cmd/mcp.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yarrasys/kdbx/internal/mcpserver"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "mcp",
			Short: "Run a read-only MCP server over stdio",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return mcpserver.Serve(c.Context())
			},
		})
	})
}
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/mcpserver/... -v && go build ./...`
Expected: all PASS.

```bash
git add internal/mcpserver cmd/mcp.go go.mod go.sum
git commit -m "feat(mcp): stdio MCP server exposing the five read-only kdbx tools"
```

---

## Task 22: `completion`, the engine-boundary guard test, and help polish

**Files:**
- Create: `cmd/completion.go`, `internal/boundary_test.go`, `testdata/script/help.txtar`

**Interfaces:**
- Consumes: cobra's built-in completion generator.
- Produces: `kdbx completion [bash|zsh|fish|powershell]`, plus an automated test enforcing spec D2.

- [ ] **Step 1: Write the failing boundary test**

Create `internal/boundary_test.go`:

```go
package internal_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOnlyVaultImportsTheEngine enforces spec D2: internal/vault is the single
// swap point for the KDBX engine. Any other package importing gokeepasslib is a
// design regression, not a style nit.
func TestOnlyVaultImportsTheEngine(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	const engine = "github.com/tobischo/gokeepasslib"

	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == ".git" || name == "dist" || name == "interop" || name == "testdata" {
			return filepath.SkipDir
		}
		pkg, perr := build.ImportDir(p, 0)
		if perr != nil {
			return nil // not a Go package directory
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		for _, imp := range append(pkg.Imports, pkg.TestImports...) {
			if !strings.HasPrefix(imp, engine) {
				continue
			}
			if rel == "internal/vault" {
				continue
			}
			t.Errorf("engine boundary violated: %s imports %s (only internal/vault may)", rel, imp)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run to verify it passes on the current tree**

Run: `go test ./internal/ -run TestOnlyVaultImportsTheEngine -v`
Expected: PASS. To prove the test has teeth, temporarily add `_ "github.com/tobischo/gokeepasslib/v3"` to `internal/paths/paths.go`, re-run (expect FAIL), then remove it.

- [ ] **Step 3: Add the completion command**

Create `cmd/completion.go`:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:       "completion [bash|zsh|fish|powershell]",
			Short:     "Emit a shell completion script",
			Args:      cobra.ExactValidArgs(1),
			ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
			RunE: func(c *cobra.Command, args []string) error {
				out := c.OutOrStdout()
				switch args[0] {
				case "bash":
					return c.Root().GenBashCompletionV2(out, true)
				case "zsh":
					return c.Root().GenZshCompletion(out)
				case "fish":
					return c.Root().GenFishCompletion(out, true)
				case "powershell":
					return c.Root().GenPowerShellCompletionWithDesc(out)
				}
				return fmt.Errorf("unsupported shell %q", args[0])
			},
		})
	})
}
```

- [ ] **Step 4: Write the help/CLI-surface contract test**

Create `testdata/script/help.txtar`:

```
# every documented operation is present
exec kdbx --help
stdout 'init'
stdout 'set'
stdout 'get'
stdout 'list'
stdout 'delete'
stdout 'mv'
stdout 'run'
stdout 'export'
stdout 'import'
stdout 'check'
stdout 'envs'
stdout 'rekey'
stdout 'mcp'
stdout 'guard'
stdout 'completion'

# install-launcher is gone (spec D5) — the binary is the launcher
! stdout 'install-launcher'

# the test-only confirmation bypass stays hidden
! stdout 'test-only'

# completion scripts generate
exec kdbx completion bash
stdout 'complete'
exec kdbx completion zsh
stdout 'compdef'
```

- [ ] **Step 5: Run and commit**

Run: `go test ./... -v`
Expected: all PASS.

```bash
git add cmd/completion.go internal/boundary_test.go testdata/script/help.txtar
git commit -m "feat(completion): shell completions; test: enforce the engine boundary and CLI surface"
```

---

## Task 23: Release engineering — GoReleaser, install.sh, signing, docs

**Files:**
- Create: `.goreleaser.yaml`, `install.sh`, `.github/workflows/release.yml`, `Dockerfile`, `README.md`, `SECURITY.md`, `NOTICE`, `CHANGELOG.md`, `AGENTS.md`, `CLAUDE.md` (symlink)

**Interfaces:**
- Consumes: `cmd.Version` (Task 1), injected via ldflags.
- Produces: a tagged release pipeline producing signed archives for darwin/linux/windows × amd64/arm64, a Homebrew tap formula, a container image, and a curl installer.

- [ ] **Step 1: Add the GoReleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2
project_name: kdbx

before:
  hooks:
    - go mod tidy
    - go test ./...

builds:
  - id: kdbx
    main: .
    binary: kdbx
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    flags: [-trimpath]
    ldflags:
      - -s -w -X github.com/yarrasys/kdbx/cmd.Version={{.Version}}

archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    files: [README.md, LICENSE, NOTICE]

checksum:
  name_template: SHA256SUMS

signs:
  - cmd: cosign
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-signature=${signature}"
      - "--output-certificate=${certificate}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum
    output: true

brews:
  - repository:
      owner: yarrasys
      name: homebrew-tap
    homepage: https://github.com/yarrasys/kdbx
    description: Per-project, per-env KeePassXC credentials for humans and agents
    license: MIT
    test: |
      system "#{bin}/kdbx", "--version"

dockers:
  - image_templates:
      - "ghcr.io/yarrasys/kdbx:{{ .Version }}"
      - "ghcr.io/yarrasys/kdbx:latest"
    dockerfile: Dockerfile

changelog:
  use: github
  sort: asc
```

Create `Dockerfile`:

```dockerfile
FROM scratch
COPY kdbx /kdbx
ENTRYPOINT ["/kdbx"]
```

- [ ] **Step 2: Add the release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: release
on:
  push:
    tags: ["v*"]

permissions:
  contents: write
  packages: write
  id-token: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: {fetch-depth: 0}
      - uses: actions/setup-go@v5
        with: {go-version: "1.25.x"}
      - uses: sigstore/cosign-installer@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

- [ ] **Step 3: Write the installer**

Create `install.sh` (make it executable, `chmod +x install.sh`):

```sh
#!/bin/sh
# Install the kdbx binary. Usage:
#   curl -LsSf https://raw.githubusercontent.com/yarrasys/kdbx/main/install.sh | sh
# Override the destination with KDBX_INSTALL_DIR, the version with KDBX_VERSION.
set -eu

REPO="yarrasys/kdbx"
INSTALL_DIR="${KDBX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${KDBX_VERSION:-latest}"

os="$(uname -s)"
case "$os" in
  Linux)  goos=linux ;;
  Darwin) goos=darwin ;;
  *) echo "kdbx: unsupported OS '$os' (use the Windows release archive)" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) echo "kdbx: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
  tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's#.*/tag/##')"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
  tag="$VERSION"
fi

archive="kdbx_${tag#v}_${goos}_${goarch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "kdbx: downloading $archive"
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"

echo "kdbx: verifying checksum"
( cd "$tmp" && grep " $archive\$" SHA256SUMS | { sha256sum -c - 2>/dev/null || shasum -a 256 -c -; } ) \
  || { echo "kdbx: CHECKSUM MISMATCH — refusing to install" >&2; exit 1; }

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/kdbx" "$INSTALL_DIR/kdbx"

echo "kdbx: installed $INSTALL_DIR/kdbx"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "kdbx: NOTE — $INSTALL_DIR is not on your PATH; add it to your shell profile." ;;
esac
"$INSTALL_DIR/kdbx" --version
```

- [ ] **Step 4: Write the documentation set**

Create `README.md` covering, in this order: what kdbx is (one paragraph); **Install** (curl one-liner, `brew install yarrasys/tap/kdbx`, `go install github.com/yarrasys/kdbx@latest`, container); **Quick start** (`.keepassxc.json` example, `kdbx init`, `kdbx set api/openai --var OPENAI_API_KEY < secret.txt`, `kdbx run -- claude`); the **operations table** copied from spec C5; the **exit-code table** from spec C6; **Roles — agents read, humans write** (spec C10) with the `kdbx guard` and `kdbx mcp` integration snippets; **Agent/editor integration** (`.mcp.json` and hook config); **Security** (keyfile is the sole secret, back it up, cloud-sync warning); and a **Compatibility** note that vaults and pointers are interchangeable with the Python implementation in `yarrasys/extensions`.

Create `SECURITY.md`: private vulnerability reporting via GitHub Security Advisories (no public issues), the threat model in three bullets (keyfile possession is the boundary; no memory zeroization; a leaked keyfile+vault means rotate at source), and release-verification instructions (`SHA256SUMS` + `cosign verify-blob`).

Create `NOTICE` listing every dependency with its license — `gokeepasslib` (MIT), `cobra` (Apache-2.0), `flock` (BSD-3), `godotenv` (MIT), `go-sdk` (MIT), `golang.org/x/term` (BSD-3), `go-internal` (BSD-3, test-only) — and the explicit statement that kdbx has **no GPL dependency**, unlike the Python implementation's `pykeepass`.

Create `CHANGELOG.md` with a `## [Unreleased]` section listing the initial feature set and, under a **Breaking changes vs. the Python implementation** heading, exactly two entries: `install-launcher` removed (the binary is the launcher), and open failures now exit 3 per the documented contract rather than 1.

Create `AGENTS.md` with the repo's golden rules: the engine boundary (D2, enforced by `internal/boundary_test.go`); never author or observe a secret value; TDD; `gofmt`/`vet`/`golangci-lint` clean; the compatibility contract lives in the spec and changing observable behavior requires updating it. Then `ln -s AGENTS.md CLAUDE.md`.

- [ ] **Step 5: Validate the pipeline without publishing**

```bash
cd $KDBX_REPO
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser check
goreleaser build --snapshot --clean --single-target
./dist/kdbx_*/kdbx --version
sh -n install.sh
```

Expected: `goreleaser check` reports no errors; the snapshot binary prints `kdbx <version>`; `sh -n` reports no syntax errors.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore(release): GoReleaser pipeline, curl installer, container image, docs"
```

---

## Task 24: Interop and parity harness

**Files:**
- Create: `interop/conftest.py`, `interop/test_roundtrip.py`, `interop/test_parity.py`, `interop/README.md`
- Modify: `.github/workflows/ci.yml` (add the `interop` job)

**Interfaces:**
- Consumes: the built `kdbx` binary (path from `$KDBX_BIN`, default `./kdbx`) and the Python implementation (path from `$KDBX_PY_REPO`, default `$EXTENSIONS_REPO`).
- Produces: proof that vaults are interchangeable and the CLI contract matches.

- [ ] **Step 1: Write the shared fixtures**

Create `interop/conftest.py`:

```python
"""Fixtures for the kdbx Go/Python interop suite.

Run with:
  uv run --with pytest --with pykeepass python -m pytest interop -v
"""

import json
import os
import pathlib
import shutil
import subprocess

import pytest

GO_BIN = os.environ.get("KDBX_BIN", str(pathlib.Path(__file__).resolve().parents[1] / "kdbx"))
PY_REPO = pathlib.Path(
    os.environ.get("KDBX_PY_REPO", "$EXTENSIONS_REPO")
)
PY_ENTRY = PY_REPO / "skills" / "kdbx" / "kdbx.py"

POINTER = {
    "project": "interop",
    "defaultEnv": "dev",
    "envs": {"dev": {}},
}


@pytest.fixture
def project(tmp_path, monkeypatch):
    """An isolated project dir with a pointer file and a private KEEPASSXC_DIR."""
    monkeypatch.setenv("KEEPASSXC_DIR", str(tmp_path / "kpxc"))
    monkeypatch.delenv("KDBX_ENV", raising=False)
    (tmp_path / ".keepassxc.json").write_text(json.dumps(POINTER, indent=2) + "\n")
    return tmp_path


def run_go(project, *args, stdin=None, check=True):
    """Invoke the Go binary inside project."""
    return subprocess.run(
        [GO_BIN, *args],
        cwd=project,
        input=stdin,
        capture_output=True,
        text=True,
        check=check,
    )


def run_py(project, *args, stdin=None, check=True):
    """Invoke the Python reference implementation inside project."""
    if not PY_ENTRY.exists():
        pytest.skip(f"Python reference implementation not found at {PY_ENTRY}")
    return subprocess.run(
        ["uv", "run", "--locked", str(PY_ENTRY), *args],
        cwd=project,
        input=stdin,
        capture_output=True,
        text=True,
        check=check,
    )


@pytest.fixture
def has_keepassxc_cli():
    return shutil.which("keepassxc-cli") is not None
```

- [ ] **Step 2: Write the round-trip tests**

Create `interop/test_roundtrip.py`:

```python
"""Artifact compatibility: a vault written by one implementation must be fully
usable by the other, and by KeePassXC itself."""

import subprocess

import pytest

from conftest import run_go, run_py


def _vault_paths(project):
    base = project / "kpxc" / "interop"
    return base / "dev.kdbx", base / "dev.keyx"


def test_go_created_vault_is_readable_by_python(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="sk-from-go\n")

    out = run_py(project, "get", "api/openai", "--reveal")
    assert out.stdout.strip() == "sk-from-go"


def test_python_created_vault_is_readable_by_go(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="sk-from-python\n")

    out = run_go(project, "get", "api/openai", "--reveal")
    assert out.stdout.strip() == "sk-from-python"


def test_protected_custom_property_survives_both_directions(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai:ORG_ID", stdin="org-from-python\n")

    assert run_go(project, "get", "api/openai:ORG_ID", "--reveal").stdout.strip() == "org-from-python"

    run_go(project, "set", "api/openai:PROJECT_ID", stdin="proj-from-go\n")
    assert run_py(project, "get", "api/openai:PROJECT_ID", "--reveal").stdout.strip() == "proj-from-go"


def test_recycle_bin_semantics_agree(project):
    run_go(project, "init")
    run_go(project, "set", "api/temp", stdin="value\n")
    run_go(project, "delete", "api/temp")

    # Neither implementation lists a soft-deleted entry.
    assert "api/temp" not in run_go(project, "list").stdout
    assert "api/temp" not in run_py(project, "list").stdout


def test_pykeepass_reads_a_go_written_vault_directly(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-direct\n")
    vault, keyfile = _vault_paths(project)

    script = (
        "import sys;from pykeepass import PyKeePass;"
        "kp=PyKeePass(sys.argv[1],keyfile=sys.argv[2]);"
        "e=kp.find_entries(title='openai',first=True);"
        "print(e.password)"
    )
    out = subprocess.run(
        ["uv", "run", "--with", "pykeepass", "python", "-c", script, str(vault), str(keyfile)],
        capture_output=True, text=True, check=True,
    )
    assert out.stdout.strip() == "sk-direct"


def test_keepassxc_cli_reads_a_go_written_vault(project, has_keepassxc_cli):
    if not has_keepassxc_cli:
        pytest.skip("keepassxc-cli not installed")
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-cli\n")
    vault, keyfile = _vault_paths(project)

    out = subprocess.run(
        ["keepassxc-cli", "ls", "--no-password", "-k", str(keyfile), str(vault), "-R"],
        capture_output=True, text=True, check=True,
    )
    assert "openai" in out.stdout


def test_rekey_by_go_keeps_the_vault_python_readable(project):
    run_py(project, "init")
    run_py(project, "set", "api/openai", stdin="sk-rotate\n")
    run_go(project, "rekey", "--yes")

    assert run_py(project, "get", "api/openai", "--reveal").stdout.strip() == "sk-rotate"
```

- [ ] **Step 3: Write the parity tests**

Create `interop/test_parity.py`:

```python
"""Behavioral parity: the same scenario must produce the same observable
contract (stdout shape, exit code) from both implementations.

Where the Python implementation deviates from the documented contract, the
*spec* is authoritative (spec C6) and the deviation is recorded here explicitly.
"""

import pytest

from conftest import run_go, run_py


def test_masked_get_prints_the_same_sentinel(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-value\n")
    assert run_go(project, "get", "api/openai").stdout.strip() == "(set, hidden)"
    assert run_py(project, "get", "api/openai").stdout.strip() == "(set, hidden)"


def test_list_output_is_identical(project):
    run_go(project, "init")
    for path in ("api/zeta", "api/alpha", "db/primary"):
        run_go(project, "set", path, stdin="v\n")
    assert run_go(project, "list").stdout == run_py(project, "list").stdout


def test_missing_entry_exits_2_in_both(project):
    run_go(project, "init")
    assert run_go(project, "get", "api/nope", check=False).returncode == 2
    assert run_py(project, "get", "api/nope", check=False).returncode == 2


def test_check_drift_exits_5_in_both(project):
    run_go(project, "init")
    run_go(project, "set", "api/gone", "--var", "GONE_KEY", stdin="v\n")
    run_go(project, "delete", "api/gone", "--purge", "--yes")

    go = run_go(project, "check", check=False)
    py = run_py(project, "check", check=False)
    assert go.returncode == 5 and py.returncode == 5
    assert "MISSING GONE_KEY -> api/gone" in go.stdout
    assert "MISSING GONE_KEY -> api/gone" in py.stdout


def test_export_renders_identically(project):
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin='weird "value" \\ here\n')
    assert run_go(project, "export").stdout == run_py(project, "export").stdout


def test_run_passes_the_child_exit_code_in_both(project):
    run_go(project, "init")
    for impl in (run_go, run_py):
        r = impl(project, "run", "--", "python", "-c", "raise SystemExit(7)", check=False)
        assert r.returncode == 7, f"{impl.__name__} returned {r.returncode}"


def test_pointer_rewrite_produces_the_same_file(project, tmp_path):
    """set --var must write byte-identical pointer files from both sides."""
    run_go(project, "init")
    run_go(project, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="v\n")
    go_pointer = (project / ".keepassxc.json").read_text()

    # Re-run the same scenario from scratch with the Python implementation.
    import json, shutil
    fresh = tmp_path / "fresh"
    fresh.mkdir()
    (fresh / ".keepassxc.json").write_text(
        json.dumps({"project": "interop", "defaultEnv": "dev", "envs": {"dev": {}}}, indent=2) + "\n"
    )
    shutil.rmtree(project / "kpxc", ignore_errors=True)
    run_py(fresh, "init")
    run_py(fresh, "set", "api/openai", "--var", "OPENAI_API_KEY", stdin="v\n")
    py_pointer = (fresh / ".keepassxc.json").read_text()

    assert go_pointer == py_pointer


def test_symlinked_pointer_path_resolves_the_same(project, tmp_path):
    """Known parity gap found during Task 3: Python's paths.expand_path ends in
    .resolve() (follows symlinks); Go's paths.Expand uses Abs+Clean (does not).
    Both must still reach the same vault when a pointer path traverses a symlink."""
    import json

    real = tmp_path / "real-vaults"
    real.mkdir()
    link = tmp_path / "linked-vaults"
    link.symlink_to(real, target_is_directory=True)

    (project / ".keepassxc.json").write_text(
        json.dumps(
            {
                "project": "interop",
                "defaultEnv": "dev",
                "envs": {
                    "dev": {
                        "vault": f"{link}/dev.kdbx",
                        "keyFile": f"{link}/dev.keyx",
                    }
                },
            },
            indent=2,
        )
        + "\n"
    )

    run_go(project, "init")
    run_go(project, "set", "api/openai", stdin="sk-symlink\n")
    # The Python implementation must reach the same vault through the symlink.
    assert run_py(project, "get", "api/openai", "--reveal").stdout.strip() == "sk-symlink"


@pytest.mark.xfail(
    reason="documented deviation (spec C6): Python surfaces some open failures as exit 1; "
           "Go implements the documented exit 3",
    strict=False,
)
def test_missing_keyfile_exit_code(project):
    run_go(project, "init")
    (project / "kpxc" / "interop" / "dev.keyx").unlink()
    assert run_go(project, "get", "api/x", check=False).returncode == 3
    assert run_py(project, "get", "api/x", check=False).returncode == 3
```

Create `interop/README.md` explaining what the suite proves, how to run it, the two env vars it honors, and that a failure here is a **release blocker** — artifact compatibility is the migration promise.

- [ ] **Step 4: Add the coverage-parity checklist**

Append to `interop/README.md` a table mapping every test file in `<extensions>/skills/kdbx/tests/` and `<extensions>/plugins/kdbx/tests/` to its Go or interop counterpart. Generate the left column with:

```bash
ls $EXTENSIONS_REPO/skills/kdbx/tests/ \
   $EXTENSIONS_REPO/plugins/kdbx/tests/
```

For each Python test file, name the Go test file or interop test that covers the same behavior. Any row without a counterpart must either gain one in this task or carry a one-line justification for why the behavior no longer exists (e.g. `test_launcher.py` → removed, spec D5).

- [ ] **Step 5: Run the suite**

```bash
cd $KDBX_REPO
go build -o kdbx .
uv run --with pytest --with pykeepass python -m pytest interop -v
```

Expected: all pass except the one `xfail`. **Any round-trip failure is a hard stop** — report it rather than adjusting the test.

- [ ] **Step 6: Add the CI job**

Append to `.github/workflows/ci.yml`:

```yaml
  interop:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: {go-version: "1.25.x"}
      - uses: astral-sh/setup-uv@v5
      - uses: actions/checkout@v4
        with:
          repository: yarrasys/extensions
          path: extensions
      - run: go build -o kdbx .
      - name: install keepassxc-cli
        run: sudo apt-get update && sudo apt-get install -y keepassxc
      - name: interop suite
        env:
          KDBX_BIN: ${{ github.workspace }}/kdbx
          KDBX_PY_REPO: ${{ github.workspace }}/extensions
        run: uv run --with pytest --with pykeepass python -m pytest interop -v
```

- [ ] **Step 7: Commit**

```bash
git add interop .github/workflows/ci.yml
git commit -m "test(interop): vault round-trip and CLI parity against the Python reference"
```

---

## Task 25: Extensions-repo integration (spec M7)

**Files (all in `$EXTENSIONS_REPO`, on a new branch):**
- Modify: `skills/kdbx/SKILL.md`, `skills/kdbx/AGENTS.md`, `skills/kdbx/CHANGELOG.md`, `plugins/kdbx/hooks/hooks.json`, `plugins/kdbx/mcp/.mcp.json`, `plugins/kdbx/README.md`, `plugins/kdbx/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `pytest.ini`, `README.md`, `llms.txt`
- Delete: `plugins/kdbx/hooks/guard.py`, `plugins/kdbx/mcp/server.py`, `plugins/kdbx/mcp/server.py.lock`

**Interfaces:**
- Consumes: a released `kdbx` binary on PATH.
- Produces: a skill and plugin that consume the binary instead of bundling a Python implementation.

- [ ] **Step 1: Branch**

```bash
cd $EXTENSIONS_REPO
git checkout main && git pull
git checkout -b feat/kdbx-binary-integration
```

- [ ] **Step 2: Rewrite the skill's invocation contract**

In `skills/kdbx/SKILL.md`, replace the `## Invocation` section with:

```markdown
## Invocation

kdbx is a standalone binary. Check it is present, and install it if not:

```
kdbx --version    # if this fails, install:
curl -LsSf https://raw.githubusercontent.com/yarrasys/kdbx/main/install.sh | sh
```

Then invoke operations directly from the project directory:

```
kdbx <op> [args]
```

Discovery is automatic: kdbx walks up from the cwd to a committed `.keepassxc.json`.
Active env = `--env` › `$KDBX_ENV` › the pointer's `defaultEnv`.

**Legacy (uv) fallback.** The original Python implementation still lives at
`skills/kdbx/kdbx.py` and is invoked as `uv run --locked <SKILL_DIR>/kdbx.py <op>`. It is
frozen as the reference implementation, receives bug fixes only, and will be archived at
kdbx v1.0. Prefer the binary.
```

In the same file: delete the `install-launcher` row from the operations table (spec D5), and add rows for `mcp` (run a read-only MCP server over stdio) and `guard` (evaluate a PreToolUse hook payload). Add `--json` to the operations preamble as a global flag for read ops.

- [ ] **Step 3: Point the plugin at the binary**

Replace `plugins/kdbx/hooks/hooks.json` with a config invoking the binary:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "kdbx guard --hook pretooluse"
          }
        ]
      }
    ]
  }
}
```

Replace `plugins/kdbx/mcp/.mcp.json`:

```json
{
  "mcpServers": {
    "kdbx": {
      "command": "kdbx",
      "args": ["mcp"]
    }
  }
}
```

Delete the superseded Python implementations:

```bash
git rm plugins/kdbx/hooks/guard.py plugins/kdbx/mcp/server.py plugins/kdbx/mcp/server.py.lock
```

- [ ] **Step 4: Update the plugin's tests**

The plugin's Python hook tests no longer have a subject. In `plugins/kdbx/tests/`, replace the `guard.py` unit tests with a single integration test that shells out to the binary, skipping when it is absent:

```python
"""The plugin's guard is now the kdbx binary. These tests assert the wiring,
not the decision logic — that lives in the Go suite (internal/guard)."""

import json
import shutil
import subprocess

import pytest

pytestmark = pytest.mark.skipif(
    shutil.which("kdbx") is None, reason="kdbx binary not installed"
)


def _decide(command):
    out = subprocess.run(
        ["kdbx", "guard", "--hook", "pretooluse"],
        input=json.dumps({"tool_input": {"command": command}}),
        capture_output=True,
        text=True,
    )
    assert out.returncode == 0, "the guard must always exit 0 (fail open)"
    return json.loads(out.stdout) if out.stdout.strip() else None


def test_agent_write_op_is_denied():
    decision = _decide("kdbx set api/openai")
    assert decision is not None
    assert decision["hookSpecificOutput"]["permissionDecision"] == "deny"


def test_agent_read_op_is_allowed():
    assert _decide("kdbx run -- npm test") is None


def test_non_kdbx_vault_read_is_denied():
    decision = _decide("cat ~/.config/keepassxc/demo/dev.kdbx")
    assert decision is not None
    assert "leak-guard" in decision["hookSpecificOutput"]["permissionDecisionReason"]
```

- [ ] **Step 5: Freeze the Python implementation**

Add to the top of `skills/kdbx/AGENTS.md`, immediately after the H1:

```markdown
> **Status: reference implementation (frozen).** The canonical kdbx is the Go binary at
> [github.com/yarrasys/kdbx](https://github.com/yarrasys/kdbx). This Python implementation
> receives **bug fixes only** and exists to (a) serve installs that predate the binary and
> (b) back the interop/parity harness that proves the binary is compatible. Do not add
> features here. It is archived at kdbx v1.0.
```

Add to `skills/kdbx/CHANGELOG.md` under `## [Unreleased]`:

```markdown
### Changed
- kdbx is now distributed as a standalone Go binary (github.com/yarrasys/kdbx). This
  Python implementation is frozen as the reference implementation (bug fixes only).
- The skill and plugin now consume the binary: the plugin's `PreToolUse` hook is
  `kdbx guard` and its MCP server is `kdbx mcp`.

### Removed
- `plugins/kdbx/hooks/guard.py` and `plugins/kdbx/mcp/server.py` — superseded by the
  binary's built-in `guard` and `mcp` subcommands.
```

- [ ] **Step 6: Update manifests and indexes**

Bump the version in **both** `plugins/kdbx/.claude-plugin/plugin.json` and the matching entry in `.claude-plugin/marketplace.json` (CI validates they agree). In the root `README.md` and `llms.txt`, update the kdbx row to lead with the binary install one-liner.

- [ ] **Step 7: Verify the repo is still green**

```bash
cd $EXTENSIONS_REPO
uvx ruff format --check .
uvx ruff check .
uv run --with pytest --with pykeepass --with python-dotenv --with filelock \
  --with platformdirs --with "mcp>=1.0,<2" python -m pytest
python -c "import json;json.load(open('.claude-plugin/marketplace.json'))"
python -c "import json;json.load(open('plugins/kdbx/.claude-plugin/plugin.json'))"
test -f plugins/kdbx/skills/kdbx/SKILL.md && echo "bundled-skill symlink resolves"
```

Expected: ruff clean, the suite green (the plugin's guard tests skip when the binary is absent), both manifests valid JSON, symlink resolving.

- [ ] **Step 8: Commit and open the PR**

```bash
git add -A
git commit -m "feat(kdbx): consume the standalone binary; freeze the Python reference implementation

The kdbx CLI now ships as a standalone Go binary (github.com/yarrasys/kdbx),
installable via curl/brew/go install. The skill documents the binary and the
plugin invokes 'kdbx guard' and 'kdbx mcp' instead of bundling Python
equivalents. The Python implementation stays as the frozen reference that backs
the interop/parity harness."
gh pr create --fill --base main
```

⚠️ `main` is protected with required CI checks and `enforce_admins`; the PR cannot merge until every check is green, including on Windows.

---

## Self-Review

**Spec coverage.** Every spec section maps to at least one task:

| Spec | Task(s) |
|------|---------|
| C1 pointer discovery/schema/rewrite | 3, 5, 6, 7 |
| C2 entry-path grammar | 6 |
| C3 vault format + crash-safe save | 10, 12 |
| C4 keyfile v2 | 9 |
| C5 the 12 operations | 7, 11, 13, 14, 17, 18, 19 |
| C6 exit codes | 4, plus assertions in every `.txtar` |
| C7 error scrubbing / `KDBX_DEBUG` | 4 |
| C8 permissions + secret hygiene | 8, 10, 13 (sync-root warning) |
| C9 locking + integrity | 12 |
| C10 roles | 20 (guard), 21 (no MCP write tools), 23 (README), 25 (skill docs) |
| N1 `--json` | 7, 11 |
| N2 `kdbx mcp` | 21 |
| N3 `kdbx guard` | 20 |
| N4 completions | 22 |
| D2 engine boundary | 10, enforced by the test in 22 |
| D5 drop `install-launcher` | 22 (asserted absent), 25 (removed from docs) |
| D7 freeze Python | 25 |
| D9 release engineering | 23 |
| M0 spike gate | 2 |
| M7 extensions integration | 25 |
| Testing strategy | unit tests per task; `testscript` in 7/11/13/14/17/18/19/22; interop in 24 |

**Known plan-level risks, stated rather than hidden:**
1. **Task 10's `isRootGroup` heuristic** matches the top group by the literal name `"Root"`. The task carries an explicit instruction to switch to "the single top-level group" if `docs/spike-notes.md` shows otherwise. This is the most likely source of a Task 10/12 rework.
2. **Task 21's MCP SDK signature** is version-sensitive. The task pins the *contract* (`Tools()` and its tests) and instructs the implementer to adapt `Serve` — deliberately, so an SDK bump cannot silently change what is exposed.
3. **Task 15's `godotenv` interpolation** may not match Python's `interpolate=False`. The task names the fallback (hand-rolled parser) rather than leaving it to judgment.
4. **`vault.Rekey` + `cmd/rekey.go` split responsibility** for installing the new keyfile. Task 19 Step 1 carries the verification instruction; a reviewer should confirm no path leaves the vault re-keyed with the old keyfile still live.

**Type consistency:** `Handle`, `EnvPaths`, `Context`, `ToolSpec`, and `ReadOpts` are each defined once and referenced with the same field names throughout. `vaultvars.Resolve` is the single var-resolution implementation, consumed identically by `run` (Task 17) and `export` (Task 18). `pointer.EntryOf` is defined in Task 6 and consumed in Tasks 6 and 14. The hidden `--yes` flag is introduced in Task 14 and reused with the same semantics in Task 19.

