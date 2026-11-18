//go:build darwin

package hardening

import (
	"fmt"
	"io"

	"golang.org/x/sys/unix"
)

// Apply hardens the process on macOS: no core files and PT_DENY_ATTACH against
// a same-user debugger. macOS has no mlockall; own buffers use LockBuffer.
// kdfMem is unused here. Failures are warnings on warn.
func Apply(kdfMem uint64, warn io.Writer) {
	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{}); err != nil {
		fmt.Fprintf(warn, "warning: RLIMIT_CORE: %v\n", err)
	}
	if err := unix.PtraceDenyAttach(); err != nil {
		fmt.Fprintf(warn, "warning: PT_DENY_ATTACH: %v\n", err)
	}
}
