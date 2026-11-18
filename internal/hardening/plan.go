// Package hardening applies process memory hardening before any secret enters
// memory. Plan is the pure lock-or-warn decision (Seam 2); Apply does the
// platform syscalls.
package hardening

import "fmt"

// Margin is the headroom over the KDF memory that mlockall(MCL_FUTURE) needs:
// Go heap, stacks and runtime allocations also get locked.
const Margin = 16 << 20

// Decision is the outcome of Plan.
type Decision struct {
	Lock    bool   // call mlockall
	RaiseTo uint64 // new soft RLIMIT_MEMLOCK before locking; 0 = leave as is
	Warning string // set when Lock is false: printed to stderr, then carry on
}

// Plan decides whether mlockall is safe. kdfMem is the KDF memory from the
// header; soft and hard are RLIMIT_MEMLOCK (^uint64(0) means unlimited).
func Plan(kdfMem, soft, hard uint64) Decision {
	need := kdfMem + Margin
	if need < kdfMem { // overflow
		need = ^uint64(0)
	}
	switch {
	case soft >= need:
		return Decision{Lock: true}
	case hard >= need:
		return Decision{Lock: true, RaiseTo: need}
	}
	return Decision{Warning: fmt.Sprintf(
		"warning: RLIMIT_MEMLOCK hard limit (%d bytes) is below the %d bytes needed; continuing without memory lock (core dumps stay disabled)",
		hard, need)}
}
