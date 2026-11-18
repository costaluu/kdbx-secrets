# 04: References: strict syntax and resolution

**What to build:** All the forms of ADR 0004, except Reserved names (ticket 05). The parser is strict and reports the error position: empty path, `#` without a field, `/` at the start or end, `//`, unknown escape, `\` at the end. It accepts the escapes `\/`, `\#` and `\\`. `uuid:` accepts exactly 32 hex, case-insensitively. Standard fields and Custom attributes are read by exact name. The syntax is checked before the prompt. On resolution, the Root group stays out of the path, the Recycle bin is an error (by path and by `uuid:`), the history is never consulted, a duplicate title is an error with the candidate UUIDs, `{REF:` is an error and an expired Entry works with a warning on stderr. Messages cite the Reference, never the value.

**Blocked by:** 01

**Status:** done

- [ ] Table of invalid syntaxes: each case fails with 2 and with the position, without touching the terminal
- [ ] Path, escapes, `uuid:`, Standard field and Custom attribute resolve in the fixtures
- [ ] Names are case-sensitive, and spaces are part of the name
- [ ] Duplicate title: error listing the UUIDs
- [ ] Entry in the Recycle bin: error by path and by `uuid:`
- [ ] Value with `{REF:`: error "KeePass reference not supported"
- [ ] Expired Entry: value on stdout and warning on stderr
