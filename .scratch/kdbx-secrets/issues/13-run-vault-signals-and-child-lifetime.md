# 13: `run --vault`: signals and child lifetime

**What to build:** `run` supervises the child as in the spec. It ignores SIGINT, so Ctrl-C reaches only the child through the terminal's group. It forwards SIGTERM and SIGHUP to the child and keeps the server answering while the child finishes. It ties the child to the parent: `PR_SET_PDEATHSIG(SIGTERM)` on Linux and a Job Object with `KILL_ON_JOB_CLOSE` on Windows. On macOS, the child survives and the calls to the Vault fail, and this is documented.

**Blocked by:** 12

**Status:** done

- [ ] Test: SIGTERM sent to `run` reaches the child (helper process), which can still read from the server before exiting
- [ ] Test: SIGINT on `run` does not kill it before the child
- [ ] Linux: the child receives SIGTERM when `run` dies (subprocess test with `kill -9`, if stable; otherwise, recorded manual verification)
- [ ] Windows: Job Object configured; the `kill -9` equivalent stays as a recorded manual verification
