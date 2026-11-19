# Study: how KeePassXC writes a `.kdbx`, and what it takes to do it here

Date: 2026-11-19. Source read: KeePassXC `cd422365` (`~/projetos/keepassxc`), vendored `gokeepasslib` v3.7.1.

Goal: let the binary write the credentials a template needs, from the user's input, so the user only supplies values and never edits the Database by hand (today step 3 of `examples/paper-freeze/upgrade-paper-freeze-v2.sh`). Hard rule: **never rewrite what already exists**.

Outcome: a writer following this study was built and tested, then **dropped**: `adapter fill` delegates the write to `keepassxc-cli` (ADR 0006). This document stays as the reason why.

## 1. KeePassXC's save path

`cli/Add.cpp` is the smallest real example: `rootGroup()->addEntryWithPath(path)` → set fields → `database->save(Database::Atomic, {}, &err)`.

### 1.1 `Group::addEntryWithPath` (`core/Group.cpp`)

- Returns `nullptr` if an Entry already exists at that path (`findEntryByPath`). **Create-only by construction.**
- Returns `nullptr` if the group path does not exist. It never creates groups.
- New Entry: `QUuid::createUuid()`, title = last path segment, default icon, all four times = now.

### 1.2 Editing (`cli/Edit.cpp`, `core/Entry.cpp`)

`entry->beginUpdate()` copies the current Entry to a temp; `endUpdate()`, if anything changed, pushes that copy to `History` and calls `truncateHistory()` (`Meta.HistoryMaxItems`, `HistoryMaxSize`). Edits never lose the old value. Not needed for create-only, but it is the rule if we ever overwrite.

### 1.3 `Database::saveAs` (`core/Database.cpp:269`)

1. **External-change guard.** At load/save it keeps `m_fileBlockHash` = MD5 of the first `kFileBlockToHashSizeBytes` of the file. Before writing it re-reads that block; if different → `"Database file has unmerged changes."` and abort (the GUI then offers a merge).
2. **Anti-dedup slug.** `Meta/CustomData["KPXC_RANDOM_SLUG"]` = 64–512 random bytes (hex) on every save, so two saves never produce similar ciphertext sizes/blocks.
3. Resolves symlinks (`canonicalFilePath`) and writes the real file.
4. New file → permissions `0600`. Existing file → keeps permissions and birth time.

### 1.4 `performSave` — three modes

| Mode | How | Note |
|---|---|---|
| `Atomic` (default, CLI) | `QSaveFile`: temp file in the same dir, `commit()` = fsync + rename | crash-safe |
| `TempFile` | `QTemporaryFile`, `remove(orig)` + `rename` | for Dropbox/OneDrive; not atomic, restores from backup on failure |
| `DirectWrite` | serializes to RAM, truncates and writes in place | for filesystems that break renames |

Optional backup: copies the old file to `<name>.old.kdbx` before writing.

### 1.5 `Kdbx4Writer::writeDatabase` — fresh randomness on **every** save

```
masterSeed         = random(32)
encryptionIV       = random(ivSize)      // 16 AES/Twofish, 12 ChaCha20
protectedStreamKey = random(64)          // inner ChaCha20 stream for protected values
db->setKey(key, false, /*updateTransformSalt*/ true)  // new KDF salt → KDF re-run
```

Then: header → SHA-256 + HMAC of header → HMAC block stream → cipher → gzip → inner header (stream id, stream key, attachments) → XML.

`writeDatabase` also asserts the transformed key changed (`"Key not transformed. This is a bug"`). `KeePass2Writer::kdbxVersionRequired` upgrades 3.1→4→4.1 when the content needs it (custom data, tags, previous parent group…).

## 2. What `gokeepasslib` does differently

### 2.1 It reuses every nonce (blocking)

`Encoder.Encode` writes `db.Header` as it was read. Spike (`go run` over `testdata/*.kdbx`, decode → encode → decode):

```
aes-kdbx3  seed/iv reused=true
aes-kdbx4  seed/iv reused=true innerKey reused=true
argon2d    seed/iv reused=true innerKey reused=true
argon2id   seed/iv reused=true innerKey reused=true
```

Consequence: the old and new file are encrypted with the same key and IV. With ChaCha20 (outer cipher, or the inner stream that protects every Password) that is keystream reuse: `old ⊕ new` of the plaintext leaks. With git-sync every version stays in history, so the attacker has both files. **Saving through `gokeepasslib` without regenerating these is not acceptable.**

Fix, before `Encode` (all fields already exist in the header structs):

| KDBX 4 | KDBX 3.1 |
|---|---|
| `FileHeaders.MasterSeed` (32) | `MasterSeed` (32) |
| `FileHeaders.EncryptionIV` (len of current) | `EncryptionIV` (16) |
| `KdfParameters.Salt` (32) | `TransformSeed` (32) |
| `Content.InnerHeader.InnerRandomStreamKey` (64) | `ProtectedStreamKey` (32), `StreamStartBytes` (32) |

All from `crypto/rand`; the KDF then runs again on save (same cost as opening).

Order matters: `Encode` first *unlocks* protected values with the current stream key and then locks them again, so it must receive them already locked. Sequence: values in clear (as after `Open`) → regenerate keys → `LockProtectedEntries()` → `Encode`. Locking before regenerating would write values encrypted under the old key behind a header with the new one.

Also found: `KdfParameters.updateRawData` wrote the Argon2 associated data under the name `"K"` instead of `"A"`. Patched in `third_party/gokeepasslib` (see `PATCH.md`).

### 2.2 XML fidelity

`gokeepasslib` decodes into structs, so anything it does not model is **dropped on write** — that would silently rewrite existing entries.

Static check: every element name `KdbxXmlWriter.cpp` emits is modeled in `gokeepasslib`, except the `ProtectInMemory` attribute, which KeePassXC only writes in plain-XML export (not inside `.kdbx`). So a KeePassXC-written Database should round-trip. Elements from KeePass 2.x plugins or other clients are not guaranteed.

Runtime check (all current fixtures): decoded XML before == after a re-encode. But the fixtures were written by `gokeepasslib` itself (`docs/pending-1-fixtures.md`), so this proves self-consistency only. **Pending item 1 becomes blocking for this feature.** (Closed 2026-11-19: `testdata/keepassxc/`, `TestSaveKeePassXCFiles`.)

Guard that makes the risk explicit instead of silent: `db.Content.RawData` holds the decrypted inner header + XML. Before writing, collect the set of element paths (`KeePassFile/Root/Group/Entry/String/Key`…) from that raw XML and from `xml.Marshal(db.Content)`; if any path is missing in the re-marshal, refuse to write and name it.

### 2.3 Other gaps

- No `KPXC_RANDOM_SLUG`: one `CustomData` item in `Meta`, easy to replicate.
- No external-change guard, no atomic save, no permission/birth-time preservation: all ours.
- Protected values: a new Password must be written with `Protected=True` (follow `Meta.MemoryProtection`), otherwise it is stored outside the inner stream.

## 3. Proposed write path in Go

```
read file bytes B0 (resolve symlink first: filepath.EvalSymlinks)
decode → db
fidelity guard (2.2)            → refuse on lost paths
mutate: create-only (3.1)
regenerate nonces + salt (2.1), set KPXC_RANDOM_SLUG
encode to buffer
verify: decode buffer with the same password; every pre-existing Entry (by UUID) deep-equal to before; new values readable
re-read file; bytes != B0       → abort "database changed on disk (git-sync? KeePassXC?), run again"
os.CreateTemp(same dir) → write → Chmod(orig perms) → Sync → Close → os.Rename → fsync dir
```

The "verify" step is cheaper than the bug it catches: it costs one more KDF run.

Atomic rename is enough here: the Database is local (git-sync moves it, not a cloud folder), so `TempFile`/`DirectWrite` are not needed. Backup `.old.kdbx` is skipped: git-sync history already keeps every version.

The window between the final re-read and `rename` is not locked. KeePassXC's own guard has the same window; KeePassXC GUI, if it has the Database open, sees our change through its file watcher and merges or reloads. Lock files would only help against writers that honour them, and neither git-sync nor KeePassXC does.

### 3.1 Mutation policy: create-only

Same as `addEntryWithPath`, but per field:

- Entry missing → create it (new UUID, `NewTimeData()`), fill the fields.
- Entry exists, field missing → add the field; push the old Entry to History first (§1.2), bump `LastModificationTime`.
- Entry exists, field empty → treated as missing and filled (KeePassXC always writes the five Standard fields, empty or not).
- Entry exists, field set → left alone, never overwritten. Overwriting can come later with a `--force` that does the History dance.
- Group missing → create it (`kdbx-secrets` paths are always `Group/Entry`); KeePassXC CLI refuses here, but refusing would just send the user back to the GUI.
- Recycle Bin is never a target (same rule as reads).

Only `UserName`, `Password` and custom string fields. No `#TOTP` (it is derived from `otp`), no `#attachment:` in the first cut.

## 4. Template integration

No new template syntax is needed. `collectTmpl` (`internal/cli/render.go`) already lists every Reference a template uses (first pass of `render`). A new subcommand:

```
kdbx-secrets adapter fill <name>
```

1. Parse the template, collect References.
2. One password prompt (ADR 0001), open the Database.
3. For each Reference that does not resolve → prompt for the value (hidden for `Password` and any field marked protected; never echoed, never in argv/env per ADR 0005).
4. Nothing missing → "nothing to do", no write.
5. Write via §3. Print only the References that were created.

`paper-freeze` step 3 then becomes `"$SECRETS" adapter fill "$TEMPLATE"`.

Labels for the prompts: Reference itself (`AWS/paper-freeze#UserName`). If friendlier labels are wanted later, a template comment like `{{/* fill AWS/paper-freeze#UserName "Access Key ID" */}}` can be parsed without changing execution.

## 5. Order of work

1. ADR: "kdbx-secrets writes, create-only" (replaces the Read-only requirement).
2. Close pending item 1: real KeePassXC fixtures; add a round-trip test (decode → encode → decode, XML paths equal) over them.
3. `internal/db`: `Save` with §2.1 + §3, test asserting seeds/IV/inner key change and existing entries unchanged.
4. `adapter fill`.
5. Update `paper-freeze` script and README.
