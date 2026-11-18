# Glossary

- **Database**: the `.kdbx` file. Canonical term; do not use "vault" for it.
- **Vault**: only the HashiCorp KV API that `run --vault` emulates. Never a synonym for Database.
- **Entry**: an item of the Database, with Standard fields, Custom attributes and Attachments.
- **Standard field**: `Title`, `UserName`, `Password`, `URL`, `Notes`.
- **Custom attribute**: an extra field of an Entry, with a name defined by the user in KeePassXC.
- **Attachment**: a binary file stored inside an Entry.
- **Root group**: the top group of the Database. Its name is not part of the Reference.
- **Recycle bin**: the group where KeePass moves deleted Entries. Its Entries are not resolved.
- **Reference**: the address of a value in the Database. The syntax is in ADR 0004.
- **Reserved name**: a field name that the Reference interprets instead of looking among the attributes, such as `TOTP` and the `attachment:` prefix. If a Custom attribute has the same name, it becomes inaccessible through the Reference, and the Reference errors.
- **TOTP seed**: the `otp` attribute of the Entry, an `otpauth://` URI with the secret. It differs from the **TOTP code** (`#TOTP`), which is computed on the spot from the seed.
- **Adapter**: what translates secrets from the Database into the format a tool expects. It has two layers (ADR 0003): template and built into the binary.
- **Prompt**: the request for the KeePass password. It always happens on the controlling terminal, even with stdin and stdout redirected, and shows who asked (the parent process chain).
- **Vault scope**: the set of Entries that a child of `run --vault` can read. Outside it, every Entry is invisible.
