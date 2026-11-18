---
status: accepted
---

# Adapters in two layers, registered by folder

An Adapter is what translates secrets from the `.kdbx` into the format a tool expects. Everything must work the same on Linux, macOS and Windows.

1. **Template** (`~/.kdbx-secrets/templates/<name>.tmpl`, `kdbx-secrets render <name>`): most adapters just fit fields into a format (JSON, a config file). It uses the stdlib `text/template`, executed by the binary, and is portable without code.
2. **Built into the binary**: what requires logic and is generic. Today: `run` (Vault KV API) and OpenPGP `encrypt`/`decrypt`.

Registry: a file in the templates folder is a registered Adapter, and the Adapter's name is the file name. There is no other registry. `kdbx-secrets adapter create|delete|list` manages that folder: it writes the template with `0600` permission and validates it first. Creating the file by hand remains valid; the command exists because, with umask `002`, a file created in an editor ends up group-writable and would be refused by the permission check.

Configuring the target tool (for example, the `credential_process` line in `~/.aws/config`) remains the user's job.

There is no long-lived Adapter: the only long process is `run`.

Alternatives considered:
- **Generated Python** (a `.py` skeleton plus an embedded helper module that calls the binary): was planned as a third layer and removed before it existed. Without long-lived adapters, almost everything it would do a template already does. It would cost Python as a prerequisite on macOS and Windows, a module to version and a per-OS launcher. It comes back if a real case appears that a template cannot solve.
- **Free-form scripts in any language**: flexible, but an `sh` script does not run on Windows and every user reinvents portability.
- **Plugin system loaded into the binary**: requires an ABI/SDK and loads third-party code into the process that holds the password.
- **Embedded Starlark**: portable and sandboxed, but an extra dependency without a case the template does not cover.
