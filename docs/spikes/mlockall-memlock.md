# Spike (pending item 5): mlockall(MCL_FUTURE) with a low RLIMIT_MEMLOCK

Program: `internal/hardening/spike/main.go` (Go 1.26, linux/amd64, regular user, `vm.overcommit_memory=0`).
It does `mlockall(MCL_CURRENT|MCL_FUTURE)` (with the `lock` argument), allocates and touches 64 MiB.

## Steps

In a fresh shell per limit: `ulimit -S -l N; ulimit -H -l N; ./spike lock`.

## Result

| RLIMIT_MEMLOCK | `mlockall` | 64 MiB allocation |
|---|---|---|
| 8, 20, 40, 64, 96, 128, 256 MiB, 1 GiB | `ENOMEM` | ok |
| unlimited | ok | ok |

- The runtime fatal error **did not reproduce**: without the lock the allocation
  succeeds, and with `mlockall` failing the process carries on normally.
- Reason: `MCL_CURRENT` locks the process's **virtual** size, and a Go binary
  reserves hundreds of MiB of address space (VmSize ~ 1.4 GiB in this test).
  For a regular user, `mlockall` fails in the call itself (`ENOMEM`) at any
  realistic limit; it only passes with an unlimited limit (root, or
  `LimitMEMLOCK=infinity`).
- The fatal error scenario (the call passes and a later allocation overflows
  the limit) requires a limit that covers the current virtual space but not
  the future one; it was not reached here.

## Consequence for the implementation

- The check before the lock (`hardening.Plan`) still holds and avoids the lock
  when the limit is clearly short (hard limit < KDF + margin).
- `Apply` treats `ENOMEM` from `mlockall` as a warning and goes on without the
  lock, so the practical result for anyone without an unlimited limit is the
  warning on stderr, with core dump disabled and the process non-dumpable.
- If `ENOMEM` becomes the common case, `Plan`'s margin can be replaced by a
  read of `VmSize` from `/proc/self/statm`; not implemented (YAGNI until there
  is demand).
