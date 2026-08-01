# kdbx — per-project secrets in KeePassXC vaults, safe to hand an AI agent

[![ci](https://github.com/yarrasys/kdbx/actions/workflows/ci.yml/badge.svg)](https://github.com/yarrasys/kdbx/actions/workflows/ci.yml)
[![govulncheck](https://github.com/yarrasys/kdbx/actions/workflows/govulncheck.yml/badge.svg)](https://github.com/yarrasys/kdbx/actions/workflows/govulncheck.yml)
[![release](https://img.shields.io/github/v/release/yarrasys/kdbx?sort=semver)](https://github.com/yarrasys/kdbx/releases/latest)
[![openssf scorecard](https://api.scorecard.dev/projects/github.com/yarrasys/kdbx/badge)](https://scorecard.dev/viewer/?uri=github.com/yarrasys/kdbx)
[![go version](https://img.shields.io/github/go-mod/go-version/yarrasys/kdbx)](go.mod)
[![license](https://img.shields.io/github/license/yarrasys/kdbx)](LICENSE)

`kdbx` keeps a project's secrets in a **per-project, per-environment KeePassXC vault**
(KDBX4, unlocked by a key file only — no master password) and gets them into the tools that
need them **without kdbx ever printing them into a transcript, a log file, or your shell
history**.
Discovery is automatic: kdbx walks up from your current directory to a committed
`.keepassxc.json` pointer file, works out which environment is active, and takes it from
there. The headline command is `kdbx run -- <cmd>`, which resolves that environment's
variable mappings and injects them into a child process' environment. It replaces `.env`
files as the source of truth; the vault stays outside the repo, so there is nothing secret
to accidentally commit.

> Every install method below is live — the curl installer, Homebrew, `go install`, the
> `ghcr.io` container image, and the signed release archives (`SHA256SUMS` + cosign, with
> reproducible builds). The badge above tracks the current version.

## Install

```sh
# curl installer — downloads the release archive, verifies its SHA-256, installs to
# ~/.local/bin (override with KDBX_INSTALL_DIR; pin with KDBX_VERSION=v0.1.3)
curl -LsSf https://raw.githubusercontent.com/yarrasys/kdbx/main/install.sh | sh
```

```sh
brew install yarrasys/tap/kdbx                    # Homebrew
go install github.com/yarrasys/kdbx@latest        # from source, needs Go 1.25+
docker run --rm ghcr.io/yarrasys/kdbx:latest --version   # container (FROM scratch)
```

Windows: download the `_windows_` archive from the
[releases page](https://github.com/yarrasys/kdbx/releases) and put `kdbx.exe` on your PATH.

Building from source needs **Go 1.25 or newer** (the KDBX engine and `golang.org/x/term`
both declare a 1.25 floor). Release binaries are static and impose no toolchain
requirement on users.

Shell completions:

```sh
kdbx completion zsh  > "${fpath[1]}/_kdbx"
kdbx completion bash > /etc/bash_completion.d/kdbx
kdbx completion fish > ~/.config/fish/completions/kdbx.fish
kdbx completion powershell | Out-String | Invoke-Expression
```

## Quick start

**1. Commit a pointer file** at the repo root. It names the project and its environments;
it contains no secrets, so it is safe to check in.

```json
{
  "project": "demo",
  "defaultEnv": "dev",
  "envs": {
    "dev": {},
    "prod": {}
  }
}
```

An empty env object is fine — kdbx derives the artifact paths from the project and
environment name: `<keepassxc-dir>/demo/dev.kdbx` and `<keepassxc-dir>/demo/dev.keyx`,
where `<keepassxc-dir>` is `$KEEPASSXC_DIR` if set, else `%LOCALAPPDATA%\keepassxc` on
Windows, else `$XDG_CONFIG_HOME/keepassxc` or `~/.config/keepassxc`. Set `vault` and
`keyFile` explicitly if you want them somewhere else; both accept a `${KEEPASSXC_DIR}`
token and a leading `~`.

**2. Create the vault and its key file.**

```console
$ kdbx init
ACTIVE ENV: dev  vault=/home/you/.config/keepassxc/demo/dev.kdbx  (source: pointer)
created /home/you/.config/keepassxc/demo/dev.kdbx
KEYFILE: /home/you/.config/keepassxc/demo/dev.keyx — back this up; losing it makes the vault unrecoverable.
```

**3. Store a secret and map it to an environment variable.** The value never appears on
the command line — it arrives on stdin, from `--from-env`, or from an interactive prompt.

```console
$ kdbx set api/openai --var OPENAI_API_KEY < secret.txt
ACTIVE ENV: dev  vault=/home/you/.config/keepassxc/demo/dev.kdbx  (source: pointer)
modified tracked file .keepassxc.json — review and commit
```

`--var` records the mapping in the pointer file, preserving existing key order so the diff
stays reviewable:

```json
"dev": {
  "vars": {
    "OPENAI_API_KEY": "api/openai"
  }
}
```

**4. Run something with the secrets injected.**

```sh
kdbx run -- claude              # the author's actual daily use
kdbx run -- npm test
kdbx --env prod run -- ./deploy.sh
```

The child inherits your environment plus the mapped variables. Its exit code is passed
straight back out. Nothing is written to disk, and the value never appears in your shell
history.

Reading is deliberately boring:

```console
$ kdbx list
api/openai
$ kdbx get api/openai
(set, hidden)
$ kdbx check          # every mapping still resolves → exit 0, no output
$ kdbx envs
* dev
  prod
```

## Operations

Every operation accepts `--env NAME`. Read operations additionally accept `--json`.
Operations marked ✦ print the banner `ACTIVE ENV: <env>  vault=<path>  (source: <src>)` to
**stderr**; pure display operations do not.

| Op | Flags | Behavior (stdout / stderr / exit) |
|----|-------|------------------------------------|
| `init` ✦ | | create vault + key file; stderr `created <vault>` plus a KEYFILE backup warning; refuses to overwrite an existing vault or key file |
| `set PATH` ✦ | `--var NAME`, `--from-env VAR`, `--raw` | value from `--from-env`, else an interactive prompt with confirmation on a TTY, else stdin (empty → error; one trailing newline stripped unless `--raw`); an empty or whitespace-only value is refused; `--var` adds the mapping to the pointer file |
| `get PATH` | `--reveal` \| `--clip` | default: prints `(set, hidden)` — no length or prefix leak; `--reveal` prints the value with a stderr warning; `--clip` copies it and auto-clears after ~15 s; missing entry or field → exit 2 |
| `list [GROUP]` | | sorted `group/…/title` lines, filtered by the `GROUP` prefix, Recycle Bin excluded; never prints values |
| `delete PATH` ✦ | `--purge` | soft-deletes to the Recycle Bin by default; `--purge` prompts `y/N` (TTY only — a non-TTY refuses with exit 4) then removes permanently |
| `mv SRC DST` ✦ | | moves or retitles an entry, creating destination groups; re-points the active environment's var mappings that referenced `SRC`, keeping any `:field` suffix; stderr `re-pointed N var mapping(s) …` |
| `run` ✦ | `--allow-missing`, `--no-mask`, `-- CMD…` | resolves the active environment's `vars` map, injects it into the child's environment, resolves `argv[0]` through PATH (PATHEXT on Windows), forwards signals, and passes the child's exit code through; when a child stream is captured (not a TTY), injected values ≥ 8 bytes in it become `***` (`--no-mask` disables; the guard denies it for agents); no command → exit 2; an unresolved var → exit 5 unless `--allow-missing` |
| `export` ✦ | `--out FILE`, `--allow-missing` | renders the mappings as dotenv (always double-quoted; `\`, `"` and newlines escaped); `--out` writes atomically at 0600 with a gitignore reminder, otherwise stdout |
| `import FILE` ✦ | | parses a dotenv file (no `$VAR` interpolation), stores each `KEY` at `imported/KEY` and registers the mapping; stderr reminds you to delete or rotate the source file |
| `check` | | prints `MISSING VAR -> path` per broken mapping; exit 0 when clean, 5 on drift |
| `envs` | | one line per environment, the active one marked `* `; stderr `active: <env> (source: <src>)`; no pointer file → exit 2 |
| `rekey` ✦ | | prompts `y/N` (TTY only, else exit 4), mints a new key file, re-keys the vault, replaces the old key file atomically; stderr reminds you to redistribute it |

Integration surfaces:

| Command | Purpose |
|---------|---------|
| `kdbx mcp` | read-only MCP server over stdio (five tools: `kdbx_list`, `kdbx_envs`, `kdbx_check`, `kdbx_get`, `kdbx_run`) |
| `kdbx guard --hook pretooluse` | evaluates a `PreToolUse` hook payload on stdin and denies agent-issued human-only operations |
| `kdbx completion <shell>` | emits a completion script for bash, zsh, fish, or powershell |
| `kdbx --version` | prints `kdbx <version>` |

### `--json`

`--json` gives read operations a machine-readable stdout. Secret values are never included.

```console
$ kdbx --json list
{"entries":["api/openai"]}
$ kdbx --json envs
{"envs":[{"name":"dev","active":true}],"source":"pointer"}
$ kdbx --json check
{"missing":[],"ok":true}
$ kdbx --json get api/openai
{"path":"api/openai","set":true}
```

`--json` with `--reveal` is rejected (exit 7). On failure, stdout carries
`{"error":{"op":"check","exit":5,"kind":"Drift"}}` alongside the usual stderr line and exit
code.

### Entry paths

An entry path is `group/sub/Title[:field]`. The field defaults to `password`. The reserved
field names — `title`, `username`, `password`, `url`, `notes`, matched case-insensitively —
map to the KeePass entry's native attributes; anything else becomes a **protected custom
property**. More than one `:`, an empty field after `:`, or an empty `/` segment is
rejected. Variable names passed to `--var` must match `^[A-Z_][A-Z0-9_]*$`.

### Environment selection

`--env` beats `$KDBX_ENV`, which beats the pointer's `defaultEnv`, which defaults to `dev`.
`kdbx envs` and the banner report which of the three won. An environment that isn't in the
pointer file is an error (exit 2).

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | success |
| `1` | generic failure (scrubbed) |
| `2` | not found — no pointer file, unknown environment, missing entry or field, no command given to `run` |
| `3` | locked, key file missing, or credential failure |
| `4` | a destructive operation was not confirmed |
| `5` | drift — `check` found a broken mapping, or `run`/`export` could not resolve one |
| `6` | the vault changed underneath a write; re-run |
| `7` | preflight rejection (e.g. an invalid `--var` name, or `--json --reveal`) |

Failures print exactly one stderr line and never a stack trace or a secret:

```console
$ kdbx get api/nope
kdbx: get failed: NotFound
```

Set `KDBX_DEBUG=1` to additionally get the underlying error and stack on stderr.

Environment variables kdbx reads: `KDBX_ENV`, `KEEPASSXC_DIR`, `XDG_CONFIG_HOME`,
`LOCALAPPDATA`, `KDBX_DEBUG`.

## Roles — agents read, humans write

kdbx is designed to be safe to hand to a coding agent. The rule is:

- **Agent-safe:** `run`, `get` (masked), `list`, `check`, `envs`, `init`.
- **Human-only:** `set`, `delete`, `mv`, `import`, `rekey`, `export`, and
  `get --reveal` / `get --clip`.

An agent can *use* a credential: `kdbx run -- npm test` works fine, and kdbx itself never
prints the value.

**This is a guardrail, not a boundary, and it is worth being precise about why.**

The binary does not enforce the split. Possession of the key file is the real boundary:
anything that can read the key file can open the vault, whoever or whatever it is. The role
split is enforced in harnesses that support hooks, by `kdbx guard`, and it is advisory
everywhere else.

Nor does anything bind the *child* command. `run` injects into whatever argv it is handed.
When the output is captured (a pipe, an agent harness, a log), injected values in it are
replaced with `***` — so `kdbx run -- env` shows masks, not values, exactly where the bytes
were headed for a transcript. A terminal gets raw output, and `--no-mask` (denied to agents
by the guard) restores it for piped human use. Masking matches exact values only: an agent
that encodes a value on the way out defeats it, as does a test script edited to exfiltrate.
kdbx not printing a secret is a property of kdbx, not of the system it runs in.

What the split actually buys you is that an agent does not stumble into disclosure while
doing something else, and cannot author, rotate or export a credential. It does not contain
an agent that has decided to read one. If you need that, you need a boundary the agent
cannot cross as your user: a separate account, a container, or a broker holding the keys.
See [issue #11](https://github.com/yarrasys/kdbx/issues/11).

`kdbx guard` reads a `PreToolUse` payload on stdin and either prints a deny envelope or
nothing at all. It **always exits 0** — it fails open, so a guard problem can never wedge
your agent.

```console
$ echo '{"tool_input":{"command":"kdbx set api/openai"}}' | kdbx guard --hook pretooluse
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"kdbx role-guard: 'set' is a human-only operation …"}}

$ echo '{"tool_input":{"command":"kdbx run -- npm test"}}' | kdbx guard --hook pretooluse
$ echo $?
0
```

It blocks two things: agent-issued human-only operations, and non-kdbx programs reaching
for `*.kdbx` / `*.keyx` files or the KeePassXC config directory (so `cat ~/.config/keepassxc/…`
is denied too). It recognizes `kdbx`, `keepassxc-cli` and `keepassxc` invocations as
legitimate.

`kdbx mcp` applies the same contract to MCP clients: five tools, all read-only. There is no
write tool, deliberately.

## Agent and editor integration

MCP server — add to `.mcp.json` (Claude Code) or your client's equivalent:

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

`PreToolUse` hook — Claude Code `settings.json` or a plugin's `hooks.json`:

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

The [`kdbx` plugin](https://github.com/yarrasys/extensions) in `yarrasys/extensions` bundles
both, along with the skill documentation an agent loads to learn the contract. That repository
is **archived and read-only** as of 2026-07-30 — the plugin still works, but it receives no
updates, issues or PRs. The snippets above are the whole integration; you do not need the
plugin to wire `kdbx guard` or `kdbx mcp` into an agent.

## Security

- **The key file is the only secret.** There is no master password. Anyone who can read
  `<env>.keyx` can open `<env>.kdbx`. Keep them apart from your repo — the default location
  (`~/.config/keepassxc/<project>/`) is deliberately outside it.
- **Back the key file up, out of band.** Losing it makes the vault unrecoverable. There is
  no recovery path, by design.
- **Watch out for cloud sync.** A vault or key file under OneDrive, Dropbox, iCloud Drive
  or similar is replicated to a third party and to every device on the account. `kdbx init`
  warns when it detects this, but does not refuse.
- **Vault, key file and exported dotenv files are written 0600** (owner-only ACL on
  Windows), atomically, and vault saves are crash-safe (temp file → rename, with the
  previous vault kept as `.bak` until the rename lands).
- **Secret values never reach argv, logs, JSON output, or error text.** Intake is stdin,
  `--from-env`, or an interactive prompt only.
- **Accepted limitation:** secret strings are not zeroized in memory (same as the Python
  implementation). A core dump or a sufficiently privileged local process can recover them.

Vulnerabilities: see [SECURITY.md](SECURITY.md). Please do not open a public issue.

## Origin

kdbx began as a Python skill in
[`yarrasys/extensions`](https://github.com/yarrasys/extensions/tree/main/skills/kdbx), a
repository now archived. This binary is the canonical kdbx and the Python skill is retired; the
design spec that governs compatibility lives here, in
[`docs/kdbx-go-standalone-design.md`](docs/kdbx-go-standalone-design.md). Vaults are standard KDBX4
(Argon2, key-file-only), so any vault kdbx writes opens directly in `keepassxc-cli` and the
KeePassXC desktop app; `.keepassxc.json` pointers and KeePass XML v2 key files are the
ordinary formats those tools already use.

A few behaviors were chosen deliberately when porting from the original Python skill:

| Area | Behavior | Why |
|------|----------|-----|
| `install-launcher` | **removed** | the binary is its own launcher; there is nothing left to install |
| Failure to open a vault | **exit 3** | 3 is the documented contract for a locked/credential failure (the Python skill sometimes returned 1) |
| Malformed `.env` on `import` (unterminated quote) | **exit 7** | silently losing a credential during an import is worse than failing loudly |
| Child killed by a signal under `run` | `-1` → 255 | Go collapses every signal death to -1; this doesn't match the shell's `128+N`, so no portable caller depends on it. Normal exit codes pass through identically |

## Bugs & feedback

Found a bug or have a feature idea? Please [open an
issue](https://github.com/yarrasys/kdbx/issues/new/choose). Include `kdbx --version`, your
platform, and redacted reproduction steps — **never paste a real secret, vault path, or key
file**; a description of the shape of the problem is enough.

Security vulnerabilities are the one exception: report them privately through GitHub Security
Advisories (see [SECURITY.md](SECURITY.md)), **not** as a public issue.

## Contributing

Start with [CONTRIBUTING.md](CONTRIBUTING.md) for setup and the build/test loop.
`AGENTS.md` (symlinked as `CLAUDE.md`) carries the repository's rules — the engine boundary,
the secret-hygiene invariants, and the test discipline. Read it before opening a PR. CI runs
the suite on Linux, macOS and Windows plus `go vet`, `gofmt` and `golangci-lint`.

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

MIT — see [LICENSE](LICENSE). Dependency licenses are inventoried in [NOTICE](NOTICE); the
dependency tree is MIT / BSD / Apache-2.0 throughout, with no copyleft component.
