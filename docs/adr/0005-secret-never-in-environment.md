---
status: accepted
---

# Secrets never go into an environment variable

`run` delivers secrets to the child only through the Vault KV API (`--vault`). The child's environment contains only `VAULT_ADDR` and `VAULT_TOKEN`. There is no `-e NAME=<ref>` or `--env-file`.

A process's environment is readable by any process of the same user, on every OS (`/proc/<pid>/environ`, `ps -E`, PEB on Windows), and is inherited by every grandchild. Putting the secret there repeats the problem that motivated the project. The token, on the other hand, is valid only for the entries in the scope and dies with `run`.

Limit: while `run` lives, the token is equivalent to the scope's secrets for any process of the same user, which can read the child's environment and call the server. The gain from `--vault` is against passive leakage (env in logs, crash reports, a stored dump), which stops applying when `run` ends, and against access outside the scope, not against active same-user malware.

Cost: apps that only read environment variables and do not speak the Vault API are unsupported by `run`. For tools with a credential helper (for example, AWS `credential_process`), the path is `render`.

Alternatives considered:
- **`-e`/`--env-file`, like `op run`**: universal, but the secret stays in the environment of the child and of every grandchild.
- **`-e` with `exec` on Unix**: takes the Database out of kdbx-secrets' memory, but does not take the secret out of the app's environment.
