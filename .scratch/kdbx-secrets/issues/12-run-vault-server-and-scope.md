# 12: `run --vault`: server and Vault scope

**What to build:** `run --vault <mount>/<path>=<entry-ref> [--vault ...] -- <command>` executes the command with access only to the mapped Entries. The `--vault` parse is done at the first `=`. These are usage errors, before the prompt: repeated KV path, reserved mount (`sys`, `auth`, `identity`, `cubbyhole`), empty segment, `.` or `..` and a Reference with `#field`. After the prompt, the binary resolves the Vault scope and drops the rest of the Database. It starts a read-only KV v2 server on `127.0.0.1`, on a random port, with the format defined in ticket 11. The token has 256 bits, exists only in memory, is accepted in `X-Vault-Token` or `Authorization: Bearer` and is compared in constant time. The responses: 405 for a method other than `GET`, 403 without a token or with a wrong token, 404 outside the Vault scope. A path with `..` or `%2F` is refused, the query is ignored, the body is not read, and `ReadHeaderTimeout`/`ReadTimeout` are configured. `data.data` carries the Standard fields, the Custom attributes and the `TOTP` computed on every request, without the seed and without the Attachments. The child inherits the environment plus `VAULT_ADDR` and `VAULT_TOKEN`, and no secret. `run` exits with the child's code.

**Blocked by:** 05, 11

**Status:** done

- [ ] Test with a helper process (the test binary re-executed as a client): 200 inside the Vault scope, 404 outside, 403 without a token, 405 on writes, refusal of `..` and `%2F`
- [ ] The response carries the TOTP and carries neither the seed nor the Attachments
- [ ] The child's environment contains no value from the Database
- [ ] The child's exit code is passed through
- [ ] Mapping errors exit with 2, before the prompt
- [ ] Optional test: with `vault` or `hvac` available, the real client reads the secret; without them, the test is skipped
