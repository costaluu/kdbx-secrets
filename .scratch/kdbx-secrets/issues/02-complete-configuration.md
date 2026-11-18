# 02: Complete configuration

**What to build:** The configuration folder and the `config` behave as described in CONTEXT.md. `~` is expanded and a relative path is resolved from the configuration folder. Lines with `#` are comments. An unknown key is an error. A missing `config` or `database` line produces an error that says exactly what is missing. `KDBX_SECRETS_HOME` replaces the whole folder and must be absolute. On Unix, the folder and the `config` must belong to the user and must not be writable by group or others; otherwise it is an error, as in `ssh`. The same check applies to the templates (ticket 09) and must remain reusable. All these errors appear before the prompt.

**Blocked by:** 01

**Status:** done

- [ ] `~` and relative paths are resolved correctly
- [ ] Comments and empty lines are ignored; an unknown key is an error and cites the key
- [ ] Missing `config` and missing `database` produce distinct, precise messages
- [ ] A relative `KDBX_SECRETS_HOME` is a configuration error
- [ ] On Unix, a folder or `config` with the wrong owner or writable by group/others is refused (tests skipped on Windows)
- [ ] None of these errors touches the terminal (the prompt does not happen)
- [ ] The validation order is respected: arguments and syntax, config, folder, Database signature, prompt, content
