---
status: accepted
---

# Reference syntax

A Reference addresses a value in the `.kdbx`.

| Form | Example | Rule |
|---|---|---|
| Path + field | `AWS/paper-freeze#Password` | path from the root, without the Root group name |
| Standard field | `AWS/paper-freeze` | without `#`, returns the Password |
| Custom attribute | `AWS/paper-freeze#region` | attribute name as it is in KeePassXC |
| Attachment | `GPG/personal#attachment:key.asc` | returns the attachment's bytes |
| TOTP | `AWS/paper-freeze#TOTP` | code computed at the moment |
| Escape | `Web/a\/b#Password` | `\/`, `\#` and `\\` inside names |
| By UUID | `uuid:<hex>#Password` | immune to renaming and moving |

Names are case-sensitive. Standard fields: `Title`, `UserName`, `Password`, `URL`, `Notes`.

**Reserved names win.** `TOTP` and the `attachment:` prefix are interpreted by the binary. If an entry has a custom attribute with one of these names, the Reference fails instead of choosing. `#otp` is not reserved: it returns the TOTP seed (the `otpauth://` URI) that KeePassXC stores in that attribute.

**Strict parser.** These are syntax errors, with the position of the problem in the message: empty path, `#` without a field, `/` at the start or end, `//`, unknown escape (`\x`), `\` at the end and `attachment:` without a name. `uuid:` accepts exactly 32 hex characters, without hyphens and case-insensitively. There is no normalization: spaces are part of the name. The syntax is checked before the prompt.

**Recycle bin and history are left out.** An entry's version history is never consulted. An entry in the Recycle bin is an error ("entry is in the recycle bin"), whether by path or by `uuid:`: a deleted password must not keep working without anyone noticing.

**KeePass `{REF:...}` is not resolved.** If the requested value contains `{REF:`, the Reference fails with "KeePass reference not supported". Returning the literal would deliver the wrong value without warning.

**An expired entry works.** The value comes out normally, with a warning on stderr. Failing would break `credential_process` because of metadata.

**Ambiguity is an error.** KDBX allows repeated titles in the same group. If a path matches more than one entry, the command fails and lists the candidate UUIDs, so the user can switch to the `uuid:` form. It never picks the first: with secrets, silently choosing wrong is worse than failing.
