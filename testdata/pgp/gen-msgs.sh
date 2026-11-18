#!/usr/bin/env bash
# Generates gpg-made messages and a passphrase-less key from the existing
# fixture keys (passphrase "pgp-pass"). Needs gpg >= 2.5. NOT run by CI.
set -euo pipefail
cd "$(dirname "$0")"
export GNUPGHOME=$(mktemp -d)
trap 'rm -rf "$GNUPGHOME"' EXIT
g() { gpg --batch --pinentry-mode loopback --passphrase pgp-pass "$@" 2>/dev/null; }
g --import ed25519.sec.asc rsa3072.sec.asc
echo "hello gpg" > "$GNUPGHOME/p.txt"
enc() { out=$1 r=$2; shift 2; gpg --batch --armor --trust-model always -r "$r@example.test" "$@" -e -o "$out" "$GNUPGHOME/p.txt" 2>/dev/null; }
enc msg-ed25519.asc ed25519
enc msg-rsa3072.asc rsa3072
enc msg-ocb.asc ed25519 --force-ocb
enc msg-nomdc.asc ed25519 --rfc2440 --disable-mdc
gpg --batch --pinentry-mode loopback --passphrase '' --quick-gen-key "nopass <nopass@example.test>" default default never 2>/dev/null
gpg --batch --armor --export nopass@example.test > nopass.pub.asc
gpg --batch --pinentry-mode loopback --passphrase '' --armor --export-secret-keys nopass@example.test > nopass.sec.asc
enc msg-nopass.asc nopass
