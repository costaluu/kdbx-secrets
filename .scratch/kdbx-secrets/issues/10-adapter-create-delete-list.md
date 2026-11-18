# 10: `adapter create|delete|list`

**What to build:** Manage the templates folder without opening the Database. `adapter create <name> [--from <file>] [--force]` reads from `--from` or stdin, validates the syntax of the template and of the References by running the collection pass without a prompt, creates `templates/` with `0700` if it does not exist and writes `<name>.tmpl` with `0600` through a temporary file and `rename`, even with umask `002`. It refuses to overwrite without `--force`. `adapter delete <name>` deletes, and a nonexistent name is an error. `adapter list` prints the names in alphabetical order. Hand-created templates remain valid.

**Blocked by:** 02, 09

**Status:** done

- [ ] `create` writes with `0600` and the folder with `0700` (tested with umask `002`, Unix only)
- [ ] An invalid template or a Reference with wrong syntax writes nothing
- [ ] Overwriting without `--force` fails; with `--force`, it replaces
- [ ] A failure midway leaves neither a partial file nor a stray temporary one
- [ ] `delete` of a nonexistent name is an error; `list` comes out sorted
- [ ] None of the three opens the Database or touches the terminal
