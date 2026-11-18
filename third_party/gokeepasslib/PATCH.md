Vendored copy of github.com/tobischo/gokeepasslib/v3 v3.7.1 (MIT, see LICENSE.md)
with tests/examples removed and ONE local patch: Argon2id support
(`KdfArgon2id` in header.go; dispatch to `argon2.IDKey` in credentials.go).
Upstream only dispatches Argon2d. Drop this directory and the `replace` in
go.mod once upstream supports Argon2id.
