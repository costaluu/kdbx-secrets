# Pending item 1: `gokeepasslib` with real Databases

**Closed on 2026-11-19** with KeePassXC 2.8.0 (`keepassxc-cli` from flatpak).

## Result

- `testdata/keepassxc/{argon2d,argon2id,aes-kdbx4,aes-kdbx3}.kdbx`: the `gokeepasslib` fixtures opened and re-saved by KeePassXC (adding the Entry `AWS/keepassxc`). KeePassXC reads our Argon2id parameters (`2 rounds, 1024 KB` = `Memory 1<<20` bytes), so the memory conversion in the Argon2id patch is right, and we read every file KeePassXC wrote.
- `testdata/keepassxc/rich.kdbx`: imported by KeePassXC from `rich.xml` (custom icon, tags, colours, protected custom attribute, AutoType association, entry CustomData), plus an attachment and an edit (History). AES-KDF, AES cipher.
- `internal/cli.TestGetFromKeePassXCFiles` reads all five.
- `internal/cli.TestFillRealKeePassXC` runs `adapter fill` through the real `keepassxc-cli` when it is available (skipped in CI).
- `testdata/gen-fixtures.sh` regenerates them (works now; `db-create` in 2.8.0 has no KDF flag, hence the re-save approach). Found on the way: `keepassxc-cli import` segfaults on `Protected="True"` in plain XML.

Not covered: a KDBX 3.1 or Argon2d file *created* by KeePassXC (only re-saved), key files, YubiKey.

## Before (gokeepasslib-only fixtures)

- `testdata/*.kdbx`: `argon2d`, `argon2id`, `aes-kdbx4` and `aes-kdbx3`. All with the password `fixture-pass` and an Entry `AWS/paper-freeze` (UserName `AKIAFIXTURE`, Password `s3cr3t-value`) under the Root group `Root`. Low KDF parameters, to keep the tests fast.
- `tools/genfixtures`: a Go program that wrote the current fixtures with the `gokeepasslib` writer (`go run ./tools/genfixtures testdata`). They stay: the `internal/cli` tests depend on their content.

Note: some fixture entries keep Portuguese titles and attachment names (for example `GPG/pessoal` with `chave.asc`); the tests depend on them, so they were left untouched.

## Finding: `gokeepasslib` v3.7.1 does not read Argon2id

`gokeepasslib` only dispatches the Argon2d UUID (`KdfArgon2`); any other UUID falls into the AES-KDF branch and the right password shows up as "incorrect password". An Argon2id Database (an option in KeePassXC and KeePass 2) does not open.

For that reason the repository ships a local copy in `third_party/gokeepasslib` (v3.7.1, MIT, without tests and examples), wired in by `replace` in `go.mod`, with a single patch: `KdfArgon2id` in `header.go` and the use of `argon2.IDKey` in `credentials.go`. Details in `third_party/gokeepasslib/PATCH.md`. Remove the copy and the `replace` when upstream supports Argon2id.

Argon2id compatibility with KeePassXC was confirmed when pending item 1 closed (see Result).

## Header

`internal/db.CheckFile` reads the signature and the KDF parameters (KDBX 3.x and 4.x) on its own, without deriving a key, because `gokeepasslib` only exposes the header by decoding the whole file (which pays the KDF). An unknown KDF fails before the prompt.
