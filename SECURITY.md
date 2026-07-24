# Security policy

## Reporting a vulnerability

Report privately through GitHub Security Advisories:
**<https://github.com/yarrasys/kdbx/security/advisories/new>**

Please do **not** open a public issue, discussion, or pull request for a security problem.
Public disclosure before a fix exists puts every user's vault at risk.

Include what you have: affected version (`kdbx --version`), platform, reproduction steps,
and impact. Redact real secrets, vault paths, and key files from anything you attach — a
description of the shape of the problem is enough.

Expect an acknowledgement within a few days. kdbx is maintained by one person, so please be
patient with timelines; you will be credited in the advisory unless you ask not to be.

## Threat model

kdbx defends a narrow, explicit boundary. Three things are worth stating plainly:

- **Possession of the key file is the security boundary.** There is no master password.
  Anything that can read `<env>.keyx` can open `<env>.kdbx` — no exceptions, and nothing in
  the binary changes that. The roles contract ("agents read, humans write") and `kdbx guard`
  are *ergonomics and blast-radius controls* for agent harnesses, not access control. Treat
  a key file the way you would treat an unencrypted private key.
- **Secrets are not zeroized in memory.** Go's garbage-collected strings mean a decrypted
  value can persist in process memory, a core dump, or swap after use. This is an accepted
  limitation, inherited from the Python implementation and shared with most tools in this
  class. It is in scope to *reduce* (fewer copies, shorter lifetimes); it is not in scope
  to *guarantee*.
- **A leaked key file plus vault means "rotate at the source", not "re-key".** `kdbx rekey`
  rotates the file that opens the vault; it does not change the credentials stored inside
  it. If both artifacts are exposed, assume every secret in that vault is compromised and
  rotate each one with its issuer (OpenAI, AWS, your database, …). Re-keying afterwards is
  hygiene, not remediation.

Out of scope: a local attacker who already has code execution as your user (they can read
the key file, so nothing is defensible); an attacker with physical access to an unlocked
machine; and vault-format weaknesses in KDBX4/Argon2 itself, which belong upstream with
KeePassXC.

## Verifying a release

Every release publishes a `SHA256SUMS` file plus a cosign **keyless** signature over it
(`SHA256SUMS.sig`) and the associated certificate (`SHA256SUMS.pem`). Verifying the checksum
file transitively verifies every archive listed in it.

```sh
VERSION=v0.1.0
BASE="https://github.com/yarrasys/kdbx/releases/download/$VERSION"
curl -fsSLO "$BASE/SHA256SUMS"
curl -fsSLO "$BASE/SHA256SUMS.sig"
curl -fsSLO "$BASE/SHA256SUMS.pem"

cosign verify-blob \
  --certificate SHA256SUMS.pem \
  --signature SHA256SUMS.sig \
  --certificate-identity-regexp 'https://github\.com/yarrasys/kdbx/\.github/workflows/release\.yml@refs/tags/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  SHA256SUMS
```

Then check the archive you downloaded against it:

```sh
sha256sum --ignore-missing -c SHA256SUMS    # shasum -a 256 --ignore-missing -c SHA256SUMS on macOS
```

`install.sh` performs the checksum half of this automatically and refuses to install on a
mismatch. It does **not** verify the cosign signature — if you need signature verification,
do it manually with the commands above, or install through Homebrew or `go install`.

Signatures are produced by GitHub Actions' OIDC identity, so the certificate identity above
is the proof that the artifact came from this repository's release workflow and not from
someone who obtained a signing key.
