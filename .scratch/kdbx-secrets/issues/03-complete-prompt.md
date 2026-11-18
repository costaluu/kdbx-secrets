# 03: Complete prompt

**What to build:** The Prompt behaves as in the spec (stories 8 to 18). It gives up to 3 attempts. It separates "incorrect password" (header HMAC) from "incomplete or corrupted file, try again". On Ctrl-C, it restores echo and exits with 1. On Ctrl-D (EOF), it cancels without counting as an attempt. Without a controlling terminal, it gives a clear error. It refuses a password above 1024 bytes without opening the Database, accepts an empty password and uses the bytes without Unicode normalization. It shows, informatively, the parent process chain (for example, `aws ← zsh`) and the requested References, sanitized. In production, the terminal is `/dev/tty` on Unix and `CONIN$`/`CONOUT$` on Windows, even with stdin and stdout redirected.

**Blocked by:** 01

**Status:** done

- [ ] 3 wrong attempts fail with 1; getting it right on the 2nd or 3rd works
- [ ] A truncated fixture produces the incomplete file message, different from the incorrect password one
- [ ] EOF at the prompt cancels without consuming an attempt
- [ ] An empty password opens an empty-password fixture; one of 1025 bytes is refused without paying the KDF
- [ ] The prompt text shows the process chain and the References
- [ ] Ctrl-C restores echo (manual verification per OS, recorded)
- [ ] Without a tty, the message is clear (tested through the real terminal opening failing)
- [ ] Pending item 2 (Windows console with captured stdin/stdout) is recorded as manual verification
