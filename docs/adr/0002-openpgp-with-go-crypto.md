---
status: accepted
---

# OpenPGP encryption with `ProtonMail/go-crypto`, compatible with `gpg` for v4 keys

`encrypt` and `decrypt` are implemented inside the binary with `github.com/ProtonMail/go-crypto`, without depending on an installed `gpg`. The private key is stored as an attachment of an entry in the `.kdbx` and never leaves the process.

The requirement is to interoperate with `gpg`: files encrypted on one side open on the other.

Tested on 2026-11-12, GnuPG 2.5.20 x go-crypto v1.5.2, keys generated in `gpg` with a passphrase:

| Key | gpg encrypts -> Go decrypts | Go encrypts -> gpg decrypts |
|---|---|---|
| gpg 2.5 default (ed25519 + cv25519, v4) | OK | OK |
| ed25519 + cv25519 subkey (v4) | OK | OK |
| RSA-3072 (v4) | OK | OK |
| Post-quantum `pqc` (Kyber v5 subkey, LibrePGP) | fails to read the key | same |

gpg 2.5 encrypts with the LibrePGP AEAD/OCB packet; go-crypto decrypts it. Private keys exported with `--export-secret-keys` (iterated S2K) open with the passphrase.

The LibrePGP (GnuPG) vs RFC 9580 (go-crypto) split only affects v5/post-quantum keys. Post-quantum keys are unsupported: `decrypt` with one of them must fail with a clear message.

Alternatives considered:
- **Real `gpg` with a temporary `GNUPGHOME`**: full interoperability, including post-quantum, but requires `gpg` installed and writes the key to disk (tmpfs on Linux; no guaranteed tmpfs on macOS and Windows).
- **`age`**: simpler, but does not interoperate with `gpg`.
- **`golang.org/x/crypto/openpgp`**: marked "should not be used" on pkg.go.dev.
