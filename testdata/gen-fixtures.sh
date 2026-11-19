#!/bin/sh
# Regenerates testdata/keepassxc/*.kdbx with keepassxc-cli (pending item 1).
# The CI never runs this script; the generated files are committed.
#
# Tested with keepassxc-cli 2.8.0 (flatpak). Its db-create has no KDF flag, so
# the KDF variants are made by letting KeePassXC open and re-save the
# gokeepasslib fixtures (testdata/*.kdbx) while adding the Entry
# AWS/keepassxc: the result is a file written by KeePassXC, and it proves that
# KeePassXC reads the KDF parameters gokeepasslib writes.
#
# rich.kdbx is imported from rich.xml (custom icon, tags, colours, protected
# custom attribute, AutoType association, entry CustomData), then gets an
# attachment and an edit (History) through the CLI.
# Note: `import` segfaults on Protected="True" in plain XML; use
# ProtectInMemory="True" as KeePassXC's own export does.
#
#   KEEPASSXC_CLI="flatpak run --command=keepassxc-cli org.keepassxc.KeePassXC" testdata/gen-fixtures.sh
# (flatpak also needs --filesystem=<repo>/testdata before --command.)
set -eu

PASS='fixture-pass'
T="$(cd "$(dirname "$0")" && pwd)"
O="$T/keepassxc"
CLI="${KEEPASSXC_CLI:-keepassxc-cli}"
export LC_ALL=C.UTF-8 LANGUAGE=en # the root group name is localized
K() { $CLI "$@"; }

for f in argon2d argon2id aes-kdbx4 aes-kdbx3; do
  cp "$T/$f.kdbx" "$O/$f.kdbx"
  printf '%s\n%s\n' "$PASS" 'kx-value' | K add -q -u kxuser -p "$O/$f.kdbx" AWS/keepassxc
done

rm -f "$O/rich.kdbx"
printf '%s\n%s\n' "$PASS" "$PASS" | K import -q -p -t 100 "$O/rich.xml" "$O/rich.kdbx"
key="$O/.key.bin"; printf 'secret\000bytes\377' > "$key"
printf '%s\n' "$PASS" | K attachment-import -q "$O/rich.kdbx" AWS/rich key.bin "$key"
rm -f "$key"
printf '%s\n%s\n' "$PASS" 'rich-pass-2' | K edit -q -p "$O/rich.kdbx" AWS/rich
