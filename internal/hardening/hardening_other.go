//go:build !linux && !darwin && !windows

package hardening

import "io"

// Apply and the buffer lock are no-ops on platforms we do not ship.
func Apply(kdfMem uint64, warn io.Writer) {}

func lockMem(b []byte) error   { return nil }
func unlockMem(b []byte) error { return nil }
