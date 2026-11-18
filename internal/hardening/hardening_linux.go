//go:build linux

package hardening

import (
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

// Apply hardens the process on Linux: non-dumpable, no core files, then
// mlockall only if RLIMIT_MEMLOCK covers kdfMem plus Margin. Without that
// check the Go runtime dies with a fatal error on the first allocation past
// the limit. Failures to lock are warnings on warn, never fatal.
func Apply(kdfMem uint64, warn io.Writer) {
	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		fmt.Fprintf(warn, "warning: PR_SET_DUMPABLE: %v\n", err)
	}
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{}); err != nil {
		fmt.Fprintf(warn, "warning: RLIMIT_CORE: %v\n", err)
	}
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &lim); err != nil {
		fmt.Fprintf(warn, "warning: RLIMIT_MEMLOCK: %v; continuing without memory lock\n", err)
		return
	}
	d := Plan(kdfMem, lim.Cur, lim.Max)
	if !d.Lock {
		fmt.Fprintln(warn, d.Warning)
		return
	}
	if d.RaiseTo != 0 {
		lim.Cur = d.RaiseTo
		if err := unix.Setrlimit(unix.RLIMIT_MEMLOCK, &lim); err != nil {
			fmt.Fprintf(warn, "warning: raising RLIMIT_MEMLOCK: %v; continuing without memory lock\n", err)
			return
		}
	}
	if err := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE); err != nil {
		fmt.Fprintf(warn, "warning: mlockall: %v; continuing without memory lock\n", err)
	}
}
