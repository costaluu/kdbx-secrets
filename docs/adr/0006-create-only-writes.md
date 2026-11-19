---
status: accepted
---

# Create-only writes, through `adapter fill` and `keepassxc-cli`

Supersedes the "Read-only" requirement (CONTEXT.md, spec): the binary still never writes a `.kdbx` itself, but `adapter fill` has KeePassXC write one.

Setting up an Adapter used to end with "now create the Entry in KeePassXC", a manual step that breaks scripted installs (`examples/paper-freeze`). `kdbx-secrets adapter fill <name>` asks for the values the template uses that are missing in the Database and has `keepassxc-cli` write them.

Rules:
- **KeePassXC writes, we only plan.** The binary opens the Database with its reader, works out what is missing, asks for the values, then runs `keepassxc-cli mkdir` / `add` / `edit`. The Database password and the Password field go through stdin; UserName, URL and Notes go through argv (visible in the process list while `keepassxc-cli` runs).
- **Create-only.** A missing group or Entry is created; a missing or empty field is filled. A value that is already set is never asked or sent. `edit` puts the old Entry state in History (KeePassXC's own `beginUpdate`/`endUpdate`).
- **Only what `keepassxc-cli` can write:** UserName, Password, URL, Notes, and new Entries by path. Custom attributes, `#TOTP`, `#attachment:` and `uuid:` References fail before any prompt for values, with "add it in KeePassXC".
- `keepassxc-cli` from `KEEPASSXC_CLI` (split on spaces, e.g. the flatpak command) or `PATH`; missing → error before the prompt. `get`, `render`, `run`, `encrypt` and `decrypt` do not need it.

Alternatives considered:
- **Our own writer on `gokeepasslib`** (built and dropped, see `docs/STUDY-kdbx-write.md`): fresh nonces, `KPXC_RANDOM_SLUG`, atomic rename, a fidelity guard and a decode-and-compare self-check. It passed against KeePassXC-written files, but the self-check reads both sides with the same library, every other KeePass client's format and every filesystem quirk would be ours to chase, and it already had one behaviour bug (new groups with `EnableAutoType=true` instead of inherit). Not worth owning.
- **Merge from a temp Database** (`keepassxc-cli merge`) to also write custom attributes: rejected for now, merge semantics (mod times, deletions, moves) are too much surface for a setup helper.

Cost: each `keepassxc-cli` call derives the key again (one KDF per created group plus one per Entry).
