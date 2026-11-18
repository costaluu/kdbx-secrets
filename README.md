# kdbx-secrets

A Go CLI that delivers secrets stored in a KeePass Database (`.kdbx`) to the tools that need them, without writing secrets to disk and without environment variables. The only thing you type is the KeePass password, at the moment of use. Each command opens the Database, answers, wipes what it can and exits (no daemon, no cache). The Database is only read, never written.

More details: [CONTEXT.md](CONTEXT.md), [GLOSSARY.md](GLOSSARY.md), [spec](docs/specs/kdbx-secrets.md) and [ADRs](docs/adr/).

## Installation

Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/costaluu/kdbx-secrets/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/costaluu/kdbx-secrets/main/install.ps1 | iex
```

The script detects the OS and architecture, downloads the binary from the latest release and the `SHA256SUMS`, checks the SHA-256 (aborts on mismatch) and, if `gh` is installed, verifies the attestation. It installs into `~/.local/bin` (Unix) or `%LOCALAPPDATA%\kdbx-secrets\bin` (Windows); `KDBX_SECRETS_BIN_DIR` changes the folder. It warns if the folder is not in `PATH`. It does not create any configuration. To update, run the script again.

Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64.

### Verifying the binary by hand

Download the binary and `SHA256SUMS` from the [releases page](https://github.com/costaluu/kdbx-secrets/releases/latest), then:

```sh
sha256sum --check --ignore-missing SHA256SUMS      # macOS: shasum -a 256 -c
gh attestation verify kdbx-secrets-linux-amd64 --repo costaluu/kdbx-secrets
```

On Windows: `(Get-FileHash kdbx-secrets-windows-amd64.exe).Hash` and compare with the `SHA256SUMS` line. The attestation proves the binary came out of this repository's release workflow; there is no signing key of its own.

## Configuring the first Database

The configuration folder is `~/.kdbx-secrets/` (or the absolute path in `KDBX_SECRETS_HOME`). On Unix, it, the `config` and the templates must be owned by your user and must not be writable by group or others (`chmod 700` on the folder, `600` on the files).

```sh
mkdir -m 700 ~/.kdbx-secrets
printf 'database = ~/databases/secrets.kdbx\n' > ~/.kdbx-secrets/config
chmod 600 ~/.kdbx-secrets/config
```

`~` is expanded and a relative path starts from the configuration folder. An unknown key is an error.

## Commands

A **Reference** is `Group/Entry#Field`, without the Root group name. Without `#`, it returns the Password. There is also `uuid:<32 hex>`, custom attributes (`AWS/paper-freeze#region`), `#TOTP` and `#attachment:<name>`. Full syntax in [ADR 0004](docs/adr/0004-reference-syntax.md).

- `kdbx-secrets get [--show] <reference>...`: one value per line, with a single prompt. Refuses to write to a terminal without `--show`.
- `kdbx-secrets render [--show] <name>`: runs the template `~/.kdbx-secrets/templates/<name>.tmpl` (`text/template`, with `get`, `json` and `base64`).
- `kdbx-secrets adapter create <name> [--from <file>] [--force] | delete <name> | list`: manages the templates (validates before writing; refuses to overwrite without `--force`).
- `kdbx-secrets encrypt (-r <public-key-file> | --to <reference>)...`: encrypts stdin to OpenPGP (armored) for the recipients, repeatable and combinable. `-r` does not ask for a password; `--to` reads the public key from the Database.
- `kdbx-secrets decrypt --key <reference> [--show]`: decrypts stdin with the private key held in the Database (the key's passphrase is the Password of the same Entry, if the key is encrypted). Refuses to write to a terminal without `--show` and only writes after integrity has been verified.
- `kdbx-secrets run --vault <mount>/<path>=<entry-reference> [--vault ...] -- <command> [args]`: runs the command with `VAULT_ADDR` and `VAULT_TOKEN` pointing at a local, read-only imitation of the Vault KV v2 API that serves only the mapped Entries. If the command dies from a signal, `run` exits with 128+signal.
- `kdbx-secrets version`.

Exit codes: 0 ok, 1 error, 2 incorrect usage (`run` passes through the child's code).

```sh
echo secret | kdbx-secrets encrypt --to GPG/personal#attachment:pub.asc > msg.asc
kdbx-secrets decrypt --key GPG/personal#attachment:priv.asc < msg.asc
kdbx-secrets run --vault secret/aws=AWS/paper-freeze -- vault kv get secret/aws
```

### Example: `paper-freeze`

The Entry `AWS/paper-freeze` stores the access key id in UserName and the secret in Password. Template `aws-paper-freeze`:

```
{"Version":1,"AccessKeyId":{{ json (get "AWS/paper-freeze#UserName") }},"SecretAccessKey":{{ json (get "AWS/paper-freeze") }}}
```

```sh
kdbx-secrets adapter create aws-paper-freeze --from aws-paper-freeze.tmpl
```

In `~/.aws/config`:

```ini
[profile paper-freeze]
credential_process = kdbx-secrets render aws-paper-freeze
```

And the `~/.zshrc` function loses the key exports and calls `aws --profile paper-freeze` instead. The prompt shows who asked for the password (`aws ← zsh`).

## Known hardening limits

The binary is short-lived, locks memory against swap and disables core dumps (Linux: `mlockall`, `PR_SET_DUMPABLE=0`, `RLIMIT_CORE=0`; Windows: `VirtualLock`; macOS: `mlock`, core dump disabled, `PT_DENY_ATTACH`). But:

- **Windows and macOS have no `mlockall`.** Only the binary's own buffers are locked; the decrypted Database may go to swap or the pagefile. macOS swap is encrypted by default. **On Windows the pagefile is in cleartext without BitLocker: enable BitLocker.**
- **Hibernation writes RAM to disk on every OS**, with or without `mlock`. Only an encrypted disk solves it.
- On Linux, if `RLIMIT_MEMLOCK` does not cover the KDF memory, the binary warns on stderr and goes on without locking (core dump stays disabled).
- Wiping memory is best effort: `gokeepasslib` stores fields as `string`, which cannot be wiped.
- Malware running as the same user is outside what a CLI can prevent ([ADR 0005](docs/adr/0005-secret-never-in-environment.md)).
- Typing the password on every use makes a fake prompt easier; the process chain shown helps to notice it, but does not prevent it.

## Releases

`git tag vX.Y.Z && git push --tags`. The workflow tests, builds, generates `SHA256SUMS` and attestations, publishes and runs a smoke test of the install scripts on the three OSes.
