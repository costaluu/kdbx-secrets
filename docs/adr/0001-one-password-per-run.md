---
status: accepted
---

# One password per run, no daemon

Each command asks for the KeePass password, opens the `.kdbx` from disk, answers and exits. There is no `unlock`, daemon, cache or TTL. The only thing that lives longer is `run`, which lasts as long as the child process.

kdbx-secrets is used in specific scenarios; KeePassXC remains the day-to-day tool. Typing the password on every use is an accepted cost in exchange for no secret being alive outside the run.

Consequences that simplify the project:
- No IPC: no Unix socket, named pipe, peer check, or socket path length limit on macOS.
- No reload: each run reads the current `.kdbx` from disk. Changes brought by git-sync (even several in a row) are seen on the next run without special handling.
- No token on disk: the `run --vault` token exists only in memory and dies with `run`.

Cost: each run pays the Database's KDF (Argon2 or AES-KDF). An app started under `run` keeps running indefinitely, but does not come back up by itself after a reboot.

The security gain is modest: against same-user malware, this decision protects no more than a daemon (see ADR 0005). What it gains is that someone at the keyboard of an open session cannot pull secrets without the password, and the Database stays decrypted in memory for less time (swap, hibernation). In exchange, the user gets used to typing the master password whenever anyone asks, which makes a fake prompt easier, and tools that call the adapter on every invocation (the AWS CLI with `credential_process`) ask for the password on every command.

**Trigger to revisit:** if a real workflow starts asking for the password several times in a row (for example, the AWS CLI or Terraform in daily use), or if the KDF measured in pending item 4 makes each call slow. In that case, the path is the daemon with an on-demand prompt and a short TTL, described below. The alternatives through a KeePassXC backend (browser protocol) and through the kernel keyring (`keyctl`) were already discarded: the first breaks ADR 0004 and does not read attachments; the second only works on Linux.

Alternatives considered:
- **Daemon with explicit unlock and TTL** (`ssh-agent` model): one password per session, but requires `unlock` before use, per-OS IPC and a reload point when the `.kdbx` changes.
- **Daemon with on-demand prompt**: the first use asks for the password and the entries stay in memory until the TTL. More convenient, but keeps secrets alive unnecessarily for the expected usage pattern.
- **Token in `~/.vault-token`**: compatible with every Vault client, but it is a secret on disk.
- **Sealed password (TPM, Keychain, DPAPI)**: Vault's auto-unseal model. Left for later, optional, if a 24/7 app appears that needs to start without a human.
