#!/bin/sh
# Regenerates the committed .kdbx fixtures with keepassxc-cli (pending item 1).
# The CI never runs this script; the generated files are committed.
#
# Requires keepassxc-cli >= 2.7. Every fixture uses the master password below
# and holds one entry AWS/paper-freeze (UserName AKIAFIXTURE, Password
# s3cr3t-value) under the root group.
#
# STATUS: UNTESTED. Written for ticket 01 on a machine without keepassxc-cli;
# the files currently in testdata/ were produced by tools/genfixtures
# (gokeepasslib writer) instead. Run this script once on a machine with
# KeePassXC, run `go test ./...`, and commit the result to close pending item 1
# for real (see docs/pending-1-fixtures.md).
#
# KDF selection: flag names vary between keepassxc-cli versions. Adjust the
# make_db lines to what `keepassxc-cli db-create --help` shows. If your version
# has no KDF flag, create the databases once in the KeePassXC GUI (Database >
# Database Settings > Encryption: Argon2d, Argon2id, AES-KDF, low cost) with
# the same password and entry.
set -eu

PASS='fixture-pass'
OUT="$(dirname "$0")"
CLI="${KEEPASSXC_CLI:-keepassxc-cli}"
command -v "$CLI" >/dev/null || { echo "keepassxc-cli not found" >&2; exit 1; }

make_db() { # <file> <db-create flags...>
  f="$OUT/$1"; shift
  rm -f "$f"
  printf '%s\n%s\n' "$PASS" "$PASS" | "$CLI" db-create -p "$@" "$f"
  printf '%s\n%s\n' "$PASS" 's3cr3t-value' |
    "$CLI" add -u AKIAFIXTURE -p "$f" AWS/paper-freeze
}

make_db argon2d.kdbx   --kdf argon2d  --decryption-time 100
make_db argon2id.kdbx  --kdf argon2id --decryption-time 100
make_db aes-kdbx4.kdbx --kdf aes256   --decryption-time 100
echo "aes-kdbx3.kdbx must be made in the GUI (Database format: KDBX 3.1)."
