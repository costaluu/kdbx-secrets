# 05: References: Attachment, TOTP and TOTP seed

**What to build:** The Reserved names and the derived values. `#attachment:<name>` returns the Attachment's bytes (`attachment:` without a name is a syntax error). `#TOTP` returns the current TOTP code, computed from the seed with the injected clock. `#otp` returns the TOTP seed (the `otpauth://` URI). A Custom attribute named `TOTP` or `attachment:…` makes the Reference fail instead of choosing. The logic stays reusable by `run --vault` (ticket 12), which computes TOTP on every request.

**Blocked by:** 04

**Status:** done

- [ ] `#attachment:<name>` returns the exact bytes of the fixture's Attachment; a nonexistent Attachment is an error
- [ ] `#TOTP` with a fixed clock returns the expected code (checked against a known vector)
- [ ] `#TOTP` on an Entry without a seed is a clear error
- [ ] `#otp` returns the URI
- [ ] Collision with a Reserved name: error, tested with the fixture that has the `TOTP` attribute
