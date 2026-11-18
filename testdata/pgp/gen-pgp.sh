#!/usr/bin/env bash
# Regenerates the OpenPGP fixtures (public + secret keys, passphrase "pgp-pass").
# Needs gpg >= 2.4. NOT run by CI. Throwaway keys, safe to commit.
set -euo pipefail
cd "$(dirname "$0")"
export GNUPGHOME=$(mktemp -d)
trap 'rm -rf "$GNUPGHOME"' EXIT
g() { gpg --batch --pinentry-mode loopback --passphrase pgp-pass "$@" 2>/dev/null; }
gen() { # name algo usage
  g --quick-gen-key "$1 <$1@example.test>" "$2" "$3" never
  g --armor --export "$1@example.test" > "$1.pub.asc"
  g --armor --export-secret-keys "$1@example.test" > "$1.sec.asc"
}
gen ed25519 default default
gen rsa3072 rsa3072 default
g --quick-add-key "$(g --list-keys --with-colons rsa3072@example.test | awk -F: '/^fpr/{print $10; exit}')" rsa3072 encr never
g --armor --export rsa3072@example.test > rsa3072.pub.asc
g --armor --export-secret-keys rsa3072@example.test > rsa3072.sec.asc
# primary key restricted to signing, no encryption subkey
gen signonly ed25519 sign
# post-quantum (needs gpg >= 2.5); unsupported by go-crypto, used for the error test
gen pqc pqc default
rm -f pqc.sec.asc
