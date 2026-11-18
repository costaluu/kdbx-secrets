---
status: draft
date: 2026-11-10
---

# Study: git-sync as a multi-OS layer over KeePassXC

> **Migrated from the git-sync repository on 2026-11-12.** The `file:line` references
> marked **[C]** point to git-sync code (commit `e014581`), not to this
> project. The final decisions are in `CONTEXT.md`, `docs/specs/kdbx-secrets.md` and
> `docs/adr/`; where they diverge from this study, they prevail.

A feasibility and architecture study. **No project code was changed.**
The experiments were done in throwaway directories outside the repository.

Original request: (1) work on any OS; (2) GPG encryption;
(3) commit signing; (4) the project stops being just a sync and becomes a
layer on top of the system's KeePassXC.

## Evidence legend

| Mark | Meaning |
|---|---|
| **[V]** | Verified in a primary source: documentation or source code of the owning project (links in Appendix B). |
| **[C]** | Verified by reading this repository's code (`file:line`). |
| **[T]** | Verified by a local experiment. Environment: Linux, git 2.43.0, OpenSSH 9.6p1, Go 1.26.1, GnuPG 2.5.20. |
| **[S]** | Secondary source (a search result, not the owner's documentation). Treat as a hint. |
| **[I]** | My inference. Not verified. |
| **[?]** | Unknown. Needs a test or further reading before deciding. |

---

## 1. Executive summary

1. **Commit signing is the cheapest part and already works with what exists.**
   An in-memory SSH agent, like the one in `internal/sshkey/agent.go`, signs commits
   with `gpg.format=ssh`, without the private key on disk, for ed25519 and RSA **[T]**.
   The diff is small: `-c` flags on `commit` and `-S` on `Trim`'s `commit-tree`.
2. **`Trim` multiplies the cost of signing.** With a full branch, every cycle
   rewrites the 20 commits via `commit-tree`, and `commit-tree` **does not sign
   without `-S`**, even with `commit.gpgsign=true` **[T]**. Each cycle requires 20 signatures.
   That rules out any key that asks for confirmation on every use.
3. **KeePassXC only documents agent integration for SSH; there is no documented
   equivalent for AWS or GPG** **[V]**. Its Secret Service
   only exposes unprotected attributes (no attachments and no protected fields)
   **[V]**, only exists on Linux **[V]** and fights with gnome-keyring **[V][T]**.
   It does not work as a multi-OS base.
4. **Most realistic multi-OS path:** a native Go `.kdbx` reader
   (`gokeepasslib`, actively maintained **[V]**), with an `unlock` model in the style of
   `ssh-add`. The daemon keeps the already-opened secrets in memory and receives the unlock
   over IPC (Unix socket or named pipe). `keepassxc-cli` stays as a
   secondary backend and for `merge`.
5. **GPG is not needed for signing** (SSH solves it). For **encryption**, the
   `.kdbx` is already encrypted. For ordinary files, the real options are `age`
   (preferable) or `ProtonMail/go-crypto` (if it must interoperate with OpenPGP
   keys). `golang.org/x/crypto/openpgp` is marked "should not be used" **[V]**.
6. **Windows is the biggest open risk.** I did not verify whether Windows' `ssh`/`ssh-keygen`
   accepts the AF_UNIX socket that the current agent creates **[?]**. It is the first
   item of Phase 0.
7. **Friction point with existing decisions:** the spec fixes "no `config`", "one
   process per repository" and "remote always wins". This evolution touches all three.
   Each change must become an ADR (section 6).

---

## 2. Current state (code summary)

- Single binary, `systemd --user` / LaunchAgent / Task Scheduler, one process
  per repository, no central registry (`docs/specs/git-sync.md`, "Out of Scope") **[C]**.
- The private SSH key is **written in cleartext inside the binary itself**
  (`sshkey.Patch`, a 4096-byte placeholder, `internal/sshkey/key.go:16-18`) **[C]**.
  `add-key` refuses keys with a passphrase (`internal/cli/addkey.go:22-25`) **[C]**.
- On startup, `Run` brings up an in-memory SSH agent and injects `SSH_AUTH_SOCK` only into the
  `git` subprocesses via `Repo.Env` (`internal/cli/run.go:45-59`,
  `internal/gitops/gitops.go:28-30,48-50`) **[C]**.
- `agentSocketPath`: Linux `/run/user/<uid>/gitsync-agent-<pid>.sock`; macOS and
  Windows use `os.TempDir()` (`internal/sshkey/agent.go:58-68`) **[C]**.
- `Commit` is `git commit -m` (`gitops.go:159-162`) **[C]**. There is no signing.
- `SyncCycle` does `Trim` whenever `count > maxCommits`, and then
  `push --force` (`internal/engine/engine.go:177-188`). `Trim` rebuilds each
  commit with `commit-tree <tree> -p <parent>` and the message on stdin
  (`gitops.go:279-324`) **[C]**.
- "Remote always wins" policy for pull conflicts and push rejection
  (ADR 0001 and 0002) **[C]**.
- Real use case observed: a backup repository with KeePass databases (header
  `03d9a29a67fb4bb5`, the KDBX file signature), **without a `.kdbx` extension** **[T]**.
  KeePassXC is **not installed** on this machine, and gnome-keyring owns
  `org.freedesktop.secrets` **[T]**.

---

## 3. Analysis by topic

### 3.1 Reading KeePassXC secrets from Go

| Backend | Linux | macOS | Windows | Headless | Attachments / protected fields | External dependency |
|---|---|---|---|---|---|---|
| **A. `keepassxc-cli`** (subprocess) | yes | yes | yes | only with a password coming from somewhere | yes | KeePassXC installed |
| **B. Browser protocol (native messaging)** | yes | yes | yes | no (needs the GUI open and unlocked) | not documented | KeePassXC GUI running |
| **C. Secret Service (D-Bus)** | yes | **no** [I] | **no** [I] | only with the GUI open | **no** | KeePassXC GUI + giving up gnome-keyring |
| **D. Native Go reader** (`gokeepasslib`) | yes | yes | yes | yes (with in-memory unlock) | yes | none |

**A. `keepassxc-cli`** **[V]**
- Opens with `-k/--key-file`, `--no-password` and `-y/--yubikey <slot[:serial]>`; `-q` silences the password prompt.
- `show -a <attr>` can repeat `-a`, with **one value per line, in the requested order**.
  Protected attributes come out in cleartext when requested with `-a`.
- `attachment-export --stdout` writes the attachment to stdout, without a temporary file.
  That suits an attachment holding an SSH key.
- `merge <db1> <db2>` **replaces db1 with the result** and recommends a backup first.
  It accepts `-s/--same-credentials`, `--dry-run`, `--key-file-from` and `--yubikey-from`.
- **[?]** The man page does not document reading the master password from stdin. Still to test:
  `printf %s "$password" | keepassxc-cli ...` (the binary is not installed here).
  Without that, a process without a TTY cannot open the database.
- Each call pays the KDF (Argon2) cost again **[I]**. It does not suit being called on
  every cycle; it suits a single unlock or rare operations (`merge`).

**B. Browser protocol** **[V]**
- Documented actions include `get-logins`, `get-totp`, `lock-database`,
  `database-locked`/`database-unlocked` and `associate`/`test-associate`
  (each client associates with the database using a key pair).
- The documented `get-logins` example returns only `login`, `name`, `password`
  and `expired`. Attachments and custom attributes **do not appear** in the documentation.
- The transport is via `keepassxc-proxy` (native messaging). The protocol document does not
  describe a socket/pipe **[?]**.
- Advantage: git-sync would not touch the master password. Disadvantage: it depends on a running GUI
  and on user approval at each association. It does not solve SSH (attachment).

**C. Secret Service** **[V]**
- "Only one secret service provider can be enabled at a time"; GNOME Keyring or KWallet
  must be disabled. **On this machine gnome-keyring owns the bus** **[T]**.
- It only exposes Title, Username, Password, URL, Notes, TOTP and **unprotected custom
  attributes**. Attachments and protected fields are left out.
- With confirmation on, `secret-tool` asks for approval **on every call** (new PID).
- The documentation talks about D-Bus. That macOS and Windows have no equivalent is an inference **[I]**.

**D. `gokeepasslib`** **[V]** (README, `header.go`, `credentials.go`, `binary.go`, `go.mod`)
- Release v3.7.1 on 2026-10-23; last commit the same day.
- KDBX 3.1, 4.0 and 4.1 (read without an option; version detected).
- KDF: AES-KDF and Argon2 (via the `tobischo/argon2` fork). ChaCha20 and Twofish ciphers present.
- Credentials: password, **keyfile** (`ParseKeyFile`) and the `Windows` field.
- Attachments: `Binaries` / `GetContentBytes()`.
- **YubiKey challenge-response: no occurrences in the code** **[I]**: not supported.
- **Argon2id: no occurrence of the term in the code** **[?]**. Test with a real `.kdbx`
  generated by KeePassXC with each KDF before depending on it.
- `go.mod` requires Go 1.26.0 and `x/crypto` v0.58.0. The project is already on Go 1.26.1 **[C]**, so it is compatible.
- Risk: git-sync would start to **interpret a cryptographic format inside its own
  process**. Mitigation: use the lib only for reading; leave writing and `merge` to
  KeePassXC **[I]**.

**Integration with KeePassXC's SSH agent** **[V]**: it "does not provide an agent; it works
as a client of any OpenSSH-compatible agent" and loads the keys into the **system
agent**. That **does not feed** git-sync's in-process agent. In a
`systemd --user` service, `SSH_AUTH_SOCK` may not even be in the environment **[I]**. So reading the
attachment straight from the `.kdbx` is more robust than depending on the system agent.

### 3.2 Headless daemon without a password prompt: the `unlock` model

The problem: the service starts at login and **has no TTY** to ask for the master password.

Proposed model, in the same spirit as `ssh-add` **[I]**:
1. `git-sync unlock` (interactive) asks for the master password, opens the `.kdbx` and hands the daemon
   **only the necessary secrets** (SSH key, credentials), never the master password.
2. The daemon keeps them in memory, with an optional TTL, and `git-sync lock` discards everything.
3. IPC transport: Unix socket on Linux/macOS; **named pipe** on Windows
   (`go-winio` handles named pipes as a `net` transport **[V]**).
4. Without an unlock, the daemon **does not sync** and logs the reason (today's behavior
   with the embedded key stops being "always on").

Alternatives and their costs **[I]**: a keyfile on disk next to the database (defeats the
purpose); YubiKey via `keepassxc-cli -y` (requires hardware and backend A);
KeePassXC's "Quick Unlock" exists only in the GUI.

**Tension with the current architecture:** the spec decided "one process per repository,
no central daemon" (Option B). With N repositories, `unlock` would have to reach N
processes or a single secret-store process would have to exist. This becomes an ADR.

**Experiment finding [T]:** a socket path with 123 characters fails with
`bind: invalid argument`. The `sun_path` limit on Linux is 108. On macOS the limit
is lower (104) and `agentSocketPath`'s `os.TempDir()` is long **[I]**. The current code
does not yet guard against this. On Linux, `/run/user/<uid>/` is short enough.

### 3.3 Commit signing

Formats git accepts: `gpg.format` = `openpgp` (default), `x509` or `ssh`;
default programs `gpg`, `gpgsm` and `ssh-keygen` **[V]**.

**SSH signing** **[V]**
- Introduced in git 2.34. **It does not work with OpenSSH 8.7's `ssh-keygen`** (broken);
  use >= 8.8 **[V]** (RelNotes 2.34.0).
- `user.signingkey` accepts a literal public key with the `key::` prefix, and the private one
  "needs to be available via ssh-agent" **[V]**.
- `ssh-keygen -Y sign -f` accepts "a private key **or a public one whose private half
  is in ssh-agent**" **[V]**.
- Local verification uses `gpg.ssh.allowedSignersFile`; SSH has no trust levels
  and revocation uses `gpg.ssh.revocationFile` **[V]**.
- GitHub: SSH signatures are accepted ("the simplest to generate"); the existing
  authentication key can be used as the signing key. A GPG key, on the other hand, can expire or be
  revoked **[V]**. Verification is persisted per commit **[V]**.

**Experiment [T]** (Appendix A): in-memory agent with `x/crypto/ssh/agent` (v0.58.0) +
`git -c gpg.format=ssh -c user.signingkey=key::<pub> -c commit.gpgsign=true`, **private
key deleted from disk**:

| Case | ed25519 | RSA-3072 |
|---|---|---|
| signed `git commit` via agent | OK | OK |
| `verify-commit` with `allowed_signers` | "Good git signature" | "Good git signature" |
| `commit-tree -S` (as in Trim) | OK and verified | OK and verified |
| `commit-tree` **without** `-S`, with `commit.gpgsign=true` | **no signature** | **no signature** |

**Impact on `Trim`** **[C][T]**
- After 20 commits, **every cycle** becomes a Trim: it rewrites 20 commits and does `push --force`.
  So that is **20 signatures per cycle**, and the old signatures are discarded
  (the objects are recreated).
- `Trim` today passes only the tree, the parent and the message **[C]**. The author and date of the rewritten commits
  come from the current configuration/time of the machine that did the trim **[I]**.
  Consequence: the signature attests to **"the last machine that synced"**, not the
  original author. With several machines, `allowed_signers` must list all the keys.
- A key that requires confirmation per use (an agent option) would generate 20 prompts per
  cycle. The signing key must be in an agent **without confirmation**.

**How to inject the config without touching the user's** **[T][I]**
- `-c` on the command line (used in the experiment) or `GIT_CONFIG_COUNT` /
  `GIT_CONFIG_KEY_n` / `GIT_CONFIG_VALUE_n` in the environment. I tested the variables
  locally with git 2.43 **[T]**; the minimum git version that supports them was not
  checked here **[?]**. `Repo.Env` already exists for this **[C]**.

**GPG (OpenPGP)**
- git calls `gpg --status-fd=2 -bsau <key>` and looks for `[GNUPG:] SIG_CREATED ` in the
  output **[V]** (`gpg-interface.c:1019-1035`). So git-sync itself could be
  a **`gpg.program` shim**, but it would need to generate a detached OpenPGP signature
  with `go-crypto`. It is far more work than SSH **[I]**.
- Using the real `gpg` in a service without a TTY requires `gpg-agent` and `pinentry` **[I]**.
- KeePassXC warns that GnuPG's SSH agent is **not compatible** with its
  integration (it does not remove keys) **[V]**. That only applies to GPG's SSH agent mode.
- **Recommendation:** SSH as the default. GPG only if there is a concrete requirement.

**Open points on signing**
- **[?]** Does GitHub mark "Verified" only if the committer's email is verified on the
  account that owns the key? I did not confirm this in the documentation. git-sync's `commit` uses the
  machine's `user.email`.
- **Fail-open or fail-closed?** If the agent/secret store is unavailable, does the cycle
  fail or commit without a signature? A product decision (ADR).
- **Verify on the way in:** before applying "remote wins", check the received commits'
  signatures against `allowed_signers`. It blocks pushes from whoever has no key,
  but **does not block replay** of an old legitimate commit **[I]**.

### 3.4 Encryption (GPG and alternatives)

First, what is **already** encrypted: the `.kdbx` uses the format's own cipher (AES,
ChaCha20 or Twofish, with Argon2/AES KDF) **[V]** (constants in `gokeepasslib`'s `header.go`). Encrypting
again on top adds little. What stays in cleartext on GitHub is
**the other files** in the repository (the spec speaks of "file folders").

| Option | What it is | Verified points | Caveats |
|---|---|---|---|
| **`age`** | Go format and lib, small keys, no options | recipients `age1…` **or SSH public keys**; password; YubiKey plugin; README mentions post-quantum support **[V]** | does not interoperate with GPG keys |
| **`ProtonMail/go-crypto`** | OpenPGP in Go, "backwards compatible" with `x/crypto/openpgp` | active repository (last push 2026-09-29) **[V]** | the OpenPGP surface is large |
| **`golang.org/x/crypto/openpgp`** | legacy OpenPGP | pkg.go.dev: *"unsafe by design… not maintained, and should not be used"*; recommends `go-crypto`. Version `v0.1.0-deprecated` **[V]** | **discard** |
| **`git-crypt`** | git filter, AES-256-CTR with synthetic IV | **does not encrypt file names, commit messages or metadata**; does not revoke access; README says it is not the best tool for encrypting "most or all files" **[V]** | deterministic; leaks file equality |
| **`git-remote-gcrypt`** | remote helper that encrypts the whole repository with GnuPG | README admits that "every git push effectively has a `--force`" (a long-standing bug), with a flag to require an explicit force **[V]** | hides the commits from GitHub: **loses GitHub's signature verification** **[I]** and changes ADR 0002's model |

Effect of Trim: encrypted blobs do not deltify well **[I]**; the 20-commit limit already
contains the growth.

**Recommendation [I]:** (a) do not encrypt `.kdbx`; (b) if other files must be encrypted,
`age` with the **recipient being the same SSH key** as the secret store; (c) use
`go-crypto` only if a preexisting OpenPGP key must be respected. Before deciding,
we need to know what "GPG encryption" means to the user (section 7).

### 3.5 SSH agent and sockets on Windows and macOS

- The OpenSSH for Windows agent listens on the **named pipe** `\\.\pipe\openssh-ssh-agent`;
  several clients use that name as the default **[S]**.
- There is a hint of different behaviors between clients regarding `SSH_AUTH_SOCK`: JGit's
  documentation says it **must not** be set with Win32-OpenSSH,
  while libssh2 reads it first **[S]**. **[?]** What `ssh.exe` and the `ssh-keygen`
  that git uses on Windows do with an AF_UNIX path in `SSH_AUTH_SOCK` is not
  verified.
- AF_UNIX exists on Windows since build 17063 (Dec/2017), for stream sockets **[V]**
  (Microsoft Command Line blog).
- KeePassXC on Windows supports **Pageant** (default) and **OpenSSH for Windows**
  **[V]**. That is, on Windows the "KeePassXC agent" is an external agent, of a different
  nature from git-sync's in-process one.
- Consequence: `agentSocketPath` on Windows (`os.TempDir()` + `.sock`) **[C]** is an
  untested bet. Options, in order of preference **[I]**: (1) test the current AF_UNIX
  on Git for Windows and native OpenSSH; (2) serve the agent on a named pipe with
  `go-winio` and point `SSH_AUTH_SOCK` at it, if the client accepts; (3) last
  resort, a temporary file with a restricted ACL, which contradicts the current design's "key never
  touches the disk" rule.
- CI only cross-compiles for Windows and macOS (`ci.yml`) **[C]**; the spec admits that the
  real execution of `systemctl`/`launchctl`/`schtasks` is not covered. **None of this was
  exercised on Windows or macOS.**

### 3.6 Conflict: ADR 0001 ("remote wins") vs `.kdbx`

For a password database, "remote wins" **silently discards** a new entry
created on another machine within the sync window (ADR 0001, "no trace") **[C]**.
KeePassXC itself has a merge: `keepassxc-cli merge <db1> <db2>` **[V]**
(replaces db1; use `--dry-run` and a backup; `-s` when the credentials are the same).

A possible design **[I]**:
1. Detect a KDBX file by the **header signature**, not by the extension (the observed databases
   have no `.kdbx`) **[T]**.
2. On conflict, extract the remote version with `git show <remote>/<branch>:<file>` into
   a temporary directory with `0700` permission, run the merge with the credentials of the
   opened secret store and only then commit.
3. If the secret store is locked: apply ADR 0001 (remote wins) **or** preserve the local
   copy in a `*.conflict-<ts>` copy. ADR 0001 rejected "preserving a trace"
   for binary files in general; for KDBX the cost of losing a password is much higher.
   This calls for an ADR that **partially supersedes** 0001.

### 3.7 Threat model

**Assets:** the `.kdbx` content; the push/signing SSH key; stored credentials
(for example, AWS); open secrets in the daemon's memory.

**Adversaries:** whoever reads the remote repository; other collaborators on the remote (the observed
history has **commits from more than one author** **[T]**: confirm who has
access); malware running as the same user; a lost disk/laptop.

**Pre-existing weaknesses [C]:**
- The private key sits **in cleartext inside `~/.local/bin/git-sync`** (or wherever the binary
  is). Any process of the user reads it, and any backup or copy of the binary
  leaks it. It is the weakest point of the current design and a good reason for the
  KeePassXC layer: the key comes to exist only inside the `.kdbx` (encrypted) and in the
  daemon's memory.
- Keys with a passphrase are not supported (`addkey.go:22-25`).
- Any process of the same user can use the agent while the daemon runs (same as
  `ssh-agent`). On Linux, `/run/user/<uid>` is `0700` **[I]**.

**New surfaces with the secret-store layer [I]:**
- `unlock` IPC socket/pipe: restrict to the user; check the peer's credentials.
- Secrets in memory: Go's GC does not guarantee wiping; swap and core dumps can leak.
  Disable the service's core dump; short TTL for the unlock.
- Logs: the local log sits at the repository root (ignored by git) **[C]**; ensure
  it never receives a secret, not even in `keepassxc-cli` error messages.
- Integrity: whoever writes to the remote can **roll back** to an old version of the
  database; "remote wins" would accept it. Signing plus `allowed_signers` blocks unknown
  authors, but not replay of an old legitimate commit **[I]**.
- Trust in the KDBX parsing lib: a new dependency in sensitive code (3.1.D).

---

## 4. Proposed architecture (sketch)

Principle: **the minimum for each phase to deliver value on its own**; no abstraction before
a real second use exists.

- `internal/sshkey`: keeps the in-process agent; the **key's origin** stops being the
  patched binary and becomes the secret store. `Patch`/`Clear` stay as legacy until the end
  of the migration.
- `internal/signing` (new, small): produces git's flags/env for SSH signing and
  derives the public key from the PEM. Used by `gitops.Commit` and `gitops.Trim`.
- `internal/vault` (new): reads the `.kdbx` (D) and, optionally, calls `keepassxc-cli` (A)
  for `merge`. An interface only if there are two real backends.
- `unlock` IPC: Unix socket / named pipe, behind a per-OS interface, along the same
  lines as the `autostart.Installer` seam **[C]**.
- New commands (proposed): `unlock`, `lock`, `status`; `add-key` and `remove-key`
  become legacy.

---

## 5. Phased plan (lazy-first)

Each phase is worth it on its own and can be the last.

**Phase 0, spikes of 1 to 2 days, no merge:**
1. Windows: do `ssh-keygen -Y sign` and the `git`/`ssh` transport accept the current AF_UNIX agent? Test Git for Windows and native OpenSSH.
2. `gokeepasslib` reading real KeePassXC `.kdbx` files with **Argon2d, Argon2id and AES-KDF**, with a keyfile, and extracting an SSH key **attachment**.
3. macOS: length of `agentSocketPath` with a real `$TMPDIR`.
4. `keepassxc-cli` with the password via stdin and without a TTY.
5. GitHub: conditions for "Verified" with an SSH signature and `user.email`.

**Phase 1, SSH signing with the already-embedded key, without KeePassXC:**
`-c gpg.format=ssh -c user.signingkey=key::<pub> -c commit.gpgsign=true` in `Commit`;
`-S` on each `commit-tree` of `Trim`; derive the public key from the PEM. Test: create a temporary
repo, a sync cycle, `verify-commit`. The user registers the public key as a
*signing key* on GitHub.

**Phase 2, read-only secret store:** `unlock` with the Go reader; the SSH key comes from the `.kdbx`
attachment; `sshkey.Patch` becomes obsolete. Depends on the ADR on single process vs one per
repository.

**Phase 3, `.kdbx` merge:** partially supersedes ADR 0001 for KDBX files.

**Phase 4, credentials provider:** a command that delivers secret-store credentials to
other tools (for example, AWS's `credential_process`). **[?]** The format of
`credential_process` was not verified in this research.

**Phase 5, optional:** GPG (shim or external `gpg`) and/or `age` for ordinary files.

---

## 6. Decisions that become ADRs

| # | Decision | Why |
|---|---|---|
| A | **Project scope and name**: stops being just a sync | contradicts "no `config`" and "binary without configuration" (`CONTEXT.md`) |
| B | **Secrets backend**: native Go reader vs `keepassxc-cli` | external dependency vs parsing risk |
| C | **Unlock and IPC model**; relation to "one process per repository" | the spec forbids a central daemon |
| D | **SSH signing as the default**; fail-open vs fail-closed | cost and semantics of Trim |
| E | **KDBX conflict** (partially supersedes ADR 0001) | silent loss of passwords |
| F | **Encryption of ordinary files**: `age` vs `go-crypto` vs nothing | depends on the real requirement |
| G | **End of the key embedded in the binary** (`sshkey.Patch`) | biggest current weakness |

---

## 7. Open questions

1. **"GPG encryption"**: is it encrypting the repository's files before the push,
   protecting the key at rest, or only signing? It changes the answer in section 3.4.
2. **Who has access to the remote repository?** The history has commits from more than one
   author. If it is shared, collaborator threats come in.
3. **Are Windows and macOS real targets now?** Today there is only cross-compilation; nobody
   exercised autostart or the agent on those OSes.
4. **Is `git-sync unlock` at every login acceptable?** Or is a keyfile on disk / YubiKey preferred?
5. **Is an installed KeePassXC mandatory?** Or is the `.kdbx` format (native reader) enough?
6. **Signing key per machine or shared?** It affects `allowed_signers`.
7. **If the secret store is locked, does the sync stop (fail-closed) or continue without signing?**
8. **Does the project keep being called `git-sync`?**

---

## 8. Discussion of 2026-11-11: desired features and existing tools

### 8.1 Features the user wants

1. **Encrypt and decrypt with GPG keys.**
2. **Signed commits.**
3. **Credentials and tokens (AWS and any other) stored in the `.kdbx` and used
   "natively" by applications**: the user types the KeePass password and everything
   works, with no secret written to a file. This is the feature of most interest.

Features 1 and 2 follow the design in sections 3.3 and 3.4. Feature 3 was the one
discussed most.

### 8.2 General design: agent + adapters **[I]**

An agent keeps the `.kdbx` open in memory (TTL, lock on screen lock or
suspend, manual `lock`). Each application receives the secret through a thin adapter
that speaks the protocol it already understands. If the agent is locked, the
adapter opens a password prompt (graphical or in the terminal). Per-process access control
uses the peer's credentials on the socket (`SO_PEERCRED`) and is only partial
against malware running as the same user. Each entry declares its use through a
tag or attribute (for example `aws_access_key_id`, `aws_mfa_serial`).

Mechanisms considered for feature 3, from most to least robust:

| # | Mechanism | Note |
|---|---|---|
| 1 | Tools' credential helpers (AWS `credential_process`, git `credential.helper`, Docker `credHelpers`, kubectl exec plugin, Terraform, Cargo) | per-tool specific; formats not yet checked **[?]** |
| 2 | Local HTTP endpoint imitating AWS (`AWS_CONTAINER_CREDENTIALS_FULL_URI`) | covers any AWS SDK; `aws-vault --server` does this **[S]** |
| 3 | `exec` with environment variable injection | universal |
| 4 | **`.env` with references to secret-store entries**, resolved at `run` | **chosen by the user as the most interesting**: does not tie the binary to each tool's protocols |
| 5 | Virtual file (FIFO/FUSE) | last resort; does not work on Windows |
| 6 | Being the OS keyring (Secret Service) | Linux only; conflicts with KWallet/gnome-keyring; discarded as a base |

AWS-specific gains **[I]**: store only the long-term key and deliver temporary
credentials via STS; generate the MFA code from the entry's TOTP
(convenient, but it puts password and second factor in the same secret store).

For GPG (feature 1), the most pragmatic option raised: an ephemeral `GNUPGHOME` on
tmpfs (`/run/user/<uid>/…`), created at unlock with the key coming from an attachment of the
`.kdbx` and destroyed at lock, so `gpg`, `pass`, `sops` and git work without
change **[I]**. On Windows and macOS there is no guaranteed tmpfs **[?]**.

Limits of the `.env` model **[I]**: an environment variable is readable by the same user
(`/proc/<pid>/environ`) and inherited by children; apps that only read files
(`kubeconfig`, AWS SSO, `.npmrc`) need `run` to generate temporary
files on tmpfs and point the variable at them.

### 8.3 What already exists (research of 2026-11-11)

The "`.env` with references + `run`" model already exists for KeePass. Sources read
in the projects' READMEs and documentation **[V]**, unless noted:

| Project | Language | Reference | Unlock | Notes |
|---|---|---|---|---|
| [desktop-secrets](https://github.com/it-atelier-gn/desktop-secrets) | Go | `.env.tpl` with `keepass(...)`; `tplenv run` | prompt + **daemon with TTL**; optional per-use approval | the closest to the idea; multi-OS; MIT; 1 star |
| [fnox](https://fnox.jdx.dev/providers/keepass.html) | Rust | `fnox.toml`; `fnox exec` | env, config field or terminal prompt; shell hook does not ask for the password | the most mature; reads `.kdbx` directly; mentions `daemon_cache` **[?]** |
| [Varlock](https://varlock.dev/plugins/keepass/) | JS/TS | `.env.schema` with `kp("Group/Entry#Field")` | password coming from env | richer syntax; file (WASM) or `keepassxc-cli` |
| [yarrasys/kdbx](https://github.com/yarrasys/kdbx) | Go | `.keepassxc.json`; `kdbx run` | keyfile only, no daemon | a secret store per project and environment; MIT |
| [keeenv](https://github.com/scross01/keeenv) | Python | `.keeenv` with `${"Entry".Password}` | prompt on every run | simple |
| [keepassxc-run](https://pypi.org/project/keepassxc-run/) | Python | `keepassxc://…` | the KeePassXC GUI does the unlock (via `git-credential-keepassxc`) **[S]** | `op run` clone |

The UX reference for all of them is 1Password's [`op run`](https://developer.1password.com/docs/cli/secrets-environment-variables):
`.env` with `op://vault/item/field`, `op run --env-file`, stdout masking
and variables inside the reference to switch environments.

**Gap:** none combines a single agent-style unlock, a secret store synced between
machines (git-sync's role) and SSH/signing/GPG coming out of the same `.kdbx`.

### 8.4 Suggested next steps

1. Spike on `agent-dev`: a test `.kdbx` with a fake AWS entry; test
   desktop-secrets and fnox with the real flow. Decide between **adopting** one of them for
   feature 3 (and focusing on sync + SSH/signing + GPG) or **reimplementing**
   (desktop-secrets is Go and MIT, good for studying the daemon and the resolver).
2. Decide whether the layer is a separate project, with git-sync as a client, or whether it
   stays inside git-sync (ADR A in section 6).

### 8.5 Done in the same session: self-update

Implemented `git-sync update check` / `update apply`, modeled on `taskthing`,
with two differences: it preserves the embedded SSH key (`sshkey.Transplant`) and requires
the sha256 published by GitHub. See `docs/adr/0004-self-update-preserves-key.md`.

---

## Appendix A: signing experiment (reproducible)

Outside the repository (throwaway directory). Summary of the procedure:

1. A minimal Go program, equivalent to `sshkey.StartAgent`: `agent.NewKeyring()` +
   `ring.Add(...)` + `net.Listen("unix", ...)` + `agent.ServeAgent`. `golang.org/x/crypto` v0.58.0.
2. `ssh-keygen -t ed25519` (and `-t rsa -b 3072`) without a passphrase; start the agent with the
   key; **`shred -u` on the private key**.
3. Temporary repository; `SSH_AUTH_SOCK` pointing at the agent; flags
   `-c gpg.format=ssh -c user.signingkey=key::<pub> -c commit.gpgsign=true`.
4. `git commit`, `git commit-tree <tree> -S`, `git commit-tree <tree>` (without `-S`),
   and `git -c gpg.format=ssh -c gpg.ssh.allowedSignersFile=… verify-commit`.

Results: see the table in section 3.3.

**Pitfall found:** the first attempt failed because the socket path
(123 characters) exceeded `sun_path`'s limit of 108. The error showed up as
`bind: invalid argument` in the agent and as `Couldn't get agent socket?` in git. It
**was not** a signing problem.

---

## Appendix B: sources

**Primary [V]**
- KeePassXC, CLI man page: `docs/man/keepassxc-cli.1.adoc` (github.com/keepassxreboot/keepassxc)
- KeePassXC, `docs/topics/SecretService.adoc`, `SSHAgent.adoc`, `BrowserIntegration.adoc`
- Browser protocol: `keepassxc-protocol.md` (github.com/keepassxreboot/keepassxc-browser)
- `gokeepasslib` (github.com/tobischo/gokeepasslib): `README.md`, `header.go`, `credentials.go`, `binary.go`, `go.mod`, releases and commits
- git: `Documentation/config/gpg.adoc`, `Documentation/config/user.adoc`, `Documentation/git-commit-tree.adoc`, `Documentation/RelNotes/2.34.0.adoc`, `gpg-interface.c` (github.com/git/git)
- OpenSSH: `ssh-keygen.1` (github.com/openssh/openssh-portable)
- GitHub Docs: *About commit signature verification*; *Telling Git about your signing key* (github.com/github/docs)
- pkg.go.dev: `golang.org/x/crypto/openpgp`
- `ProtonMail/go-crypto`, `FiloSottile/age`, `AGWA/git-crypt`, `spwhitton/git-remote-gcrypt`, `microsoft/go-winio`
- Microsoft Command Line blog: *AF_UNIX comes to Windows* (devblogs.microsoft.com/commandline/af_unix-comes-to-windows/)

**Secondary [S]**: search results about the named pipe `\\.\pipe\openssh-ssh-agent`
(JGit `org.eclipse.jgit.ssh.apache.agent`, libssh2 `agent.c`, `asyncssh`). They are not the
owner's documentation; confirm in Phase 0.

**Local [C]/[T]**: this repository (`internal/sshkey`, `internal/gitops`,
`internal/engine`, `internal/cli`, `docs/`, `CONTEXT.md`) and the experiments in section 3.3.

## Not verified in this research

Windows (any item); macOS (any item); `keepassxc-cli` running (it is not
installed); password via stdin; Argon2id in `gokeepasslib`; minimum git version for
`GIT_CONFIG_COUNT`; the conditions for "Verified" on GitHub; the format of AWS's
`credential_process`; the `keepassxc-proxy` transport.
