---
status: ready-for-agent
---

# kdbx-secrets

## Problem Statement

Tool credentials (AWS keys, API tokens, GPG keys) sit in cleartext in files such as `~/.zshrc`, `~/.aws/credentials` or `.env`. Any process of the user can read them, every child process inherits them through the environment (`/proc/<pid>/environ`) and every backup carries a copy. The user already keeps the secrets in a `.kdbx` Database, synced between machines by git-sync, but the tools cannot read from there. So the user ends up copying the secret into a cleartext file.

The real case that motivated the project: the `paper-freeze` function in `~/.zshrc` exports `PAPER_FREEZE_AWS_KEY` and `PAPER_FREEZE_AWS_SECRET` in cleartext and passes them to the AWS CLI through environment variables.

The user also wants to encrypt and decrypt files with the GPG key stored in the Database, without it sitting in a `~/.gnupg` on disk.

## Solution

A single Go binary (`kdbx-secrets`) for Linux, macOS and Windows. It reads the Database straight from disk on every run, asking for the KeePass password on the terminal, and delivers the values through Adapters that speak each tool's format:

- `get` writes raw values to stdout;
- `render` executes a template, for example an AWS `credential_process`;
- `run --vault` exposes a subset of the Database to a child process through a local imitation of the Vault KV API;
- `encrypt` and `decrypt` do OpenPGP compatible with `gpg`, using the key stored in an Attachment.

No secret is written to disk or put in an environment variable. The password and the values exist only in the process's memory, during the run, with hardening against swap and core dump. There is no daemon, cache or `unlock`: each command asks for the password, does what it needs and exits (ADR 0001).

The binary is distributed through GitHub Releases, with checksums and a provenance attestation. A bootstrap script per OS downloads the right binary, verifies its integrity and installs it in the user's `PATH`.

## User Stories

Actors: **user** (whoever uses the binary), **maintainer** (whoever publishes versions) and **CI** (the GitHub Actions workflows).

### Configuration

1. As a user, I want to point kdbx-secrets at my Database with a `database = <path>` line in `~/.kdbx-secrets/config`, so I do not have to repeat the path in every command.
2. As a user, I want `~` to be expanded and a relative path to be resolved from the configuration folder, so the same configuration works synced between machines.
3. As a user, I want a missing `config`, or a missing `database` line, to produce an error saying exactly what is missing, so I can fix it without guessing.
4. As a user, I want an unknown key in the `config` to be an error, so a typo is not silently ignored.
5. As a user, I want to replace the whole configuration folder with the `KDBX_SECRETS_HOME` variable (with an absolute path), to use a second Database or isolate tests.
6. As a Linux or macOS user, I want the binary to refuse a configuration folder, a `config` or a template that is not mine, or that others can write to, so nobody chooses which secrets come out or which Database my password goes to.
7. As a user, I want to sync the configuration folder between machines (for example, with git-sync), so I have the same Adapters on all of them.

### Prompt and unlocking

8. As a user, I want to type the KeePass password once per command, even when the command reads several References, so the cost is one prompt per use.
9. As a user, I want the prompt to use the controlling terminal and not stdin, so it works when the tool captures the Adapter's stdin and stdout (as the AWS CLI does).
10. As a user, I want the prompt to show the process chain that asked for the password (for example, `aws ← zsh`) and the requested References, so I notice an unexpected request.
11. As a user, I want up to 3 password attempts before the command fails, so a typo does not force me to start over.
12. As a user, I want different messages for "incorrect password" and for "incomplete or corrupted file, try again", so I know whether I got the password wrong or git-sync was writing the Database at that moment.
13. As a user, I want Ctrl-C at the prompt to restore the terminal's echo and exit, so my terminal is not left broken.
14. As a user, I want Ctrl-D at the prompt to cancel without counting as an attempt, so I can give up without spending an attempt.
15. As a user, I want a clear error when there is no terminal (app opened from the menu, cron), so I understand why I was not asked.
16. As a user, I want usage, syntax, configuration and file errors to show up before the prompt, so I do not pay the KDF or type the password for nothing.
17. As a user, I want an empty password to work, because KeePass allows it, and a password above 1024 bytes to be refused without opening the Database.
18. As a user, I want the password to be used exactly as I typed it, without Unicode normalization, so it matches the one in KeePassXC.

### References

19. As a user, I want to point at a value by path and field (`AWS/paper-freeze#UserName`), without the Root group name, so I write short, readable References.
20. As a user, I want a Reference without `#` to return the Password, so the most common case is the shortest.
21. As a user, I want to read Custom attributes by the name they have in KeePassXC, so I can use fields like `region`.
22. As a user, I want to read an Attachment's bytes with `#attachment:<name>`, so I can use keys and certificates stored in the Database.
23. As a user, I want to get the current TOTP code with `#TOTP`, so I can use a second factor in scripts.
24. As a user, I want to get the TOTP seed with `#otp`, for the rare cases where the tool needs the seed.
25. As a user, I want to address an Entry by `uuid:<hex>`, so the Reference keeps working when I rename or move the Entry.
26. As a user, I want to escape `/`, `#` and `\` inside names, so I can use Entries with those characters in the title.
27. As a user, I want a Reference with invalid syntax to error with the position of the problem, so I fix it quickly.
28. As a user, I want a duplicate title to error listing the candidate UUIDs, so the binary never silently picks the wrong Entry.
29. As a user, I want an Entry in the Recycle bin to error, so a deleted password does not keep working without my noticing.
30. As a user, I want a value with `{REF:` to error instead of returning the literal text, so I do not receive a wrong value without warning.
31. As a user, I want an expired Entry to work, with a warning on stderr, so metadata does not break my tool.
32. As a user, I want a Custom attribute with the same name as a Reserved name (`TOTP`, `attachment:…`) to make the Reference fail, so I never receive an ambiguous value.

### get

33. As a user, I want `get <ref>` to write the raw value to stdout, without an extra line break, so I can use it in pipes and redirections.
34. As a user, I want `get` to accept several References and return one value per line, in the requested order, so I can read several fields with a single prompt.
35. As a user, I want `get` with several References to fail if any value has a line break, so the output is never ambiguous.

### render and Adapters

36. As a user, I want `render <name>` to execute the template `<name>` from my templates folder and write the result to stdout, so I deliver secrets in the format the tool expects.
37. As a user, I want to use the functions `get "<ref>"`, `json` and `base64` in the template, so I can build JSON, INI and other formats without escaping anything by hand.
38. As a user, I want all of a template's References to be resolved with a single prompt, so an Adapter costs only one password.
39. As a user, I want a template that, in the real run, asks for a Reference that did not appear in the collection pass to fail, so the binary never asks for a second password or reads something I did not see listed in the prompt.
40. As a user, I want a template error to say "template <name>: line N: <error kind>", without repeating the original text, so no value leaks into an error message.
41. As an AWS CLI user, I want to point a profile's `credential_process` at `kdbx-secrets render <name>`, to take the keys out of `~/.zshrc` and `~/.aws/credentials`.
42. As a user, I want to create a template with `adapter create <name>`, from `--from <file>` or stdin, to register an Adapter with the right permission even when my umask is `002`.
43. As a user, I want `adapter create` to validate the syntax of the template and of the References before saving, so I find errors at creation and not at first use.
44. As a user, I want `adapter create` to refuse to overwrite an existing template without `--force`, so I do not lose an Adapter by mistake.
45. As a user, I want `adapter create` to write the file atomically, so a failure never leaves a half-written template.
46. As a user, I want to delete an Adapter with `adapter delete <name>` and list the Adapters with `adapter list`, to manage them without remembering where they live.
47. As a user, I want to keep being able to create and edit templates by hand, so I do not depend on the command when I already know what I am doing.

### run --vault

48. As a user, I want `run --vault <mount>/<path>=<entry-ref> -- <command>` to execute the command with access only to the Entries I mapped, to give an app the minimum it needs.
49. As a user of Vault clients (the `vault` CLI, Terraform, `hvac`), I want them to work without an Adapter, receiving only `VAULT_ADDR` and `VAULT_TOKEN`, so I do not change the app.
50. As a user, I want to choose the KV path the app expects (for example, `secret/myapp/db`), independent of my Database's organization, so I do not expose my KeePass structure.
51. As a user, I want reading a path outside the Vault scope to answer 404, without revealing which paths exist, so the app does not discover the rest of the Database.
52. As a user, I want `data.data` to carry the Standard fields, the Custom attributes and the current TOTP code, but never the TOTP seed or the Attachments, so the app does not take the second factor or files.
53. As a user, I want the token to be random, exist only in memory and die with `run`, so a token leaked after the end is useless.
54. As a user, I want the server to listen only on `127.0.0.1`, accept only `GET` and refuse paths with `..` or `%2F`, to reduce the exposed surface.
55. As a user, I want `run` to end with the same exit code as the child, so scripts and supervisors see the real result.
56. As a user, I want Ctrl-C to reach the child and SIGTERM and SIGHUP to be forwarded to it, so the app ends cleanly while the server still answers.
57. As a Linux or Windows user, I want the child to die if `run` dies (`kill -9`, OOM), so no app is left running without an owner.
58. As a user, I want `run` to discard the rest of the Database and keep only the Entries in the Vault scope, to reduce what stays in memory during a long run.
59. As a user, I want a repeated KV path, a reserved mount (`sys`, `auth`, `identity`, `cubbyhole`), an empty segment, `.` or `..`, or a Reference with `#field` to be a usage error, to catch the wrong mapping before the prompt.

### encrypt / decrypt

60. As a user, I want to encrypt stdin to stdout in armored OpenPGP with `encrypt -r <public-key-file>`, without a prompt, to encrypt without opening the Database.
61. As a user, I want to encrypt to a public key stored in the Database with `encrypt --to <ref>`, so I do not keep the public key in a file.
62. As a user, I want to repeat and combine `-r` and `--to`, to encrypt to several recipients at once.
63. As a user, I want to decrypt with `decrypt --key <ref>`, with the private key read from an Attachment and the key's passphrase read from the Password of the same Entry, so the key never exists outside the Database.
64. As a user, I want everything to be compatible with `gpg` in both directions for v4 keys, to exchange files with whoever uses GnuPG.
65. As a user, I want the decrypted text to come out only after integrity (MDC/AEAD) is verified, and a message without integrity protection to be refused, so I never receive tampered text.
66. As a user, I want a clear error for a post-quantum key, a public key without an encryption subkey or a message above 64 MiB, to understand why the operation was refused.

### Output and messages

67. As a user, I want `get`, `render` and `decrypt` to refuse to write to a terminal without `--show`, so secrets do not end up in scrollback, in a session recording or in a screen share.
68. As a user, I want error messages to cite the Reference and never the value, so logs and screens do not leak secrets.
69. As a user, I want names coming from outside (processes, Entries, References) to have control characters replaced by `?` before reaching the terminal, so a malicious title does not inject escape sequences.
70. As a user, I want the exit codes `0` (ok), `1` (error) and `2` (incorrect usage), to handle failures in scripts.

### Memory hardening

71. As a Linux user, I want the process to lock memory against swap, disable core dump and become non-dumpable, so secrets do not land on disk or get read by `ptrace` from another process of mine.
72. As a Linux user with a low `RLIMIT_MEMLOCK`, I want the binary to check whether the limit covers the KDF memory before locking and, if not, warn and go on without the lock, so the Go runtime does not abort in the middle of Argon2.
73. As a macOS user, I want `mlock` on the binary's own buffers, core dump disabled and `PT_DENY_ATTACH`, to get the protection the platform allows.
74. As a Windows user, I want `VirtualLock` on the binary's own buffers, to get the protection the platform allows.
75. As a user, I want the password and the binary's outputs to be wiped after use, knowing that this is best effort, to reduce the time they stay in memory.

### Distribution and installation

76. As CI, I want to build on every `vX.Y.Z` tag one binary per OS and architecture (Linux amd64 and arm64, macOS amd64 and arm64, Windows amd64), without CGO, so nobody needs to compile anything.
77. As CI, I want to record the tag's version in the binary, so `kdbx-secrets version` says what is installed.
78. As CI, I want to publish a `SHA256SUMS` and a GitHub provenance attestation for each binary, so a swapped binary is detectable before receiving the master password.
79. As CI, I want to run `go vet` and `go test -race` on Linux, macOS and Windows on every push and PR, to catch OS-specific breakage before the release.
80. As CI, I want the release to be published only if the tests pass, so no broken binary reaches users.
81. As a new user, I want a bootstrap script (`install.sh` for Linux and macOS, `install.ps1` for Windows) that detects the OS and architecture and downloads the right binary from the latest release, to install with one line.
82. As a new user, I want the script to check the binary's SHA-256 against the release's `SHA256SUMS` and abort on mismatch, so a corrupted download is never installed.
83. As a user with `gh` installed, I want the script to also verify the provenance attestation, and, without `gh`, to explain how to verify, to confirm that the binary came out of the repository's workflow.
84. As a new user, I want the script to install into `~/.local/bin` (Unix) or `%LOCALAPPDATA%\kdbx-secrets\bin` (Windows), going through a temporary file and a `rename`, and warn me if the folder is not in `PATH`, so the command works right after.
85. As a user, I want the README to explain how to verify the binary by hand and how to configure the first Database, to use it without reading the spec.
86. As a maintainer, I want to publish a release with just `git tag vX.Y.Z && git push --tags`, so publishing is one step.

## Implementation Decisions

### Architecture

- **One password per run, no daemon** (ADR 0001). Each command opens the Database from disk, answers and exits; `run` is the only long process and lasts as long as the child. **Trigger to revisit** (recorded in the ADR): a real workflow that asks for the password several times in a row or a KDF too slow per call. In that case, the path is an on-demand daemon with a short TTL.
- **The binary never writes the Database** (ADR 0006). `adapter fill` has `keepassxc-cli` add what a template needs (standard fields only), never changing a value already set.
- **Password-only unlocking.** No keyfile, YubiKey or sealed password.
- **Native Go reader** (`gokeepasslib`) instead of `keepassxc-cli`, the browser protocol or Secret Service: it is the only one that works on the three OSes, without a GUI, and reads Attachments (see the study).
- **OpenPGP with `ProtonMail/go-crypto`** (ADR 0002): compatible with `gpg` for v4 keys.
- **Adapters in two layers** (ADR 0003). The first is `text/template` templates in a folder, which works as the registry. The second is built into the binary: `run --vault` and `encrypt`/`decrypt`. No Python, plugins or Starlark.
- **Secrets never go into an environment variable** (ADR 0005). There is no `-e` or `--env-file`. The environment of `run`'s child contains only `VAULT_ADDR` and `VAULT_TOKEN`. Recorded limit: while `run` lives, the token is equivalent to the Vault scope's secrets for any process of the same user. The gain is against passive leakage and access outside the Vault scope.

### Modules

- **Configuration.** Resolves the folder (`~/.kdbx-secrets/` or `KDBX_SECRETS_HOME`, which must be absolute) and reads the `config` in `key = value` format, with `#` as a comment. The only key is `database`. An unknown key is an error. On Unix, it checks the owner and permissions of the folder, the `config` and the templates, refusing those writable by group or others. The ACL check on Windows is left for later.
- **Reference** (ADR 0004). A strict parser with error position, and resolution over the open Database: path without the Root group, `uuid:` with 32 hex, escapes `\/` `\#` `\\`, Reserved names `TOTP` and `attachment:`. The Recycle bin and the history are left out, an ambiguous title is an error with the list of UUIDs, `{REF:` is an error and an expired Entry yields a warning. Names are case-sensitive and not normalized. TOTP is computed with an injectable clock.
- **Database.** Before the prompt, it checks that the file exists, is regular and starts with the KDBX signature `03d9a29a67fb4bb5`, and reads the KDF parameters from the header. After the prompt, it opens with `gokeepasslib` and separates "incorrect password" (header HMAC) from "incomplete or corrupted file".
- **Prompt.** Receives the terminal as an `io.ReadWriter`. In production, it is `/dev/tty` on Unix and `CONIN$`/`CONOUT$` on Windows. The prompt:
  - shows the parent process chain and the References, informatively and not as authentication;
  - gives 3 attempts;
  - on Ctrl-C, restores echo and exits with 1;
  - on Ctrl-D, aborts without counting an attempt;
  - limits the password to 1024 bytes and uses the bytes without normalization.
- **Hardening.** Runs before any secret enters memory.
  - **Linux:**
    1. `PR_SET_DUMPABLE=0` and `RLIMIT_CORE=0`.
    2. Raises the soft limit of `RLIMIT_MEMLOCK` up to the hard limit.
    3. Computes the KDF memory from the header.
    4. Calls `mlockall(MCL_CURRENT|MCL_FUTURE)` only if the limit covers the KDF plus a margin; if not, warns on stderr and goes on.
  - **macOS:** `mlock` on the binary's own buffers, `RLIMIT_CORE=0` and `PT_DENY_ATTACH`.
  - **Windows:** `VirtualLock` on the binary's own buffers. Excluding WER dumps is pending item 6.
  - Wiping memory is best effort, only for the password and the outputs: the `gokeepasslib` `string`s cannot be wiped.
  - The "lock or warn" decision is a pure function, separate from the syscalls.
- **Output.**
  - A single sanitizer replaces control characters with `?` in everything that goes to stderr or the tty.
  - Another single point refuses to write a secret to a stdout that is a terminal without `--show`.
  - Error messages cite the Reference, never the value.
- **Template.**
  - Executes in two passes: collection (`get` notes the Reference and returns `""`), then the prompt, then the real execution.
  - A new Reference in the real execution is an error.
  - Functions: `get`, `json` and `base64`.
  - Errors are rewritten as "template <name>: line N: <kind>".
- **Adapter.** `create` takes `<name>`, `--from` or stdin and `--force`. It:
  - validates the template and runs the collection pass without a prompt;
  - creates `templates/` with `0700` if it does not exist;
  - writes `<name>.tmpl` with `0600` through a temporary file and `rename`.

  `delete` deletes, and a nonexistent name is an error. `list` prints the names in alphabetical order. None of the three opens the Database.
- **Vault server.** Implements the KV v2 read (`GET /v1/<mount>/data/<path>`). `data.data` carries the Standard fields, the Custom attributes and the `TOTP` computed on each request (omitted without a seed), without the seed and without the Attachments. Protections:
  - listens only on `127.0.0.1`, on a random port;
  - 256-bit token, accepted in `X-Vault-Token` or `Authorization: Bearer` and compared in constant time;
  - a method other than `GET` answers 405; a missing or wrong token, 403; outside the Vault scope, 404;
  - a path with `..` or `%2F` is refused, the query is ignored and the body is not read;
  - `ReadHeaderTimeout` and `ReadTimeout` configured.

  The exact response format is pending item 3.
- **`run` supervisor.**
  - Parses `--vault` at the first `=`.
  - Resolves the Vault scope and drops the rest of the Database.
  - Starts the child with the inherited environment plus `VAULT_ADDR` and `VAULT_TOKEN`.
  - Ignores SIGINT and forwards SIGTERM and SIGHUP.
  - Ties the child to the parent: `PR_SET_PDEATHSIG(SIGTERM)` on Linux, a Job Object with `KILL_ON_JOB_CLOSE` on Windows. On macOS, the child survives and the calls to the Vault fail.
  - Redacts the child's stdout/stderr: every served Password of 8+ bytes and any value with a 16+ char alphanumeric run of entropy >= 3.5 bits/char (also JSON-escaped) becomes `*****`. A tail that may start a value is held until more output or 100 ms idle. On Linux, a terminal output gets a PTY with `OPOST` off; elsewhere a pipe. `--disable-redaction` turns it off.
  - `--command '<shell>'` runs the string with `sh -c` (`cmd /C` on Windows), instead of `-- <command>`.
  - Exits with the child's code.
- **OpenPGP.**
  - `encrypt` accepts `-r <file>` (no prompt) and `--to <ref>` (with a prompt), repeatable and combinable, with at least one recipient.
  - `decrypt --key <ref>` reads the private key (armored or binary) from the Reference's value and uses the Password of the same Entry only if the key is encrypted. The decrypted text sits in a buffer of up to 64 MiB and is only written after integrity is verified. A message without MDC/AEAD is refused, and a post-quantum key yields a clear error.
- **CLI entry point.** A function takes the injected environment and returns the exit code. The environment carries:
  - args and environment variables;
  - the terminal, stdin, stdout and stderr;
  - whether stdout is a terminal;
  - the clock.

  `main` only assembles the real environment. The validation order is always the same: arguments and syntax, config, folder, Database signature, prompt and, finally, content errors.

### CLI contracts

- `get <ref> [<ref>...]`
- `render <name>`
- `adapter create <name> [--from <file>] [--force]`, `adapter delete <name>`, `adapter list`
- `run --vault <mount>/<path>=<entry-ref> [--vault ...] [--disable-redaction] (--command '<shell>' | -- <command> [args])`
- `encrypt (-r <file> | --to <ref>)...`, `decrypt --key <ref>`
- `version`
- `--show` on `get`, `render` and `decrypt`.
- A template name matches `^[A-Za-z0-9._-]+$` and does not start with `.`.
- Exit codes: `0` ok, `1` error, `2` incorrect usage. `run` exits with the child's code.

### Distribution

- **Release.** A GitHub Actions workflow, triggered by a `v*.*.*` tag:
  1. runs the tests;
  2. builds with `CGO_ENABLED=0`, `-trimpath` and the tag's version injected into the binary, for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64 and windows/amd64;
  3. generates `SHA256SUMS`;
  4. creates a provenance attestation per binary (`actions/attest-build-provenance`);
  5. publishes everything as release assets.

  Names follow `kdbx-secrets-<goos>-<goarch>[.exe]`, the same pattern as git-sync. The attestation (Sigstore) is the signature: there is no signing key of its own to keep.
- **CI.** A workflow on push and PR runs `go vet` and `go test -race` on an `ubuntu`/`macos`/`windows` matrix, plus a cross `go build` for the five targets.
- **Bootstrap.** There is a POSIX `install.sh` and an `install.ps1`, published in the repository and downloaded through the raw URL. They:
  1. detect the OS and architecture;
  2. download the asset and the `SHA256SUMS` through `releases/latest/download`, without an API and without JSON;
  3. check the SHA-256 and abort on mismatch;
  4. if `gh` exists, run `gh attestation verify --repo <repo>`; if not, print the command to verify by hand;
  5. install through a temporary file and `rename` into `~/.local/bin` or `%LOCALAPPDATA%\kdbx-secrets\bin`, overridable by `KDBX_SECRETS_BIN_DIR`;
  6. warn if the folder is not in `PATH`.

  They do not create the configuration. They follow the taskthing model.
- **Post-release smoke.** A job, after publishing, runs `install.sh` on `ubuntu` and `macos` and `install.ps1` on `windows` against the just-published release, and executes `kdbx-secrets version`.
- **Version.** The tag is the source of truth, and `version` prints the injected version.
- **No self-update.** Updating is running the bootstrap again.

## Testing Decisions

- **What a good test is here:** externally observable behavior at the CLI entry point. That is: exit code, bytes on stdout, messages on stderr, what the prompt wrote to the terminal, files created in the configuration folder and what a `run` child can read through the API. Never the internal structure of the parser, the server or the Database reader.
- **Seam 1, the CLI entry point.** Covers almost everything. Each test:
  - points `KDBX_SECRETS_HOME` at a temporary directory with `config` and templates;
  - injects a pipe as the terminal, with the password written on the other side;
  - injects buffers as stdin, stdout and stderr, and says whether stdout is a tty;
  - injects a fixed clock for TOTP.

  Coverage:
  - **References:** tables of invalid syntax that fail before the prompt (the terminal stays untouched), each Reference form, ambiguity, Recycle bin, `{REF:`, expired and collision with a Reserved name.
  - **Prompt:** 3 attempts, incorrect password vs truncated file, EOF, empty password and the 1024-byte limit.
  - **Output:** refusal of a tty without `--show`, control sanitization and no value in an error message.
  - **Configuration:** unknown key, missing `config`, wrong permissions (Unix only) and relative `KDBX_SECRETS_HOME`.
  - **`render`:** two passes, a new Reference in the real execution and a rewritten template error.
  - **`adapter`:** `0600`/`0700` permissions, refusal without `--force`, validation before writing and `list`/`delete`.
  - **`run --vault`:** the child is the test binary itself re-executed as an HTTP client, in the helper process pattern. It reads `VAULT_ADDR`/`VAULT_TOKEN`, makes requests and reports the result. This verifies:
    - the Vault scope (200 inside, 404 outside);
    - 403 without a token, 405 on writes, refusal of `..` and `%2F`;
    - the TOTP present and the seed and Attachments absent;
    - the environment without secrets;
    - the exit code passed through.
  - **`encrypt`/`decrypt`:** round trip, and keys and messages generated by `gpg` stored in `testdata/` to prove compatibility in both directions. Also the refusal of a message without integrity and the size limit.
- **Seam 2, the lock decision.** It is a pure function that takes the KDF memory and the `RLIMIT_MEMLOCK` (soft and hard) and returns "raise to X and lock" or "warn and go on". Table tests. Complement: a subprocess test, on Linux, re-executes the test binary, applies the hardening and checks `PR_GET_DUMPABLE == 0` and `RLIMIT_CORE == 0`.
- **Fixtures.** The `.kdbx` files with a known password live in `testdata/`, generated once by `keepassxc-cli` and committed: Argon2d, Argon2id, AES-KDF, Attachment, TOTP, duplicate title, Entry in the Recycle bin, expired Entry, `{REF:}`, title with a control character and a Custom attribute named `TOTP`. A versioned script regenerates the fixtures, but CI does not run that script. The fixtures use low KDF parameters so the tests are fast, except one with realistic parameters to measure the cost (pending item 4).
- **Optional test.** If `vault` or `python3 -c 'import hvac'` are available, a test runs the real client under `run --vault`; otherwise, it is skipped. It covers pending item 3 without making CI depend on those tools.
- **Outside automation, with manual verification per platform:**
  - the real `/dev/tty` and the Windows console when the parent captured stdin and stdout (pending item 2);
  - `PDEATHSIG` and the Job Object with `kill -9` on the parent;
  - `mlockall` with a low `RLIMIT_MEMLOCK` (pending item 5);
  - WER (pending item 6);
  - the bootstrap scripts, covered only by the post-release smoke.
- **Rules:**
  - No environment variable with a password in the binary, not even "just for tests".
  - No test touches the network beyond `127.0.0.1`.
  - `go test -race` on the three OSes.
- **Prior art:** none in this repository (new project). git-sync established the pattern of testing against real resources in temporary directories; taskthing, that of testing through the CLI with the environment injected.

## Out of Scope

- Writing to the Database and merging Databases.
- Daemon, unlock cache, TTL and `unlock` (see the trigger in ADR 0001).
- Keyfile, YubiKey and sealed password (TPM/`systemd-creds`, Keychain, DPAPI). Future path if a 24/7 app appears that needs to start without a human: `run --sealed`.
- SSH agent and commit signing (KeePassXC already handles that).
- A graphical prompt; a command that writes the tools' configuration by itself (for example, editing `~/.aws/config`).
- Masking secrets in the child's output: it requires a pipe and takes the TTY away from the child.
- Injecting secrets into an environment variable (`-e`, `--env-file`), ADR 0005.
- Writing to the Vault API, engines other than KV v2, `sys/*`, child tokens and leases.
- Restricting the `run --vault` server to the child's descendants, by identifying the client PID from the TCP connection. It only works on Linux, is laborious and only protects the token; it comes back if the token in the environment becomes a concrete problem.
- ACL check on Windows for the configuration folder.
- OpenPGP v6 and post-quantum keys; OpenPGP signing and verification.
- Self-update; distribution through a package manager (Homebrew, apt, winget, Scoop); macOS (notarization) and Windows (Authenticode) code signing.
- Integration with a running KeePassXC (browser protocol, Secret Service).

## Further Notes

- **Delivery order.** Each slice works on its own:
  1. Configuration, Database, prompt, References, hardening and **`get`**. Starts with pending item 1, together with CI.
  2. **`render`** and **`adapter create|delete|list`**. Closes the `paper-freeze` case.
  3. **`run --vault`**. Depends on pending item 3.
  4. **`encrypt` / `decrypt`**.
  5. **Release, bootstrap and smoke.** CI enters in slice 1; the release, when slice 2 is usable.
- **Verification pending items:**
  1. `gokeepasslib` with real `.kdbx` files generated by KeePassXC: Argon2d, Argon2id, AES-KDF, Attachments, TOTP.
  2. Prompt on the Windows console when the process is a child of another that captured stdin and stdout.
  3. KV v2 response format accepted by the `vault` CLI, by Terraform and by `hvac`. Check whether the CLI calls `/v1/sys/internal/ui/mounts/<path>` before reading and whether the Terraform provider creates a child token (`auth/token/create`, `skip_child_token`).
  4. KDF cost per run with the real parameters of the user's Database.
  5. Spike of `mlockall(MCL_FUTURE)` with Argon2: confirm the runtime fatal error with a low `RLIMIT_MEMLOCK` and that the header check before the lock avoids that error.
  6. Crash dumps on Windows: how to exclude the process from Windows Error Reporting dumps.
- **Known hardening limits** (recorded in `CONTEXT.md`):
  - Windows and macOS have no `mlockall`, so the decrypted Database may go to swap or the pagefile. macOS swap is encrypted by default; on Windows, the recommendation is to use BitLocker.
  - Hibernation writes RAM to disk on every OS.
  - Malware running as the same user is outside what a CLI can prevent.
  - Typing the master password on every use makes a fake prompt easier. The process chain in the prompt helps to notice it, but does not prevent it.
- **Reference example, the `paper-freeze` case:**
  - The Entry `AWS/paper-freeze` stores the access key id in UserName and the secret in Password.
  - The template `aws-paper-freeze` produces the JSON `{"Version":1,"AccessKeyId":…,"SecretAccessKey":…}` using `get` and `json`.
  - The `paper-freeze` profile in `~/.aws/config` points `credential_process` at `kdbx-secrets render aws-paper-freeze`.
  - The function in `~/.zshrc` loses the key exports and calls `aws --profile paper-freeze` instead.
  - Verified with AWS CLI 2.34.15: `credential_process` keeps the controlling terminal and the CLI does not cache the credential in `~/.aws`. The chain shown in the prompt is `aws ← zsh`, because `paper-freeze` is a shell function and not a process.
- **Related documents:**
  - [CONTEXT.md](../../CONTEXT.md): requirements, configuration and hardening.
  - [GLOSSARY.md](../../GLOSSARY.md): Database, Vault, Entry, Reference, Reserved name, Adapter, Vault scope.
  - ADRs [0001](../adr/0001-one-password-per-run.md), [0002](../adr/0002-openpgp-with-go-crypto.md), [0003](../adr/0003-two-layer-adapters.md), [0004](../adr/0004-reference-syntax.md) and [0005](../adr/0005-secret-never-in-environment.md).
  - The originating study: [STUDY-keepassxc-layer.md](../STUDY-keepassxc-layer.md).
- **Issue tracker.** This repository has no remote or authenticated `gh`, so the spec was saved locally with `status: ready-for-agent`, as in git-sync. To publish it to a tracker with the `ready-for-agent` label, configure the remote and run `/setup-matt-pocock-skills`.
