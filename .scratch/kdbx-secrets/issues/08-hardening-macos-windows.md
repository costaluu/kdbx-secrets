# 08: Memory hardening on macOS and Windows

**What to build:** The protection each platform allows. On macOS: `mlock` on the binary's own buffers (password and output), `RLIMIT_CORE=0` and `ptrace(PT_DENY_ATTACH)`. On Windows: `VirtualLock` on the binary's own buffers. The known limits (no `mlockall`, cleartext pagefile without BitLocker, hibernation) are documented. Pending item 6, excluding the process from WER dumps, is recorded as manual verification.

**Blocked by:** 07

**Status:** done

- [ ] macOS: hardening applies `RLIMIT_CORE=0` (subprocess test) and `PT_DENY_ATTACH`
- [ ] The password and output buffers go through `mlock`/`VirtualLock`; a lock failure becomes a warning, not an error
- [ ] CI is green on the three OSes
- [ ] Pending item 6 is recorded, with what was investigated
