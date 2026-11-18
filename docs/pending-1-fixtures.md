# Pending item 1: `gokeepasslib` with real Databases

State after ticket 01: **verified only against files written by `gokeepasslib` itself, not against files written by KeePassXC.** The item stays open until someone regenerates the fixtures with KeePassXC.

## What exists

- `testdata/*.kdbx`: `argon2d`, `argon2id`, `aes-kdbx4` and `aes-kdbx3`. All with the password `fixture-pass` and an Entry `AWS/paper-freeze` (UserName `AKIAFIXTURE`, Password `s3cr3t-value`) under the Root group `Root`. Low KDF parameters, to keep the tests fast.
- `testdata/gen-fixtures.sh`: the script that was supposed to generate them with `keepassxc-cli`. **It was never run** (`keepassxc-cli` was not available). The KDF flag names vary between versions and need adjusting. CI does not run the script.
- `tools/genfixtures`: a Go program that wrote the current fixtures with the `gokeepasslib` writer (`go run ./tools/genfixtures testdata`). It is the temporary replacement for the script.

When running the script with KeePassXC, the `internal/cli` tests (`TestGetPasswordFromEveryKDB`) must keep passing with the new files; any failure is the finding of this item. After that, update this document.

Note: some fixture entries keep Portuguese titles and attachment names (for example `GPG/pessoal` with `chave.asc`); the tests depend on them, so they were left untouched.

## Finding: `gokeepasslib` v3.7.1 does not read Argon2id

`gokeepasslib` only dispatches the Argon2d UUID (`KdfArgon2`); any other UUID falls into the AES-KDF branch and the right password shows up as "incorrect password". An Argon2id Database (an option in KeePassXC and KeePass 2) does not open.

For that reason the repository ships a local copy in `third_party/gokeepasslib` (v3.7.1, MIT, without tests and examples), wired in by `replace` in `go.mod`, with a single patch: `KdfArgon2id` in `header.go` and the use of `argon2.IDKey` in `credentials.go`. Details in `third_party/gokeepasslib/PATCH.md`. Remove the copy and the `replace` when upstream supports Argon2id.

Limit: Argon2id support was exercised only in a round trip with the patched writer, so only an Argon2id Database written by KeePassXC proves the parameters' compatibility (for example, the conversion of memory from bytes to KiB).

## Header

`internal/db.CheckFile` reads the signature and the KDF parameters (KDBX 3.x and 4.x) on its own, without deriving a key, because `gokeepasslib` only exposes the header by decoding the whole file (which pays the KDF). An unknown KDF fails before the prompt.
