# 07: Memory hardening on Linux

**What to build:** Hardening runs before any secret enters memory. The sequence: `PR_SET_DUMPABLE=0` and `RLIMIT_CORE=0`; raise the soft limit of `RLIMIT_MEMLOCK` up to the hard limit; compute the KDF memory from the header; call `mlockall(MCL_CURRENT|MCL_FUTURE)` only if the limit covers the KDF plus a margin. If it does not, warn on stderr and go on, with core dump staying disabled. The "lock or warn" decision is a pure function (Seam 2), separate from the syscalls. It includes the spike of pending item 5: confirm the runtime fatal error with a low `RLIMIT_MEMLOCK` and that the check before the lock avoids it.

**Blocked by:** 01

**Status:** done

- [ ] Table tests of the pure function: KDF memory x soft x hard -> "raise to X and lock" or "warn and go on"
- [ ] Subprocess test, Linux only: after hardening, `PR_GET_DUMPABLE == 0` and `RLIMIT_CORE == 0`
- [ ] The "going on without lock" warning shows up on stderr when the limit does not cover
- [ ] The findings of pending item 5 are recorded (steps and result)
- [ ] The other OSes compile (stubs until ticket 08)
