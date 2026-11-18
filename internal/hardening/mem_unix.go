//go:build linux || darwin

package hardening

import "golang.org/x/sys/unix"

func lockMem(b []byte) error   { return unix.Mlock(b) }
func unlockMem(b []byte) error { return unix.Munlock(b) }
