package hardening

import (
	"fmt"
	"io"
)

// LockBuffer pins b in RAM (mlock/VirtualLock) so the password and output
// buffers stay out of swap and the pagefile. A failure is a warning on warn,
// never an error. The returned func unlocks; call it after zeroing b.
// On Linux Apply may already hold mlockall; locking again is harmless.
func LockBuffer(b []byte, warn io.Writer) (unlock func()) {
	return lockBuffer(b, warn, lockMem, unlockMem)
}

func lockBuffer(b []byte, warn io.Writer, lock, unlock func([]byte) error) func() {
	if len(b) == 0 {
		return func() {}
	}
	if err := lock(b); err != nil {
		fmt.Fprintf(warn, "warning: locking buffer in memory: %v; continuing without lock\n", err)
		return func() {}
	}
	return func() { unlock(b) }
}
