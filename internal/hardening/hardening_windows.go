//go:build windows

package hardening

import (
	"io"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Apply does nothing on Windows: there is no mlockall and no core file.
// Buffers are locked with LockBuffer. Excluding the process from WER dumps
// is open item 6 (manual check).
func Apply(kdfMem uint64, warn io.Writer) {}

func lockMem(b []byte) error {
	return windows.VirtualLock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}

func unlockMem(b []byte) error {
	return windows.VirtualUnlock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}
