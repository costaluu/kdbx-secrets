# kdbx-secrets

kdbx-secrets is a CLI application that delivers secrets stored in a KeePass Database (`.kdbx`) to the applications that need them, without writing secrets to disk. The user deals with a single thing: the KeePass password, typed at the moment of use.

It grew out of the study `docs/STUDY-keepassxc-layer.md`, done in the git-sync project (2026-11-10 to 2026-11-12). git-sync syncs the `.kdbx` between machines; kdbx-secrets reads that `.kdbx`. The two are independent: git-sync keeps the SSH key embedded in its own binary and does not depend on kdbx-secrets.

# Initial Requirements

- Single Go binary for Linux, macOS and Windows.
- **One password per run.** There is no `unlock`, daemon, cache or TTL. Each command asks for the KeePass password, opens the `.kdbx` straight from disk, does what it needs, wipes memory and exits. The only thing that lives longer is `run`, which lasts as long as the child process (ADR 0001).
- **The binary never writes the `.kdbx`.** `adapter fill` has `keepassxc-cli` add missing groups, Entries and standard fields, never changing a value already set (ADR 0006). Every other command is read-only.
- Unlock **by password only**: no keyfile, no YubiKey, no password sealed in TPM/Keychain/DPAPI (left for later, if a 24/7 app appears that needs to survive a reboot).
- No SSH scope: KeePassXC already handles the SSH agent.
- **Memory hardening from day one**. Real protection comes from three things: a short-lived process, memory locked against swap and core dump disabled. On Linux: `mlockall` + `PR_SET_DUMPABLE=0` + `RLIMIT_CORE=0`; on Windows, `VirtualLock`; on macOS, `mlock` + core dump disabled + `ptrace(PT_DENY_ATTACH)` against a same-user debugger. **Beware of `mlockall(MCL_FUTURE)`:** the call usually succeeds, and the failure only shows up later, in an allocation that overflows `RLIMIT_MEMLOCK` (8 MiB on many distros versus ~64 MiB for Argon2). The Go runtime then aborts with a fatal error, without warning. So the binary reads the `.kdbx` header before the prompt, raises the soft limit up to the hard limit and only calls `mlockall(MCL_CURRENT|MCL_FUTURE)` if the limit covers the KDF memory plus a margin. If it does not, it warns on stderr and goes on without locking; core dump stays disabled. Wiping memory is best effort and only applies to the binary's own buffers (the password and the output): `gokeepasslib` stores fields as `string`, and a Go `string` cannot be wiped.
- **Known hardening limits.** Windows and macOS have no `mlockall`: only the binary's own buffers are locked, and the decrypted Database (the `gokeepasslib` `string`s) may go to swap or the pagefile. On macOS swap is encrypted by default; on Windows the pagefile is in cleartext without BitLocker, which is why the documentation recommends BitLocker. On every OS hibernation writes RAM to disk, with or without `mlock`; only an encrypted disk solves it. Malware running as the same user is outside what a CLI can prevent (see ADR 0005).

# Configuration

- Folder `~/.kdbx-secrets/`, which can be replaced entirely by the `KDBX_SECRETS_HOME` variable (tests, a second Database). `KDBX_SECRETS_HOME` must be an absolute path.
- On Unix, the folder, the `config` and the templates must belong to the user and must not be writable by group or others; otherwise it is an error (same rule as `ssh`). The reason: whoever writes there decides which secrets come out and which `.kdbx` the password goes to. On Windows, this ACL check is left for later.
- `~/.kdbx-secrets/config`, format `key = value`:
  - `database = /path/to/database.kdbx`
  - `~` is expanded; a relative path is resolved from the configuration folder.
  - Lines starting with `#` are comments. An unknown key is an error.
  - Without `config` or without `database`: an error stating exactly which line is missing.
- Templates in `~/.kdbx-secrets/templates/<name>.tmpl`. A file in the folder is a registered Adapter; there is no other registry.
- The folder can be synced between machines (for example, by git-sync); `config` points to the local `.kdbx`.

# Language

See `GLOSSARY.md`.
